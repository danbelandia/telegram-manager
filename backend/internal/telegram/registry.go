package telegram

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/telegram-manager/backend/internal/tenants"
)

// Estados del runtime por tenant (espejan tenants.Status*).
const (
	tenantActive   = tenants.StatusActive
	tenantDegraded = tenants.StatusDegraded
)

// Publisher es la vista minima del bus de eventos que el registry
// necesita: *events.Bus la satisface sin importar el paquete events
// (evita un ciclo telegram↔events). Cada tenant tiene SU propio bus
// (D5): cero cambios en Bus/Update/handlers y un handler lento no
// contamina otros tenants. Exportada porque main la usa en busFor.
type Publisher interface {
	Publish(*Update)
}

// DecryptFunc descifra el token cifrado del tenant a texto plano. La
// provee main con el Crypter (clave TENANT_TOKEN_ENC_KEY); el token
// en claro solo vive en memoria dentro del Adapter (D6).
type DecryptFunc func(blob []byte) (string, error)

// StatusFunc observa cambios de estado del tenant (active/degraded).
// Main la conecta a tenants.Repository.SetStatus; nil = sin persistir.
type StatusFunc func(ctx context.Context, tenantID int64, status string)

// tenantRuntime es el runtime vivo de un tenant: su adapter (con rate
// limiter propio ~25 req/s heredado del Adapter), su bus y la
// cancelacion de su poller.
type tenantRuntime struct {
	adapter *Adapter
	bus     Publisher
	cancel  context.CancelFunc
	status  string
}

// Registry posee un adapter+poller+bus por tenant (bot-per-tenant).
// El modo sigue siendo global (TELEGRAM_MODE, Q2): en polling hay N
// pollers; en webhook el multiplexado queda fuera del slice y el
// registry no se usa.
type Registry struct {
	mu       sync.Mutex
	tenants  map[int64]*tenantRuntime
	logger   *slog.Logger
	onStatus StatusFunc
	// newAdapter crea adapters (seam de test: apunta a httptest).
	newAdapter func(token string) *Adapter
}

// NewRegistry crea un registry vacio.
func NewRegistry(logger *slog.Logger, onStatus StatusFunc) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		tenants:    make(map[int64]*tenantRuntime),
		logger:     logger,
		onStatus:   onStatus,
		newAdapter: func(token string) *Adapter { return NewAdapter(token) },
	}
}

// BootAll levanta un poller por cada tenant con token registrado. El
// fallo de un tenant NO bloquea a los demas y nunca loguea tokens:
// solo tenant_id y slug.
//
// Por tenant: descifrar → NewAdapter → GetMe de validacion. Token
// rechazado (ErrInvalidToken) → estado degraded + loop con backoff
// exponencial (1s→max 5 min) que reintenta GetMe hasta que el token
// vuelva a ser valido; resto de errores → Poller normal (su backoff
// transitorio ya existe).
func (r *Registry) BootAll(ctx context.Context, list []tenants.Tenant, decrypt DecryptFunc, busFor func(tenantID int64) Publisher) {
	for i := range list {
		t := list[i]
		if len(t.BotTokenEncrypted) == 0 {
			continue // tenant legacy sin token propio: lo cubre el adapter legacy
		}
		plain, err := decrypt(t.BotTokenEncrypted)
		if err != nil {
			r.logger.Error("registry: boot: no se pudo descifrar el token",
				"tenant_id", t.ID, "slug", t.Slug, "error", err)
			continue
		}
		r.startLocked(ctx, t.ID, t.Slug, plain, busFor(t.ID))
	}
}

// RegisterHot levanta (o reemplaza) el poller de un tenant en caliente,
// sin tocar los demas (signup 201 → activo inmediato). Detiene el
// runtime previo bajo mu y valida/arranca fuera de ella (GetMe es red:
// nunca bloquear el mapa durante una llamada de red). Dos RegisterHot
// concurrentes del MISMO tenant son imposibles en la practica (el slug
// UNIQUE impide doble signup); ante esa carrera gana la ultima
// escritura del mapa.
func (r *Registry) RegisterHot(ctx context.Context, tenantID int64, slug, tokenPlain string, bus Publisher) {
	r.mu.Lock()
	r.stopLocked(tenantID)
	r.mu.Unlock()
	r.startLocked(ctx, tenantID, slug, tokenPlain, bus)
}

// AdapterFor devuelve el adapter del tenant para el servicio de
// moderacion por tenant (o el mapa de services que construye main).
func (r *Registry) AdapterFor(tenantID int64) (*Adapter, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.tenants[tenantID]
	if !ok {
		return nil, false
	}
	return rt.adapter, true
}

// Status devuelve el estado del runtime (active/degraded).
func (r *Registry) Status(tenantID int64) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.tenants[tenantID]
	if !ok {
		return "", false
	}
	return rt.status, true
}

// StopAll cancela cada ctx de tenant; main lo llama en el shutdown
// graceful. No borra el mapa: Status sigue consultable.
func (r *Registry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.tenants {
		r.stopLocked(id)
	}
}

// startLocked valida con GetMe y arranca el loop que corresponda.
// Escribe el mapa bajo mu interna; los loops hijos usan el cancel del
// runtime (RegisterHot/StopAll lo invocan para reemplazar o apagar).
func (r *Registry) startLocked(ctx context.Context, tenantID int64, slug, tokenPlain string, bus Publisher) {
	adapter := r.newAdapter(tokenPlain)
	if bus == nil {
		r.logger.Error("registry: sin bus para el tenant, poller no levantado",
			"tenant_id", tenantID, "slug", slug)
		return
	}

	childCtx, cancel := context.WithCancel(ctx)
	if _, err := adapter.GetMe(childCtx); err != nil {
		if errors.Is(err, ErrInvalidToken) {
			// Token revocado: degraded + backoff, sin tumbar a nadie.
			r.setRuntime(tenantID, &tenantRuntime{adapter: adapter, bus: bus, cancel: cancel, status: tenantDegraded})
			r.reportStatus(ctx, tenantID, tenantDegraded)
			r.logger.Warn("registry: token rechazado por Telegram; backoff degradado",
				"tenant_id", tenantID, "slug", slug)
			go r.degradedLoop(childCtx, tenantID, slug, adapter, bus)
			return
		}
		// Error transitorio (red): Poller normal con su backoff.
		r.logger.Warn("registry: validacion inicial fallo (transitorio); polling igual",
			"tenant_id", tenantID, "slug", slug, "error", err)
	}
	r.setRuntime(tenantID, &tenantRuntime{adapter: adapter, bus: bus, cancel: cancel, status: tenantActive})
	r.reportStatus(ctx, tenantID, tenantActive)
	r.logger.Info("registry: poller activo", "tenant_id", tenantID, "slug", slug)
	go r.pollLoop(childCtx, tenantID, slug, adapter, bus)
}

// degradedLoop reintenta GetMe con backoff exponencial 1s→max 5 min.
// Cuando el token vuelve a ser valido, transiciona al poller normal y
// marca active. El loop muere con el ctx del tenant.
func (r *Registry) degradedLoop(ctx context.Context, tenantID int64, slug string, adapter *Adapter, bus Publisher) {
	backoff := time.Second
	const maxBackoff = 5 * time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if _, err := adapter.GetMe(ctx); err == nil {
			r.mu.Lock()
			if rt, ok := r.tenants[tenantID]; ok && rt.adapter == adapter {
				rt.status = tenantActive
			}
			r.mu.Unlock()
			r.reportStatus(ctx, tenantID, tenantActive)
			r.logger.Info("registry: token valido de nuevo; poller activo",
				"tenant_id", tenantID, "slug", slug)
			go r.pollLoop(ctx, tenantID, slug, adapter, bus)
			return
		} else if !errors.Is(err, ErrInvalidToken) {
			r.logger.Warn("registry: degraded: error transitorio en revalidacion",
				"tenant_id", tenantID, "slug", slug, "error", err)
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// pollLoop corre el Poller normal y publica cada lote en el bus propio
// del tenant. Un error fatal (token revocado en caliente, webhook en
// conflicto) degrada el runtime en vez de tumbar el backend: marca
// degraded y pasa al degradedLoop con el mismo ctx del tenant.
func (r *Registry) pollLoop(ctx context.Context, tenantID int64, slug string, adapter *Adapter, bus Publisher) {
	poller := NewPoller(adapter, WithPollerLogger(r.logger))
	err := poller.Run(ctx, func(updates []Update) {
		for i := range updates {
			bus.Publish(&updates[i])
		}
	})
	if ctx.Err() != nil {
		return // shutdown: salida limpia
	}
	// Fatal del poller: solo ErrInvalidToken/ErrWebhookConflict llegan
	// aca (rate limits y transitorios se absorben adentro).
	r.logger.Warn("registry: poller detenido con error fatal; pasando a degraded",
		"tenant_id", tenantID, "slug", slug, "error", err)
	r.mu.Lock()
	if rt, ok := r.tenants[tenantID]; ok && rt.adapter == adapter {
		rt.status = tenantDegraded
	}
	r.mu.Unlock()
	r.reportStatus(ctx, tenantID, tenantDegraded)
	go r.degradedLoop(ctx, tenantID, slug, adapter, bus)
}

func (r *Registry) setRuntime(tenantID int64, rt *tenantRuntime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[tenantID] = rt
}

func (r *Registry) stopLocked(tenantID int64) {
	if rt, ok := r.tenants[tenantID]; ok {
		rt.cancel()
		delete(r.tenants, tenantID)
	}
}

func (r *Registry) reportStatus(ctx context.Context, tenantID int64, status string) {
	if r.onStatus == nil {
		return
	}
	// Sin timeout propio: el ctx del boot/shutdown manda. Un fallo de
	// persistencia no tumba el poller: solo se loguea.
	if err := func() error {
		defer func() {
			// onStatus no debe paniquear el registry.
			_ = recover()
		}()
		r.onStatus(ctx, tenantID, status)
		return nil
	}(); err != nil {
		r.logger.Warn("registry: onStatus fallo", "tenant_id", tenantID, "status", status, "error", err)
	}
}

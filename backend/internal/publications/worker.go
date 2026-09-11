// Worker in-process del modulo publications (slice 3 — AGENTS.md §22).
// Reclama filas con status='scheduled' cuya scheduled_at <= now()
// (canonico SELECT ... FOR UPDATE SKIP LOCKED dentro de la
// transaccion de Repository.ClaimScheduledDue), las transiciona a
// status='sending' y luego invoca publishOne (helper privado del
// servicio, compartido con slice 2) por cada fila. Asi el worker
// reutiliza la logica de permissionOk (bugfix #172), dispatch
// SendPhoto/SendMessage, UpdateStatus y log PUBLISH_MESSAGE sin
// duplicar el path de envio.
//
// El Scheduler NO toca directamente la API de Telegram ni el
// repositorio de publicaciones por debajo de la interfaz PubStore: el
// resto del backend queda libre de detalles del job queue
// (rate-limiter, retries) y eso respeta AGENTS §18.1 (rate limit
// dentro del adapter) y §15 (TelegramService como interfaz).
package publications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
)

// claimBatchSize limita las filas que cada tick reclama. 25 * 1s por
// llamada (rate limit aproximado de la Bot API para chats unicos,
// §18.1) cabe holgadamente en el intervalo por defecto (30s): si se
// atrasa, el siguiente tick recoge el resto.
const claimBatchSize = 25

// DefaultSchedulerInterval es el intervalo por defecto entre ticks en
// produccion. Los tests inyectan intervalos mucho menores (5ms).
const DefaultSchedulerInterval = 30 * time.Second

// Scheduler lanza la goroutine que reclama publicaciones programadas
// y las entrega al helper publishOne. Se compone con
// signal.NotifyContext en cmd/server/main.go (mismo lifecycle que
// telegram.Poller).
//
// Dependencias via interfaces (lado consumidor, AGENTS-backend-skill):
//   - PubStore: extienda List/ListByTelegramID/ClaimScheduledDue/Cancel.
//   - GroupReader: ya estaba para publishOne.
//   - MessageSender: SendMessage + SendPhoto del adapter.
//   - LogWriter: Create del repo de logs.
type Scheduler struct {
	store    PubStore
	groups   GroupReader
	tg       MessageSender
	log      LogWriter
	interval time.Duration
	logger   *slog.Logger
	// tenantID aisla el scheduler (slice 0): un scheduler por tenant,
	// cada uno con el adapter de su bot. El claim filtra por tenant.
	tenantID int64
}

// NewScheduler construye el scheduler con sus dependencias e intervalo.
// Si logger es nil se usa slog.Default() (json handler del main).
func NewScheduler(store PubStore, groups GroupReader, tg MessageSender, logs LogWriter, interval time.Duration, logger *slog.Logger, tenantID int64) *Scheduler {
	if interval <= 0 {
		interval = DefaultSchedulerInterval
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:    store,
		groups:   groups,
		tg:       tg,
		log:      logs,
		interval: interval,
		logger:   logger,
		tenantID: tenantID,
	}
}

// Run ejecuta el loop hasta que ctx.Done() se dispara. Retorna nil
// siempre (cancelacion limpia); errores internos se loguean y el
// siguiente tick reintenta.
func (s *Scheduler) Run(ctx context.Context) error {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	s.logger.Info("publications scheduler started", "interval", s.interval.String())

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("publications scheduler stopping")
			return nil
		case <-t.C:
			s.tick(ctx)
		}
	}
}

// tick ejecuta un ciclo: claim + dispatch por cada fila. Es el metodo
// que los tests invocan directamente para evitar esperar al ticker.
func (s *Scheduler) tick(ctx context.Context) {
	rows, err := s.store.ClaimScheduledDue(ctx, s.tenantID, claimBatchSize)
	if err != nil {
		// Errores no-recuperables del SQL: log + siguiente tick reintenta.
		// NO abortamos Run: el bug seria permanente, no transitorio.
		s.logger.Error("publications worker claim failed", "error", err)
		return
	}
	for i := range rows {
		s.processClaimed(ctx, &rows[i])
	}
	s.logger.Info("publications worker tick",
		"due", len(rows),
		"interval", s.interval.String(),
	)
}

// processClaimed procesa una fila ya marcada como `sending` por el
// ClaimScheduledDue (transicion scheduled -> sending). Reusa
// publishOneFinalize (helper del Service que comparte el path de
// dispatch + UpdateStatus + log con publishOne) para no duplicar
// codigo y respetar el bugfix #172 (permissionOk via
// g.BotStatus == StatusAdministrator, ya validado por publishOne en
// el POST original; aca re-validamos contra el estado actual del
// grupo para tolerar cambios concurrentes como admin removido).
func (s *Scheduler) processClaimed(ctx context.Context, row *Publication) {
	var actorID int64
	if row.ActorID != nil {
		actorID = *row.ActorID
	}
	entry := &logs.Entry{
		TenantID: s.tenantID,
		ActorID:  &actorID,
		GroupID:  row.TelegramID,
		Action:   logs.ActionPublishMessage,
		Metadata: map[string]any{"publication_id": row.ID},
	}

	// Re-check del estado del grupo: si el bot dejo de ser admin
	// entre el POST y el tick, la fila debe quedar failed (no retry).
	group, err := s.groups.GetByTenant(ctx, s.tenantID, row.TelegramID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			msg := ErrGroupNotFound.Error()
			_ = s.store.UpdateStatus(ctx, s.tenantID, row.ID, StatusFailed, nil, &msg)
			row.Status = StatusFailed
			row.ErrorMessage = &msg
			entry.Status = logs.StatusNotFound
			entry.ErrorMessage = &msg
			_ = s.log.Create(ctx, entry)
			return
		}
		msg := fmt.Sprintf("error leyendo grupo: %v", err)
		_ = s.store.UpdateStatus(ctx, s.tenantID, row.ID, StatusFailed, nil, &msg)
		row.Status = StatusFailed
		row.ErrorMessage = &msg
		entry.Status = logs.StatusInternalError
		entry.ErrorMessage = &msg
		_ = s.log.Create(ctx, entry)
		return
	}
	if !s.permissionOk(group) {
		msg := ErrBotPermission.Error()
		_ = s.store.UpdateStatus(ctx, s.tenantID, row.ID, StatusFailed, nil, &msg)
		row.Status = StatusFailed
		row.ErrorMessage = &msg
		entry.Status = logs.StatusPermissionDenied
		entry.ErrorMessage = &msg
		_ = s.log.Create(ctx, entry)
		return
	}

	buttons, err := UnmarshalButtons(row.Buttons)
	if err != nil {
		msg := fmt.Sprintf("buttons invalidos: %v", err)
		_ = s.store.UpdateStatus(ctx, s.tenantID, row.ID, StatusFailed, nil, &msg)
		row.Status = StatusFailed
		row.ErrorMessage = &msg
		entry.Status = logs.StatusInternalError
		entry.ErrorMessage = &msg
		_ = s.log.Create(ctx, entry)
		return
	}

	hasPhoto := row.PhotoURL != nil && *row.PhotoURL != ""
	hasVideo := row.VideoURL != nil && *row.VideoURL != ""
	// El Service expone publishOneFinalize (no se llama desde afuera
	// del paquete). Reusamos esa funcion via un Service efimero para
	// no romper la interfaz publica del Service.
	svc := &Service{groups: s.groups, tg: s.tg, logs: s.log, store: s.store}
	svc.publishOneFinalize(ctx, row, entry, hasPhoto, row.PhotoURL, hasVideo, row.VideoURL, buttons)
}

// permissionOk expone el mismo check del Service (bugfix #172) para
// que el worker NO dependa de la API publica del paquete. Si esa
// invariante cambia, falla el test de servicio que verifica el bugfix.
func (s *Scheduler) permissionOk(g *groups.Group) bool {
	return g != nil && g.BotStatus == groups.StatusAdministrator
}

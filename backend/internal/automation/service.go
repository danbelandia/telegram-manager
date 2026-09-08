package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// SettingsRepo es la vista minima del repositorio que el Service
// necesita para settings. *Repository lo satisface; los tests usan un
// fake in-memory.
type SettingsRepo interface {
	GetSettings(ctx context.Context, groupID int64) (*Settings, error)
	UpsertSettings(ctx context.Context, s *Settings) error
}

// WarnRepo es la vista minima del repositorio que el Service necesita
// para warning_state.
type WarnRepo interface {
	GetWarningState(ctx context.Context, groupID, userID int64) (*WarningState, error)
	UpsertWarningState(ctx context.Context, ws *WarningState) error
	CreateWarningStateIfMissing(ctx context.Context, groupID, userID int64) error
}

// ListsRepo es la vista minima del repositorio para las listas
// (banned_words y link_allowlist). El Service la usa para pre-cargar
// una vez por mensaje antes de invocar al registry (evita N queries
// por rule cuando hay varias reglas DB-dependent), y los handlers la
// usan para CRUD via Service.{Add,Remove}{BannedWord,LinkAllowlist}.
type ListsRepo interface {
	ListBannedWords(ctx context.Context, groupID int64) ([]string, error)
	AddBannedWord(ctx context.Context, groupID int64, word string) error
	RemoveBannedWord(ctx context.Context, groupID int64, word string) error
	ListLinkAllowlist(ctx context.Context, groupID int64) ([]string, error)
	AddLinkAllowlist(ctx context.Context, groupID int64, domain string) error
	RemoveLinkAllowlist(ctx context.Context, groupID int64, domain string) error
}

// Service orquesta el pipeline de moderacion automatica. HandleMessage
// es el unico punto de entrada invocado desde el Subscriber
// (events.Bus). Implementa el pipeline de 8 pasos del spec REQ-7.
//
// Invariante (bugfix #172): permissionOkAdmin = g.BotStatus ==
// groups.StatusAdministrator. NUNCA leer claves can_* — la deteccion
// de grupos no las puebla completas; la Bot API no las exige para
// sendMessage/restrictChatMember/banChatMember en grupos siendo admin.
type Service struct {
	settingsRepo SettingsRepo
	warnRepo     WarnRepo
	listsRepo    ListsRepo
	registry     *Registry
	logs         LogWriter
	groups       GroupReader
	autoActionCh chan<- AutoAction
	logger       *slog.Logger
	now          func() time.Time
}

// NewService construye el Service. autoActionCh es el buffer que el
// Worker drena; el Service hace non-blocking send (select con default)
// para no bloquear el bus.
//
// listsRepo puede ser nil solo si NINGUNA regla registrada depende
// de listas (registro puro de FloodRule); en produccion el main.go
// siempre lo inyecta.
func NewService(
	settingsRepo SettingsRepo,
	warnRepo WarnRepo,
	listsRepo ListsRepo,
	registry *Registry,
	logs LogWriter,
	groups GroupReader,
	autoActionCh chan<- AutoAction,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		settingsRepo: settingsRepo,
		warnRepo:     warnRepo,
		listsRepo:    listsRepo,
		registry:     registry,
		logs:         logs,
		groups:       groups,
		autoActionCh: autoActionCh,
		logger:       logger,
		now:          time.Now,
	}
}

// HandleMessage ejecuta el pipeline de moderacion automatica para un
// mensaje entrante. Pasos (spec REQ-7):
//
//  1. Si msg.From == nil o msg.From.ID == 0 → return sin acción.
//  2. Cargar settings (auto-create si falta).
//  3. Si !settings.Enabled → return (skip silencioso).
//  4. permissionOkAdmin(g); si falso → return (sin log).
//  5. Cargar warning_state (auto-create si falta).
//  6. hit := Registry.Evaluate(msg, settings, ws).
//  7. Si hit → ws.WarningCount++, actualizar last_warning_at, registrar
//     log ActionRuleTriggered.
//  8. Si ws.WarningCount >= autoban → enqueue AutoAction{ban}; sino
//     si >= automute → enqueue AutoAction{mute, minutes: settings.AutomuteMinutes}.
//
// Retorna error SOLO en fallo interno de DB; el pipeline es best-effort
// por naturaleza (un log que falla no debe cortar la entrega del
// update al bus).
func (s *Service) HandleMessage(ctx context.Context, msg *telegram.Message) error {
	if msg == nil || msg.From == nil || msg.From.ID == 0 {
		return nil
	}

	// Paso 2: cargar settings (auto-create con defaults si falta).
	settings, err := s.LoadOrCreateSettings(ctx, msg.Chat.ID)
	if err != nil {
		return fmt.Errorf("automation: handle: load settings: %w", err)
	}

	// Paso 3: skip silencioso si la moderacion automatica esta off.
	if !settings.Enabled {
		return nil
	}

	// Paso 4: permissionOkAdmin. Si falla, skip silencioso (sin log:
	// el bot no es admin y el modulo no aplica).
	group, err := s.groups.GetByTelegramID(ctx, msg.Chat.ID)
	if err != nil {
		// Grupo no existe (raro: el bus publica desde un grupo que la
		// deteccion no vio). Skip silencioso — no tiene sentido
		// auditar PERMISSION_DENIED para un grupo que no esta en la DB.
		return nil
	}
	if !permissionOkAdmin(group) {
		return nil
	}

	// Paso 5: cargar warning_state (auto-create si falta).
	ws, err := s.LoadOrCreateWarningState(ctx, msg.Chat.ID, msg.From.ID)
	if err != nil {
		return fmt.Errorf("automation: handle: load warning state: %w", err)
	}

	// Paso 6: evaluar reglas. Si el toggle de BannedWords o AntiLink
	// esta activo, pre-cargamos las listas correspondientes UNA vez
	// por mensaje y las pasamos al registry (spec REQ-13). Si ambos
	// toggles estan off, omitimos las 2 queries (optimizacion:
	// ahorra I/O en grupos que solo usan Flood).
	lists := s.preloadLists(ctx, settings)
	hit := s.registry.Evaluate(ctx, msg, settings, ws, lists, s.now())
	if hit == nil {
		return nil
	}

	// Paso 7: incrementar counter y registrar log RULE_TRIGGERED.
	ws.WarningCount++
	now := s.now()
	ws.LastWarningAt = &now
	if settings.WarningExpireDays > 0 {
		expires := now.Add(time.Duration(settings.WarningExpireDays) * 24 * time.Hour)
		ws.ExpiresAt = &expires
	}
	if err := s.warnRepo.UpsertWarningState(ctx, ws); err != nil {
		return fmt.Errorf("automation: handle: upsert warning state: %w", err)
	}

	targetUserID := msg.From.ID
	ruleEntry := &logs.Entry{
		ActorID:      nil, // sistema, NO admin
		GroupID:      msg.Chat.ID,
		Action:       logs.ActionRuleTriggered,
		TargetUserID: &targetUserID,
		Status:       logs.StatusSuccess,
		Metadata: map[string]any{
			"rule_name":     hit.RuleName,
			"reason":        hit.Reason,
			"warning_count": ws.WarningCount,
		},
	}
	if err := s.logs.Create(ctx, ruleEntry); err != nil {
		s.logger.Error("automation: failed to write RULE_TRIGGERED log",
			"rule", hit.RuleName,
			"group_id", msg.Chat.ID,
			"user_id", msg.From.ID,
			"error", err)
		// No abortamos: el counter ya esta persistido.
	}

	// Paso 8: encolar auto-action si corresponde.
	if ws.WarningCount >= settings.AutobanWarnings {
		s.enqueueAction(AutoAction{
			Kind:         AutoActionBan,
			GroupID:      msg.Chat.ID,
			UserID:       msg.From.ID,
			RuleName:     hit.RuleName,
			WarningCount: ws.WarningCount,
		})
	} else if ws.WarningCount >= settings.AutomuteWarnings {
		s.enqueueAction(AutoAction{
			Kind:         AutoActionMute,
			GroupID:      msg.Chat.ID,
			UserID:       msg.From.ID,
			MinutesUntil: settings.AutomuteMinutes,
			RuleName:     hit.RuleName,
			WarningCount: ws.WarningCount,
		})
	}
	return nil
}

// LoadOrCreateSettings devuelve los settings del grupo, creando la
// fila con defaults si no existe (idempotente).
//
// Defaults: enabled=false, todos los toggles=false, flood_messages=5,
// flood_seconds=10, warning_limit=3, automute_warnings=3,
// automute_minutes=10, autoban_warnings=5, warning_expire_days=30.
func (s *Service) LoadOrCreateSettings(ctx context.Context, groupID int64) (*Settings, error) {
	settings, err := s.settingsRepo.GetSettings(ctx, groupID)
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	// Auto-create con defaults.
	defaults := &Settings{
		GroupID:           groupID,
		Enabled:           false,
		FloodEnabled:      false,
		FloodMessages:     5,
		FloodSeconds:      10,
		WarningLimit:      3,
		AutomuteWarnings:  3,
		AutomuteMinutes:   10,
		AutobanWarnings:   5,
		WarningExpireDays: 30,
	}
	if err := s.settingsRepo.UpsertSettings(ctx, defaults); err != nil {
		return nil, fmt.Errorf("automation: create default settings: %w", err)
	}
	// Releer para obtener updated_at puesto por la DB.
	return s.settingsRepo.GetSettings(ctx, groupID)
}

// LoadOrCreateWarningState devuelve el warning_state, creando la fila
// con warning_count=0 si no existe. El reset logico por expiracion
// ocurre en HandleMessage (paso 5).
func (s *Service) LoadOrCreateWarningState(ctx context.Context, groupID, userID int64) (*WarningState, error) {
	ws, err := s.warnRepo.GetWarningState(ctx, groupID, userID)
	if err == nil {
		return ws, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err := s.warnRepo.CreateWarningStateIfMissing(ctx, groupID, userID); err != nil {
		return nil, fmt.Errorf("automation: create default warning state: %w", err)
	}
	return s.warnRepo.GetWarningState(ctx, groupID, userID)
}

// --- API handler delegates (slice 2) ---
//
// Cada uno de estos es un thin wrapper sobre el repo: el handler HTTP
// no deberia hablar SQL directamente, asi que el Service expone una
// API por operacion que el handler invoca. Para Settings, los handlers
// usan LoadOrCreateSettings (auto-create de defaults cuando el grupo
// nunca configuro moderacion automatica).

// GetSettings devuelve los settings (ErrNotFound si la fila no existe).
// Para el handler GET, se prefiere LoadOrCreateSettings para no romper
// al admin cuando el grupo no configuro nada: el primer GET crea la
// fila con defaults.
func (s *Service) GetSettings(ctx context.Context, groupID int64) (*Settings, error) {
	return s.settingsRepo.GetSettings(ctx, groupID)
}

// UpsertSettings reemplaza/crea la fila de settings del grupo. El
// caller (handler) ya valido los rangos; los CHECK constraints en la
// DB catchean cualquier inconsistencia residual.
func (s *Service) UpsertSettings(ctx context.Context, settings *Settings) error {
	return s.settingsRepo.UpsertSettings(ctx, settings)
}

// ListBannedWords devuelve las palabras prohibidas del grupo (slice 2).
func (s *Service) ListBannedWords(ctx context.Context, groupID int64) ([]string, error) {
	return s.listsRepo.ListBannedWords(ctx, groupID)
}

// AddBannedWord agrega una palabra (idempotente via ON CONFLICT).
func (s *Service) AddBannedWord(ctx context.Context, groupID int64, word string) error {
	return s.listsRepo.AddBannedWord(ctx, groupID, word)
}

// RemoveBannedWord remueve una palabra (idempotente: nil si no existia).
func (s *Service) RemoveBannedWord(ctx context.Context, groupID int64, word string) error {
	return s.listsRepo.RemoveBannedWord(ctx, groupID, word)
}

// ListLinkAllowlist devuelve los dominios permitidos del grupo.
func (s *Service) ListLinkAllowlist(ctx context.Context, groupID int64) ([]string, error) {
	return s.listsRepo.ListLinkAllowlist(ctx, groupID)
}

// AddLinkAllowlist agrega un dominio (idempotente).
func (s *Service) AddLinkAllowlist(ctx context.Context, groupID int64, domain string) error {
	return s.listsRepo.AddLinkAllowlist(ctx, groupID, domain)
}

// RemoveLinkAllowlist remueve un dominio (idempotente).
func (s *Service) RemoveLinkAllowlist(ctx context.Context, groupID int64, domain string) error {
	return s.listsRepo.RemoveLinkAllowlist(ctx, groupID, domain)
}

// enqueueAction hace un non-blocking send al canal. Si el buffer esta
// lleno, dropea la accion y emite un warn log (spec REQ-11 mitigation).
// NUNCA bloquea el bus.
func (s *Service) enqueueAction(action AutoAction) {
	select {
	case s.autoActionCh <- action:
	default:
		s.logger.Warn("automation: autoActionCh full, dropping",
			"rule", action.RuleName,
			"group_id", action.GroupID,
			"user_id", action.UserID,
			"kind", string(action.Kind))
	}
}

// permissionOkAdmin exige que el bot sea administrador del grupo
// (bugfix #172). Es local al paquete automation: NO se reutiliza
// publications.permissionOk (misma logica, copy defensivo para
// mantener los modulos desacoplados y para no importar publications
// desde automation).
func permissionOkAdmin(g *groups.Group) bool {
	return g != nil && g.BotStatus == groups.StatusAdministrator
}

// preloadLists devuelve las listas pre-cargadas segun los toggles del
// settings. Si NINGUN toggle dependiente esta activo, devuelve nil
// (cero queries). Si solo uno esta activo, trae SOLO esa lista y la
// otra queda como slice vacio en la struct (las reglas verifican
// len(list) == 0 y vuelven nil sin evaluar).
//
// Si listsRepo es nil (caso defensivo en tests con Flood unico),
// devuelve nil.
func (s *Service) preloadLists(ctx context.Context, settings *Settings) *Lists {
	if s.listsRepo == nil {
		return nil
	}
	if !settings.BannedWordsEnabled && !settings.AntiLinkEnabled {
		return nil
	}
	out := &Lists{}
	if settings.BannedWordsEnabled {
		words, err := s.listsRepo.ListBannedWords(ctx, settings.GroupID)
		if err != nil {
			// Falla silenciosa: log + cero banned words. La regla
			// BannedWordsRule opera con lista vacia y vuelve nil.
			s.logger.Warn("automation: preload banned words failed",
				"group_id", settings.GroupID, "error", err)
			words = nil
		}
		out.BannedWords = words
	}
	if settings.AntiLinkEnabled {
		allow, err := s.listsRepo.ListLinkAllowlist(ctx, settings.GroupID)
		if err != nil {
			s.logger.Warn("automation: preload link allowlist failed",
				"group_id", settings.GroupID, "error", err)
			allow = nil
		}
		out.LinkAllowlist = allow
	}
	return out
}

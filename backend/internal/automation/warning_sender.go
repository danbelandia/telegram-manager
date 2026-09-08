// WarningSender (Fase 3, slice 2.1, REQ-26..28): interfaz consumer-side
// que el Service.HandleMessage invoca en el paso 7.5 cuando
// ws.WarningCount alcanza automute-1 (pre-mute) o autoban-1 (pre-ban).
// La implementacion concreta (tgWarningSender) envia el texto via
// telegram.Service.SendMessage y registra WARN_USER_SENT en la
// auditoria. Send sincrono con timeout 5s (REQs D5 + D1) — falla del
// sender NO aborta el pipeline (es best-effort feedback).
//
// Invariante (bugfix #172): el check de admin es permissionOkAdmin,
// local al paquete automation. NUNCA leer claves can_* — la deteccion
// de grupos no las puebla completas; la Bot API no las exige para
// sendMessage siendo admin. El check se hace 2 veces:
//
//  1. Service.HandleMessage paso 4 (antes del upsert).
//  2. tgWarningSender.SendWarning paso 1 (re-check por si el bot fue
//     removido entre el hit y el send).
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

// WarningKind clasifica el warning que se envia. Coincide 1:1 con
// TemplateKind (separados para que templates.go no importe telegram).
type WarningKind string

const (
	// WarningPreMute: warning antes del auto-mute (count == automute-1).
	WarningPreMute WarningKind = "pre_mute"
	// WarningPreBan: warning antes del auto-ban (count == autoban-1).
	WarningPreBan WarningKind = "pre_ban"
)

// warningSendTimeout acota el sendMessage para que un Telegram lento
// no bloquee el bus mas de este margen. Spec D5: 5s es 50x el send
// tipico (<100ms); bajo costo operativo.
const warningSendTimeout = 5 * time.Second

// WarningSender es la interfaz consumer-side. El Service la usa via
// inyeccion en NewService; los tests inyectan un fake.
//
// La firma recibe el *telegram.Message completo (no solo IDs) para
// que el sender pueda extraer FirstName/Username del From y renderear
// {nombre}. Coherente con AutoActioner.Execute(action AutoAction) que
// recibe la action completa con RuleName + WarningCount.
type WarningSender interface {
	SendWarning(ctx context.Context, msg *telegram.Message, count int16, kind WarningKind) error
}

// TelegramMesseger es la vista minima del adapter que el
// tgWarningSender necesita. *telegram.Adapter la satisface. Es una
// vista mas pequena que Service: NO exponemos GetMe, MuteUser, etc.
// para que el consumer-side no dependa de todo el adapter.
type TelegramMesseger interface {
	SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool, keyboard *telegram.InlineKeyboardMarkup) (int64, error)
}

// SettingsReader es la vista minima del repo de settings que el
// tgWarningSender necesita para releer la plantilla custom antes del
// send (por si el admin la cambio entre el upsert y el send).
// *Repository la satisface. nil-safe en el caller (Service) pero
// requerido por la implementacion concreta. Scopeado por tenant.
type SettingsReader interface {
	GetSettings(ctx context.Context, tenantID, groupID int64) (*Settings, error)
}

// --- Implementacion concreta ---

// tgWarningSender es el wrapper que produce el texto y envia a
// Telegram. Workflow de SendWarning:
//
//  1. Re-check permissionOkAdmin (bugfix #172). Si falla → log
//     PERMISSION_DENIED, return sin enviar.
//  2. Releer settings para obtener el template custom vigente (por si
//     el admin lo cambio entre el hit y el send). Si falla → cae al
//     default (best-effort, log warn).
//  3. RenderTemplate(kind, settings, msg, count, custom).
//  4. telegram.Service.SendMessage(chatID, rendered, false, nil).
//     Pasamos keyboard=nil (warning unidireccional, sin botones).
//  5. Log WARN_USER_SENT con ActorID=nil (sistema) + metadata
//     {warning_count, threshold_kind, template_used}.
type tgWarningSender struct {
	tg       TelegramMesseger
	logs     LogWriter
	settings SettingsReader
	groups   GroupReader
	logger   *slog.Logger
	now      func() time.Time
	// tenantID aisla el sender (slice 0): un sender por tenant, con el
	// adapter de su bot. Main lo inyecta; el Service ya es por tenant.
	tenantID int64
}

// NewWarningSender construye el wrapper concreto. logger puede ser nil
// (usa slog.Default()).
func NewWarningSender(tg TelegramMesseger, logs LogWriter, settings SettingsReader, groups GroupReader, logger *slog.Logger, tenantID int64) WarningSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &tgWarningSender{
		tg:       tg,
		logs:     logs,
		settings: settings,
		groups:   groups,
		logger:   logger,
		now:      time.Now,
		tenantID: tenantID,
	}
}

// SendWarning ejecuta el flujo documentado arriba. ctx ya trae el
// timeout 5s aplicado por el caller (Service.HandleMessage). NO
// reaplicamos timeout aca para evitar doble wrapping; si el caller no
// lo aplica, Telegram puede bloquear mas de 5s.
func (w *tgWarningSender) SendWarning(ctx context.Context, msg *telegram.Message, count int16, kind WarningKind) error {
	if kind == "" {
		// Defensive: el caller deberia filtrar los kinds vacios
		// (thresholdKindFor retorna "" cuando no aplica). Si llegamos
		// aca con "", algo del caller esta roto; salimos sin enviar.
		return fmt.Errorf("automation: warning sender: empty kind")
	}
	if msg == nil || msg.From == nil || msg.From.ID == 0 {
		return fmt.Errorf("automation: warning sender: nil message or from")
	}
	groupID := msg.Chat.ID
	userID := msg.From.ID

	// Paso 1: re-check permissionOkAdmin. Re-leer el grupo: el bot
	// pudo haber sido removido entre el hit del pipeline (paso 4 de
	// HandleMessage) y este send.
	g, err := w.groups.GetByTenant(ctx, w.tenantID, groupID)
	if err != nil {
		return w.logNotFound(ctx, groupID, userID, count, kind)
	}
	if !permissionOkAdmin(g) {
		return w.logPermissionDenied(ctx, groupID, userID, count, kind)
	}

	// Paso 2: releer settings para el template custom. Best-effort:
	// si falla, usamos defaults puros.
	settings, err := w.settings.GetSettings(ctx, w.tenantID, groupID)
	if err != nil || settings == nil {
		w.logger.Warn("automation: warning sender: get settings failed, using default template",
			"group_id", groupID, "error", err)
		settings = DefaultSettings(w.tenantID, groupID)
	}

	// Paso 3: renderizar el texto.
	tplKind := TemplateKind(kind)
	templateUsed := "default"
	custom := settings.WarnUserTemplate
	if custom != nil && *custom != "" {
		templateUsed = "custom"
	}
	text := RenderTemplate(tplKind, settings, msg, count, custom)

	// Paso 4: enviar. Pasamos keyboard=nil (warning unidireccional,
	// sin botones inline).
	if _, err := w.tg.SendMessage(ctx, groupID, text, false, nil); err != nil {
		return w.logSendFailure(ctx, groupID, userID, count, kind, templateUsed, err)
	}

	// Paso 5: log SUCCESS.
	return w.logSuccess(ctx, groupID, userID, count, kind, templateUsed)
}

// --- Log helpers ---

// logSuccess escribe WARN_USER_SENT con ActorID=nil + metadata.
func (w *tgWarningSender) logSuccess(ctx context.Context, groupID, userID int64, count int16, kind WarningKind, templateUsed string) error {
	targetUserID := userID
	e := &logs.Entry{
		TenantID:     w.tenantID,
		ActorID:      nil, // sistema (no admin del panel)
		GroupID:      groupID,
		Action:       logs.ActionWarnUserSent,
		TargetUserID: &targetUserID,
		Status:       logs.StatusSuccess,
		Metadata: map[string]any{
			"warning_count":  count,
			"threshold_kind": string(kind),
			"template_used":  templateUsed,
		},
	}
	if err := w.logs.Create(ctx, e); err != nil {
		w.logger.Error("automation: failed to write WARN_USER_SENT success log",
			"group_id", groupID, "user_id", userID, "error", err)
	}
	return nil
}

// logSendFailure mapea errores del adapter al status de log apropiado.
func (w *tgWarningSender) logSendFailure(ctx context.Context, groupID, userID int64, count int16, kind WarningKind, templateUsed string, sendErr error) error {
	status := mapWarningError(sendErr)
	targetUserID := userID
	e := &logs.Entry{
		TenantID:     w.tenantID,
		ActorID:      nil,
		GroupID:      groupID,
		Action:       logs.ActionWarnUserSent,
		TargetUserID: &targetUserID,
		Status:       status,
		Metadata: map[string]any{
			"warning_count":  count,
			"threshold_kind": string(kind),
			"template_used":  templateUsed,
		},
	}
	msg := sendErr.Error()
	e.ErrorMessage = &msg
	if lerr := w.logs.Create(ctx, e); lerr != nil {
		w.logger.Error("automation: failed to write WARN_USER_SENT failure log",
			"group_id", groupID, "user_id", userID, "error", lerr)
	}
	return sendErr
}

// logPermissionDenied: bot removido entre hit y send (re-check
// fallido). Log warn + return.
func (w *tgWarningSender) logPermissionDenied(ctx context.Context, groupID, userID int64, count int16, kind WarningKind) error {
	targetUserID := userID
	e := &logs.Entry{
		TenantID:     w.tenantID,
		ActorID:      nil,
		GroupID:      groupID,
		Action:       logs.ActionWarnUserSent,
		TargetUserID: &targetUserID,
		Status:       logs.StatusPermissionDenied,
		Metadata: map[string]any{
			"warning_count":  count,
			"threshold_kind": string(kind),
			"template_used":  "n/a",
		},
	}
	msg := "automation: bot no es administrador del grupo (warning re-check fallido)"
	e.ErrorMessage = &msg
	if lerr := w.logs.Create(ctx, e); lerr != nil {
		w.logger.Error("automation: failed to write WARN_USER_SENT permission log", "error", lerr)
	}
	return fmt.Errorf("automation: bot no es administrador del grupo")
}

// logNotFound: grupo eliminado entre hit y send.
func (w *tgWarningSender) logNotFound(ctx context.Context, groupID, userID int64, count int16, kind WarningKind) error {
	targetUserID := userID
	e := &logs.Entry{
		TenantID:     w.tenantID,
		ActorID:      nil,
		GroupID:      groupID,
		Action:       logs.ActionWarnUserSent,
		TargetUserID: &targetUserID,
		Status:       logs.StatusNotFound,
		Metadata: map[string]any{
			"warning_count":  count,
			"threshold_kind": string(kind),
			"template_used":  "n/a",
		},
	}
	msg := "automation: grupo no encontrado (warning re-check fallido)"
	e.ErrorMessage = &msg
	if lerr := w.logs.Create(ctx, e); lerr != nil {
		w.logger.Error("automation: failed to write WARN_USER_SENT not_found log", "error", lerr)
	}
	return fmt.Errorf("automation: grupo no encontrado")
}

// mapWarningError mapea errores del adapter a Status de log. Mismo
// patron que autoactioner.mapTelegramError.
func mapWarningError(err error) logs.Status {
	switch {
	case err == nil:
		return logs.StatusSuccess
	case errors.Is(err, telegram.ErrPermissionDenied):
		return logs.StatusPermissionDenied
	case errors.Is(err, telegram.ErrTelegramNotFound):
		return logs.StatusNotFound
	default:
		return logs.StatusTelegramError
	}
}

// Aseguramos que el import de groups se use (compile-time check). El
// permissionOkAdmin declarado en service.go usa groups.StatusAdministrator;
// este file solo referencia el paquete via la variable para no
// necesitar import directo (mantiene consumer-side desacoplado).
var _ = groups.StatusAdministrator

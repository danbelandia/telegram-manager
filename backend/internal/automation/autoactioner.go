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

// AutoActioner es la interfaz consumer-side que el Worker invoca para
// ejecutar la accion sobre Telegram. La firma Execute toma el
// AutoAction completo (no solo IDs) porque el wrapper necesita
// `RuleName` y `WarningCount` para la metadata del log.
//
// Vive como interfaz (no struct concreto) para que los tests inyecten
// un fake y para que el Worker no dependa de la implementacion
// concreta (mismo patron que publications.MessageSender).
type AutoActioner interface {
	Execute(ctx context.Context, action AutoAction) error
}

// --- Vistas minimas (consumer-side) ---

// LogWriter (consumer-side) — *logs.Repository lo satisface.
type LogWriter interface {
	Create(ctx context.Context, e *logs.Entry) error
}

// GroupReader (consumer-side) — *groups.Repository lo satisface.
type GroupReader interface {
	GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error)
}

// TelegramActor es la vista minima del adapter que el AutoActioner
// necesita. *telegram.Adapter lo satisface y pasa por el token bucket
// del adapter (rate-limit §18.1). NUNCA se llama a la Bot API por
// fuera de este adapter.
type TelegramActor interface {
	MuteUser(ctx context.Context, chatID, userID int64, untilDate int64) error
	BanUser(ctx context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error
}

// --- Implementacion concreta ---

// tgAutoActioner es la implementacion concreta que el Worker usa en
// produccion. Re-lee el grupo antes de despachar (re-check de
// permissionOkAdmin) por si el bot fue removido entre el hit y el
// dispatch. Si la lectura falla o el bot ya no es admin, loguea
// PERMISSION_DENIED con ActorID=nil y NO llama a Telegram.
type tgAutoActioner struct {
	tg     TelegramActor
	logs   LogWriter
	groups GroupReader
	logger *slog.Logger
	now    func() time.Time
}

// NewAutoActioner construye el wrapper concreto. logger puede ser nil
// (usa slog.Default()). now puede ser nil (usa time.Now) — util en
// tests para calcular untilDate deterministico.
func NewAutoActioner(tg TelegramActor, logs LogWriter, groups GroupReader, logger *slog.Logger) AutoActioner {
	if logger == nil {
		logger = slog.Default()
	}
	return &tgAutoActioner{
		tg:     tg,
		logs:   logs,
		groups: groups,
		logger: logger,
		now:    time.Now,
	}
}

// Execute despacha la AutoAction. Workflow:
//
//  1. Re-leer el grupo (bot pudo haber sido removido entre el hit y
//     aca). Si el grupo no existe → log NOT_FOUND, no llama a
//     Telegram.
//  2. permissionOkAdmin (bugfix #172): g.BotStatus ==
//     groups.StatusAdministrator. NUNCA leer claves can_* (la
//     deteccion de grupos no las puebla completas; la Bot API no las
//     exige para sendMessage/restrictChatMember/banChatMember en
//     grupos siendo admin).
//  3. Si no admin → log PERMISSION_DENIED, no llama a Telegram.
//  4. Si admin → mute (calcula untilDate) o ban (indefinido+revoke).
//     Log SUCCESS o mapea error a status.
func (a *tgAutoActioner) Execute(ctx context.Context, action AutoAction) error {
	g, err := a.groups.GetByTelegramID(ctx, action.GroupID)
	if err != nil {
		// Grupo no existe o error de DB → log NOT_FOUND, no despacha.
		return a.logNotFound(ctx, action)
	}
	if g.BotStatus != groups.StatusAdministrator {
		return a.logPermissionDenied(ctx, action)
	}

	switch action.Kind {
	case AutoActionMute:
		until := a.now().Add(time.Duration(action.MinutesUntil) * time.Minute).Unix()
		if err := a.tg.MuteUser(ctx, action.GroupID, action.UserID, until); err != nil {
			a.logFailure(ctx, action, logs.ActionAutomuteUser, err)
			return err
		}
		a.logSuccess(ctx, action, logs.ActionAutomuteUser)
		return nil

	case AutoActionBan:
		// Ban indefinido (untilDate=0) + revoke=true.
		if err := a.tg.BanUser(ctx, action.GroupID, action.UserID, 0, true); err != nil {
			a.logFailure(ctx, action, logs.ActionAutobanUser, err)
			return err
		}
		a.logSuccess(ctx, action, logs.ActionAutobanUser)
		return nil

	default:
		return fmt.Errorf("automation: unknown action kind %q", action.Kind)
	}
}

// logSuccess escribe un log con ActorID=nil (auto-action del sistema),
// status=SUCCESS, action=ActionXxx, metadata={rule_name, warning_count}.
func (a *tgAutoActioner) logSuccess(ctx context.Context, action AutoAction, actionConst string) {
	e := &logs.Entry{
		ActorID:      nil, // sistema, NO admin
		GroupID:      action.GroupID,
		Action:       actionConst,
		TargetUserID: &action.UserID,
		Status:       logs.StatusSuccess,
		Metadata: map[string]any{
			"rule_name":     action.RuleName,
			"warning_count": action.WarningCount,
		},
	}
	if err := a.logs.Create(ctx, e); err != nil {
		a.logger.Error("automation: failed to write success log",
			"action", actionConst,
			"group_id", action.GroupID,
			"user_id", action.UserID,
			"error", err)
	}
}

// logFailure mapea un error del adapter al status de log apropiado.
func (a *tgAutoActioner) logFailure(ctx context.Context, action AutoAction, actionConst string, err error) {
	status := mapTelegramError(err)
	e := &logs.Entry{
		ActorID:      nil,
		GroupID:      action.GroupID,
		Action:       actionConst,
		TargetUserID: &action.UserID,
		Status:       status,
		Metadata: map[string]any{
			"rule_name":     action.RuleName,
			"warning_count": action.WarningCount,
		},
	}
	msg := err.Error()
	e.ErrorMessage = &msg
	if lerr := a.logs.Create(ctx, e); lerr != nil {
		a.logger.Error("automation: failed to write failure log",
			"action", actionConst,
			"group_id", action.GroupID,
			"user_id", action.UserID,
			"error", lerr)
	}
}

// logPermissionDenied es el path "bot removido entre hit y dispatch":
// no hay error del adapter, solo el re-check falla. status=PERMISSION_DENIED.
func (a *tgAutoActioner) logPermissionDenied(ctx context.Context, action AutoAction) error {
	actionConst := actionConstFor(action)
	e := &logs.Entry{
		ActorID:      nil,
		GroupID:      action.GroupID,
		Action:       actionConst,
		TargetUserID: &action.UserID,
		Status:       logs.StatusPermissionDenied,
		Metadata: map[string]any{
			"rule_name":     action.RuleName,
			"warning_count": action.WarningCount,
		},
	}
	msg := "automation: bot no es administrador del grupo (re-check fallido)"
	e.ErrorMessage = &msg
	if lerr := a.logs.Create(ctx, e); lerr != nil {
		a.logger.Error("automation: failed to write permission_denied log", "error", lerr)
	}
	return fmt.Errorf("automation: bot no es administrador del grupo")
}

// logNotFound cubre el caso "grupo eliminado" entre hit y dispatch.
func (a *tgAutoActioner) logNotFound(ctx context.Context, action AutoAction) error {
	actionConst := actionConstFor(action)
	e := &logs.Entry{
		ActorID:      nil,
		GroupID:      action.GroupID,
		Action:       actionConst,
		TargetUserID: &action.UserID,
		Status:       logs.StatusNotFound,
		Metadata: map[string]any{
			"rule_name":     action.RuleName,
			"warning_count": action.WarningCount,
		},
	}
	msg := "automation: grupo no encontrado (re-check fallido)"
	e.ErrorMessage = &msg
	if lerr := a.logs.Create(ctx, e); lerr != nil {
		a.logger.Error("automation: failed to write not_found log", "error", lerr)
	}
	return fmt.Errorf("automation: grupo no encontrado")
}

// actionConstFor devuelve el constante de log segun el kind.
func actionConstFor(action AutoAction) string {
	if action.Kind == AutoActionBan {
		return logs.ActionAutobanUser
	}
	return logs.ActionAutomuteUser
}

// mapTelegramError mapea errores del adapter a Status de log.
func mapTelegramError(err error) logs.Status {
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

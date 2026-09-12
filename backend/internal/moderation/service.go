// Package moderation orquesta las acciones administrativas sobre un
// grupo (AGENTS.md §8-10): valida que el grupo exista, que el bot tenga
// el permiso necesario, llama a Telegram y registra el resultado en
// logs. El resto del backend no llama a Telegram directo: pasa por
// este flujo (design D7: la autorizacion por grupo es capa de servicio,
// no middleware).
package moderation

import (
	"context"
	"errors"
	"fmt"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/joinrequests"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// Errores de dominio del flujo (el handler los mapea a los codigos de
// la seccion 18 del spec).
var (
	// ErrGroupNotFound: el grupo no existe en nuestra base (404).
	ErrGroupNotFound = errors.New("moderation: group not found")
	// ErrBotPermission: el bot no tiene la bot_permission requerida;
	// no se llama a Telegram (403 PERMISSION_DENIED).
	ErrBotPermission = errors.New("moderation: el bot no tiene el permiso necesario")
	// ErrRequestNotFound: la solicitud de ingreso no existe o no
	// pertenece al grupo (404).
	ErrRequestNotFound = errors.New("moderation: join request not found")
	// ErrRequestAlreadyDecided: la solicitud ya no esta pendiente (409).
	ErrRequestAlreadyDecided = errors.New("moderation: join request already decided")
)

// GroupPermissionReader es la vista minima del repositorio de grupos
// que el service necesita (lado consumidor; *groups.Repository la
// satisface). Scopeado por tenant (slice 0): grupo ajeno →
// groups.ErrNotFound → ErrGroupNotFound (404, D9).
type GroupPermissionReader interface {
	GetByTenant(ctx context.Context, tenantID, id int64) (*groups.Group, error)
}

// TelegramActions es la vista minima del Service de Telegram para las
// acciones de moderacion (lado consumidor; *telegram.Adapter la
// satisface).
type TelegramActions interface {
	BanUser(ctx context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error
	UnbanUser(ctx context.Context, chatID, userID int64) error
	MuteUser(ctx context.Context, chatID, userID int64, untilDate int64) error
	UnmuteUser(ctx context.Context, chatID, userID int64) error
	DeleteMessage(ctx context.Context, chatID, messageID int64) error
	PinMessage(ctx context.Context, chatID, messageID int64) error
	LockGroup(ctx context.Context, chatID int64) error
	UnlockGroup(ctx context.Context, chatID int64) error
	ApproveJoinRequest(ctx context.Context, chatID, userID int64) error
	RejectJoinRequest(ctx context.Context, chatID, userID int64) error
}

// RequestStore es la vista minima del repositorio de solicitudes para
// decidir (aprove/reject). GetByID verifica ownership del tenant
// (slice 0): solicitud ajena → joinrequests.ErrNotFound.
type RequestStore interface {
	GetByID(ctx context.Context, tenantID, id int64) (*joinrequests.Request, error)
	Resolve(ctx context.Context, id int64, status joinrequests.Status, decidedBy *int64) error
}

// LogWriter es la vista minima del repositorio de logs
// (*logs.Repository la satisface).
type LogWriter interface {
	Create(ctx context.Context, e *logs.Entry) error
}

// Service ejecuta el flujo por accion. ActorID es el id del admin
// autenticado (claims); se registra como actor de auditoria en el log.
//
// Slice 0: la instancia es por tenant (main construye una por bot con
// su adapter). tenantID aisla grupo, solicitud y log en cada llamada.
type Service struct {
	tenantID int64
	groups   GroupPermissionReader
	tg       TelegramActions
	requests RequestStore
	logs     LogWriter
}

// NewService construye el servicio de moderacion para un tenant.
func NewService(tenantID int64, groups GroupPermissionReader, tg TelegramActions, requests RequestStore, logs LogWriter) *Service {
	return &Service{tenantID: tenantID, groups: groups, tg: tg, requests: requests, logs: logs}
}

// Ban banea a un usuario del grupo (untilDate==0: indefinido).
func (s *Service) Ban(ctx context.Context, actorID, groupID, userID int64, untilDate int64, revokeMessages bool) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionBanUser, &userID, func() error {
		return s.tg.BanUser(ctx, groupID, userID, untilDate, revokeMessages)
	})
}

// Unban desbanea a un usuario.
func (s *Service) Unban(ctx context.Context, actorID, groupID, userID int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionUnbanUser, &userID, func() error {
		return s.tg.UnbanUser(ctx, groupID, userID)
	})
}

// Mute restringe el envio de mensajes (untilDate==0: indefinido).
func (s *Service) Mute(ctx context.Context, actorID, groupID, userID int64, untilDate int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionMuteUser, &userID, func() error {
		return s.tg.MuteUser(ctx, groupID, userID, untilDate)
	})
}

// Unmute restaura el envio de mensajes.
func (s *Service) Unmute(ctx context.Context, actorID, groupID, userID int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionUnmuteUser, &userID, func() error {
		return s.tg.UnmuteUser(ctx, groupID, userID)
	})
}

// DeleteMessage borra un mensaje del grupo.
func (s *Service) DeleteMessage(ctx context.Context, actorID, groupID, messageID int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionDeleteMessage, nil, func() error {
		return s.tg.DeleteMessage(ctx, groupID, messageID)
	})
}

// PinMessage fija un mensaje del grupo.
func (s *Service) PinMessage(ctx context.Context, actorID, groupID, messageID int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionPinMessage, nil, func() error {
		return s.tg.PinMessage(ctx, groupID, messageID)
	})
}

// Lock cierra el envio de mensajes para los no-administradores.
func (s *Service) Lock(ctx context.Context, actorID, groupID int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionLockGroup, nil, func() error {
		return s.tg.LockGroup(ctx, groupID)
	})
}

// Unlock reabre el envio de mensajes.
func (s *Service) Unlock(ctx context.Context, actorID, groupID int64) error {
	return s.runAction(ctx, actorID, groupID, logs.ActionUnlockGroup, nil, func() error {
		return s.tg.UnlockGroup(ctx, groupID)
	})
}

// Approve aprueba una solicitud de ingreso pendiente.
func (s *Service) Approve(ctx context.Context, actorID, groupID, requestID int64) error {
	return s.decide(ctx, actorID, groupID, requestID,
		logs.ActionApproveJoinRequest, joinrequests.StatusApproved,
		func(ctx context.Context, chatID, userID int64) error {
			return s.tg.ApproveJoinRequest(ctx, chatID, userID)
		},
	)
}

// Reject rechaza una solicitud de ingreso pendiente.
func (s *Service) Reject(ctx context.Context, actorID, groupID, requestID int64) error {
	return s.decide(ctx, actorID, groupID, requestID,
		logs.ActionRejectJoinRequest, joinrequests.StatusRejected,
		func(ctx context.Context, chatID, userID int64) error {
			return s.tg.RejectJoinRequest(ctx, chatID, userID)
		},
	)
}

// BatchItemResult es el resultado de una operacion individual dentro de
// un batch approve/reject. Si Error es nil, la operacion fue exitosa.
type BatchItemResult struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// BatchDecide procesa N solicitudes de ingreso en un solo batch.
// Resuelve el grupo UNA vez y luego itera sobre cada requestID llamando
// a decide() (que reutiliza grupo→solicitud→permiso→telegram→log).
// Devuelve resultados individuales: cada item puede fallar
// independientemente de los demas.
func (s *Service) BatchDecide(ctx context.Context, actorID, groupID int64, action string, requestIDs []int64) ([]BatchItemResult, error) {
	// Resolver el grupo una sola vez: si el grupo no existe o el bot
	// no tiene permisos, todas las operaciones fallan con el mismo error
	// (fail-fast para grupo invalido, no per-item).
	group, err := s.groups.GetByTenant(ctx, s.tenantID, groupID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("moderation: batch %s: get group: %w", action, err)
	}
	if !permissionOk(group) {
		return nil, ErrBotPermission
	}

	var call func(ctx context.Context, chatID, userID int64) error
	var status joinrequests.Status
	var logAction string
	switch action {
	case "approve":
		call = func(ctx context.Context, chatID, userID int64) error {
			return s.tg.ApproveJoinRequest(ctx, chatID, userID)
		}
		status = joinrequests.StatusApproved
		logAction = logs.ActionApproveJoinRequest
	case "reject":
		call = func(ctx context.Context, chatID, userID int64) error {
			return s.tg.RejectJoinRequest(ctx, chatID, userID)
		}
		status = joinrequests.StatusRejected
		logAction = logs.ActionRejectJoinRequest
	default:
		return nil, fmt.Errorf("moderation: batch: invalid action %q", action)
	}

	results := make([]BatchItemResult, 0, len(requestIDs))
	for _, rid := range requestIDs {
		entry := &logs.Entry{TenantID: s.tenantID, ActorID: &actorID, GroupID: groupID, Action: logAction}

		req, err := s.requests.GetByID(ctx, s.tenantID, rid)
		if err != nil {
			if errors.Is(err, joinrequests.ErrNotFound) {
				msg := ErrRequestNotFound.Error()
				entry.TargetUserID = nil
				entry.Status = logs.StatusNotFound
				entry.ErrorMessage = &msg
				_ = s.logs.Create(ctx, entry)
				results = append(results, BatchItemResult{ID: rid, Status: "error", Error: msg})
				continue
			}
			// Error inesperado del repositorio: abortamos el batch.
			return results, fmt.Errorf("moderation: batch %s: get request %d: %w", action, rid, err)
		}
		if req.GroupID != groupID {
			msg := ErrRequestNotFound.Error()
			entry.Status = logs.StatusNotFound
			entry.ErrorMessage = &msg
			_ = s.logs.Create(ctx, entry)
			results = append(results, BatchItemResult{ID: rid, Status: "error", Error: msg})
			continue
		}
		if req.Status != joinrequests.StatusPending {
			msg := ErrRequestAlreadyDecided.Error()
			entry.Status = logs.StatusValidationError
			entry.ErrorMessage = &msg
			_ = s.logs.Create(ctx, entry)
			results = append(results, BatchItemResult{ID: rid, Status: "error", Error: msg})
			continue
		}

		userID := req.UserID
		if err := call(ctx, groupID, userID); err != nil {
			entry.Status = statusForTelError(err)
			msg := err.Error()
			entry.ErrorMessage = &msg
			_ = s.logs.Create(ctx, entry)
			results = append(results, BatchItemResult{ID: rid, Status: "error", Error: msg})
			continue
		}

		if err := s.requests.Resolve(ctx, rid, status, &actorID); err != nil {
			return results, fmt.Errorf("moderation: batch %s: resolve %d: %w", action, rid, err)
		}
		entry.Status = logs.StatusSuccess
		_ = s.logs.Create(ctx, entry)
		results = append(results, BatchItemResult{ID: rid, Status: string(status)})
	}

	return results, nil
}

// decide es el flujo de approve/reject: grupo → solicitud → permiso →
// telegram → resolver → log.
func (s *Service) decide(ctx context.Context, actorID, groupID, requestID int64, action string, status joinrequests.Status, call func(ctx context.Context, chatID, userID int64) error) error {
	group, err := s.groups.GetByTenant(ctx, s.tenantID, groupID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			return ErrGroupNotFound
		}
		return fmt.Errorf("moderation: %s: get group: %w", action, err)
	}

	req, err := s.requests.GetByID(ctx, s.tenantID, requestID)
	if err != nil {
		if errors.Is(err, joinrequests.ErrNotFound) {
			return ErrRequestNotFound
		}
		return fmt.Errorf("moderation: %s: get request: %w", action, err)
	}
	if req.GroupID != groupID {
		// La solicitud existe pero no pertenece a este grupo: mismo
		// error que inexistente, no se filtra informacion.
		return ErrRequestNotFound
	}
	if req.Status != joinrequests.StatusPending {
		return ErrRequestAlreadyDecided
	}

	entry := &logs.Entry{TenantID: s.tenantID, ActorID: &actorID, GroupID: groupID, Action: action}
	if !permissionOk(group) {
		return s.logFailure(ctx, entry, logs.StatusPermissionDenied, ErrBotPermission)
	}

	userID := req.UserID
	if err := call(ctx, groupID, userID); err != nil {
		return s.logFailure(ctx, entry, statusForTelError(err), err)
	}

	if err := s.requests.Resolve(ctx, req.ID, status, &actorID); err != nil {
		return fmt.Errorf("moderation: %s: resolve: %w", action, err)
	}
	entry.Status = logs.StatusSuccess
	if err := s.logs.Create(ctx, entry); err != nil {
		return fmt.Errorf("moderation: %s: log: %w", action, err)
	}
	return nil
}

// runAction es el flujo generico de las acciones sobre un usuario o
// mensaje: grupo existe → permiso bot → telegram → log.
func (s *Service) runAction(ctx context.Context, actorID, groupID int64, action string, target *int64, call func() error) error {
	group, err := s.groups.GetByTenant(ctx, s.tenantID, groupID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			return ErrGroupNotFound
		}
		return fmt.Errorf("moderation: %s: get group: %w", action, err)
	}

	entry := &logs.Entry{
		TenantID:     s.tenantID,
		ActorID:      &actorID,
		GroupID:      groupID,
		Action:       action,
		TargetUserID: target,
	}
	if !permissionOk(group) {
		return s.logFailure(ctx, entry, logs.StatusPermissionDenied, ErrBotPermission)
	}

	if err := call(); err != nil {
		return s.logFailure(ctx, entry, statusForTelError(err), err)
	}

	entry.Status = logs.StatusSuccess
	if err := s.logs.Create(ctx, entry); err != nil {
		return fmt.Errorf("moderation: %s: log: %w", action, err)
	}
	return nil
}

// logFailure registra un log de fallo y devuelve el error original
// envuelto (el handler mapea los codigos §18 con errors.Is). Si el log
// falla no se tapa el error real de la accion.
func (s *Service) logFailure(ctx context.Context, entry *logs.Entry, status logs.Status, cause error) error {
	entry.Status = status
	msg := cause.Error()
	entry.ErrorMessage = &msg
	if err := s.logs.Create(ctx, entry); err != nil {
		return fmt.Errorf("moderation: %s: log failure: %w", entry.Action, err)
	}
	return fmt.Errorf("moderation: %s: %w", entry.Action, cause)
}

// statusForTelError mapea un error del adapter al status de log.
func statusForTelError(err error) logs.Status {
	switch {
	case errors.Is(err, telegram.ErrPermissionDenied):
		return logs.StatusPermissionDenied
	case errors.Is(err, telegram.ErrTelegramNotFound):
		return logs.StatusNotFound
	default:
		return logs.StatusTelegramError
	}
}

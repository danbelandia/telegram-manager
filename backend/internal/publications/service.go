// Package publications orquesta el flujo de creacion + publicacion
// (AGENTS.md §22, slice 1): valida el texto, verifica que el grupo
// exista y que el bot tenga permiso can_manage_chat, inserta con status
// sending, llama a telegram.SendMessage, actualiza sent/failed + log.
// El resto del backend no llama a Telegram directo: pasa por este
// servicio (design D3, D4, D5, D6).
package publications

import (
	"context"
	"errors"
	"fmt"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// maxTextLength es el limite de la Bot API para mensajes de texto
// (docs/telegram_api_reference.md §sendMessage). Se aplica en el
// servicio, no en el handler (design D5).
const maxTextLength = 4096

// Errores de dominio del flujo.
var (
	// ErrBotPermission: el bot no tiene can_manage_chat en el grupo; no
	// se llama a Telegram (403 PERMISSION_DENIED).
	ErrBotPermission = errors.New("publications: el bot no tiene el permiso necesario")
	// ErrGroupNotFound: el grupo no existe en nuestra base (404).
	ErrGroupNotFound = errors.New("publications: group not found")
)

// GroupReader es la vista minima del repositorio de grupos (lado
// consumidor; *groups.Repository la satisface).
type GroupReader interface {
	GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error)
}

// MessageSender es la vista minima del Service de Telegram para
// publicaciones (*telegram.Adapter la satisface).
type MessageSender interface {
	SendMessage(ctx context.Context, chatID int64, text string, disableWebPagePreview bool) (int64, error)
}

// LogWriter es la vista minima del repositorio de logs
// (*logs.Repository la satisface).
type LogWriter interface {
	Create(ctx context.Context, e *logs.Entry) error
}

// PubStore es la vista minima del repositorio de publicaciones
// (*publications.Repository la satisface).
type PubStore interface {
	Create(ctx context.Context, p *Publication) error
	GetByID(ctx context.Context, id int64) (*Publication, error)
	List(ctx context.Context) ([]Publication, error)
	UpdateStatus(ctx context.Context, id int64, status Status, messageID *int64, errMsg *string) error
}

// Service ejecuta el flujo de publicaciones. ActorID es el id del admin
// autenticado (claims); se registra como actor en la fila y en el log.
type Service struct {
	groups GroupReader
	tg     MessageSender
	logs   LogWriter
	store  PubStore
}

// NewService construye el servicio de publicaciones.
func NewService(groups GroupReader, tg MessageSender, store PubStore, logs LogWriter) *Service {
	return &Service{groups: groups, tg: tg, logs: logs, store: store}
}

// Publish crea una publicacion y la envia al grupo seleccionado. Orden
// (design D4): validar text → grupo existe (404) → permiso (403) →
// create(sending) → sendMessage → update(sent/failed) → log.
func (s *Service) Publish(ctx context.Context, actorID, groupID int64, text string) (*Publication, error) {
	if text == "" {
		return nil, ErrTextEmpty
	}
	if len(text) > maxTextLength {
		return nil, ErrTextTooLong
	}

	group, err := s.groups.GetByTelegramID(ctx, groupID)
	if err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("publications: publish: get group: %w", err)
	}

	entry := &logs.Entry{
		ActorID: &actorID,
		GroupID: groupID,
		Action:  logs.ActionPublishMessage,
	}
	if !permissionOk(group) {
		return nil, s.logFailure(ctx, entry, logs.StatusPermissionDenied, ErrBotPermission)
	}

	// Estado intermedio: sending (transitorio; design D6).
	pub := &Publication{
		TelegramID: groupID,
		Text:       text,
		Status:     StatusSending,
		ActorID:    &actorID,
	}
	if err := s.store.Create(ctx, pub); err != nil {
		return nil, fmt.Errorf("publications: publish: create: %w", err)
	}
	entry.Metadata = map[string]any{"publication_id": pub.ID}

	messageID, err := s.tg.SendMessage(ctx, groupID, text, false)
	if err != nil {
		// status=failed (D6), error_message poblado, log de fallo.
		errMsg := err.Error()
		if uerr := s.store.UpdateStatus(ctx, pub.ID, StatusFailed, nil, &errMsg); uerr != nil {
			return nil, fmt.Errorf("publications: publish: update failed: %w", uerr)
		}
		pub.Status = StatusFailed
		pub.ErrorMessage = &errMsg
		return nil, s.logFailure(ctx, entry, statusForTelError(err), err)
	}

	// Exito: status=sent, message_id guardado.
	if err := s.store.UpdateStatus(ctx, pub.ID, StatusSent, &messageID, nil); err != nil {
		return nil, fmt.Errorf("publications: publish: update sent: %w", err)
	}
	pub.Status = StatusSent
	pub.MessageID = &messageID
	entry.Metadata["message_id"] = messageID
	entry.Status = logs.StatusSuccess
	if err := s.logs.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("publications: publish: log: %w", err)
	}
	return pub, nil
}

// GetByID devuelve una publicacion por su ID interno.
func (s *Service) GetByID(ctx context.Context, id int64) (*Publication, error) {
	return s.store.GetByID(ctx, id)
}

// List devuelve las publicaciones mas recientes (max 50, desc).
func (s *Service) List(ctx context.Context) ([]Publication, error) {
	return s.store.List(ctx)
}

// permissionOk verifica en groups.bot_permissions la clave
// can_manage_chat (permiso minimo de administrador para publicar).
// Sin permisos conocidos (nil) la accion NO se habilita (principio 4).
func permissionOk(g *groups.Group) bool {
	return g != nil && g.BotPermissions["can_manage_chat"]
}

// logFailure registra un log de fallo y devuelve el error original
// envuelto (el handler mapea los codigos §18 con errors.Is).
func (s *Service) logFailure(ctx context.Context, entry *logs.Entry, status logs.Status, cause error) error {
	entry.Status = status
	msg := cause.Error()
	entry.ErrorMessage = &msg
	if err := s.logs.Create(ctx, entry); err != nil {
		return fmt.Errorf("publications: %s: log failure: %w", entry.Action, err)
	}
	return fmt.Errorf("publications: %s: %w", entry.Action, cause)
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

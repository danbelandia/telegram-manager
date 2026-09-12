package joinrequests

import (
	"context"

	"github.com/telegram-manager/backend/internal/telegram"
)

// HandleChatJoinRequest mapea un update chat_join_request a un Request
// (rellena r) sin tocar la base de datos (funcion pura, testeable;
// patron groups.HandleMyChatMember). El bus luego persiste con
// Repository.UpsertPending.
//
// Si upd es nil, r no cambia y no hay error (el bus sigue).
func HandleChatJoinRequest(ctx context.Context, upd *telegram.ChatJoinRequest, r *Request) error {
	if upd == nil {
		return nil
	}
	r.GroupID = upd.Chat.ID
	r.UserID = upd.From.ID
	r.Status = StatusPending
	return nil
}

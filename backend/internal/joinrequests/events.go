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

// HandleChatMember extrae groupID y userID de un update chat_member
// (funcion pura, testeable). Devuelve relevant=true solo cuando el
// nuevo estado es "member" (el usuario se unio al chat) y el usuario
// es no-nil — es el caso en que podemos marcar una solicitud pendiente
// como aprobada (el usuario fue aprobado directamente en Telegram).
//
// Si upd es nil o el evento no es relevante, relevant=false y los ids
// quedan en cero (el caller no hace nada).
func HandleChatMember(upd *telegram.ChatMemberUpdated) (groupID, userID int64, relevant bool) {
	if upd == nil || upd.NewChatMember.User == nil {
		return 0, 0, false
	}
	if upd.NewChatMember.Status != telegram.MemberStatusMember {
		return 0, 0, false
	}
	if upd.Chat == nil {
		return 0, 0, false
	}
	return upd.Chat.ID, upd.NewChatMember.User.ID, true
}

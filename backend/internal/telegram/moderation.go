package telegram

import (
	"context"
)

// Actions de moderacion: BanUser, UnbanUser, MuteUser, UnmuteUser,
// DeleteMessage, PinMessage, LockGroup, UnlockGroup, ApproveJoinRequest,
// RejectJoinRequest, GetChatMember y GetChatAdministrators. Todas
// pasan por doWithRetry + doPost: ante 429 el adapter espera
// retry_after y reintenta (AGENTS.md §18.1), y los errores de la Bot
// API se mapean en handleEnvelope (docs/telegram_api_reference.md §8).

// banChatMemberParams corresponde a banChatMember de la Bot API.
// RevokeMessages NO lleva omitempty: hay que enviar siempre el valor
// explicito, porque el default de Telegram es true y omitir el campo
// con false cambiaria el comportamiento.
type banChatMemberParams struct {
	ChatID         int64 `json:"chat_id"`
	UserID         int64 `json:"user_id"`
	UntilDate      int64 `json:"until_date,omitempty"`
	RevokeMessages bool  `json:"revoke_messages"`
}

// BanUser banea a un usuario. untilDate==0 => baneo indefinido
// (Telegram omite until_date).
func (a *Adapter) BanUser(ctx context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "banChatMember", banChatMemberParams{
			ChatID:         chatID,
			UserID:         userID,
			UntilDate:      untilDate,
			RevokeMessages: revokeMessages,
		}, nil)
	})
}

// unbanChatMemberParams corresponde a unbanChatMember.
type unbanChatMemberParams struct {
	ChatID       int64 `json:"chat_id"`
	UserID       int64 `json:"user_id"`
	OnlyIfBanned bool  `json:"only_if_banned"`
}

// UnbanUser desbanea. only_if_banned=true evita que Telegram falle si
// el usuario no estaba baneado.
func (a *Adapter) UnbanUser(ctx context.Context, chatID, userID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "unbanChatMember", unbanChatMemberParams{
			ChatID:       chatID,
			UserID:       userID,
			OnlyIfBanned: true,
		}, nil)
	})
}

// ChatPermissions son los permisos de envio de un miembro (campos de
// la Bot API 5.4+; can_send_media_messages ya no existe). Los campos
// "media" son *bool porque Telegram necesita que esten presentes para
// interpretar los permisos correctamente.
type ChatPermissions struct {
	CanSendMessages       *bool `json:"can_send_messages,omitempty"`
	CanSendAudios         *bool `json:"can_send_audios,omitempty"`
	CanSendDocuments      *bool `json:"can_send_documents,omitempty"`
	CanSendPhotos         *bool `json:"can_send_photos,omitempty"`
	CanSendVideos         *bool `json:"can_send_videos,omitempty"`
	CanSendVideoNotes     *bool `json:"can_send_video_notes,omitempty"`
	CanSendVoiceNotes     *bool `json:"can_send_voice_notes,omitempty"`
	CanSendPolls          *bool `json:"can_send_polls,omitempty"`
	CanSendOtherMessages  *bool `json:"can_send_other_messages,omitempty"`
	CanAddWebPagePreviews *bool `json:"can_add_web_page_previews,omitempty"`
	CanChangeInfo         *bool `json:"can_change_info,omitempty"`
	CanInviteUsers        *bool `json:"can_invite_users,omitempty"`
	CanPinMessages        *bool `json:"can_pin_messages,omitempty"`
	CanManageTopics       *bool `json:"can_manage_topics,omitempty"`
}

func boolPtr(v bool) *bool { return &v }

// mutePermissions cierra el envio por completo (todos los punteros en
// false). reutilizable via mutex/const: se construye por llamada para
// evitar mutacion accidental.
func mutePermissions() ChatPermissions {
	return ChatPermissions{
		CanSendMessages:       boolPtr(false),
		CanSendAudios:         boolPtr(false),
		CanSendDocuments:      boolPtr(false),
		CanSendPhotos:         boolPtr(false),
		CanSendVideos:         boolPtr(false),
		CanSendVideoNotes:     boolPtr(false),
		CanSendVoiceNotes:     boolPtr(false),
		CanSendPolls:          boolPtr(false),
		CanSendOtherMessages:  boolPtr(false),
		CanAddWebPagePreviews: boolPtr(false),
	}
}

// unmutePermissions abre el envio estandar: mensajes, todos los media,
// polls, otros y previews. Los campos de administracion (change_info,
// invite, pin, topics) quedan sin enviar (nil) para no tocar lo que el
// usuario ya tenia.
func unmutePermissions() ChatPermissions {
	return ChatPermissions{
		CanSendMessages:       boolPtr(true),
		CanSendAudios:         boolPtr(true),
		CanSendDocuments:      boolPtr(true),
		CanSendPhotos:         boolPtr(true),
		CanSendVideos:         boolPtr(true),
		CanSendVideoNotes:     boolPtr(true),
		CanSendVoiceNotes:     boolPtr(true),
		CanSendPolls:          boolPtr(true),
		CanSendOtherMessages:  boolPtr(true),
		CanAddWebPagePreviews: boolPtr(true),
	}
}

// restrictChatMemberParams corresponde a restrictChatMember.
type restrictChatMemberParams struct {
	ChatID      int64           `json:"chat_id"`
	UserID      int64           `json:"user_id"`
	Permissions ChatPermissions `json:"permissions"`
	UntilDate   int64           `json:"until_date,omitempty"`
}

// MuteUser restringe el envio de mensajes con permissions cerradas.
// untilDate==0 => restriccion indefinida.
func (a *Adapter) MuteUser(ctx context.Context, chatID, userID int64, untilDate int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "restrictChatMember", restrictChatMemberParams{
			ChatID:      chatID,
			UserID:      userID,
			Permissions: mutePermissions(),
			UntilDate:   untilDate,
		}, nil)
	})
}

// UnmuteUser restaura el envio con permissions abiertas y sin
// until_date (la restriccion queda por tiempo indefinido, en modo
// abierto).
func (a *Adapter) UnmuteUser(ctx context.Context, chatID, userID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "restrictChatMember", restrictChatMemberParams{
			ChatID:      chatID,
			UserID:      userID,
			Permissions: unmutePermissions(),
		}, nil)
	})
}

// deleteMessageParams corresponde a deleteMessage.
type deleteMessageParams struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

// DeleteMessage borra un mensaje (requiere can_delete_messages o ser
// creador).
func (a *Adapter) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "deleteMessage", deleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		}, nil)
	})
}

// pinChatMessageParams corresponde a pinChatMessage.
type pinChatMessageParams struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

// PinMessage fija un mensaje (requiere can_pin_messages o ser creador).
func (a *Adapter) PinMessage(ctx context.Context, chatID, messageID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "pinChatMessage", pinChatMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		}, nil)
	})
}

// setChatPermissionsParams corresponde a setChatPermissions.
type setChatPermissionsParams struct {
	ChatID      int64           `json:"chat_id"`
	Permissions ChatPermissions `json:"permissions"`
}

// LockGroup cierra el envio de mensajes del grupo: setChatPermissions
// con permissions cerradas aplica a todos los no-administradores.
func (a *Adapter) LockGroup(ctx context.Context, chatID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "setChatPermissions", setChatPermissionsParams{
			ChatID:      chatID,
			Permissions: mutePermissions(),
		}, nil)
	})
}

// UnlockGroup reabre el envio con permisos estandar.
func (a *Adapter) UnlockGroup(ctx context.Context, chatID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "setChatPermissions", setChatPermissionsParams{
			ChatID:      chatID,
			Permissions: unmutePermissions(),
		}, nil)
	})
}

// approveChatJoinRequestParams corresponde a approveChatJoinRequest.
type approveChatJoinRequestParams struct {
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
}

// ApproveJoinRequest aprueba una solicitud de ingreso (requiere
// can_invite_users; docs/telegram_api_reference.md §7).
func (a *Adapter) ApproveJoinRequest(ctx context.Context, chatID, userID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "approveChatJoinRequest", approveChatJoinRequestParams{
			ChatID: chatID,
			UserID: userID,
		}, nil)
	})
}

// RejectJoinRequest rechaza una solicitud de ingreso (requiere
// can_invite_users).
func (a *Adapter) RejectJoinRequest(ctx context.Context, chatID, userID int64) error {
	return a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "declineChatJoinRequest", approveChatJoinRequestParams{
			ChatID: chatID,
			UserID: userID,
		}, nil)
	})
}

// GetChatMember devuelve el estado del miembro en el chat. Es el
// lookup puntual del MVP: la Bot API no permite listar todos los
// miembros de un grupo.
func (a *Adapter) GetChatMember(ctx context.Context, chatID, userID int64) (ChatMember, error) {
	var result ChatMember
	err := a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "getChatMember", approveChatJoinRequestParams{
			ChatID: chatID,
			UserID: userID,
		}, &result)
	})
	return result, err
}

// GetChatAdministrators lista los administradores del chat. Junto con
// GetChatMember conforma la "lista de usuarios" que el MVP puede
// mostrar (limitacion documentada en AGENTS.md §7).
func (a *Adapter) GetChatAdministrators(ctx context.Context, chatID int64) ([]ChatMember, error) {
	var result []ChatMember
	err := a.doWithRetry(ctx, func() error {
		return a.doPost(ctx, "getChatAdministrators", struct {
			ChatID int64 `json:"chat_id"`
		}{ChatID: chatID}, &result)
	})
	return result, err
}

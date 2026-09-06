package groups

import (
	"context"

	"github.com/telegram-manager/backend/internal/telegram"
)

// tiposChatRegistrables son los chats que el sistema administra; los
// DM del bot (private) se ignoran.
var tiposChatRegistrables = map[string]bool{
	"supergroup": true,
	"group":      true,
	"channel":    true,
}

// HandleMyChatMember mapea un update my_chat_member a un Group
// (rellena g) sin tocar la base de datos (funcion pura, testeable).
//
// Reglas:
//   - Chats private o sin Chat/NewChatMember → g no cambia, err nil
//     (se ignora sin error; el bus sigue).
//   - BotStatus es SIEMPRE el status del new_chat_member (incluye
//     left/kicked: el registro conserva el ultimo estado conocido).
//   - BotPermissions se llena solo con los can_* presentes; nil si
//     Telegram no los incluyo (el bot no es admin). El upsert luego
//     preserva el valor previo cuando llega nil.
//   - MemberCount NO se toca (se obtiene en un paso posterior).
func HandleMyChatMember(ctx context.Context, upd *telegram.ChatMemberUpdated, g *Group) error {
	if upd == nil || upd.Chat == nil || !tiposChatRegistrables[upd.Chat.Type] {
		return nil
	}

	g.TelegramID = upd.Chat.ID
	g.Title = upd.Chat.Title
	g.Username = optionalString(upd.Chat.Username)
	g.Type = upd.Chat.Type
	g.BotStatus = BotStatus(upd.NewChatMember.Status)

	perms := permissionsFromMember(&upd.NewChatMember)
	if len(perms) > 0 {
		g.BotPermissions = perms
	} else {
		g.BotPermissions = nil // ausentes → no pisar valor previo en upsert
	}
	return nil
}

// permissionsFromMember copia los can_* presentes del miembro a un
// mapa; devuelve nil si Telegram no incluyo ninguno.
func permissionsFromMember(m *telegram.ChatMember) map[string]bool {
	if m == nil || m.Status != telegram.MemberStatusAdministrator {
		return nil
	}

	perms := make(map[string]bool)
	setBool(perms, "can_delete_messages", m.CanDeleteMessages)
	setBool(perms, "can_restrict_members", m.CanRestrictMembers)
	setBool(perms, "can_pin_messages", m.CanPinMessages)
	setBool(perms, "can_invite_users", m.CanInviteUsers)
	setBool(perms, "can_promote_members", m.CanPromoteMembers)
	setBool(perms, "can_change_info", m.CanChangeInfo)
	if len(perms) == 0 {
		return nil
	}
	return perms
}

func setBool(m map[string]bool, key string, v *bool) {
	if v != nil {
		m[key] = *v
	}
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

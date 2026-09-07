package moderation

import (
	"github.com/telegram-manager/backend/internal/logs"
)

// actionPermissions mapea cada accion administrativa a la
// bot_permission que el bot debe tener (tasks 2.6). La referencia de la
// Bot API corrige reject: approve y reject requieren can_invite_users
// (docs/telegram_api_reference.md §7); el design inicial decia
// can_restrict_members para reject.
//
// Reverse: el mapa de la tabla groups.bot_permissions usa las mismas
// claves can_* (poblado por la deteccion de grupos, paso 8).
var actionPermissions = map[string]string{
	logs.ActionBanUser:            "can_restrict_members",
	logs.ActionUnbanUser:          "can_restrict_members",
	logs.ActionMuteUser:           "can_restrict_members",
	logs.ActionUnmuteUser:         "can_restrict_members",
	logs.ActionLockGroup:          "can_restrict_members",
	logs.ActionUnlockGroup:        "can_restrict_members",
	logs.ActionDeleteMessage:      "can_delete_messages",
	logs.ActionPinMessage:         "can_pin_messages",
	logs.ActionApproveJoinRequest: "can_invite_users",
	logs.ActionRejectJoinRequest:  "can_invite_users",
}

// permissionFor devuelve la bot_permission requerida para la accion.
func permissionFor(action string) string {
	return actionPermissions[action]
}

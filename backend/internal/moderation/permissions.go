package moderation

import (
	"github.com/telegram-manager/backend/internal/groups"
)

// permissionOk decide si el bot puede actuar en el grupo (bugfix #172,
// invariante del slice 0): SOLO por bot_status == administrator.
//
// No se lee ninguna clave de permiso individual del mapa
// bot_permissions para decidir: la deteccion de grupos no las puebla
// completas y la Bot API no las exige para estas acciones siendo
// admin. Misma logica que publications.permissionOk y
// automation.permissionOkAdmin (copia defensiva por paquete, como en
// esos modulos, para no acoplar dominios).
func permissionOk(g *groups.Group) bool {
	return g != nil && g.BotStatus == groups.StatusAdministrator
}

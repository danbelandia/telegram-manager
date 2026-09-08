// Package logs registra la auditoria de acciones administrativas
// (AGENTS.md §11): cada accion importante sobre un grupo genera una
// fila con actor, grupo, accion, target, resultado y error. Nunca se
// guardan secretos (token del bot, passwords, JWT) en metadata.
package logs

import (
	"time"
)

// Status son los codigos de resultado de la seccion 18 del spec.
type Status string

// Codigos de resultado de una accion administrativa.
const (
	StatusSuccess          Status = "SUCCESS"
	StatusPermissionDenied Status = "PERMISSION_DENIED"
	StatusTelegramError    Status = "TELEGRAM_ERROR"
	StatusValidationError  Status = "VALIDATION_ERROR"
	StatusNotFound         Status = "NOT_FOUND"
	StatusInternalError    Status = "INTERNAL_ERROR"
)

// Acciones administrativas registradas (§11). Los eventos del sistema
// (sin admin) tambien pueden registrar acciones propias.
//
// Slice 1 (Fase 3): ActionRuleTriggered / ActionAutomuteUser /
// ActionAutobanUser se registran con ActorID=nil porque son auto-
// actions del pipeline (no las gatillo un admin desde el panel).
//
// Slice 2 agrega las 5 constantes de cambios manuales de settings y
// listas: a diferencia del slice 1, estas SI tienen ActorID (el admin
// que toco el toggle / agrego la palabra / quito el dominio desde el
// panel). La distincion manual vs auto vive en model.go §Entry:
// manual → ActorID != nil; auto → ActorID = nil.
const (
	ActionBanUser            = "BAN_USER"
	ActionUnbanUser          = "UNBAN_USER"
	ActionMuteUser           = "MUTE_USER"
	ActionUnmuteUser         = "UNMUTE_USER"
	ActionDeleteMessage      = "DELETE_MESSAGE"
	ActionPinMessage         = "PIN_MESSAGE"
	ActionLockGroup          = "LOCK_GROUP"
	ActionUnlockGroup        = "UNLOCK_GROUP"
	ActionApproveJoinRequest = "APPROVE_JOIN_REQUEST"
	ActionRejectJoinRequest  = "REJECT_JOIN_REQUEST"
	ActionPublishMessage     = "PUBLISH_MESSAGE"
	ActionRuleTriggered      = "RULE_TRIGGERED"
	ActionAutomuteUser       = "AUTOMUTE_USER"
	ActionAutobanUser        = "AUTOBAN_USER"
	// Slice 2 — moderacion automatica UI (actor = admin del panel).
	ActionUpdateAutomationSettings = "UPDATE_AUTOMATION_SETTINGS"
	ActionAddBannedWord            = "ADD_BANNED_WORD"
	ActionRemoveBannedWord         = "REMOVE_BANNED_WORD"
	ActionAddLinkAllowlist         = "ADD_LINK_ALLOWLIST"
	ActionRemoveLinkAllowlist      = "REMOVE_LINK_ALLOWLIST"
)

// Entry es una fila de auditoria. ActorID es el id del admin del panel
// que ejecuto la accion; es nil cuando la accion la genera el sistema
// (evento de Telegram). GroupID es el telegram id del grupo.
// TargetUserID es nil cuando la accion no recae sobre un usuario
// (lock/unlock/delete/pin guardan el target en Metadata cuando aplica).
type Entry struct {
	ID           int64
	ActorID      *int64
	GroupID      int64
	Action       string
	TargetUserID *int64
	Metadata     map[string]any
	Status       Status
	ErrorMessage *string
	CreatedAt    time.Time
}

// Package automation implementa el motor de moderacion automatica del
// bot (AGENTS.md §23, Fase 3 — slice 1 foundation). Detecta floods y
// otras reglas sobre cada mensaje entrante, mantiene un contador de
// advertencias por (grupo, usuario), y cuando se cruzan los thresholds
// configurados encola auto-actions (mute / ban) que un worker
// secuencial despacha via el adapter de Telegram.
//
// Invariante (bugfix #172, observation #172): el check de admin usa
// `g.BotStatus == groups.StatusAdministrator`. NO reintroducir checks
// sobre claves `can_*` — la deteccion de grupos jamas las puebla
// completas, y la Bot API no las exige para sendMessage/restrictChat
// Member/banChatMember en grupos (el estado admin basta). El check
// vive en dos puntos:
//
//  1. permissionOkAdmin() en service.go (pipeline principal).
//  2. permissionOkAdmin() en autoactioner.go (re-check entre hit y
//     dispatch por si el bot fue removido del grupo).
//
// Slice 1 es backend-only: el frontend no se toca, la verificacion se
// hace con psql + logs. Slices futuros (2, 3) agregan reglas (anti-link,
// anti-spam, banned-words) y UI.
package automation

import (
	"errors"
	"time"
)

// Errores de dominio del modulo (los handlers los mapearian a los
// codigos §18; este slice no expone handlers todavia).
var (
	// ErrNotFound: settings o warning_state inexistente.
	ErrNotFound = errors.New("automation: not found")
	// ErrAutomationGroupNotFound: el group_id de un endpoint de
	// automation no existe en la tabla groups (404 al admin).
	// Distinto de ErrNotFound: ErrNotFound es para sub-filas
	// (settings, warning_state, word); ErrAutomationGroupNotFound
	// es para el grupo de Telegram mismo.
	ErrAutomationGroupNotFound = errors.New("automation: group not found")
)

// Lists son las dos listas que el Service pre-carga una vez por
// mensaje y propaga a las reglas via Rule.Evaluate. Si el Service
// detecta que ningun toggle dependiente esta activo, pasa nil para
// ahorrar 2 queries a la DB. nil es defensivo en todas las reglas
// (que vuelven nil si lists == nil y la lista correspondiente
// estuviera vacia).
type Lists struct {
	BannedWords   []string
	LinkAllowlist []string
}

// DefaultSettings devuelve los valores por defecto del modulo de
// moderacion automatica (mismos que Service.LoadOrCreateSettings:
// todos los toggles en false, flood 5/10, warning_limit 3,
// automute 3/10min, autoban 5, expire 30d). Usado por el handler
// GET /automation/settings cuando la fila no existe: el primer PUT
// la persiste con UPSERT.
//
// Consistente con Service.LoadOrCreateSettings (model.go de
// service.go); cualquier cambio en defaults debe replicarse en
// ambos lugares.
func DefaultSettings(groupID int64) *Settings {
	return &Settings{
		GroupID:           groupID,
		Enabled:           false,
		FloodEnabled:      false,
		FloodMessages:     5,
		FloodSeconds:      10,
		WarningLimit:      3,
		AutomuteWarnings:  3,
		AutomuteMinutes:   10,
		AutobanWarnings:   5,
		WarningExpireDays: 30,
		// Slice 2.1: warning visual ON por default out-of-the-box; el
		// admin lo apaga o customiza por grupo desde el panel.
		WarnUserEnabled:  true,
		WarnUserTemplate: nil,
	}
}

// Settings es la fila de group_moderation_settings. GroupID es el
// telegram_id del grupo (id natural que usamos en todas las llamadas a
// la Bot API). UpdatedAt lo setea la DB.
//
// Slice 2.1: WarnUserEnabled (default true) + WarnUserTemplate (*string,
// default nil) controlan el warning visual al usuario antes de mute/ban
// (REQ-22..28). nil en WarnUserTemplate = usar default hardcoded en
// automation/templates.go.
type Settings struct {
	GroupID            int64
	Enabled            bool
	AntiSpamEnabled    bool
	AntiLinkEnabled    bool
	BannedWordsEnabled bool
	FloodEnabled       bool
	FloodMessages      int16
	FloodSeconds       int16
	WarningLimit       int16
	AutomuteWarnings   int16
	AutomuteMinutes    int16
	AutobanWarnings    int16
	WarningExpireDays  int16
	WarnUserEnabled    bool
	WarnUserTemplate   *string
	UpdatedAt          time.Time
}

// WarningState es la fila de user_warning_state. PK compuesta
// (GroupID, UserID). WarningCount puede llegar a SMALLINT.MAX y nunca
// baja sola: el reset vive en Service.HandleMessage cuando el estado
// expira (WarningExpireDays).
type WarningState struct {
	GroupID       int64
	UserID        int64
	WarningCount  int16
	LastWarningAt *time.Time
	LastActionAt  *time.Time
	ExpiresAt     *time.Time
}

// RuleHit es el resultado de una regla individual. RuleName es el
// identificador estable (ej. "flood"); Reason es la frase legible para
// logs y metadata.
type RuleHit struct {
	RuleName string
	Reason   string
}

// AutoActionKind clasifica la accion que el worker debe ejecutar.
type AutoActionKind string

const (
	// AutoActionMute: el usuario excedio automute_warnings; el worker
	// llama tg.MuteUser con UntilDate = now + minutes*60.
	AutoActionMute AutoActionKind = "mute"
	// AutoActionBan: el usuario excedio autoban_warnings; el worker
	// llama tg.BanUser con UntilDate=0 (indefinido) y revoke=true.
	AutoActionBan AutoActionKind = "ban"
)

// AutoAction es el payload que el Service encola y el worker despacha.
// RuleName y WarningCount se persisten en metadata del log de
// auditoria; UserID y GroupID son los ids de Telegram. Para mute,
// MinutesUntil es el offset en minutos desde "ahora" que se traduce a
// unix-time en el dispatch (UntilDate).
type AutoAction struct {
	Kind         AutoActionKind
	GroupID      int64
	UserID       int64
	MinutesUntil int16 // solo mute
	RuleName     string
	WarningCount int16
}

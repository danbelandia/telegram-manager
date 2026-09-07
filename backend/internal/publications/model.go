// Package publications modela las publicaciones del panel (AGENTS.md
// §22). Slice 1: publish-now text a un grupo. La tabla usa UN solo
// status que cubre todo el ciclo de vida (draft|scheduled|sending|
// sent|failed) y scheduled_at nullable (NULL = inmediato); asi la
// programacion (slice 3) no requiere migracion de esquema.
//
// Slice 2 agrega `photo_url` (TEXT NULL) y `buttons` (JSONB NULL): foto
// por URL + teclado inline opcional, envio multi-grupo secuencial y
// filtro por grupo en el listado. Los bytes del JSONB se almacenan
// verbatim (json.RawMessage) — la capa service se ocupa de
// (des)serializar a `[][]telegram.InlineKeyboardButton` con los helpers
// `MarshalButtons`/`UnmarshalButtons` definidos aqui.
package publications

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/telegram-manager/backend/internal/telegram"
)

// Status es el estado de una publicacion.
type Status string

// Estados posibles del ciclo de vida de una publicacion.
const (
	StatusDraft     Status = "draft"
	StatusScheduled Status = "scheduled"
	StatusSending   Status = "sending"
	StatusSent      Status = "sent"
	StatusFailed    Status = "failed"
)

// Limites operacionales (design D10 + spec REQ Validaciones payload).
// La Bot API fija caption<=1024 para sendPhoto y text<=4096 para
// sendMessage; los limites de botones (8x8) son autoimpuestos.
const (
	maxTextLength       = 4096 // sin foto
	maxCaptionLength    = 1024 // con foto (caption de sendPhoto)
	maxPhotoURLLength   = 2048
	maxButtonsRows      = 8
	maxButtonsPerRow    = 8
	maxButtonTextLength = 64
	maxButtonURLLength  = 256
	maxGroupsPerPublish = 10
)

// Publication es una publicacion. TelegramID es groups.telegram_id (id
// natural para llamadas a Telegram). MessageID se puebla en exito;
// ErrorMessage en fallo; ScheduledAt es NULL para publicacion inmediata.
// ActorID es el id del admin del panel que la creo (auditoria).
// PhotoURL es opcional (slice 2). Buttons se almacena como bytes JSONB
// raw — la conversion a `[][]telegram.InlineKeyboardButton` ocurre en
// la capa service (helper UnmarshalButtons).
type Publication struct {
	ID           int64
	TelegramID   int64
	Text         string
	Status       Status
	MessageID    *int64
	ScheduledAt  *time.Time
	ErrorMessage *string
	ActorID      *int64
	PhotoURL     *string
	Buttons      json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Errores de dominio del flujo (el handler los mapea a los codigos §18).
var (
	// ErrTextTooLong: el texto excede el limite de la Bot API (4096 sin
	// foto / 1024 con foto). El mensaje distingue ambos casos para
	// que el panel muestre el limite real.
	ErrTextEmpty   = errors.New("texto no puede estar vacio")
	ErrTextTooLong = errors.New("texto excede el limite permitido")
	// ErrPhotoURLEmpty: photo_url fue enviado como string vacio. El
	// cliente debe omitir el campo si no quiere foto.
	ErrPhotoURLEmpty = errors.New("la URL de la foto no puede estar vacia")
	// ErrPhotoURLScheme: la URL no usa esquema http/https.
	ErrPhotoURLScheme = errors.New("la URL de la foto debe empezar con http o https")
	// ErrPhotoURLTooLong: la URL excede maxPhotoURLLength.
	ErrPhotoURLTooLong = errors.New("la URL de la foto excede 2048 caracteres")
	// ErrButtonsMalformed: los botones no cumplen la estructura
	// esperada (array de filas; cada fila array de {text,url}).
	ErrButtonsMalformed = errors.New("los botones deben ser una lista de filas con texto y URL")
	// ErrButtonsLimit: se excedio 8 filas o 8 botones por fila.
	ErrButtonsLimit = errors.New("los botones exceden el limite permitido")
	// ErrButtonTextLong: text de un boton > 64 chars.
	ErrButtonTextLong = errors.New("el texto del boton excede 64 caracteres")
	// ErrButtonTextEmpty: text de un boton vacio.
	ErrButtonTextEmpty = errors.New("el texto del boton no puede estar vacio")
	// ErrButtonURLScheme: url de un boton no es http(s).
	ErrButtonURLScheme = errors.New("la URL del boton debe empezar con http o https")
	// ErrButtonURLLong: url de un boton > 256 chars.
	ErrButtonURLLong = errors.New("la URL del boton excede 256 caracteres")
	// ErrGroupsEmpty: group_ids vacio.
	ErrGroupsEmpty = errors.New("se requiere al menos un grupo")
	// ErrGroupsLimit: group_ids > 10.
	ErrGroupsLimit = errors.New("maximo 10 grupos por publicacion")
	// ErrNotFound: la publicacion no existe.
	ErrNotFound = errors.New("publications: not found")

	// Errores nuevos de slice 3:
	// ErrScheduledInPast: scheduled_at es <= now() (400 VALIDATION_ERROR).
	ErrScheduledInPast = errors.New("scheduled_at debe ser una fecha futura")
	// ErrInvalidPagination: limit<1, limit>100 o offset<0 (400 VALIDATION_ERROR).
	ErrInvalidPagination = errors.New("limit debe estar entre 1 y 100 y offset debe ser >= 0")
	// ErrCancelNotAllowed: cancelar una fila no-scheduled (409 INVALID_STATUS).
	ErrCancelNotAllowed = errors.New("no se puede cancelar una publicacion ya enviada o en curso")
)

// Limites de paginacion del listado (slice 3).
const (
	defaultListLimit = 50
	maxListLimit     = 100
)

// MarshalButtons serializa una estructura de botones ([][]InlineKeyboardButton)
// a JSONB raw. Devuelve nil si la entrada esta vacia (tratar como NULL
// en DB, no como JSONB con array vacio) — el adapter interpreta un
// reply_markup nil como "sin teclado" (omitempty).
func MarshalButtons(rows [][]telegram.InlineKeyboardButton) (json.RawMessage, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	return json.Marshal(rows)
}

// UnmarshalButtons deserializa JSONB raw a la estructura de botones.
// Si `raw` es nil o vacio, devuelve nil (sin teclado).
func UnmarshalButtons(raw json.RawMessage) ([][]telegram.InlineKeyboardButton, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var rows [][]telegram.InlineKeyboardButton
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// NormalizeScheduledAt parsea un RFC3339 con offset y normaliza a UTC.
// Acepta tanto "2027-01-01T10:00:00Z" como "2027-01-01T17:00:00+03:00".
// Si el parseo falla o el timestamp es <= now(), retorna ErrScheduledInPast
// (el handler lo mapea a 400 VALIDATION_ERROR). `nowFn` se inyecta para
// determinismo en tests; default time.Now.
func NormalizeScheduledAt(raw string, nowFn func() time.Time) (*time.Time, error) {
	if nowFn == nil {
		nowFn = time.Now
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, ErrScheduledInPast
	}
	if err := validateScheduledAt(t, nowFn); err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}

// validateScheduledAt exige que el timestamp sea estrictamente futuro.
// Tolerancia cero: igual a now() o pasado -> ErrScheduledInPast.
func validateScheduledAt(t time.Time, nowFn func() time.Time) error {
	if nowFn == nil {
		nowFn = time.Now
	}
	if !t.After(nowFn()) {
		return ErrScheduledInPast
	}
	return nil
}

// ValidatePagination valida limit/offset del listado. Fuera de rango ->
// ErrInvalidPagination (handler lo mapea a 400).
//   - limit < 1 o limit > 100 -> error.
//   - offset < 0 -> error.
//
// Si ambos son cero (caso "no se enviaron query params"), NormalizePagination
// aplica los defaults (limit=50, offset=0).
func ValidatePagination(limit, offset int) error {
	if limit < 1 || limit > maxListLimit || offset < 0 {
		return ErrInvalidPagination
	}
	return nil
}

// NormalizePagination aplica defaults a limit/offset. Si ambos son
// cero (caso "no query params"), usa defaultListLimit. Si solo limit
// es 0 (caso "solo offset"), es un 400; esa validacion la hace el
// handler antes de llamar aca, asi que aqui nunca llega ese caso.
func NormalizePagination(limit, offset int) (int, int) {
	if limit == 0 && offset == 0 {
		return defaultListLimit, 0
	}
	return limit, offset
}

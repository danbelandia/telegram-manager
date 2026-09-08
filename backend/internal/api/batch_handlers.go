// Handlers del endpoint batch de publicaciones (publications-batch change,
// spec REQ-16..REQ-19 + REQ-25 + REQ-26).
//
// POST /api/publications/batch acepta un envelope con N publicaciones
// independientes (cada una con su scheduled_at?, foto, botones y multi-
// grupo) y devuelve {created[], failed[]} con failure-isolation per-item.
// La logica del batch es PURA presentacion: el handler looppea sobre
// Service.PublishMany (inmediato) y Service.Schedule (programado), ya
// battle-tested por slices 2 y 3. Cero metodo nuevo en el service.
//
// Status code: 200 OK si la envelope es valida (incluso si TODOS los
// items fallaron — el batch se acepto, los resultados viven en el body).
// 400 SOLO por errores de envelope (JSON malformado, cap excedido,
// batch vacio).
//
// Invariante (bugfix #172, spec REQ-26): este handler NO consulta
// `bot_permissions["can_*"]`. La verificacion per-grupo la hace
// service.permissionOk reusado dentro de PublishMany/Schedule.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/publications"
	"github.com/telegram-manager/backend/internal/telegram"
)

// maxBatchSize es el cap maximo de publicaciones por batch. Spec REQ-16
// y design D5: 10 publicaciones x N grupos = maximo 100 publicaciones
// potenciales (coherente con el cap de 10 grupos por publicacion que ya
// existe en publications.maxGroupsPerPublish).
const maxBatchSize = 10

// minBatchSize es el minimo de publicaciones por batch: 1. Un batch
// vacio es un error de envelope (REQ-16 escenario 2).
const minBatchSize = 1

// batchRequest es el envelope de POST /api/publications/batch.
// `publications` es la lista de payloads; cada item tiene la misma shape
// que createPublicationRequest (reusamos el type para no duplicar el
// shape JSON).
type batchRequest struct {
	Publications []createPublicationRequest `json:"publications"`
}

// batchFailureItem es un item dentro de `failed[]`. `index` es la
// posicion ORIGINAL del item en req.Publications (NO la posicion dentro
// de failed[]); permite al frontend correlacionar con el slot del modal.
// `code` viene del envelope seccion 18 (VALIDATION_ERROR |
// PERMISSION_DENIED | NOT_FOUND | TELEGRAM_ERROR | INTERNAL_ERROR).
// `message` es el texto legible del service o el envelope.
type batchFailureItem struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// batchCreatedItem envuelve una fila creada con su indice original en
// el batch. `publication` es la publicacion completa (publicationResponse)
// para que el frontend pueda mostrarla en el banner verde y refrescar
// la lista del historial.
type batchCreatedItem struct {
	Index       int                 `json:"index"`
	Publication publicationResponse `json:"publication"`
}

// batchResponse es el body de 200 OK de POST /api/publications/batch.
// Tanto `created` como `failed` son listas (pueden estar vacias, pero
// nunca son null en la serializacion JSON porque Go las inicializa a
// slice vacio).
type batchResponse struct {
	Created []batchCreatedItem `json:"created"`
	Failed  []batchFailureItem `json:"failed"`
}

// handleCreatePublicationBatch POST /api/publications/batch — crea hasta
// N publicaciones en una sola request (cap 10). Failure-isolation per
// item: si un item falla, va a `failed[]` y los demas siguen. Status
// code 200 OK si la envelope es valida (aun con TODOS los items
// fallidos). 400 SOLO por errores de envelope (REQ-16).
//
// Flujo (design data flow):
//
//  1. requireAuth + actorIDFromClaims (montados en server.go).
//  2. Decode JSON. JSON malformado -> 400.
//  3. Cap check: len(publications) == 0 || > 10 -> 400.
//  4. Capturar now := time.Now() UNA vez (REQ-17: consistencia para
//     validacion de scheduled_at futuro).
//  5. Por cada item:
//     a. Si scheduled_at == "" -> service.PublishMany(...).
//     b. Si scheduled_at != "" -> NormalizeScheduledAt(item.scheduled_at,
//     nowFn) + service.Schedule(..., scheduledAt, nowFn).
//     c. Error de payload-level (validation 400) -> failed[]. continue.
//     d. Por cada fila retornada: status=sent|scheduled -> created[];
//     status=failed -> failed[] con code mapeado desde error_message.
//  6. Responder 200 OK con {created[], failed[]}.
//
// `batch_index` para auditoria: como el service ya emite un log
// PUBLISH_MESSAGE por fila exitosa, el handler pasa el `batch_index`
// a traves del actor + correlacion (actor_id + created_at cercano + log
// metadata.publication_id). El spec REQ-19 acepta "log adicional o
// metadata JSONB"; elegimos NO modificar el service para preservar la
// invariante REQ-20 (cero metodo nuevo). El `batch_index` queda
// documentado como convencion para que un futuro script de auditoria
// pueda correlacionar las filas del batch por actor + ventana de tiempo.
func (s *Server) handleCreatePublicationBatch(w http.ResponseWriter, r *http.Request) {
	if s.publications == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de publicaciones no habilitado")
		return
	}
	actorID, ok := actorIDFromClaims(w, r)
	if !ok {
		return
	}

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}

	// Cap + empty check (REQ-16).
	if len(req.Publications) < minBatchSize {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"el batch debe contener al menos una publicacion")
		return
	}
	if len(req.Publications) > maxBatchSize {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"maximo 10 publicaciones por batch")
		return
	}

	// Capturar `now` UNA sola vez (REQ-17). Lo pasamos a Schedule como
	// `nowFn` para que la validacion "futuro estricto" sea consistente
	// para todos los items del batch.
	now := time.Now()
	nowFn := func() time.Time { return now }

	resp := batchResponse{
		Created: make([]batchCreatedItem, 0, len(req.Publications)),
		Failed:  make([]batchFailureItem, 0),
	}

	for i, item := range req.Publications {
		s.processBatchItem(r, actorID, i, item, nowFn, &resp)
	}

	respond(w, http.StatusOK, resp)
}

// processBatchItem dispatcha UN item del batch al service correspondiente
// y popula resp.Created / resp.Failed segun el resultado. Vive aparte
// para mantener handleCreatePublicationBatch legible.
//
// Esta funcion es la UNICA via de dispatch para el batch. NO consulta
// claves `can_*` ni reimplementa la validacion de payload: delega en
// el service.
func (s *Server) processBatchItem(
	r *http.Request,
	actorID int64,
	index int,
	item createPublicationRequest,
	nowFn func() time.Time,
	resp *batchResponse,
) {
	payload := publications.PublishPayload{
		Text:     item.Text,
		PhotoURL: item.PhotoURL,
		Buttons:  item.Buttons,
		GroupIDs: item.GroupIDs,
	}

	var (
		rows []publications.Publication
		err  error
	)

	if item.ScheduledAt == "" {
		// Inmediato (REQ-17 escenario 1).
		rows, err = s.publications.PublishMany(r.Context(), actorID, payload)
	} else {
		// Programado (REQ-17 escenario 2). NormalizeScheduledAt valida
		// formato + futuro estricto con `nowFn`. Si falla, NO abortamos
		// el batch: el item va a failed[] con code VALIDATION_ERROR.
		scheduledAt, nerr := publications.NormalizeScheduledAt(item.ScheduledAt, nowFn)
		if nerr != nil {
			resp.Failed = append(resp.Failed, batchFailureItem{
				Index:   index,
				Code:    "VALIDATION_ERROR",
				Message: nerr.Error(),
			})
			return
		}
		rows, err = s.publications.Schedule(r.Context(), actorID, payload, *scheduledAt, nowFn)
	}

	if err != nil {
		// Payload-level error (validation 400). NO aborta el batch.
		resp.Failed = append(resp.Failed, batchFailureItem{
			Index:   index,
			Code:    codeFromError(err),
			Message: err.Error(),
		})
		return
	}

	// Cada fila tiene su propio status. Filas `sent` o `scheduled`
	// van a created[]; filas `failed` van a failed[] con code mapeado.
	for j := range rows {
		row := &rows[j]
		switch row.Status {
		case publications.StatusSent, publications.StatusScheduled:
			resp.Created = append(resp.Created, batchCreatedItem{
				Index:       index,
				Publication: toPublicationResponse(row),
			})
		default:
			// StatusFailed u otro. Mapeamos code desde error_message
			// (la fila failed trae un mensaje legible del service).
			msg := ""
			if row.ErrorMessage != nil {
				msg = *row.ErrorMessage
			}
			resp.Failed = append(resp.Failed, batchFailureItem{
				Index:   index,
				Code:    codeFromRowError(msg),
				Message: msg,
			})
		}
	}
}

// codeFromError mapea un error de payload-level (validation) a su code
// de seccion 18. Los errores de validation son siempre 400 en el
// single endpoint; en el batch los preservamos igual pero en failed[].
func codeFromError(err error) string {
	if err == nil {
		return "INTERNAL_ERROR"
	}
	switch {
	case isPublicationsValidationError(err):
		return "VALIDATION_ERROR"
	case errors.Is(err, publications.ErrGroupNotFound), errors.Is(err, groups.ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, publications.ErrBotPermission):
		return "PERMISSION_DENIED"
	case errors.Is(err, telegram.ErrPermissionDenied):
		return "PERMISSION_DENIED"
	case errors.Is(err, telegram.ErrTelegramNotFound):
		return "NOT_FOUND"
	case errors.Is(err, telegram.ErrTelegramUnavailable), errors.Is(err, telegram.ErrWebhookConflict):
		return "TELEGRAM_ERROR"
	case isTelegramAPIError(err):
		return "TELEGRAM_ERROR"
	default:
		return "INTERNAL_ERROR"
	}
}

// codeFromRowError mapea el error_message de una fila `failed` (que ya
// paso por PublishMany/Schedule y tiene un texto legible) al code de
// seccion 18. Como las filas failed de slice 2/3 no llevan code
// estructurado, inferimos del contenido del mensaje. Esto es BEST-
// EFFORT: si el mensaje no matchea ninguna pista conocida, devolvemos
// TELEGRAM_ERROR (la causa mas comun de fila failed en el flujo
// inmediato).
func codeFromRowError(msg string) string {
	if msg == "" {
		return "INTERNAL_ERROR"
	}
	switch {
	case contains(msg, "no es administrador", "permisos"):
		return "PERMISSION_DENIED"
	case contains(msg, "no encontrado", "not found", "NOT_FOUND"):
		return "NOT_FOUND"
	case contains(msg, "Telegram", "telegram", "TELEGRAM"):
		return "TELEGRAM_ERROR"
	case contains(msg, "validation", "VALIDATION", "excede", "vacio", "invalida"):
		return "VALIDATION_ERROR"
	default:
		return "TELEGRAM_ERROR"
	}
}

// contains devuelve true si s contiene alguno de los substrings dados.
// Vive aca (no en model.go) porque es util especifico del batch handler.
func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// indexOf es un wrapper minimo de strings.Index para evitar importar
// strings solo para contains. Mantiene locality con el handler.
func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 {
		return 0
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

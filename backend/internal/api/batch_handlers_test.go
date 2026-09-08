// Tests del endpoint batch de publicaciones (publications-batch change,
// spec REQ-16/17/18/25/26). 8+ casos cubriendo cap, vacio, JSON mal
// formado, todo-success, todo-fail, mixto, mix sched+inmediato, auth
// ausente, boundary 10, multi-grupo per-item.
//
// Mocking strategy (§21.1): extendemos fakePublicationStore existente
// con contadores separados publishManyCalls / scheduleCalls para poder
// verificar que el handler dispatcha correctamente. Cero llamadas
// reales a Bot API.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/publications"
	"github.com/telegram-manager/backend/internal/telegram"
)

// batchFakeStore extiende fakePublicationStore con contadores separados
// para PublishMany y Schedule. Asi el handler puede verificar que el
// dispatch fue correcto por item (inmediato vs programado).
//
// publishManyFunc y scheduleFunc permiten inyectar el comportamiento
// por-item (ej: item[0] devuelve filas sent, item[1] devuelve filas
// failed). Si no se setean, usan el comportamiento default del fake
// base (pub o list, scheduleList o pub).
type batchFakeStore struct {
	*fakePublicationStore
	publishManyCalls int
	scheduleCalls    int

	// publishManyFunc sobreescribe el comportamiento default; permite
	// distinguir items por payload (ej: text especifico).
	publishManyFunc func(payload publications.PublishPayload) ([]publications.Publication, error)
	// scheduleFunc sobreescribe Schedule; idem.
	scheduleFunc func(payload publications.PublishPayload, scheduledAt time.Time) ([]publications.Publication, error)
}

// newBatchFakeStore construye un batchFakeStore con un fakePublicationStore
// subyacente. Mantiene la interface publicationStore (Publish, PublishMany,
// Schedule, GetByID, List, ListByTelegramID, CancelScheduled) para que
// pueda inyectarse en WithPublications sin cambios.
func newBatchFakeStore() *batchFakeStore {
	return &batchFakeStore{
		fakePublicationStore: &fakePublicationStore{},
	}
}

// PublishMany: incrementa publishManyCalls + actor + payload, y delega
// a publishManyFunc si esta seteada. Si no, cae al default del fake
// base (f.pub o f.list).
func (b *batchFakeStore) PublishMany(_ context.Context, _ int64, actorID int64, payload publications.PublishPayload) ([]publications.Publication, error) {
	b.publishManyCalls++
	b.actor = actorID
	b.payload = payload
	if b.err != nil {
		return nil, b.err
	}
	if b.publishManyFunc != nil {
		return b.publishManyFunc(payload)
	}
	if b.pub != nil {
		return []publications.Publication{*b.pub}, nil
	}
	return b.list, nil
}

// Schedule: incrementa scheduleCalls + actor + payload + scheduledAt, y
// delega a scheduleFunc si esta seteada.
func (b *batchFakeStore) Schedule(_ context.Context, _ int64, actorID int64, payload publications.PublishPayload, scheduledAt time.Time, nowFn func() time.Time) ([]publications.Publication, error) {
	b.scheduleCalls++
	b.actor = actorID
	b.payload = payload
	b.lastScheduledAt = scheduledAt
	if b.scheduleErr != nil {
		return nil, b.scheduleErr
	}
	if b.scheduleFunc != nil {
		return b.scheduleFunc(payload, scheduledAt)
	}
	if b.scheduleList != nil {
		return b.scheduleList, nil
	}
	if b.pub != nil {
		return []publications.Publication{*b.pub}, nil
	}
	return []publications.Publication{}, nil
}

// buildBatchServer construye un Server con auth fija + modulo de
// publicaciones habilitado (fake batch inyectado).
func buildBatchServer(t *testing.T, store *batchFakeStore, opts ...Option) *Server {
	t.Helper()
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithPublications(store),
	)
	for _, o := range opts {
		o(server)
	}
	return server
}

// --- Tests del batch handler ---

// TestBatch_RequireAuth: POST /api/publications/batch sin Authorization
// -> 401 (REQ-16 escenario 4).
func TestBatch_RequireAuth(t *testing.T) {
	store := newBatchFakeStore()
	server := buildBatchServer(t, store)

	rr := doRequest(server, "POST", "/api/publications/batch",
		`{"publications":[{"text":"hola","group_ids":[-1001]}]}`, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401 (body: %s)", rr.Code, rr.Body.String())
	}
	if store.publishManyCalls != 0 || store.scheduleCalls != 0 {
		t.Errorf("handler no debio dispatchar: pubMany=%d sched=%d",
			store.publishManyCalls, store.scheduleCalls)
	}
}

// TestBatch_CapExceeded: 11 items -> 400 VALIDATION_ERROR (REQ-16
// escenario 1) sin dispatchar al service.
func TestBatch_CapExceeded(t *testing.T) {
	store := newBatchFakeStore()
	server := buildBatchServer(t, store)

	items := make([]map[string]any, 11)
	for i := range items {
		items[i] = map[string]any{
			"text":      "hola",
			"group_ids": []int64{-1001},
		}
	}
	body, _ := json.Marshal(map[string]any{"publications": items})

	rr := doRequest(server, "POST", "/api/publications/batch", string(body), validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "10 publicaciones") {
		t.Errorf("body sin mensaje de cap: %s", rr.Body.String())
	}
	if store.publishManyCalls != 0 || store.scheduleCalls != 0 {
		t.Errorf("handler debio abortar antes de dispatchar: pubMany=%d sched=%d",
			store.publishManyCalls, store.scheduleCalls)
	}
}

// TestBatch_CapBoundary: exactamente 10 items -> 200 OK (REQ-16 cap en
// el boundary, inclusive).
func TestBatch_CapBoundary(t *testing.T) {
	future := time.Date(2027, 1, 1, 10, 0, 0, 0, time.UTC)
	mid := int64(1)
	store := newBatchFakeStore()
	// Cada item (inmediato) genera 1 fila sent.
	store.publishManyFunc = func(_ publications.PublishPayload) ([]publications.Publication, error) {
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: "x", Status: publications.StatusSent,
			MessageID: &mid, ActorID: int64Ptr(1),
		}}, nil
	}
	_ = future
	server := buildBatchServer(t, store)

	items := make([]map[string]any, 10)
	for i := range items {
		items[i] = map[string]any{
			"text":      "x",
			"group_ids": []int64{-1001},
		}
	}
	body, _ := json.Marshal(map[string]any{"publications": items})

	rr := doRequest(server, "POST", "/api/publications/batch", string(body), validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if store.publishManyCalls != 10 {
		t.Errorf("publishManyCalls = %d, want 10", store.publishManyCalls)
	}
}

// TestBatch_Empty: publications:[] -> 400 VALIDATION_ERROR (REQ-16
// escenario 2).
func TestBatch_Empty(t *testing.T) {
	store := newBatchFakeStore()
	server := buildBatchServer(t, store)

	rr := doRequest(server, "POST", "/api/publications/batch",
		`{"publications":[]}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "al menos una publicacion") {
		t.Errorf("body sin mensaje de batch vacio: %s", rr.Body.String())
	}
}

// TestBatch_MalformedJSON: body no JSON -> 400 VALIDATION_ERROR.
func TestBatch_MalformedJSON(t *testing.T) {
	store := newBatchFakeStore()
	server := buildBatchServer(t, store)

	rr := doRequest(server, "POST", "/api/publications/batch",
		`{not json`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if store.publishManyCalls != 0 || store.scheduleCalls != 0 {
		t.Errorf("handler debio abortar antes de dispatchar")
	}
}

// TestBatch_AllSuccess: 3 items todos validos -> 200 con created[] de
// 3 entries y failed[] vacio (REQ-16 escenario general).
func TestBatch_AllSuccess(t *testing.T) {
	mid := int64(1)
	store := newBatchFakeStore()
	store.publishManyFunc = func(_ publications.PublishPayload) ([]publications.Publication, error) {
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: "x", Status: publications.StatusSent,
			MessageID: &mid, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[
		{"text":"a","group_ids":[-1001]},
		{"text":"b","group_ids":[-1002]},
		{"text":"c","group_ids":[-1003]}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if store.publishManyCalls != 3 {
		t.Errorf("publishManyCalls = %d, want 3", store.publishManyCalls)
	}

	var resp struct {
		Data batchResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Created) != 3 {
		t.Errorf("created = %d, want 3", len(resp.Data.Created))
	}
	if len(resp.Data.Failed) != 0 {
		t.Errorf("failed = %d, want 0 (body: %s)", len(resp.Data.Failed), rr.Body.String())
	}
	// Verificar indices preservados (0, 1, 2).
	for i, c := range resp.Data.Created {
		if c.Index != i {
			t.Errorf("created[%d].index = %d, want %d", i, c.Index, i)
		}
	}
}

// TestBatch_AllFail: 3 items todos invalidos (texto vacio, foto mal,
// grupo inexistente) -> 200 con failed[] de 3 y created[] vacio.
// Status 200 (no 4xx) porque la envelope es valida; el fallo es
// per-item (REQ-16 escenario 3 + design D7).
func TestBatch_AllFail(t *testing.T) {
	store := newBatchFakeStore()
	store.publishManyFunc = func(p publications.PublishPayload) ([]publications.Publication, error) {
		if p.Text == "" {
			return nil, publications.ErrTextEmpty
		}
		if p.PhotoURL != nil && *p.PhotoURL == "ftp://x" {
			return nil, publications.ErrPhotoURLScheme
		}
		return nil, publications.ErrGroupNotFound
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[
		{"text":"","group_ids":[-1001]},
		{"text":"x","photo_url":"ftp://x","group_ids":[-1001]},
		{"text":"y","group_ids":[-999]}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data batchResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Created) != 0 {
		t.Errorf("created = %d, want 0", len(resp.Data.Created))
	}
	if len(resp.Data.Failed) != 3 {
		t.Fatalf("failed = %d, want 3 (body: %s)", len(resp.Data.Failed), rr.Body.String())
	}
	// Verificar codes por item.
	if resp.Data.Failed[0].Code != "VALIDATION_ERROR" {
		t.Errorf("failed[0].code = %s, want VALIDATION_ERROR", resp.Data.Failed[0].Code)
	}
	if resp.Data.Failed[1].Code != "VALIDATION_ERROR" {
		t.Errorf("failed[1].code = %s, want VALIDATION_ERROR", resp.Data.Failed[1].Code)
	}
	if resp.Data.Failed[2].Code != "NOT_FOUND" {
		t.Errorf("failed[2].code = %s, want NOT_FOUND", resp.Data.Failed[2].Code)
	}
	// Verificar indices preservados.
	for i, f := range resp.Data.Failed {
		if f.Index != i {
			t.Errorf("failed[%d].index = %d, want %d", i, f.Index, i)
		}
	}
}

// TestBatch_MixedImmediateAndScheduled: 3 items: 1 inmediato, 1
// programado futuro, 1 programado pasado -> 200 con dispatch correcto
// a PublishMany (1) y Schedule (1); el pasado va a failed[] con
// VALIDATION_ERROR (REQ-17 escenario 3 + design AD2).
func TestBatch_MixedImmediateAndScheduled(t *testing.T) {
	future := time.Date(2027, 1, 1, 10, 0, 0, 0, time.UTC)
	mid := int64(1)
	store := newBatchFakeStore()
	store.publishManyFunc = func(_ publications.PublishPayload) ([]publications.Publication, error) {
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: "now", Status: publications.StatusSent,
			MessageID: &mid, ActorID: int64Ptr(1),
		}}, nil
	}
	store.scheduleFunc = func(_ publications.PublishPayload, _ time.Time) ([]publications.Publication, error) {
		return []publications.Publication{{
			ID: 2, TelegramID: -1002, Text: "future", Status: publications.StatusScheduled,
			ScheduledAt: &future, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[
		{"text":"a","group_ids":[-1001]},
		{"text":"b","group_ids":[-1002],"scheduled_at":"2027-01-01T10:00:00Z"},
		{"text":"c","group_ids":[-1003],"scheduled_at":"2020-01-01T00:00:00Z"}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if store.publishManyCalls != 1 {
		t.Errorf("publishManyCalls = %d, want 1", store.publishManyCalls)
	}
	if store.scheduleCalls != 1 {
		t.Errorf("scheduleCalls = %d, want 1", store.scheduleCalls)
	}

	var resp struct {
		Data batchResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Created) != 2 {
		t.Errorf("created = %d, want 2 (1 sent + 1 scheduled)", len(resp.Data.Created))
	}
	if len(resp.Data.Failed) != 1 {
		t.Fatalf("failed = %d, want 1 (body: %s)", len(resp.Data.Failed), rr.Body.String())
	}
	if resp.Data.Failed[0].Index != 2 {
		t.Errorf("failed[0].index = %d, want 2", resp.Data.Failed[0].Index)
	}
	if resp.Data.Failed[0].Code != "VALIDATION_ERROR" {
		t.Errorf("failed[0].code = %s, want VALIDATION_ERROR", resp.Data.Failed[0].Code)
	}
}

// TestBatch_MultiGroupPerItem: item con group_ids=[g1,g2] -> 2 filas en
// created[] (una por grupo), mismo index. Esto valida que el handler NO
// pierde filas en el flatten de PublishMany (REQ-17 escenario general).
func TestBatch_MultiGroupPerItem(t *testing.T) {
	mid := int64(1)
	store := newBatchFakeStore()
	store.publishManyFunc = func(p publications.PublishPayload) ([]publications.Publication, error) {
		// Devolvemos N filas segun group_ids (uno por grupo).
		out := make([]publications.Publication, 0, len(p.GroupIDs))
		for _, gid := range p.GroupIDs {
			out = append(out, publications.Publication{
				ID: int64(len(out) + 1), TelegramID: gid, Text: p.Text,
				Status: publications.StatusSent, MessageID: &mid, ActorID: int64Ptr(1),
			})
		}
		return out, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[
		{"text":"multi","group_ids":[-1001,-1002]}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data batchResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Created) != 2 {
		t.Errorf("created = %d, want 2 (una por grupo)", len(resp.Data.Created))
	}
	for _, c := range resp.Data.Created {
		if c.Index != 0 {
			t.Errorf("created.index = %d, want 0 (mismo item del batch)", c.Index)
		}
	}
}

// TestBatch_PerItemFailureIsolation: 3 items, item[1] falla, los demas
// siguen -> created=2, failed=1 (con index=1). Verifica que el fallo
// per-item NO aborta el batch (REQ-17 escenario general).
func TestBatch_PerItemFailureIsolation(t *testing.T) {
	mid := int64(1)
	store := newBatchFakeStore()
	store.publishManyFunc = func(p publications.PublishPayload) ([]publications.Publication, error) {
		if p.Text == "invalid" {
			return nil, publications.ErrTextEmpty
		}
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: p.Text, Status: publications.StatusSent,
			MessageID: &mid, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[
		{"text":"ok1","group_ids":[-1001]},
		{"text":"invalid","group_ids":[-1001]},
		{"text":"ok2","group_ids":[-1002]}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data batchResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Created) != 2 {
		t.Errorf("created = %d, want 2", len(resp.Data.Created))
	}
	if len(resp.Data.Failed) != 1 {
		t.Fatalf("failed = %d, want 1 (body: %s)", len(resp.Data.Failed), rr.Body.String())
	}
	if resp.Data.Failed[0].Index != 1 {
		t.Errorf("failed.index = %d, want 1", resp.Data.Failed[0].Index)
	}
	// Indices de created: 0 y 2 (saltamos 1).
	if resp.Data.Created[0].Index != 0 {
		t.Errorf("created[0].index = %d, want 0", resp.Data.Created[0].Index)
	}
	if resp.Data.Created[1].Index != 2 {
		t.Errorf("created[1].index = %d, want 2", resp.Data.Created[1].Index)
	}
}

// TestBatch_ScheduledAllFutureDispatch: 3 items todos programados a
// futuro -> 3 filas en created[] con status=scheduled, 0 failed.
// Verifica que el path Schedule es independiente y consistente.
func TestBatch_ScheduledAllFutureDispatch(t *testing.T) {
	store := newBatchFakeStore()
	store.scheduleFunc = func(p publications.PublishPayload, sa time.Time) ([]publications.Publication, error) {
		utc := sa.UTC()
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: p.Text, Status: publications.StatusScheduled,
			ScheduledAt: &utc, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[
		{"text":"a","group_ids":[-1001],"scheduled_at":"2027-06-15T10:00:00Z"},
		{"text":"b","group_ids":[-1002],"scheduled_at":"2027-06-15T11:00:00Z"},
		{"text":"c","group_ids":[-1003],"scheduled_at":"2027-06-15T12:00:00Z"}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if store.scheduleCalls != 3 {
		t.Errorf("scheduleCalls = %d, want 3", store.scheduleCalls)
	}
	if store.publishManyCalls != 0 {
		t.Errorf("publishManyCalls = %d, want 0", store.publishManyCalls)
	}
}

// TestBatch_RowFailedPopulatesFailedArray: el service retorna una fila
// con status=failed -> el handler la mueve a failed[] (no a created[]).
func TestBatch_RowFailedPopulatesFailedArray(t *testing.T) {
	store := newBatchFakeStore()
	msg := "el bot no es administrador del grupo"
	store.publishManyFunc = func(_ publications.PublishPayload) ([]publications.Publication, error) {
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: "x", Status: publications.StatusFailed,
			ErrorMessage: &msg, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[{"text":"x","group_ids":[-1001]}]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data batchResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Created) != 0 {
		t.Errorf("created = %d, want 0", len(resp.Data.Created))
	}
	if len(resp.Data.Failed) != 1 {
		t.Fatalf("failed = %d, want 1", len(resp.Data.Failed))
	}
	if resp.Data.Failed[0].Code != "PERMISSION_DENIED" {
		t.Errorf("failed[0].code = %s, want PERMISSION_DENIED", resp.Data.Failed[0].Code)
	}
}

// TestBatch_NoCanChecksInvariant: el handler NO contiene referencias a
// claves `can_*`. Este test es estático (lee el .go del disco): si
// alguien las agrega por error, el test falla (REQ-26 invariante
// bugfix #172). El check es defensivo: cubre cualquier `can_*`
// conocida del Bot API.
func TestBatch_NoCanChecksInvariant(t *testing.T) {
	data, err := osReadFile("batch_handlers.go")
	if err != nil {
		t.Fatalf("read batch_handlers.go: %v", err)
	}
	src := string(data)
	banned := []string{
		"can_post_messages",
		"can_edit_messages",
		"can_delete_messages",
		"can_manage_chat",
		"can_pin_messages",
		"can_invite_users",
		"can_promote_members",
		"can_change_info",
		"can_restrict_members",
	}
	for _, k := range banned {
		if strings.Contains(src, k) {
			t.Errorf("batch_handlers.go contiene la clave prohibida %q (bugfix #172)", k)
		}
	}
}

// osReadFile wrappea os.ReadFile con una indireccion para que el
// import de os quede localizado a este archivo de test.
func osReadFile(name string) ([]byte, error) {
	return osReadFileImpl(name)
}

func osReadFileImpl(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Aseguramos que el helper de fechas del service (publications.NormalizeScheduledAt)
// no rompe el handler. Este test es defensivo: confirma que el batch
// acepta scheduled_at con offset y lo dispatcha al service ya
// normalizado a UTC (validacion interna del service).
func TestBatch_ScheduledWithOffset_NormalizesToUTC(t *testing.T) {
	store := newBatchFakeStore()
	var captured time.Time
	store.scheduleFunc = func(p publications.PublishPayload, sa time.Time) ([]publications.Publication, error) {
		captured = sa
		utc := sa.UTC()
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: p.Text, Status: publications.StatusScheduled,
			ScheduledAt: &utc, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	// Offset +03:00 -> UTC 14:00.
	body := `{"publications":[
		{"text":"x","group_ids":[-1001],"scheduled_at":"2027-06-15T17:00:00+03:00"}
	]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if captured.Location() != time.UTC {
		t.Errorf("scheduledAt.location = %v, want UTC (normalizado)", captured.Location())
	}
	want := time.Date(2027, 6, 15, 14, 0, 0, 0, time.UTC)
	if !captured.Equal(want) {
		t.Errorf("scheduledAt = %v, want %v", captured, want)
	}
}

// TestBatch_PayloadPreservedToService: el handler pasa el payload al
// service verbatim (text, photo_url, buttons, group_ids, scheduled_at).
func TestBatch_PayloadPreservedToService(t *testing.T) {
	mid := int64(1)
	store := newBatchFakeStore()
	var seenPayload publications.PublishPayload
	store.publishManyFunc = func(p publications.PublishPayload) ([]publications.Publication, error) {
		seenPayload = p
		return []publications.Publication{{
			ID: 1, TelegramID: -1001, Text: p.Text, Status: publications.StatusSent,
			MessageID: &mid, ActorID: int64Ptr(1),
		}}, nil
	}
	server := buildBatchServer(t, store)

	body := `{"publications":[{
		"text":"hola",
		"photo_url":"https://example.com/x.jpg",
		"buttons":[[{"text":"Ir","url":"https://example.com"}]],
		"group_ids":[-1001,-1002]
	}]}`
	rr := doRequest(server, "POST", "/api/publications/batch", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if seenPayload.Text != "hola" {
		t.Errorf("text = %q, want hola", seenPayload.Text)
	}
	if seenPayload.PhotoURL == nil || *seenPayload.PhotoURL != "https://example.com/x.jpg" {
		t.Errorf("photoURL = %v, want https://example.com/x.jpg", seenPayload.PhotoURL)
	}
	if len(seenPayload.Buttons) != 1 || len(seenPayload.Buttons[0]) != 1 {
		t.Errorf("buttons = %v, want 1 fila de 1 boton", seenPayload.Buttons)
	}
	if len(seenPayload.GroupIDs) != 2 {
		t.Errorf("groupIDs = %v, want 2 entries", seenPayload.GroupIDs)
	}
}

// Sanity check: importamos telegram para que el package compense sus
// tipos referenciados por fakePublicationStore.
var _ = telegram.InlineKeyboardButton{}

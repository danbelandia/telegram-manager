package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/publications"
	"github.com/telegram-manager/backend/internal/telegram"
)

// fakePublicationStore satisface publicationStore para los handlers.
// Slice 2: agrega PublishMany + ListByTelegramID.
// Slice 3: limit/offset en List/ListByTelegramID; CancelScheduled.
type fakePublicationStore struct {
	pub       *publications.Publication
	list      []publications.Publication
	listByGid map[int64][]publications.Publication
	err       error

	// campos para verificar que el handler llamo con los parametros
	// esperados.
	calls      int
	actor      int64
	payload    publications.PublishPayload
	lastGidQ   int64
	gotQ       bool
	lastLimit  int
	lastOffset int

	// cancel registra la ultima cancelacion solicitada.
	cancelCalls []int64
	cancelErr   error

	// schedule slice 3.
	scheduleList    []publications.Publication
	scheduleErr     error
	lastScheduledAt time.Time
}

func (f *fakePublicationStore) Publish(_ context.Context, tenantID, actorID, groupID int64, text string, photoURL *string, buttons [][]telegram.InlineKeyboardButton) (*publications.Publication, error) {
	f.calls++
	f.actor = actorID
	f.payload = publications.PublishPayload{
		Text:     text,
		PhotoURL: photoURL,
		Buttons:  buttons,
		GroupIDs: []int64{groupID},
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.pub, nil
}

func (f *fakePublicationStore) PublishMany(_ context.Context, tenantID, actorID int64, payload publications.PublishPayload) ([]publications.Publication, error) {
	f.calls++
	f.actor = actorID
	f.payload = payload
	if f.err != nil {
		return nil, f.err
	}
	if f.pub != nil {
		return []publications.Publication{*f.pub}, nil
	}
	return f.list, nil
}

// Schedule (slice 3): si scheduleErr esta set, lo retorna. Si
// scheduleList esta poblado, lo devuelve. Si no, devuelve una sola
// fila copiando f.pub (modo simple).
func (f *fakePublicationStore) Schedule(_ context.Context, tenantID, actorID int64, payload publications.PublishPayload, scheduledAt time.Time, nowFn func() time.Time) ([]publications.Publication, error) {
	f.calls++
	f.actor = actorID
	f.payload = payload
	f.lastScheduledAt = scheduledAt
	if f.scheduleErr != nil {
		return nil, f.scheduleErr
	}
	if f.scheduleList != nil {
		return f.scheduleList, nil
	}
	if f.pub != nil {
		return []publications.Publication{*f.pub}, nil
	}
	return []publications.Publication{}, nil
}

func (f *fakePublicationStore) GetByID(_ context.Context, _ int64, id int64) (*publications.Publication, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.pub == nil {
		return nil, publications.ErrNotFound
	}
	return f.pub, nil
}

func (f *fakePublicationStore) List(_ context.Context, _ int64, limit, offset int) ([]publications.Publication, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func (f *fakePublicationStore) ListByTelegramID(_ context.Context, _ int64, telegramID int64, limit, offset int) ([]publications.Publication, error) {
	f.gotQ = true
	f.lastGidQ = telegramID
	f.lastLimit = limit
	f.lastOffset = offset
	if f.err != nil {
		return nil, f.err
	}
	if f.listByGid != nil {
		return f.listByGid[telegramID], nil
	}
	return f.list, nil
}

// CancelScheduled (slice 3): delega en cancelErr si esta set, sino OK.
func (f *fakePublicationStore) CancelScheduled(_ context.Context, _ int64, id int64) error {
	f.cancelCalls = append(f.cancelCalls, id)
	if f.cancelErr != nil {
		return f.cancelErr
	}
	return nil
}

// buildPublicationsServer construye un Server con auth fija y el modulo
// de publicaciones habilitado (fake inyectado).
func buildPublicationsServer(t *testing.T, pub *fakePublicationStore, opts ...Option) *Server {
	t.Helper()
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithPublications(pub),
	)
	for _, o := range opts {
		o(server)
	}
	return server
}

func TestPublicationsRoutes_RequireAuth(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	cases := []struct {
		method, path string
	}{
		{"POST", "/api/publications"},
		{"GET", "/api/publications"},
		{"GET", "/api/publications/1"},
	}
	for _, tc := range cases {
		rr := doRequest(server, tc.method, tc.path, "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: code = %d, want 401", tc.method, tc.path, rr.Code)
		}
	}
}

// TestPublications_CreateSuccess: POST /api/publications con 1 grupo
// devuelve 201 con `data.publications` (envelope del slice 2) y la
// fila traza `status=sent`.
func TestPublications_CreateSuccess(t *testing.T) {
	mid := int64(123)
	pub := &fakePublicationStore{pub: &publications.Publication{
		ID:         1,
		TelegramID: -100123,
		Text:       "Hola mundo",
		Status:     publications.StatusSent,
		MessageID:  &mid,
		ActorID:    int64Ptr(1),
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"Hola mundo","group_ids":[-100123]}`, validToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	if pub.calls != 1 || pub.actor != 1 {
		t.Errorf("PublishMany params = actor=%d, want 1", pub.actor)
	}
	if len(pub.payload.GroupIDs) != 1 || pub.payload.GroupIDs[0] != -100123 {
		t.Errorf("group_ids = %v, want [-100123]", pub.payload.GroupIDs)
	}
	if !strings.Contains(rr.Body.String(), `"publications":`) {
		t.Errorf("body sin envelope `publications`: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"sent"`) {
		t.Errorf("body sin status sent: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"error":null`) {
		t.Errorf("body sin error null: %s", rr.Body.String())
	}
}

// TestPublications_CreateMultiGrupo_201: POST con 2 grupos OK
// devuelve 201 con `publications:[pub1,pub2]` y status sent en ambas.
func TestPublications_CreateMultiGrupo_201(t *testing.T) {
	mid1, mid2 := int64(11), int64(22)
	pub := &fakePublicationStore{list: []publications.Publication{
		{ID: 1, TelegramID: -1001, Text: "Hola", Status: publications.StatusSent, MessageID: &mid1},
		{ID: 2, TelegramID: -1002, Text: "Hola", Status: publications.StatusSent, MessageID: &mid2},
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"Hola","group_ids":[-1001,-1002]}`, validToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data struct {
			Publications []publications.Publication `json:"publications"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body no JSON: %v", err)
	}
	if len(resp.Data.Publications) != 2 {
		t.Errorf("len(publications) = %d, want 2", len(resp.Data.Publications))
	}
	if len(pub.payload.GroupIDs) != 2 {
		t.Errorf("group_ids = %v, want [-1001 -1002]", pub.payload.GroupIDs)
	}
}

// TestPublications_CreateOK_SingleConFotoYBotones: foto+botones se
// envian al servicio en el payload.
func TestPublications_CreateOK_SingleConFotoYBotones(t *testing.T) {
	mid := int64(99)
	pub := &fakePublicationStore{pub: &publications.Publication{
		ID:         1,
		TelegramID: -1001,
		Text:       "con foto",
		Status:     publications.StatusSent,
		MessageID:  &mid,
		PhotoURL:   strPtr("https://example.com/x.jpg"),
		Buttons:    json.RawMessage(`[[{"text":"Ir","url":"https://example.com"}]]`),
	}}
	server := buildPublicationsServer(t, pub)

	body := `{"text":"con foto","photo_url":"https://example.com/x.jpg","buttons":[[{"text":"Ir","url":"https://example.com"}]],"group_ids":[-1001]}`
	rr := doRequest(server, "POST", "/api/publications", body, validToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	if pub.payload.PhotoURL == nil || *pub.payload.PhotoURL != "https://example.com/x.jpg" {
		t.Errorf("photo_url = %v, want https://example.com/x.jpg", pub.payload.PhotoURL)
	}
	if len(pub.payload.Buttons) != 1 || len(pub.payload.Buttons[0]) != 1 {
		t.Errorf("buttons = %v, want 1 fila de 1 boton", pub.payload.Buttons)
	}
}

// TestPublications_Create_400_GroupIDsVacio: payload valido a nivel
// tipos pero semantica invalida -> 400 VALIDATION_ERROR.
func TestPublications_Create_400_GroupIDsVacio(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrGroupsEmpty}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"hola","group_ids":[]}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "al menos un grupo") {
		t.Errorf("body sin mensaje legible: %s", rr.Body.String())
	}
}

// TestPublications_Create_400_URLNoHTTP: photo_url con esquema no
// http(s) -> 400 VALIDATION_ERROR (mapeo de ErrPhotoURLScheme).
func TestPublications_Create_400_URLNoHTTP(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrPhotoURLScheme}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"hola","photo_url":"ftp://x/y.jpg","group_ids":[-1001]}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
}

// TestPublications_Create_400_BotonesMas8x8: excede limite -> 400.
func TestPublications_Create_400_BotonesMas8x8(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrButtonsLimit}
	server := buildPublicationsServer(t, pub)

	// 9 filas: excede el limite autoimpuesto (8).
	rows := make([][]map[string]string, 9)
	for i := range rows {
		rows[i] = []map[string]string{{"text": "A", "url": "https://a"}}
	}
	raw, _ := json.Marshal(rows)
	body := `{"text":"hola","buttons":` + string(raw) + `,"group_ids":[-1001]}`

	rr := doRequest(server, "POST", "/api/publications", body, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
}

// TestPublications_Create_400_GruposExcede10: 11 grupos -> 400.
func TestPublications_Create_400_GruposExcede10(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrGroupsLimit}
	server := buildPublicationsServer(t, pub)

	ids := make([]int64, 11)
	for i := range ids {
		ids[i] = int64(-1000 - i)
	}
	raw, _ := json.Marshal(ids)
	body := `{"text":"hola","group_ids":` + string(raw) + `}`

	rr := doRequest(server, "POST", "/api/publications", body, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "10 grupos") {
		t.Errorf("body sin mensaje de limite: %s", rr.Body.String())
	}
}

// TestPublications_Create_EmptyText: 400 VALIDATION_ERROR por texto vacio.
func TestPublications_Create_EmptyText(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrTextEmpty}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"","group_ids":[-100123]}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
}

// TestPublications_Create_GroupNotFound: per-grupo 404 -> 201 con fila
// failed y error_message legible (el handler mapea via respondPublicationError
// solo errores top-level; aqui simulamos el caso de payload invalido).
func TestPublications_Create_GroupNotFound(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrGroupNotFound}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"Hola","group_ids":[-999]}`, validToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "NOT_FOUND") {
		t.Errorf("body sin NOT_FOUND: %s", rr.Body.String())
	}
}

// TestPublications_Create_403_BotPermission: bot sin admin -> 403.
func TestPublications_Create_403_BotPermission(t *testing.T) {
	pub := &fakePublicationStore{err: publications.ErrBotPermission}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "POST", "/api/publications",
		`{"text":"Hola","group_ids":[-100123]}`, validToken)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "PERMISSION_DENIED") {
		t.Errorf("body sin PERMISSION_DENIED: %s", rr.Body.String())
	}
}

func TestPublications_GetByID(t *testing.T) {
	mid := int64(123)
	pub := &fakePublicationStore{pub: &publications.Publication{
		ID:         1,
		TelegramID: -100123,
		Text:       "Hola mundo",
		Status:     publications.StatusSent,
		MessageID:  &mid,
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications/1", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"id":1`) {
		t.Errorf("body sin id 1: %s", rr.Body.String())
	}
}

func TestPublications_GetByIDNotFound(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications/999", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestPublications_List(t *testing.T) {
	mid := int64(123)
	pub := &fakePublicationStore{list: []publications.Publication{
		{ID: 1, TelegramID: -100123, Text: "Primera", Status: publications.StatusSent, MessageID: &mid},
		{ID: 2, TelegramID: -100123, Text: "Segunda", Status: publications.StatusFailed},
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"text":"Primera"`) || !strings.Contains(rr.Body.String(), `"text":"Segunda"`) {
		t.Errorf("body sin ambas publicaciones: %s", rr.Body.String())
	}
}

// TestPublications_List_ConFiltro: GET /api/publications?group_id=X
// delega en ListByTelegramID (con el X parseado).
func TestPublications_List_ConFiltro(t *testing.T) {
	mid := int64(1)
	pub := &fakePublicationStore{
		list: []publications.Publication{
			{ID: 1, TelegramID: -1001, Text: "solo g1", Status: publications.StatusSent, MessageID: &mid},
		},
		listByGid: map[int64][]publications.Publication{
			-1001: {{ID: 1, TelegramID: -1001, Text: "solo g1", Status: publications.StatusSent, MessageID: &mid}},
			-1002: {{ID: 2, TelegramID: -1002, Text: "solo g2", Status: publications.StatusSent, MessageID: &mid}},
		},
	}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications?group_id=-1001", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !pub.gotQ {
		t.Error("handler no llamo a ListByTelegramID")
	}
	if pub.lastGidQ != -1001 {
		t.Errorf("group_id = %d, want -1001", pub.lastGidQ)
	}
	if !strings.Contains(rr.Body.String(), `"solo g1"`) {
		t.Errorf("body sin fila esperada: %s", rr.Body.String())
	}
}

// TestPublications_List_ConFiltroInvalido: ?group_id=abc -> 400.
func TestPublications_List_ConFiltroInvalido(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications?group_id=abc", "", validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
}

func strPtr(s string) *string { return &s }

// --- Tests de slice 3: scheduled_at, paginacion, DELETE ---

// TestPublications_Create_ScheduledFuture_201: POST con scheduled_at
// futuro delega en Schedule y devuelve 201 con N filas `scheduled`.
func TestPublications_Create_ScheduledFuture_201(t *testing.T) {
	future := time.Date(2027, 1, 1, 10, 0, 0, 0, time.UTC)
	pub := &fakePublicationStore{
		scheduleList: []publications.Publication{
			{ID: 1, TelegramID: -1001, Text: "Hola", Status: publications.StatusScheduled, ScheduledAt: &future},
			{ID: 2, TelegramID: -1002, Text: "Hola", Status: publications.StatusScheduled, ScheduledAt: &future},
		},
	}
	server := buildPublicationsServer(t, pub)

	body := `{"text":"Hola","group_ids":[-1001,-1002],"scheduled_at":"2027-01-01T10:00:00Z"}`
	rr := doRequest(server, "POST", "/api/publications", body, validToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"scheduled"`) {
		t.Errorf("body sin status scheduled: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"scheduled_at"`) {
		t.Errorf("body sin scheduled_at: %s", rr.Body.String())
	}
}

// TestPublications_Create_ScheduledPast_400: scheduled_at en el pasado
// -> 400 VALIDATION_ERROR sin crear fila.
func TestPublications_Create_ScheduledPast_400(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	body := `{"text":"Hola","group_ids":[-1001],"scheduled_at":"2020-01-01T00:00:00Z"}`
	rr := doRequest(server, "POST", "/api/publications", body, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
}

// TestPublications_Create_ScheduledOffset_Normalized: scheduled_at con
// offset se normaliza a UTC antes de persistir.
func TestPublications_Create_ScheduledOffset_Normalized(t *testing.T) {
	captured := time.Time{}
	pub := &fakePublicationStore{
		scheduleList: []publications.Publication{
			{ID: 1, TelegramID: -1001, Status: publications.StatusScheduled},
		},
	}
	// Override Schedule para capturar el scheduledAt normalizado.
	pub.scheduleList = nil
	pub.scheduleErr = nil
	// El fake ya captura lastScheduledAt. Solo necesitamos un listado
	// cualquiera para que el handler no falle.
	pub.pub = &publications.Publication{
		ID: 1, TelegramID: -1001, Status: publications.StatusScheduled,
	}
	server := buildPublicationsServer(t, pub)

	body := `{"text":"Hola","group_ids":[-1001],"scheduled_at":"2027-06-15T17:00:00+03:00"}`
	rr := doRequest(server, "POST", "/api/publications", body, validToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}
	captured = pub.lastScheduledAt
	if captured.Location() != time.UTC {
		t.Errorf("scheduledAt.location = %v, want UTC (normalizado)", captured.Location())
	}
	if expected := time.Date(2027, 6, 15, 14, 0, 0, 0, time.UTC); !captured.Equal(expected) {
		t.Errorf("scheduledAt = %v, want %v", captured, expected)
	}
}

// TestPublications_List_LimitOffset: GET ?limit=10&offset=20 propaga
// al servicio y devuelve el slice correcto.
func TestPublications_List_LimitOffset(t *testing.T) {
	mid := int64(1)
	pub := &fakePublicationStore{list: []publications.Publication{
		{ID: 21, TelegramID: -1001, Text: "p21", Status: publications.StatusSent, MessageID: &mid},
		{ID: 22, TelegramID: -1001, Text: "p22", Status: publications.StatusSent, MessageID: &mid},
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications?limit=10&offset=20", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if pub.lastLimit != 10 || pub.lastOffset != 20 {
		t.Errorf("service recibio limit=%d offset=%d, want 10/20", pub.lastLimit, pub.lastOffset)
	}
}

// TestPublications_List_LimitOutOfRange_400: limit=101 -> 400.
func TestPublications_List_LimitOutOfRange_400(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)
	rr := doRequest(server, "GET", "/api/publications?limit=101", "", validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
}

// TestPublications_List_OffsetNegative_400: offset=-1 -> 400.
func TestPublications_List_OffsetNegative_400(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)
	rr := doRequest(server, "GET", "/api/publications?offset=-1", "", validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
}

// TestPublications_List_NoParams_DefaultLimit: sin query params, el
// handler aplica default (limit=50).
func TestPublications_List_NoParams_DefaultLimit(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)
	rr := doRequest(server, "GET", "/api/publications", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rr.Code)
	}
	if pub.lastLimit != 50 {
		t.Errorf("default limit = %d, want 50", pub.lastLimit)
	}
	if pub.lastOffset != 0 {
		t.Errorf("default offset = %d, want 0", pub.lastOffset)
	}
}

// TestPublications_DeleteScheduled_204: DELETE sobre fila scheduled -> 204.
func TestPublications_DeleteScheduled_204(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "DELETE", "/api/publications/10", "", validToken)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204 (body: %s)", rr.Code, rr.Body.String())
	}
	if len(pub.cancelCalls) != 1 || pub.cancelCalls[0] != 10 {
		t.Errorf("cancelCalls = %v, want [10]", pub.cancelCalls)
	}
}

// TestPublications_DeleteSent_409: DELETE sobre fila sent -> 409 INVALID_STATUS.
func TestPublications_DeleteSent_409(t *testing.T) {
	pub := &fakePublicationStore{cancelErr: publications.ErrCancelNotAllowed}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "DELETE", "/api/publications/20", "", validToken)
	if rr.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "INVALID_STATUS") {
		t.Errorf("body sin INVALID_STATUS: %s", rr.Body.String())
	}
}

// TestPublications_DeleteNotFound_404: id inexistente -> 404 NOT_FOUND.
func TestPublications_DeleteNotFound_404(t *testing.T) {
	pub := &fakePublicationStore{cancelErr: publications.ErrNotFound}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "DELETE", "/api/publications/999", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestPublications_DeleteInvalidID_400: id no parseable -> 400.
func TestPublications_DeleteInvalidID_400(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "DELETE", "/api/publications/abc", "", validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
}

// TestPublications_Delete_RequireAuth: DELETE sin token -> 401.
func TestPublications_Delete_RequireAuth(t *testing.T) {
	pub := &fakePublicationStore{}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "DELETE", "/api/publications/1", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rr.Code)
	}
}

// TestPublications_List_GroupFilter_CombinedWithPagination: el handler
// combina ?group_id= con ?limit=&offset=.
func TestPublications_List_GroupFilter_CombinedWithPagination(t *testing.T) {
	mid := int64(1)
	pub := &fakePublicationStore{listByGid: map[int64][]publications.Publication{
		-1001: {{ID: 11, TelegramID: -1001, Text: "g1.p11", Status: publications.StatusSent, MessageID: &mid}},
	}}
	server := buildPublicationsServer(t, pub)

	rr := doRequest(server, "GET", "/api/publications?group_id=-1001&limit=10&offset=10", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !pub.gotQ {
		t.Error("handler no llamo a ListByTelegramID")
	}
	if pub.lastGidQ != -1001 {
		t.Errorf("group_id = %d, want -1001", pub.lastGidQ)
	}
	if pub.lastLimit != 10 || pub.lastOffset != 10 {
		t.Errorf("service recibio limit=%d offset=%d, want 10/10", pub.lastLimit, pub.lastOffset)
	}
}

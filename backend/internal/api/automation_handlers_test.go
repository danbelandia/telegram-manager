// Tests de los handlers de moderacion automatica (Fase 3, slice 2).
// Cubre los 9 endpoints bajo /api/groups/{id}/automation/... con
// fakes para automationService, automationLogWriter y
// automationGroupChecker. Sin PostgreSQL ni Telegram (matches §21.1).
package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/telegram-manager/backend/internal/automation"
	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
)

// fakeAutomationService satisface automationService para los tests
// de handlers. Mantiene un solo set de settings + listas en memoria
// (multi-grupo via map).
type fakeAutomationService struct {
	mu sync.Mutex
	// settings[groupID] -> Settings
	settings map[int64]*automation.Settings
	// bannedWords[groupID] -> []string
	bannedWords map[int64][]string
	// linkAllowlist[groupID] -> []string
	linkAllowlist map[int64][]string
	// errForzar errores por operacion.
	errGetSettings  error
	errUpsert       error
	errListBanned   error
	errAddBanned    error
	errRemoveBanned error
	errListAllow    error
	errAddAllow     error
	errRemoveAllow  error
}

func newFakeAutomationService() *fakeAutomationService {
	return &fakeAutomationService{
		settings:      make(map[int64]*automation.Settings),
		bannedWords:   make(map[int64][]string),
		linkAllowlist: make(map[int64][]string),
	}
}

func (f *fakeAutomationService) GetSettings(_ context.Context, groupID int64) (*automation.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetSettings != nil {
		return nil, f.errGetSettings
	}
	s, ok := f.settings[groupID]
	if !ok {
		return nil, automation.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeAutomationService) UpsertSettings(_ context.Context, s *automation.Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errUpsert != nil {
		return f.errUpsert
	}
	cp := *s
	f.settings[s.GroupID] = &cp
	return nil
}

func (f *fakeAutomationService) ListBannedWords(_ context.Context, groupID int64) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListBanned != nil {
		return nil, f.errListBanned
	}
	cp := make([]string, len(f.bannedWords[groupID]))
	copy(cp, f.bannedWords[groupID])
	return cp, nil
}

func (f *fakeAutomationService) AddBannedWord(_ context.Context, groupID int64, word string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errAddBanned != nil {
		return f.errAddBanned
	}
	for _, w := range f.bannedWords[groupID] {
		if w == word {
			return nil // idempotent
		}
	}
	f.bannedWords[groupID] = append(f.bannedWords[groupID], word)
	return nil
}

func (f *fakeAutomationService) RemoveBannedWord(_ context.Context, groupID int64, word string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errRemoveBanned != nil {
		return f.errRemoveBanned
	}
	out := f.bannedWords[groupID][:0]
	for _, w := range f.bannedWords[groupID] {
		if w != word {
			out = append(out, w)
		}
	}
	f.bannedWords[groupID] = out
	return nil
}

func (f *fakeAutomationService) ListLinkAllowlist(_ context.Context, groupID int64) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListAllow != nil {
		return nil, f.errListAllow
	}
	cp := make([]string, len(f.linkAllowlist[groupID]))
	copy(cp, f.linkAllowlist[groupID])
	return cp, nil
}

func (f *fakeAutomationService) AddLinkAllowlist(_ context.Context, groupID int64, domain string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errAddAllow != nil {
		return f.errAddAllow
	}
	for _, d := range f.linkAllowlist[groupID] {
		if d == domain {
			return nil // idempotent
		}
	}
	f.linkAllowlist[groupID] = append(f.linkAllowlist[groupID], domain)
	return nil
}

func (f *fakeAutomationService) RemoveLinkAllowlist(_ context.Context, groupID int64, domain string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errRemoveAllow != nil {
		return f.errRemoveAllow
	}
	out := f.linkAllowlist[groupID][:0]
	for _, d := range f.linkAllowlist[groupID] {
		if d != domain {
			out = append(out, d)
		}
	}
	f.linkAllowlist[groupID] = out
	return nil
}

// fakeAutomationLogs captura entradas para verificar que los handlers
// escriben con ActorID del admin (distinto del patron slice 1 donde
// los auto-actions del pipeline llevan ActorID=nil).
type fakeAutomationLogs struct {
	mu      sync.Mutex
	entries []*logs.Entry
}

func (f *fakeAutomationLogs) Create(_ context.Context, e *logs.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *e
	f.entries = append(f.entries, &cp)
	return nil
}

// fakeAutomationGroups satisface automationGroupChecker con un set
// de grupos existentes en memoria. Si el id no esta, devuelve
// groups.ErrNotFound (404 al admin).
type fakeAutomationGroups struct {
	groups map[int64]*groups.Group
}

func (f *fakeAutomationGroups) GetByTelegramID(_ context.Context, id int64) (*groups.Group, error) {
	g, ok := f.groups[id]
	if !ok {
		return nil, groups.ErrNotFound
	}
	return g, nil
}

// buildAutomationServer construye un Server con auth fija y el modulo
// de automation habilitado (fakes inyectados).
func buildAutomationServer(t *testing.T, auto *fakeAutomationService, ag *fakeAutomationGroups, al *fakeAutomationLogs) *Server {
	t.Helper()
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithAutomation(auto, al, ag),
	)
	return server
}

// TestAutomationRoutes_RequireAuth: todas las rutas del modulo
// exigen access token. Sin token → 401.
func TestAutomationRoutes_RequireAuth(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	cases := []struct{ method, path string }{
		{"GET", "/api/groups/-100/automation/settings"},
		{"PUT", "/api/groups/-100/automation/settings"},
		{"GET", "/api/groups/-100/automation/banned-words"},
		{"POST", "/api/groups/-100/automation/banned-words"},
		{"DELETE", "/api/groups/-100/automation/banned-words/spam"},
		{"GET", "/api/groups/-100/automation/link-allowlist"},
		{"POST", "/api/groups/-100/automation/link-allowlist"},
		{"DELETE", "/api/groups/-100/automation/link-allowlist/example.com"},
	}
	for _, tc := range cases {
		rr := doRequest(server, tc.method, tc.path, "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: code = %d, want 401", tc.method, tc.path, rr.Code)
		}
	}
}

// TestAutomationSettings_GetCreatesDefaults: GET cuando no existe la
// fila devuelve defaults (200) sin persistir. PUT subsiguiente crea
// la fila.
func TestAutomationSettings_GetCreatesDefaults(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "GET", "/api/groups/-1001/automation/settings", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"enabled":false`) {
		t.Errorf("body sin defaults: %s", rr.Body.String())
	}
}

// TestAutomationSettings_Put_LogsUpdateAction: PUT persiste + log
// UPDATE_AUTOMATION_SETTINGS con ActorID del admin (Subject del
// access token, "1" en testClaims).
func TestAutomationSettings_Put_LogsUpdateAction(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	body := `{"enabled":true,"flood_enabled":true,"flood_messages":7}`
	rr := doRequest(server, "PUT", "/api/groups/-1001/automation/settings", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if _, ok := auto.settings[-1001]; !ok {
		t.Fatal("settings no persistida")
	}
	if !auto.settings[-1001].Enabled {
		t.Errorf("Enabled = false, want true")
	}
	// Log con ActorID=1 (subject del testClaims) y action UPDATE_AUTOMATION_SETTINGS.
	var found *logs.Entry
	for _, e := range al.entries {
		if e.Action == logs.ActionUpdateAutomationSettings {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatalf("no log UPDATE_AUTOMATION_SETTINGS, entries = %d", len(al.entries))
	}
	if found.ActorID == nil || *found.ActorID != 1 {
		t.Errorf("ActorID = %v, want ptr(1) (admin)", found.ActorID)
	}
}

// TestAutomationSettings_Put_BadJSON_400: body invalido → 400 sin log.
func TestAutomationSettings_Put_BadJSON_400(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "PUT", "/api/groups/-1001/automation/settings", `{`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rr.Code)
	}
	if len(al.entries) != 0 {
		t.Errorf("se logueo a pesar de body invalido: %d", len(al.entries))
	}
}

// TestAutomationSettings_GroupNotFound_404: GET sobre grupo inexistente
// → 404 NOT_FOUND.
func TestAutomationSettings_GroupNotFound_404(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{}} // vacio
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "GET", "/api/groups/-999/automation/settings", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "NOT_FOUND") {
		t.Errorf("body sin NOT_FOUND: %s", rr.Body.String())
	}
}

// TestBannedWords_CRUD: GET vacio, POST agrega, GET lista, DELETE
// remueve. Cada POST/DELETE loguea con ActorID admin.
func TestBannedWords_CRUD(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	// GET inicial vacio.
	rr := doRequest(server, "GET", "/api/groups/-1001/automation/banned-words", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET code = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"words":[]`) {
		t.Errorf("GET inicial sin lista vacia: %s", rr.Body.String())
	}

	// POST "spam".
	rr = doRequest(server, "POST", "/api/groups/-1001/automation/banned-words",
		`{"word":"spam"}`, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"words":["spam"]`) {
		t.Errorf("POST body sin spam: %s", rr.Body.String())
	}

	// POST "viagra".
	rr = doRequest(server, "POST", "/api/groups/-1001/automation/banned-words",
		`{"word":"viagra"}`, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST 2 code = %d, want 200", rr.Code)
	}

	// DELETE "spam".
	rr = doRequest(server, "DELETE", "/api/groups/-1001/automation/banned-words/spam", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE code = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"viagra"`) {
		t.Errorf("DELETE body sin viagra restante: %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"spam"`) {
		t.Errorf("DELETE body aun contiene spam: %s", rr.Body.String())
	}

	// Logs: 2 ADD + 1 REMOVE con ActorID=1.
	adds := 0
	rms := 0
	for _, e := range al.entries {
		switch e.Action {
		case logs.ActionAddBannedWord:
			adds++
			if e.ActorID == nil || *e.ActorID != 1 {
				t.Errorf("ADD ActorID = %v, want ptr(1)", e.ActorID)
			}
		case logs.ActionRemoveBannedWord:
			rms++
			if e.ActorID == nil || *e.ActorID != 1 {
				t.Errorf("REMOVE ActorID = %v, want ptr(1)", e.ActorID)
			}
		}
	}
	if adds != 2 || rms != 1 {
		t.Errorf("logs adds=%d removes=%d, want 2/1", adds, rms)
	}
}

// TestBannedWords_InvalidWord_400: word con caracteres no permitidos
// → 400 sin persistir.
func TestBannedWords_InvalidWord_400(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "POST", "/api/groups/-1001/automation/banned-words",
		`{"word":"<script>alert(1)</script>"}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if len(auto.bannedWords[-1001]) != 0 {
		t.Errorf("word invalida persistida")
	}
}

// TestBannedWords_Lowercased: POST con MAYUSCULAS persiste en
// minusculas (case-insensitive server-side).
func TestBannedWords_Lowercased(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "POST", "/api/groups/-1001/automation/banned-words",
		`{"word":"SPAM"}`, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rr.Code)
	}
	if len(auto.bannedWords[-1001]) != 1 || auto.bannedWords[-1001][0] != "spam" {
		t.Errorf("word persisted = %v, want [spam]", auto.bannedWords[-1001])
	}
}

// TestLinkAllowlist_CRUD: GET, POST, DELETE same pattern.
func TestLinkAllowlist_CRUD(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	// POST example.com
	rr := doRequest(server, "POST", "/api/groups/-1001/automation/link-allowlist",
		`{"domain":"example.com"}`, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST code = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"example.com"`) {
		t.Errorf("POST body sin example.com: %s", rr.Body.String())
	}

	// GET
	rr = doRequest(server, "GET", "/api/groups/-1001/automation/link-allowlist", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET code = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"domains":["example.com"]`) {
		t.Errorf("GET body sin domains: %s", rr.Body.String())
	}

	// DELETE
	rr = doRequest(server, "DELETE", "/api/groups/-1001/automation/link-allowlist/example.com", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE code = %d, want 200", rr.Code)
	}

	// Logs: 1 ADD + 1 REMOVE.
	adds := 0
	rms := 0
	for _, e := range al.entries {
		switch e.Action {
		case logs.ActionAddLinkAllowlist:
			adds++
		case logs.ActionRemoveLinkAllowlist:
			rms++
		}
	}
	if adds != 1 || rms != 1 {
		t.Errorf("logs adds=%d removes=%d, want 1/1", adds, rms)
	}
}

// TestLinkAllowlist_InvalidDomain_400: dominio vacio → 400.
func TestLinkAllowlist_InvalidDomain_400(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "POST", "/api/groups/-1001/automation/link-allowlist",
		`{"domain":"   "}`, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
}

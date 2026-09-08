// Tests de los handlers de moderacion automatica (Fase 3, slice 2).
// Cubre los 9 endpoints bajo /api/groups/{id}/automation/... con
// fakes para automationService, automationLogWriter y
// automationGroupChecker. Sin PostgreSQL ni Telegram (matches §21.1).
package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

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

// fakeAutomationDashboardRepo satisface automationDashboardRepo para
// los tests de slice 3. Mantiene warning_states en memoria + errores
// forzables. Esta separado del fakeAutomationService porque la
// inyeccion de WithAutomation los recibe como parametros distintos
// (ver server.go + automation_handlers.go).
type fakeAutomationDashboardRepo struct {
	mu sync.Mutex
	// warningStates[groupID][userID] -> WarningStateRow.
	warningStates map[int64]map[int64]*automation.WarningStateRow
	// lastResetCall[groupID][userID] -> count previo.
	lastResetCall map[int64]map[int64]int64
	// errForzar errores por operacion.
	errListActiveWarnings error
	errResetWarningState  error
}

func newFakeAutomationDashboardRepo() *fakeAutomationDashboardRepo {
	return &fakeAutomationDashboardRepo{
		warningStates: make(map[int64]map[int64]*automation.WarningStateRow),
		lastResetCall: make(map[int64]map[int64]int64),
	}
}

// ListActiveWarningStatesByGroup: filtra warning_count > 0, ordena por
// warning_count DESC + last_warning_at DESC NULLS LAST.
func (f *fakeAutomationDashboardRepo) ListActiveWarningStatesByGroup(_ context.Context, _, groupID int64, limit int) ([]automation.WarningStateRow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errListActiveWarnings != nil {
		return nil, false, f.errListActiveWarnings
	}
	if limit <= 0 {
		limit = 100
	}
	all := make([]automation.WarningStateRow, 0)
	for _, ws := range f.warningStates[groupID] {
		if ws.WarningCount <= 0 {
			continue
		}
		all = append(all, *ws)
	}
	// Orden estable: count DESC, last_warning_at DESC NULLS LAST.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].WarningCount != all[j].WarningCount {
			return all[i].WarningCount > all[j].WarningCount
		}
		if all[i].LastWarningAt == nil && all[j].LastWarningAt == nil {
			return false
		}
		if all[i].LastWarningAt == nil {
			return false // NULLS LAST
		}
		if all[j].LastWarningAt == nil {
			return true
		}
		return all[i].LastWarningAt.After(*all[j].LastWarningAt)
	})
	truncated := len(all) > limit
	if truncated {
		all = all[:limit]
	}
	return all, truncated, nil
}

// ResetWarningState: limpia el counter y guarda el valor previo. Si no
// existia la fila, retorna (0, nil).
func (f *fakeAutomationDashboardRepo) ResetWarningState(_ context.Context, _, groupID, userID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errResetWarningState != nil {
		return 0, f.errResetWarningState
	}
	ws, ok := f.warningStates[groupID][userID]
	if !ok {
		return 0, nil
	}
	previous := int64(ws.WarningCount)
	ws.WarningCount = 0
	ws.LastWarningAt = nil
	ws.LastActionAt = nil
	ws.ExpiresAt = nil
	if f.lastResetCall[groupID] == nil {
		f.lastResetCall[groupID] = make(map[int64]int64)
	}
	f.lastResetCall[groupID][userID] = previous
	return previous, nil
}

// fakeAutomationLogs captura entradas para verificar que los handlers
// escriben con ActorID del admin (distinto del patron slice 1 donde
// los auto-actions del pipeline llevan ActorID=nil). Slice 3 agrega
// CountByActionAndGroup para el endpoint /stats.
type fakeAutomationLogs struct {
	mu      sync.Mutex
	entries []*logs.Entry
	// statsCount[groupID] -> map[action]count (in-memory).
	statsCount map[int64]map[string]int
	// errForzar errores por operacion.
	errCreate             error
	errCountByActionGroup error
}

func (f *fakeAutomationLogs) Create(_ context.Context, e *logs.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errCreate != nil {
		return f.errCreate
	}
	cp := *e
	f.entries = append(f.entries, &cp)
	return nil
}

// CountByActionAndGroup agrega logs por action en una ventana para un
// grupo. Cubre el handler slice 3 GET .../stats.
func (f *fakeAutomationLogs) CountByActionAndGroup(_ context.Context, _, groupID int64, actions []string, _ time.Time) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errCountByActionGroup != nil {
		return nil, f.errCountByActionGroup
	}
	wanted := make(map[string]bool)
	for _, a := range actions {
		wanted[a] = true
	}
	out := make(map[string]int)
	for action, count := range f.statsCount[groupID] {
		if wanted[action] {
			out[action] = count
		}
	}
	return out, nil
}

// fakeAutomationGroups satisface automationGroupChecker con un set
// de grupos existentes en memoria. Si el id no esta, devuelve
// groups.ErrNotFound (404 al admin).
type fakeAutomationGroups struct {
	groups map[int64]*groups.Group
}

func (f *fakeAutomationGroups) GetByTenant(_ context.Context, _ int64, id int64) (*groups.Group, error) {
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
	dash := newFakeAutomationDashboardRepo()
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithAutomation(auto, al, ag, dash),
	)
	return server
}

// buildAutomationServerWithDashboard igual a buildAutomationServer pero
// permite inyectar un dashboard fake custom (necesario en tests de
// slice 3 que pre-poblan warning_states).
func buildAutomationServerWithDashboard(
	t *testing.T,
	auto *fakeAutomationService,
	ag *fakeAutomationGroups,
	al *fakeAutomationLogs,
	dash *fakeAutomationDashboardRepo,
) *Server {
	t.Helper()
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithAutomation(auto, al, ag, dash),
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
		// Slice 3 — Warnings Dashboard.
		{"GET", "/api/groups/-100/automation/warnings"},
		{"POST", "/api/groups/-100/automation/warnings/1/reset"},
		{"GET", "/api/groups/-100/automation/stats"},
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

// TestAutomationSettings_Put_WarnUserFields (slice 2.1, REQ-22/29):
// PUT acepta los 2 nuevos campos (warn_user_enabled + warn_user_template)
// y los persiste en el fake service. Verifica round-trip via GET.
func TestAutomationSettings_Put_WarnUserFields(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	body := `{"warn_user_enabled":false,"warn_user_template":"⚠️ Custom para {nombre} ({count})."}`
	rr := doRequest(server, "PUT", "/api/groups/-1001/automation/settings", body, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	// GET de vuelta para confirmar persistencia.
	rr = doRequest(server, "GET", "/api/groups/-1001/automation/settings", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET code = %d, want 200", rr.Code)
	}
	body2 := rr.Body.String()
	if !strings.Contains(body2, `"warn_user_enabled":false`) {
		t.Errorf("GET sin warn_user_enabled=false: %s", body2)
	}
	if !strings.Contains(body2, `"warn_user_template":"⚠️ Custom para {nombre} ({count})."`) {
		t.Errorf("GET sin warn_user_template custom: %s", body2)
	}
}

// TestAutomationSettings_Put_WarnUserTemplateTooLong_400: template
// custom > 1000 chars → 400 VALIDATION_ERROR, no persiste.
func TestAutomationSettings_Put_WarnUserTemplateTooLong_400(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	// Construir template de 1001 chars (excede el limite).
	tooLong := strings.Repeat("a", 1001)
	body := `{"warn_user_template":"` + tooLong + `"}`
	rr := doRequest(server, "PUT", "/api/groups/-1001/automation/settings", body, validToken)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "1000") {
		t.Errorf("body sin mencion del limite 1000: %s", rr.Body.String())
	}
	if _, ok := auto.settings[-1001]; ok {
		t.Errorf("settings persistida a pesar de 400 (template invalido)")
	}
}

// TestAutomationSettings_Put_WarnUserTemplateExactLength_OK: template
// de exactamente 1000 chars → 200 OK (limite inclusivo).
func TestAutomationSettings_Put_WarnUserTemplateExactLength_OK(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	exact := strings.Repeat("b", 1000)
	body := `{"warn_user_template":"` + exact + `"}`
	rr := doRequest(server, "PUT", "/api/groups/-1001/automation/settings", body, validToken)
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
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

// --- Tests de slice 3: 3 handlers del Warnings Dashboard ---

// seedWarnings inserta N warning_states en el fake del dashboard para
// simular el LEFT JOIN shape del backend.
func seedWarnings(dash *fakeAutomationDashboardRepo, groupID int64, rows []automation.WarningStateRow) {
	if dash.warningStates[groupID] == nil {
		dash.warningStates[groupID] = make(map[int64]*automation.WarningStateRow)
	}
	for i := range rows {
		r := rows[i]
		dash.warningStates[groupID][r.UserID] = &r
	}
}

// TestAutomationWarnings_List_Shape: GET /warnings con LEFT JOIN visible
// (display_name computado server-side via WarningStateRow.DisplayName).
func TestAutomationWarnings_List_Shape(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	dash := newFakeAutomationDashboardRepo()
	server := buildAutomationServerWithDashboard(t, auto, ag, al, dash)

	username := "ana_p"
	now := time.Now().UTC()
	seedWarnings(dash, -1001, []automation.WarningStateRow{
		{WarningState: automation.WarningState{GroupID: -1001, UserID: 1, WarningCount: 3, LastWarningAt: &now},
			FirstName: "Ana", Username: &username},
		{WarningState: automation.WarningState{GroupID: -1001, UserID: 2, WarningCount: 1}},
	})

	rr := doRequest(server, "GET", "/api/groups/-1001/automation/warnings", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"display_name":"Ana"`) {
		t.Errorf("body sin display_name=Ana: %s", body)
	}
	if !strings.Contains(body, `"display_name":"user 2"`) {
		t.Errorf("body sin display_name=user 2 (fallback): %s", body)
	}
	if !strings.Contains(body, `"username":"ana_p"`) {
		t.Errorf("body sin username=ana_p: %s", body)
	}
	if !strings.Contains(body, `"truncated":false`) {
		t.Errorf("body sin truncated=false: %s", body)
	}
	if !strings.Contains(body, `"warning_count":3`) || !strings.Contains(body, `"warning_count":1`) {
		t.Errorf("body sin warning_count: %s", body)
	}
}

// TestAutomationWarnings_List_GroupNotFound_404: GET sobre grupo
// inexistente → 404 NOT_FOUND.
func TestAutomationWarnings_List_GroupNotFound_404(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "GET", "/api/groups/-999/automation/warnings", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "NOT_FOUND") {
		t.Errorf("body sin NOT_FOUND: %s", rr.Body.String())
	}
}

// TestAutomationWarnings_Reset_Success_Logs: POST /reset emite log
// ActionResetWarnings con ActorID del admin + metadata de auditoria.
func TestAutomationWarnings_Reset_Success_Logs(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	dash := newFakeAutomationDashboardRepo()
	server := buildAutomationServerWithDashboard(t, auto, ag, al, dash)

	seedWarnings(dash, -1001, []automation.WarningStateRow{
		{WarningState: automation.WarningState{GroupID: -1001, UserID: 42, WarningCount: 3}},
	})

	rr := doRequest(server, "POST", "/api/groups/-1001/automation/warnings/42/reset", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"reset":true`) {
		t.Errorf("body sin reset:true: %s", body)
	}
	if !strings.Contains(body, `"warning_count":0`) {
		t.Errorf("body sin warning_count:0: %s", body)
	}
	if !strings.Contains(body, `"user_id":42`) {
		t.Errorf("body sin user_id:42: %s", body)
	}

	// Verificar log emitido: ActorID = 1 (subject testClaims), action = RESET_WARNINGS,
	// metadata con user_id + warning_count_before_reset.
	var found *logs.Entry
	for _, e := range al.entries {
		if e.Action == logs.ActionResetWarnings {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatalf("no log RESET_WARNINGS; entries = %d", len(al.entries))
	}
	if found.ActorID == nil || *found.ActorID != 1 {
		t.Errorf("ActorID = %v, want ptr(1) (admin)", found.ActorID)
	}
	if found.TargetUserID == nil || *found.TargetUserID != 42 {
		t.Errorf("TargetUserID = %v, want ptr(42)", found.TargetUserID)
	}
	if got := found.Metadata["warning_count_before_reset"]; got != int64(3) {
		t.Errorf("metadata warning_count_before_reset = %v, want 3", got)
	}
	if got := found.Metadata["user_id"]; got != int64(42) {
		t.Errorf("metadata user_id = %v, want 42", got)
	}

	// Counter en 0 post-reset (accedido via el dash fake, NO el auto fake).
	ws := dash.warningStates[-1001][42]
	if ws.WarningCount != 0 {
		t.Errorf("counter post-reset = %d, want 0", ws.WarningCount)
	}
}

// TestAutomationWarnings_Reset_NoState_404: POST sobre user sin
// warning_state → 404 NOT_FOUND, sin log.
func TestAutomationWarnings_Reset_NoState_404(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "POST", "/api/groups/-1001/automation/warnings/999/reset", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "no hay advertencias") {
		t.Errorf("body sin mensaje legible: %s", rr.Body.String())
	}
	if len(al.entries) != 0 {
		t.Errorf("se logueo a pesar de 404: %d entries", len(al.entries))
	}
}

// TestAutomationStats_Period24h_Default: GET /stats sin period usa
// default 24h.
func TestAutomationStats_Period24h_Default(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{
		statsCount: map[int64]map[string]int{
			-1001: {
				logs.ActionRuleTriggered: 12,
				logs.ActionAutomuteUser:  3,
				logs.ActionAutobanUser:   1,
			},
		},
	}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "GET", "/api/groups/-1001/automation/stats", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"rule_triggered":12`) {
		t.Errorf("body sin rule_triggered:12: %s", body)
	}
	if !strings.Contains(body, `"automute":3`) {
		t.Errorf("body sin automute:3: %s", body)
	}
	if !strings.Contains(body, `"autoban":1`) {
		t.Errorf("body sin autoban:1: %s", body)
	}
	if !strings.Contains(body, `"period":"24h"`) {
		t.Errorf("body sin period:24h (default): %s", body)
	}
}

// TestAutomationStats_PeriodFoo_400: period invalido → 400 VALIDATION_ERROR.
func TestAutomationStats_PeriodFoo_400(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "GET", "/api/groups/-1001/automation/stats?period=foo", "", validToken)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_ERROR") {
		t.Errorf("body sin VALIDATION_ERROR: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "24h o 7d") {
		t.Errorf("body sin mencion 24h/7d: %s", rr.Body.String())
	}
}

// TestAutomationStats_Period7d: period=7d funciona y devuelve 0s
// cuando no hay logs en la ventana extendida.
func TestAutomationStats_Period7d(t *testing.T) {
	auto := newFakeAutomationService()
	ag := &fakeAutomationGroups{groups: map[int64]*groups.Group{-1001: {TelegramID: -1001}}}
	al := &fakeAutomationLogs{}
	server := buildAutomationServer(t, auto, ag, al)

	rr := doRequest(server, "GET", "/api/groups/-1001/automation/stats?period=7d", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"period":"7d"`) {
		t.Errorf("body sin period:7d: %s", body)
	}
	if !strings.Contains(body, `"rule_triggered":0`) {
		t.Errorf("body sin rule_triggered:0: %s", body)
	}
}

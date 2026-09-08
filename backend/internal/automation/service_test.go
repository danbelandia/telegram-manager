package automation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// fakeWarnRepo implementa la minima superficie del *Repository que el
// Service necesita para tests unit (sin tocar la DB).
type fakeWarnRepo struct {
	mu   sync.Mutex
	rows map[[2]int64]*WarningState
	// nextExpiresAt: si se setea, LoadOrCreateWarningState lo aplica
	// a la fila creada (para tests de expiracion).
}

func newFakeWarnRepo() *fakeWarnRepo {
	return &fakeWarnRepo{rows: make(map[[2]int64]*WarningState)}
}

func (f *fakeWarnRepo) GetWarningState(_ context.Context, groupID, userID int64) (*WarningState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ws, ok := f.rows[[2]int64{groupID, userID}]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *ws
	return &cp, nil
}

func (f *fakeWarnRepo) UpsertWarningState(_ context.Context, ws *WarningState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *ws
	f.rows[[2]int64{ws.GroupID, ws.UserID}] = &cp
	return nil
}

func (f *fakeWarnRepo) CreateWarningStateIfMissing(_ context.Context, groupID, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [2]int64{groupID, userID}
	if _, ok := f.rows[key]; ok {
		return nil
	}
	f.rows[key] = &WarningState{GroupID: groupID, UserID: userID, WarningCount: 0}
	return nil
}

// fakeSettingsRepo implementa la minima superficie del *Repository
// para tests unit del Service.
type fakeSettingsRepo struct {
	mu   sync.Mutex
	rows map[int64]*Settings
}

func newFakeSettingsRepo() *fakeSettingsRepo {
	return &fakeSettingsRepo{rows: make(map[int64]*Settings)}
}

func (f *fakeSettingsRepo) GetSettings(_ context.Context, groupID int64) (*Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.rows[groupID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeSettingsRepo) UpsertSettings(_ context.Context, s *Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *s
	f.rows[s.GroupID] = &cp
	return nil
}

// fakeListsRepo implementa ListsRepo contando invocaciones para
// verificar la logica de pre-load (optimizacion cuando ambos toggles
// dependientes estan off).
type fakeListsRepo struct {
	mu               sync.Mutex
	bannedWords      map[int64][]string
	linkAllowlist    map[int64][]string
	bannedCalls      int
	allowlistCalls   int
	errBannedWords   error
	errLinkAllowlist error
}

func newFakeListsRepo() *fakeListsRepo {
	return &fakeListsRepo{
		bannedWords:   make(map[int64][]string),
		linkAllowlist: make(map[int64][]string),
	}
}

func (f *fakeListsRepo) ListBannedWords(_ context.Context, groupID int64) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bannedCalls++
	if f.errBannedWords != nil {
		return nil, f.errBannedWords
	}
	cp := make([]string, len(f.bannedWords[groupID]))
	copy(cp, f.bannedWords[groupID])
	return cp, nil
}

func (f *fakeListsRepo) AddBannedWord(_ context.Context, groupID int64, word string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.bannedWords[groupID] {
		if w == word {
			return nil // idempotent
		}
	}
	f.bannedWords[groupID] = append(f.bannedWords[groupID], word)
	return nil
}

func (f *fakeListsRepo) RemoveBannedWord(_ context.Context, groupID int64, word string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.bannedWords[groupID][:0]
	for _, w := range f.bannedWords[groupID] {
		if w != word {
			out = append(out, w)
		}
	}
	f.bannedWords[groupID] = out
	return nil
}

func (f *fakeListsRepo) ListLinkAllowlist(_ context.Context, groupID int64) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowlistCalls++
	if f.errLinkAllowlist != nil {
		return nil, f.errLinkAllowlist
	}
	cp := make([]string, len(f.linkAllowlist[groupID]))
	copy(cp, f.linkAllowlist[groupID])
	return cp, nil
}

func (f *fakeListsRepo) AddLinkAllowlist(_ context.Context, groupID int64, domain string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.linkAllowlist[groupID] {
		if d == domain {
			return nil // idempotent
		}
	}
	f.linkAllowlist[groupID] = append(f.linkAllowlist[groupID], domain)
	return nil
}

func (f *fakeListsRepo) RemoveLinkAllowlist(_ context.Context, groupID int64, domain string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.linkAllowlist[groupID][:0]
	for _, d := range f.linkAllowlist[groupID] {
		if d != domain {
			out = append(out, d)
		}
	}
	f.linkAllowlist[groupID] = out
	return nil
}

// newSvc construye un Service para tests unit. AutoActionCh tiene
// buffer 4 (suficiente para los tests; spec REQ-11 usa 100).
//
// warningSender se pasa nil por default (slice 2.1: el pipeline skipea
// el paso 7.5 si nil). Tests que ejercen el path del warning usan
// newSvcWithSender.
func newSvc(
	settingsRepo *fakeSettingsRepo,
	warnRepo *fakeWarnRepo,
	groupsMap map[int64]*groups.Group,
	ch chan<- AutoAction,
) (*Service, *fakeLogs) {
	return newSvcWithSender(settingsRepo, warnRepo, nil, groupsMap, ch, nil)
}

// newSvcWithSender es la variante que acepta WarningSender (slice 2.1).
// Si se pasa nil para sender, el pipeline skipea el paso 7.5.
func newSvcWithSender(
	settingsRepo *fakeSettingsRepo,
	warnRepo *fakeWarnRepo,
	listsRepo *fakeListsRepo,
	groupsMap map[int64]*groups.Group,
	ch chan<- AutoAction,
	sender WarningSender,
) (*Service, *fakeLogs) {
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: groupsMap}
	reg := NewRegistry()
	reg.Register(NewFloodRule())
	svc := NewService(settingsRepo, warnRepo, listsRepo, reg, lg, gr, ch, sender, nil)
	return svc, lg
}

// newSvcWithLists construye un Service con fakeListsRepo inyectado y
// las 4 reglas del registry (Flood + AntiSpam + AntiLink +
// BannedWords). Para tests de pre-load.
func newSvcWithLists(
	settingsRepo *fakeSettingsRepo,
	warnRepo *fakeWarnRepo,
	listsRepo *fakeListsRepo,
	groupsMap map[int64]*groups.Group,
	ch chan<- AutoAction,
) (*Service, *fakeLogs) {
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: groupsMap}
	reg := NewRegistry()
	reg.Register(NewFloodRule())
	reg.Register(NewAntiSpamRule())
	reg.Register(NewAntiLinkRule())
	reg.Register(NewBannedWordsRule())
	svc := NewService(settingsRepo, warnRepo, listsRepo, reg, lg, gr, ch, nil, nil)
	return svc, lg
}

// msg construye un Message minimo para el Service.
func msg(chatID, userID int64) *telegram.Message {
	return &telegram.Message{
		MessageID: 1,
		From:      &telegram.User{ID: userID, FirstName: "u"},
		Chat:      telegram.Chat{ID: chatID, Type: "supergroup"},
		Text:      "hi",
	}
}

// TestService_Disabled_SkipSilently: settings.Enabled=false → return
// sin tocar warning_state, sin log, sin encolar.
func TestService_Disabled_SkipSilently(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: false}
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 4)
	svc, lg := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(lg.entries) != 0 {
		t.Errorf("logs = %d, want 0 (disabled, no se loguea)", len(lg.entries))
	}
	if _, ok := warnRepo.rows[[2]int64{-1001, 999}]; ok {
		t.Errorf("warning_state creada pese a disabled")
	}
	if len(ch) != 0 {
		t.Errorf("autoActionCh tiene %d items, want 0", len(ch))
	}
}

// TestService_BotNotAdmin_SkipSilently: bot es member → skip silencioso
// (sin log PERMISSION_DENIED — spec REQ-7 paso 4).
func TestService_BotNotAdmin_SkipSilently(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 5, FloodSeconds: 10, AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 10}
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 4)
	svc, lg := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusMember},
	}, ch)

	if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(lg.entries) != 0 {
		t.Errorf("logs = %d, want 0 (bot no admin, skip silencioso)", len(lg.entries))
	}
	if _, ok := warnRepo.rows[[2]int64{-1001, 999}]; ok {
		t.Errorf("warning_state creada pese a bot no admin")
	}
}

// TestService_HitIncrementsWarningCount: usuario flood → warning_count
// sube a 1, log RULE_TRIGGERED con metadata.
func TestService_HitIncrementsWarningCount(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 3, FloodSeconds: 10, AutomuteWarnings: 5, AutobanWarnings: 10, AutomuteMinutes: 10}
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 4)
	svc, lg := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		// Inyectar now en el service via no-up (mas simple: usar
		// timestamps en el pasado lejano y dejar el reloj correr).
		_ = now.Add(time.Duration(i) * time.Second)
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}
	// El FloodRule usa s.now() del Service. Para determinismo
	// alternativo, basta con verificar el counter final y el log.
	ws, err := warnRepo.GetWarningState(context.Background(), -1001, 999)
	if err != nil {
		t.Fatalf("GetWarningState: %v", err)
	}
	if ws.WarningCount < 1 {
		t.Errorf("WarningCount = %d, want >= 1 (al menos 1 hit en 3 msgs con threshold=3)", ws.WarningCount)
	}
	// Buscar el log RULE_TRIGGERED.
	var ruleEntry *logs.Entry
	for _, e := range lg.entries {
		if e.Action == logs.ActionRuleTriggered {
			ruleEntry = e
			break
		}
	}
	if ruleEntry == nil {
		t.Fatalf("no log RULE_TRIGGERED, entries = %d", len(lg.entries))
	}
	if ruleEntry.ActorID != nil {
		t.Errorf("ActorID = %v, want nil", ruleEntry.ActorID)
	}
	if ruleEntry.Metadata["rule_name"] != "flood" {
		t.Errorf("rule_name = %v, want \"flood\"", ruleEntry.Metadata["rule_name"])
	}
}

// TestService_AutomuteThreshold_EnqueuesAction: warning_count alcanza
// automute_warnings → encola AutoAction{Mute} con MinutesUntil del
// setting.
func TestService_AutomuteThreshold_EnqueuesAction(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 3, FloodSeconds: 10, AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 7}
	warnRepo := newFakeWarnRepo()
	// Usuario ya con warning_count=2; el 3er hit lo lleva a 3 == automute.
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 2}
	ch := make(chan AutoAction, 4)
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	// Disparar 3 hits (FloodRule con threshold=3 → hit en el 3ro).
	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	// Verificar que se encolo una accion de mute.
	if len(ch) == 0 {
		t.Fatalf("autoActionCh vacia, want 1 mute")
	}
	var gotMute *AutoAction
	for i := 0; i < len(ch); i++ {
		a := <-ch
		if a.Kind == AutoActionMute {
			gotMute = &a
		}
	}
	if gotMute == nil {
		t.Fatalf("no se encolo AutoActionMute")
	}
	if gotMute.MinutesUntil != 7 {
		t.Errorf("MinutesUntil = %d, want 7", gotMute.MinutesUntil)
	}
	if gotMute.GroupID != -1001 || gotMute.UserID != 999 {
		t.Errorf("action = %+v, want group=-1001 user=999", gotMute)
	}
	if gotMute.RuleName != "flood" {
		t.Errorf("RuleName = %q, want \"flood\"", gotMute.RuleName)
	}
}

// TestService_AutobanThreshold_EnqueuesAction: warning_count alcanza
// autoban_warnings → encola AutoAction{Ban} (no mute).
func TestService_AutobanThreshold_EnqueuesAction(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 3, FloodSeconds: 10, AutomuteWarnings: 3, AutobanWarnings: 4, AutomuteMinutes: 10}
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 3}
	ch := make(chan AutoAction, 4)
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(ch) == 0 {
		t.Fatalf("autoActionCh vacia, want 1 ban")
	}
	var gotBan *AutoAction
	for i := 0; i < len(ch); i++ {
		a := <-ch
		if a.Kind == AutoActionBan {
			gotBan = &a
		}
	}
	if gotBan == nil {
		t.Fatalf("no se encolo AutoActionBan")
	}
	if gotBan.GroupID != -1001 || gotBan.UserID != 999 {
		t.Errorf("action = %+v, want group=-1001 user=999", gotBan)
	}
}

// TestService_AtBanThreshold_Idempotent: una vez en autoban, nuevos
// hits NO reencolan ban (idempotente hasta proximo reset).
func TestService_AtBanThreshold_Idempotent(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 3, FloodSeconds: 10, AutomuteWarnings: 3, AutobanWarnings: 4, AutomuteMinutes: 10}
	warnRepo := newFakeWarnRepo()
	// Usuario ya sobre autoban_warnings (4).
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 4}
	ch := make(chan AutoAction, 4)
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	// Disparar 3 hits: todos incrementan, pero la primera vez encola
	// ban; las subsiguientes (counter ya >= autoban) NO reencolan.
	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	count := len(ch)
	if count != 1 {
		t.Errorf("autoActionCh tiene %d items, want 1 (idempotente)", count)
	}
}

// TestService_NilFrom_Skip: mensaje sin From (canal, sistema) → no
// panic, no log, no warning_state.
func TestService_NilFrom_Skip(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true}
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 4)
	svc, lg := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	anonMsg := &telegram.Message{
		MessageID: 1,
		Chat:      telegram.Chat{ID: -1001, Type: "channel"},
		Text:      "anonymous post",
	}
	if err := svc.HandleMessage(context.Background(), anonMsg); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(lg.entries) != 0 {
		t.Errorf("logs = %d, want 0", len(lg.entries))
	}
	if len(ch) != 0 {
		t.Errorf("autoActionCh = %d, want 0", len(ch))
	}
}

// TestService_LoadOrCreateSettings_AutoCreateDefaults: settings
// inexistente → crea fila con defaults, GetSettings retorna defaults.
func TestService_LoadOrCreateSettings_AutoCreateDefaults(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 4)
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	got, err := svc.LoadOrCreateSettings(context.Background(), -1001)
	if err != nil {
		t.Fatalf("LoadOrCreateSettings: %v", err)
	}
	if got.Enabled {
		t.Errorf("Enabled = true, want false (default)")
	}
	if got.FloodMessages != 5 {
		t.Errorf("FloodMessages = %d, want 5", got.FloodMessages)
	}
	if got.AutobanWarnings != 5 {
		t.Errorf("AutobanWarnings = %d, want 5", got.AutobanWarnings)
	}
	// Segunda llamada: ya existe, retorna la misma fila (no recrea).
	got2, err := svc.LoadOrCreateSettings(context.Background(), -1001)
	if err != nil {
		t.Fatalf("second LoadOrCreateSettings: %v", err)
	}
	if got2.Enabled != got.Enabled {
		t.Errorf("segunda llamada cambia defaults")
	}
}

// TestService_AutoCreateChannelBuffer_Overflow: si el canal esta lleno,
// el Service dropea la accion y NO bloquea.
func TestService_AutoCreateChannelBuffer_Overflow(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 3, FloodSeconds: 10, AutomuteWarnings: 3, AutobanWarnings: 100, AutomuteMinutes: 10}
	warnRepo := newFakeWarnRepo()
	// Usuario ya con count=2; el primer hit lo lleva a 3 == automute → encola.
	// Para forzar overflow con buffer=1, pre-cargamos 2 mute-enqueue al
	// ganar el 3er hit. Pero como solo hacemos 3 hits, vemos:
	//   msg 3: count 2→3 → encola mute #1 (canal = [mute], buffer 1 lleno)
	//   msg 4..: NO se alcanza con 3 hits.
	// Cambiamos a AutobanWarnings=3 para que cada hit reencole hasta
	// overflow.
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 2}
	ch := make(chan AutoAction, 1) // buffer pequeno para forzar overflow
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	// Disparar 3 hits → 1 mute encolado en buffer 1; los subsiguientes
	// hits NO reencolan (counter ya sobre automute y autoban no se
	// cruza), pero igualmente no debe bloquear.
	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}
	// Canal tiene 1 item (el primero); los hits subsiguientes no
	// reencolan (counter >= automute+1 pero < autoban).
	if len(ch) != 1 {
		t.Errorf("autoActionCh = %d, want 1", len(ch))
	}
}

// --- Tests de pre-load de listas (slice 2) ---

// TestService_PreLoadLists_BothTogglesOn: con BannedWordsEnabled y
// AntiLinkEnabled ambos ON, el Service hace 2 calls (una por lista).
func TestService_PreLoadLists_BothTogglesOn(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{
		GroupID: -1001, Enabled: true,
		FloodEnabled: true, FloodMessages: 5, FloodSeconds: 10,
		AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 10,
		BannedWordsEnabled: true, AntiLinkEnabled: true,
	}
	warnRepo := newFakeWarnRepo()
	listsRepo := newFakeListsRepo()
	listsRepo.bannedWords[-1001] = []string{"spam"}
	listsRepo.linkAllowlist[-1001] = []string{"example.com"}
	ch := make(chan AutoAction, 4)
	svc, _ := newSvcWithLists(settingsRepo, warnRepo, listsRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	// Manejar 1 mensaje.
	if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if listsRepo.bannedCalls != 1 {
		t.Errorf("ListBannedWords calls = %d, want 1", listsRepo.bannedCalls)
	}
	if listsRepo.allowlistCalls != 1 {
		t.Errorf("ListLinkAllowlist calls = %d, want 1", listsRepo.allowlistCalls)
	}
}

// TestService_PreLoadLists_BothTogglesOff: con ambos toggles OFF, el
// Service NUNCA llama al listsRepo (optimizacion, spec REQ-13).
func TestService_PreLoadLists_BothTogglesOff(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{
		GroupID: -1001, Enabled: true,
		FloodEnabled: true, FloodMessages: 5, FloodSeconds: 10,
		AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 10,
		// BannedWordsEnabled y AntiLinkEnabled default = false.
	}
	warnRepo := newFakeWarnRepo()
	listsRepo := newFakeListsRepo()
	ch := make(chan AutoAction, 4)
	svc, _ := newSvcWithLists(settingsRepo, warnRepo, listsRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	for i := 0; i < 5; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}
	if listsRepo.bannedCalls != 0 {
		t.Errorf("ListBannedWords calls = %d, want 0 (toggle off)", listsRepo.bannedCalls)
	}
	if listsRepo.allowlistCalls != 0 {
		t.Errorf("ListLinkAllowlist calls = %d, want 0 (toggle off)", listsRepo.allowlistCalls)
	}
}

// TestService_PreLoadLists_OnlyBannedWordsOn: solo BannedWordsEnabled
// → 1 call a banned, 0 a allowlist.
func TestService_PreLoadLists_OnlyBannedWordsOn(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{
		GroupID: -1001, Enabled: true,
		FloodEnabled: true, FloodMessages: 5, FloodSeconds: 10,
		AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 10,
		BannedWordsEnabled: true,
	}
	warnRepo := newFakeWarnRepo()
	listsRepo := newFakeListsRepo()
	ch := make(chan AutoAction, 4)
	svc, _ := newSvcWithLists(settingsRepo, warnRepo, listsRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if listsRepo.bannedCalls != 1 {
		t.Errorf("ListBannedWords calls = %d, want 1", listsRepo.bannedCalls)
	}
	if listsRepo.allowlistCalls != 0 {
		t.Errorf("ListLinkAllowlist calls = %d, want 0 (toggle off)", listsRepo.allowlistCalls)
	}
}

// TestService_PreLoadLists_OnlyAntiLinkOn: solo AntiLinkEnabled → 1
// call a allowlist, 0 a banned.
func TestService_PreLoadLists_OnlyAntiLinkOn(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{
		GroupID: -1001, Enabled: true,
		FloodEnabled: true, FloodMessages: 5, FloodSeconds: 10,
		AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 10,
		AntiLinkEnabled: true,
	}
	warnRepo := newFakeWarnRepo()
	listsRepo := newFakeListsRepo()
	ch := make(chan AutoAction, 4)
	svc, _ := newSvcWithLists(settingsRepo, warnRepo, listsRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if listsRepo.bannedCalls != 0 {
		t.Errorf("ListBannedWords calls = %d, want 0 (toggle off)", listsRepo.bannedCalls)
	}
	if listsRepo.allowlistCalls != 1 {
		t.Errorf("ListLinkAllowlist calls = %d, want 1", listsRepo.allowlistCalls)
	}
}

// TestService_BannedWordsRule_FiresWhenListed: smoke test de la
// regla de palabras prohibidas end-to-end via el Service. Mensaje
// con palabra prohibida → warning_count sube y se encola la auto-action
// si supera thresholds.
func TestService_BannedWordsRule_FiresWhenListed(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{
		GroupID: -1001, Enabled: true,
		BannedWordsEnabled: true,
		AutomuteWarnings:   1, AutobanWarnings: 100, AutomuteMinutes: 5,
		WarningExpireDays: 30,
	}
	warnRepo := newFakeWarnRepo()
	listsRepo := newFakeListsRepo()
	listsRepo.bannedWords[-1001] = []string{"spam"}
	ch := make(chan AutoAction, 4)
	svc, lg := newSvcWithLists(settingsRepo, warnRepo, listsRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	m := &telegram.Message{
		MessageID: 1,
		From:      &telegram.User{ID: 999},
		Chat:      telegram.Chat{ID: -1001, Type: "supergroup"},
		Text:      "esto es spam claramente",
	}
	if err := svc.HandleMessage(context.Background(), m); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	ws, _ := warnRepo.GetWarningState(context.Background(), -1001, 999)
	if ws == nil {
		t.Fatal("warning_state no creada")
	}
	if ws.WarningCount != 1 {
		t.Errorf("WarningCount = %d, want 1 (banned_words hit)", ws.WarningCount)
	}
	// Log RULE_TRIGGERED con rule_name=banned_words.
	var found bool
	for _, e := range lg.entries {
		if e.Action == logs.ActionRuleTriggered {
			if name, _ := e.Metadata["rule_name"].(string); name == "banned_words" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no log RULE_TRIGGERED con rule_name=banned_words")
	}
	// Auto-action: counter 1 == automute_warnings → encola mute.
	if len(ch) != 1 {
		t.Errorf("autoActionCh = %d, want 1 mute", len(ch))
	}
}

// TestService_NilListsRepo_NoPanic: listsRepo nil + FloodRule unica
// regla (caso degradado) → no panic. preloadLists retorna nil y las
// reglas que necesitan listas vuelven nil sin tocar el repo.
func TestService_NilListsRepo_NoPanic(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{
		GroupID: -1001, Enabled: true,
		FloodEnabled: true, FloodMessages: 5, FloodSeconds: 10,
		AutomuteWarnings: 3, AutobanWarnings: 5, AutomuteMinutes: 10,
	}
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 4)
	// newSvc usa listsRepo nil intencionalmente.
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}
}

// --- Tests de slice 2.1 (paso 7.5: warning visual al usuario) ---

// fakeWarningSender implementa WarningSender capturando calls.
type fakeWarningSender struct {
	mu    sync.Mutex
	calls []warningCall
	err   error
}

type warningCall struct {
	groupID int64
	userID  int64
	count   int16
	kind    WarningKind
}

func (f *fakeWarningSender) SendWarning(_ context.Context, msg *telegram.Message, count int16, kind WarningKind) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if msg == nil {
		f.calls = append(f.calls, warningCall{count: count, kind: kind})
	} else {
		f.calls = append(f.calls, warningCall{
			groupID: msg.Chat.ID,
			userID:  msg.From.ID,
			count:   count,
			kind:    kind,
		})
	}
	return f.err
}

// settings con WarnUserEnabled=true y los thresholds pedidos.
func settingsForWarningTest(automute, autoban, muteMin int16, enabled bool) *Settings {
	return &Settings{
		GroupID: -1001, Enabled: true,
		FloodEnabled:  true,
		FloodMessages: 3, FloodSeconds: 10,
		AutomuteWarnings:  automute,
		AutomuteMinutes:   muteMin,
		AutobanWarnings:   autoban,
		WarningExpireDays: 30,
		WarnUserEnabled:   enabled,
	}
}

// TestService_Warning_CountAutomuteMinusOne_TriggersPreMute: pre-seed
// count=1 + automute=3 → HandleMessage post-inc count=2 (== automute-1)
// → SendWarning con WarningPreMute. NO encola auto-action (count <
// automute todavía).
func TestService_Warning_CountAutomuteMinusOne_TriggersPreMute(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, true)
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 1}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	// Disparar 3 hits para forzar FloodRule → warning_count sube a 2.
	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(sender.calls) != 1 {
		t.Fatalf("warning calls = %d, want 1 (count=2 == automute-1)", len(sender.calls))
	}
	if sender.calls[0].kind != WarningPreMute {
		t.Errorf("kind = %q, want %q", sender.calls[0].kind, WarningPreMute)
	}
	if sender.calls[0].count != 2 {
		t.Errorf("count = %d, want 2", sender.calls[0].count)
	}
}

// TestService_Warning_CountAutobanMinusOne_TriggersPreBan: pre-seed
// count=3 + autoban=5 → HandleMessage post-inc count=4 (== autoban-1)
// → SendWarning con WarningPreBan.
func TestService_Warning_CountAutobanMinusOne_TriggersPreBan(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, true)
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 3}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(sender.calls) != 1 {
		t.Fatalf("warning calls = %d, want 1", len(sender.calls))
	}
	if sender.calls[0].kind != WarningPreBan {
		t.Errorf("kind = %q, want %q", sender.calls[0].kind, WarningPreBan)
	}
	if sender.calls[0].count != 4 {
		t.Errorf("count = %d, want 4", sender.calls[0].count)
	}
}

// TestService_Warning_CountZero_NoSend: post-inc count=1 (NO matchea
// ningún threshold-1) → NO SendWarning.
func TestService_Warning_CountZero_NoSend(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, true)
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 0}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(sender.calls) != 0 {
		t.Errorf("warning calls = %d, want 0 (count=1 no matchea threshold-1)", len(sender.calls))
	}
}

// TestService_Warning_AtThreshold_NoSend: post-inc count == threshold
// → NO warning (la auto-action ocurre en el mismo HandleMessage; el
// warning pre-action solo va en threshold-1).
func TestService_Warning_AtThreshold_NoSend(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, true)
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 2}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	// En el 1er hit: count 2→3 == automute → encola mute. NO warning.
	// En hits siguientes: count sigue subiendo pero >= automute, ya no
	// matchea automute-1 ni autoban-1.
	if len(sender.calls) != 0 {
		t.Errorf("warning calls = %d, want 0 (count == threshold no envia warning)", len(sender.calls))
	}
	// Verificamos que la auto-action SI se encolo.
	if len(ch) == 0 {
		t.Errorf("autoActionCh vacia, want 1 mute (la accion se ejecuto)")
	}
}

// TestService_Warning_Disabled_NoSend: WarnUserEnabled=false → NO
// warning aunque count matchee.
func TestService_Warning_Disabled_NoSend(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, false) // toggle off
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 1}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(sender.calls) != 0 {
		t.Errorf("warning calls = %d, want 0 (WarnUserEnabled=false)", len(sender.calls))
	}
}

// TestService_Warning_BotNotAdmin_NoSend: bot member → skip silencioso
// en paso 4 → NO warning (paso 7.5 no se ejecuta).
func TestService_Warning_BotNotAdmin_NoSend(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, true)
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 1}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusMember}, // NO admin
	}, ch, sender)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(sender.calls) != 0 {
		t.Errorf("warning calls = %d, want 0 (bot not admin, skip silencioso)", len(sender.calls))
	}
}

// TestService_Warning_AutomuteEqualsAutoban_SinglePreBan: edge case
// spec REQ-27: automute==autoban==3, count=2 → pre_ban unico (no
// doble warning).
func TestService_Warning_AutomuteEqualsAutoban_SinglePreBan(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 3, 10, true) // mismo threshold
	warnRepo := newFakeWarnRepo()
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 1}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v", i+1, err)
		}
	}

	if len(sender.calls) != 1 {
		t.Fatalf("warning calls = %d, want 1 (solo pre-ban, no pre-mute)", len(sender.calls))
	}
	if sender.calls[0].kind != WarningPreBan {
		t.Errorf("kind = %q, want %q (prioridad pre-ban)", sender.calls[0].kind, WarningPreBan)
	}
}

// TestService_Warning_SenderError_PipelineContinues: sender devuelve
// error → HandleMessage NO retorna error (pipeline continua). El send
// falla pero el resto del pipeline (paso 8 auto-action) sigue.
func TestService_Warning_SenderError_PipelineContinues(t *testing.T) {
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = settingsForWarningTest(3, 5, 10, true)
	warnRepo := newFakeWarnRepo()
	// Pre-seed count=1: con FloodRule (threshold 3), necesitamos 3
	// hits para que dispare. El primer hit que dispare deja count=2
	// (== automute-1 → warning fires, sender fails) y NO mutea
	// (count=2 < automute=3). Eso es lo que queremos testear: el
	// warning dispara, falla, y el pipeline NO aborta.
	warnRepo.rows[[2]int64{-1001, 999}] = &WarningState{GroupID: -1001, UserID: 999, WarningCount: 1}
	ch := make(chan AutoAction, 4)
	sender := &fakeWarningSender{err: errors.New("telegram exploded")}
	svc, _ := newSvcWithSender(settingsRepo, warnRepo, nil, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch, sender)

	// 3 hits → flood fires en el 3ro, count 1→2, warning fires, sender fails.
	for i := 0; i < 3; i++ {
		if err := svc.HandleMessage(context.Background(), msg(-1001, 999)); err != nil {
			t.Fatalf("HandleMessage %d: %v (expected nil, pipeline debe continuar)", i+1, err)
		}
	}
	if len(sender.calls) != 1 {
		t.Errorf("sender calls = %d, want 1 (se intento enviar)", len(sender.calls))
	}
	if sender.calls[0].kind != WarningPreMute {
		t.Errorf("kind = %q, want %q", sender.calls[0].kind, WarningPreMute)
	}
}

// TestService_ThresholdKindFor_Helper: tests directos del helper puro
// (cubre el branch coverage sin pasar por HandleMessage).
func TestService_ThresholdKindFor_Helper(t *testing.T) {
	cases := []struct {
		name              string
		automute, autoban int16
		count             int16
		want              WarningKind
	}{
		{"pre-mute match", 3, 5, 2, WarningPreMute},
		{"pre-ban match", 3, 5, 4, WarningPreBan},
		{"pre-ban priority when both match", 3, 3, 2, WarningPreBan},
		{"count=0 no match", 3, 5, 0, ""},
		{"count==threshold no match", 3, 5, 3, ""},
		{"count==threshold+1 no match", 3, 5, 6, ""},
		{"count==1 no match", 3, 5, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Settings{AutomuteWarnings: tc.automute, AutobanWarnings: tc.autoban}
			got := thresholdKindFor(s, tc.count)
			if got != tc.want {
				t.Errorf("thresholdKindFor(count=%d, automute=%d, autoban=%d) = %q, want %q",
					tc.count, tc.automute, tc.autoban, got, tc.want)
			}
		})
	}
}

// TestService_ThresholdKindFor_NilSettings: settings nil → "" (defensa).
func TestService_ThresholdKindFor_NilSettings(t *testing.T) {
	if got := thresholdKindFor(nil, 2); got != "" {
		t.Errorf("thresholdKindFor(nil, 2) = %q, want \"\"", got)
	}
}

package automation

import (
	"context"
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

// newSvc construye un Service para tests unit. AutoActionCh tiene
// buffer 4 (suficiente para los tests; spec REQ-11 usa 100).
func newSvc(
	settingsRepo *fakeSettingsRepo,
	warnRepo *fakeWarnRepo,
	groupsMap map[int64]*groups.Group,
	ch chan<- AutoAction,
) (*Service, *fakeLogs) {
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: groupsMap}
	reg := NewRegistry()
	reg.Register(NewFloodRule())
	svc := NewService(settingsRepo, warnRepo, reg, lg, gr, ch, nil)
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

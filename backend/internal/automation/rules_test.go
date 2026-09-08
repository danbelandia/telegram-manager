package automation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/telegram"
)

// fakeMsg construye un Message minimo para tests de reglas. chatID
// identifica al grupo; userID es el autor.
func fakeMsg(chatID, userID int64, text string) *telegram.Message {
	return &telegram.Message{
		MessageID: 1,
		From:      &telegram.User{ID: userID, FirstName: "u"},
		Chat:      telegram.Chat{ID: chatID, Type: "supergroup"},
		Text:      text,
	}
}

// baseSettings devuelve settings con flood enabled y los thresholds
// que el caller pase.
func baseSettings(chatID int64, msgs int16, secs int16) *Settings {
	return &Settings{
		GroupID:          chatID,
		Enabled:          true,
		FloodEnabled:     true,
		FloodMessages:    msgs,
		FloodSeconds:     secs,
		AutomuteWarnings: 3,
		AutobanWarnings:  5,
	}
}

// TestFloodRule_BelowThreshold_NoHit: 4 mensajes en ventana con
// threshold=5 → no hit.
func TestFloodRule_BelowThreshold_NoHit(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)
	ws := &WarningState{GroupID: -1001, UserID: 999}

	for i := 0; i < 4; i++ {
		hit := rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, nil, now.Add(time.Duration(i)*time.Second))
		if hit != nil {
			t.Fatalf("msg %d: hit = %+v, want nil", i+1, hit)
		}
	}
}

// TestFloodRule_AtThreshold_Hit: 5 mensajes en 10s con threshold=5 →
// hit en el 5to mensaje.
func TestFloodRule_AtThreshold_Hit(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)
	ws := &WarningState{GroupID: -1001, UserID: 999}

	var lastHit *RuleHit
	for i := 0; i < 5; i++ {
		lastHit = rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, nil, now.Add(time.Duration(i)*time.Second))
	}
	if lastHit == nil {
		t.Fatal("hit = nil, want no-nil en el 5to mensaje")
	}
	if lastHit.RuleName != "flood" {
		t.Errorf("RuleName = %q, want \"flood\"", lastHit.RuleName)
	}
	if !strings.Contains(lastHit.Reason, "5 messages in 10 seconds") {
		t.Errorf("Reason = %q, want contiene \"5 messages in 10 seconds\"", lastHit.Reason)
	}
}

// TestFloodRule_OverThreshold_Hit: 8 mensajes en 10s con threshold=5 →
// hit en el 5to (y siguientes) mensaje.
func TestFloodRule_OverThreshold_Hit(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)
	ws := &WarningState{GroupID: -1001, UserID: 999}

	hits := 0
	for i := 0; i < 8; i++ {
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, nil, now.Add(time.Duration(i)*time.Second)); hit != nil {
			hits++
		}
	}
	if hits != 4 {
		// mensajes 5, 6, 7, 8 disparan (los 4 primeros no).
		t.Errorf("hits = %d, want 4 (msgs 5..8)", hits)
	}
}

// TestFloodRule_WindowExpiry_ResetsAfterWindow: 5 mensajes en t=0..4s,
// luego un mensaje en t=30s con window=10s → no hit (la ventana expiro
// y solo queda el ultimo mensaje).
func TestFloodRule_WindowExpiry_ResetsAfterWindow(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)
	ws := &WarningState{GroupID: -1001, UserID: 999}

	// 5 mensajes seguidos: el ultimo dispara hit (5 en 10s).
	hit := (*RuleHit)(nil)
	for i := 0; i < 5; i++ {
		hit = rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, nil, now.Add(time.Duration(i)*time.Second))
	}
	if hit == nil {
		t.Fatalf("setup: hit = nil, queria hit en msg 5")
	}

	// 30 segundos mas tarde, ventana 10s → los 5 mensajes previos
	// quedaron fuera. Un nuevo mensaje: ventana = [nuevo], size=1 < 5.
	hit = rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, nil, now.Add(30*time.Second))
	if hit != nil {
		t.Errorf("hit = %+v, want nil (los 5 previos expiraron)", hit)
	}
}

// TestFloodRule_DifferentUsers_IndependentCounters: user1 manda 4
// mensajes y user2 manda 4 mensajes en la misma ventana, threshold=5
// → ninguno dispara.
func TestFloodRule_DifferentUsers_IndependentCounters(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)

	for i := 0; i < 4; i++ {
		ts := now.Add(time.Duration(i) * time.Second)
		ws1 := &WarningState{GroupID: -1001, UserID: 1001}
		ws2 := &WarningState{GroupID: -1001, UserID: 1002}
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 1001, "a"), s, ws1, nil, ts); hit != nil {
			t.Fatalf("user1 msg %d: hit = %+v, want nil", i+1, hit)
		}
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 1002, "b"), s, ws2, nil, ts); hit != nil {
			t.Fatalf("user2 msg %d: hit = %+v, want nil", i+1, hit)
		}
	}
}

// TestFloodRule_FloodDisabled_NoHitEvenIfCountExceeds: con
// flood_enabled=false, aunque el usuario mande 10 mensajes, no hay
// hit.
func TestFloodRule_FloodDisabled_NoHitEvenIfCountExceeds(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)
	s.FloodEnabled = false
	ws := &WarningState{GroupID: -1001, UserID: 999}

	for i := 0; i < 10; i++ {
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, nil, now.Add(time.Duration(i)*time.Second)); hit != nil {
			t.Errorf("msg %d: hit = %+v, want nil (flood disabled)", i+1, hit)
		}
	}
}

// TestFloodRule_NilFrom_NoHit: mensaje sin From (canal, sistema) → no
// panic y no hit.
func TestFloodRule_NilFrom_NoHit(t *testing.T) {
	rule := NewFloodRule()
	ctx := context.Background()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	s := baseSettings(-1001, 5, 10)
	ws := &WarningState{GroupID: -1001, UserID: 0}

	msg := &telegram.Message{
		MessageID: 1,
		Chat:      telegram.Chat{ID: -1001, Type: "channel"},
		Text:      "system post",
	}
	if hit := rule.Evaluate(ctx, msg, s, ws, nil, now); hit != nil {
		t.Errorf("hit = %+v, want nil (From nil)", hit)
	}
}

// --- Registry tests ---

// TestRegistry_Evaluate_FirstHitShortCircuit: dos reglas registradas;
// R1 siempre dispara, R2 tiene un counter interno que verifica que no
// se llamo.
func TestRegistry_Evaluate_FirstHitShortCircuit(t *testing.T) {
	type alwaysRule struct {
		name  string
		calls *int
	}
	reg := NewRegistry()
	calls1 := 0
	reg.Register(&stubRule{name: "R1", calls: &calls1, hit: &RuleHit{RuleName: "R1"}})
	calls2 := 0
	reg.Register(&stubRule{name: "R2", calls: &calls2, hit: nil})

	hit := reg.Evaluate(context.Background(), fakeMsg(-1001, 1, "x"), baseSettings(-1001, 5, 10), nil, nil, time.Now())
	if hit == nil || hit.RuleName != "R1" {
		t.Fatalf("hit = %+v, want R1", hit)
	}
	if calls1 != 1 {
		t.Errorf("R1 calls = %d, want 1", calls1)
	}
	if calls2 != 0 {
		t.Errorf("R2 calls = %d, want 0 (short-circuit)", calls2)
	}
}

// TestRegistry_Evaluate_NoHit_ReturnsNil: regla que nunca dispara →
// registry retorna nil.
func TestRegistry_Evaluate_NoHit_ReturnsNil(t *testing.T) {
	reg := NewRegistry()
	calls := 0
	reg.Register(&stubRule{name: "never", calls: &calls, hit: nil})

	hit := reg.Evaluate(context.Background(), fakeMsg(-1001, 1, "x"), baseSettings(-1001, 5, 10), nil, nil, time.Now())
	if hit != nil {
		t.Errorf("hit = %+v, want nil", hit)
	}
}

// TestRegistry_Evaluate_NilMessage_ReturnsNil: defensivo, no panic.
func TestRegistry_Evaluate_NilMessage_ReturnsNil(t *testing.T) {
	reg := NewRegistry()
	if hit := reg.Evaluate(context.Background(), nil, baseSettings(-1001, 5, 10), nil, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil", hit)
	}
}

// TestRegistry_Register_DuplicateNameIgnored: registrar dos reglas con
// el mismo nombre -> solo la primera queda.
func TestRegistry_Register_DuplicateNameIgnored(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubRule{name: "X", hit: &RuleHit{RuleName: "X"}})
	reg.Register(&stubRule{name: "X", hit: &RuleHit{RuleName: "X-dup"}})

	rules := reg.Rules()
	if len(rules) != 1 {
		t.Fatalf("len = %d, want 1 (duplicate ignored)", len(rules))
	}
}

// stubRule es una Rule falsa para Registry tests. No es para FloodRule.
type stubRule struct {
	name  string
	calls *int
	hit   *RuleHit
}

func (s *stubRule) Name() string { return s.name }
func (s *stubRule) Evaluate(ctx context.Context, msg *telegram.Message, _ *Settings, _ *WarningState, _ *Lists, _ time.Time) *RuleHit {
	if s.calls != nil {
		*s.calls++
	}
	return s.hit
}

// --- AntiSpamRule ---

// TestAntiSpamRule_AllCaps_Hit: >70% mayusculas con len>10 → hit.
func TestAntiSpamRule_AllCaps_Hit(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "COMPRÁ AHORA MISMO LA PROMOCIÓN"), s, ws, nil, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (all-caps)")
	}
}

// TestAntiSpamRule_AllCaps_NoHitIfShort: texto <=10 chars no dispara
// all-caps (umbral del detector).
func TestAntiSpamRule_AllCaps_NoHitIfShort(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "HOLA"), s, ws, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (len<=10)", hit)
	}
}

// TestAntiSpamRule_RepeatedChars_Hit: 5+ chars iguales consecutivos
// → hit.
func TestAntiSpamRule_RepeatedChars_Hit(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "noooooo se aguanta"), s, ws, nil, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (repeated)")
	}
}

// TestAntiSpamRule_ShortURL_Hit: URL con cuerpo < 20 chars → hit.
func TestAntiSpamRule_ShortURL_Hit(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "mira esto https://t.me/x"), s, ws, nil, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (short URL)")
	}
}

// TestAntiSpamRule_Disabled_NoHit: toggle off → nil.
func TestAntiSpamRule_Disabled_NoHit(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: false}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "COMPRÁ AHORA MISMO LA PROMOCIÓN"), s, ws, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (toggle off)", hit)
	}
}

// TestAntiSpamRule_EmptyText_NoHit: msg sin texto → nil.
func TestAntiSpamRule_EmptyText_NoHit(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, ""), s, ws, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (empty text)", hit)
	}
}

// TestAntiSpamRule_NormalText_NoHit: texto normal sin patrones → nil.
func TestAntiSpamRule_NormalText_NoHit(t *testing.T) {
	rule := NewAntiSpamRule()
	s := &Settings{AntiSpamEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "Hola, como estan todos hoy? Espero que bien."), s, ws, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (normal text)", hit)
	}
}

// --- AntiLinkRule ---

// TestAntiLinkRule_URLHitWhenNoAllowlist: URL fuera de allowlist
// (allowlist vacia) → hit.
func TestAntiLinkRule_URLHitWhenNoAllowlist(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "mira https://example.com/x"), s, ws, nil, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (URL sin allowlist)")
	}
}

// TestAntiLinkRule_ExactDomainAllowed: dominio exacto en allowlist
// → nil.
func TestAntiLinkRule_ExactDomainAllowed(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: true}
	ws := &WarningState{}
	lists := &Lists{LinkAllowlist: []string{"example.com"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "mira https://example.com/x"), s, ws, lists, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (allowlist exacto)", hit)
	}
}

// TestAntiLinkRule_SubdomainAllowed: subdominio de allowlist → nil.
func TestAntiLinkRule_SubdomainAllowed(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: true}
	ws := &WarningState{}
	lists := &Lists{LinkAllowlist: []string{"example.com"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "mira https://sub.example.com/x"), s, ws, lists, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (subdominio permitido)", hit)
	}
}

// TestAntiLinkRule_PrefixFalseRejected: "notexample.com" NO matchea
// "example.com" (precedente bugfix comun). El helper exige "." antes.
func TestAntiLinkRule_PrefixFalseRejected(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: true}
	ws := &WarningState{}
	lists := &Lists{LinkAllowlist: []string{"example.com"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "mira https://notexample.com/x"), s, ws, lists, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (notexample.com NO es subdominio)")
	}
}

// TestAntiLinkRule_Disabled_NoHit: toggle off → nil (incluso si hay
// URLs).
func TestAntiLinkRule_Disabled_NoHit(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: false}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "https://example.com"), s, ws, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (toggle off)", hit)
	}
}

// TestAntiLinkRule_NilLists_HitsAnything: sin listas (toggle on pero
// service omitio el pre-load) → cualquier URL es hit. Defensivo.
func TestAntiLinkRule_NilLists_HitsAnything(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "https://example.com"), s, ws, nil, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (lists nil, URL presente)")
	}
}

// TestAntiLinkRule_NoURL_NoHit: mensaje sin URLs → nil.
func TestAntiLinkRule_NoURL_NoHit(t *testing.T) {
	rule := NewAntiLinkRule()
	s := &Settings{AntiLinkEnabled: true}
	ws := &WarningState{}
	lists := &Lists{LinkAllowlist: []string{"example.com"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "Hola mundo, sin links."), s, ws, lists, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (sin URL)", hit)
	}
}

// --- BannedWordsRule ---

// TestBannedWordsRule_CaseInsensitive_Hit: palabra en MAYUSCULAS en el
// texto matchea lista en minusculas (case-insensitive substring).
func TestBannedWordsRule_CaseInsensitive_Hit(t *testing.T) {
	rule := NewBannedWordsRule()
	s := &Settings{BannedWordsEnabled: true}
	ws := &WarningState{}
	lists := &Lists{BannedWords: []string{"spam"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "esto es SPAM claramente"), s, ws, lists, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (case-insensitive)")
	}
}

// TestBannedWordsRule_SubstringMatch: matchea como substring, no como
// token entero.
func TestBannedWordsRule_SubstringMatch(t *testing.T) {
	rule := NewBannedWordsRule()
	s := &Settings{BannedWordsEnabled: true}
	ws := &WarningState{}
	lists := &Lists{BannedWords: []string{"free"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "get freemoney here"), s, ws, lists, time.Now()); hit == nil {
		t.Error("hit = nil, want no-nil (substring match)")
	}
}

// TestBannedWordsRule_EmptyList_NoHit: lista vacia → nil.
func TestBannedWordsRule_EmptyList_NoHit(t *testing.T) {
	rule := NewBannedWordsRule()
	s := &Settings{BannedWordsEnabled: true}
	ws := &WarningState{}
	lists := &Lists{BannedWords: []string{}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "spam viagra casino"), s, ws, lists, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (empty list)", hit)
	}
}

// TestBannedWordsRule_Disabled_NoHit: toggle off → nil.
func TestBannedWordsRule_Disabled_NoHit(t *testing.T) {
	rule := NewBannedWordsRule()
	s := &Settings{BannedWordsEnabled: false}
	ws := &WarningState{}
	lists := &Lists{BannedWords: []string{"spam"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "esto es spam"), s, ws, lists, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (toggle off)", hit)
	}
}

// TestBannedWordsRule_NilLists_NoHit: lists nil → nil (defensivo).
func TestBannedWordsRule_NilLists_NoHit(t *testing.T) {
	rule := NewBannedWordsRule()
	s := &Settings{BannedWordsEnabled: true}
	ws := &WarningState{}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "spam"), s, ws, nil, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (lists nil)", hit)
	}
}

// TestBannedWordsRule_NoMatch_NoHit: texto sin palabras prohibidas
// → nil.
func TestBannedWordsRule_NoMatch_NoHit(t *testing.T) {
	rule := NewBannedWordsRule()
	s := &Settings{BannedWordsEnabled: true}
	ws := &WarningState{}
	lists := &Lists{BannedWords: []string{"spam", "viagra"}}
	if hit := rule.Evaluate(context.Background(), fakeMsg(-1001, 1, "hola mundo normal"), s, ws, lists, time.Now()); hit != nil {
		t.Errorf("hit = %+v, want nil (no match)", hit)
	}
}

// --- Registry order (cheap → expensive) ---

// TestRegistry_OrderCheapFirst: FloodRule (in-mem, cheap) registrado
// primero, BannedWordsRule ultimo. Si el mensaje dispara flood, la
// regla de banned words NUNCA se llama.
func TestRegistry_OrderCheapFirst(t *testing.T) {
	reg := NewRegistry()
	callsFlood := 0
	callsBanned := 0
	// FloodRule fake (siempre hit).
	reg.Register(&stubRule{name: "flood", calls: &callsFlood, hit: &RuleHit{RuleName: "flood"}})
	// BannedWordsRule fake (nunca se debe llamar porque flood short-circuita).
	reg.Register(&stubRule{name: "banned_words", calls: &callsBanned, hit: nil})

	hit := reg.Evaluate(context.Background(), fakeMsg(-1001, 1, "x"), &Settings{}, nil, nil, time.Now())
	if hit == nil || hit.RuleName != "flood" {
		t.Fatalf("hit = %+v, want flood", hit)
	}
	if callsFlood != 1 {
		t.Errorf("flood calls = %d, want 1", callsFlood)
	}
	if callsBanned != 0 {
		t.Errorf("banned_words calls = %d, want 0 (short-circuit)", callsBanned)
	}
}

// --- Rule.Evaluate con lists ---

// TestRule_Evaluate_NilListsDefensive: las 3 reglas nuevas son
// defensivas ante lists nil (no panic). Toggle off + lists nil
// deberia volver nil en todas.
func TestRule_Evaluate_NilListsDefensive(t *testing.T) {
	rules := []Rule{
		NewAntiSpamRule(),
		NewAntiLinkRule(),
		NewBannedWordsRule(),
	}
	for _, r := range rules {
		s := &Settings{AntiSpamEnabled: true, AntiLinkEnabled: true, BannedWordsEnabled: true}
		if hit := r.Evaluate(context.Background(), fakeMsg(-1001, 1, "test"), s, nil, nil, time.Now()); hit != nil {
			t.Errorf("%s.Evaluate con lists nil = %+v, want nil (defensivo)", r.Name(), hit)
		}
	}
}

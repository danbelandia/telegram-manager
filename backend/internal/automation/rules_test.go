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
		hit := rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, now.Add(time.Duration(i)*time.Second))
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
		lastHit = rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, now.Add(time.Duration(i)*time.Second))
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
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, now.Add(time.Duration(i)*time.Second)); hit != nil {
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
		hit = rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, now.Add(time.Duration(i)*time.Second))
	}
	if hit == nil {
		t.Fatalf("setup: hit = nil, queria hit en msg 5")
	}

	// 30 segundos mas tarde, ventana 10s → los 5 mensajes previos
	// quedaron fuera. Un nuevo mensaje: ventana = [nuevo], size=1 < 5.
	hit = rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, now.Add(30*time.Second))
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
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 1001, "a"), s, ws1, ts); hit != nil {
			t.Fatalf("user1 msg %d: hit = %+v, want nil", i+1, hit)
		}
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 1002, "b"), s, ws2, ts); hit != nil {
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
		if hit := rule.Evaluate(ctx, fakeMsg(-1001, 999, "hi"), s, ws, now.Add(time.Duration(i)*time.Second)); hit != nil {
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
	if hit := rule.Evaluate(ctx, msg, s, ws, now); hit != nil {
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

	hit := reg.Evaluate(context.Background(), fakeMsg(-1001, 1, "x"), baseSettings(-1001, 5, 10), nil, time.Now())
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

	hit := reg.Evaluate(context.Background(), fakeMsg(-1001, 1, "x"), baseSettings(-1001, 5, 10), nil, time.Now())
	if hit != nil {
		t.Errorf("hit = %+v, want nil", hit)
	}
}

// TestRegistry_Evaluate_NilMessage_ReturnsNil: defensivo, no panic.
func TestRegistry_Evaluate_NilMessage_ReturnsNil(t *testing.T) {
	reg := NewRegistry()
	if hit := reg.Evaluate(context.Background(), nil, baseSettings(-1001, 5, 10), nil, time.Now()); hit != nil {
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
func (s *stubRule) Evaluate(ctx context.Context, msg *telegram.Message, _ *Settings, _ *WarningState, _ time.Time) *RuleHit {
	if s.calls != nil {
		*s.calls++
	}
	return s.hit
}

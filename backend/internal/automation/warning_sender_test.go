// Tests del WarningSender (Fase 3, slice 2.1, REQ-26..28). Cubre:
// happy path → log SUCCESS + SendMessage; permission denied → log
// PERMISSION_DENIED; not found; custom template metadata;
// TELEGRAM_ERROR; empty kind.
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

// fakeTelegramMesseger implementa TelegramMesseger registrando calls.
type fakeTelegramMesseger struct {
	mu    sync.Mutex
	calls []messageCall
	err   error
	delay time.Duration // para tests de timeout
}

type messageCall struct {
	chatID   int64
	text     string
	preview  bool
	keyboard *telegram.InlineKeyboardMarkup
}

func (f *fakeTelegramMesseger) SendMessage(_ context.Context, chatID int64, text string, preview bool, keyboard *telegram.InlineKeyboardMarkup) (int64, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, messageCall{chatID: chatID, text: text, preview: preview, keyboard: keyboard})
	return 12345, f.err
}

// fakeSettingsReader sirve GetSettings desde un mapa en memoria.
type fakeSettingsReader struct {
	mu   sync.Mutex
	rows map[[2]int64]*Settings
	err  error
}

func newFakeSettingsReader() *fakeSettingsReader {
	return &fakeSettingsReader{rows: make(map[[2]int64]*Settings)}
}

func (f *fakeSettingsReader) GetSettings(_ context.Context, tenantID, groupID int64) (*Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.rows[[2]int64{tenantID, groupID}]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

// TestWarningSender_HappyPath_DefaultTemplate: bot admin, template
// default → SendMessage llamado con el texto correcto, log SUCCESS
// con ActorID=nil, metadata {warning_count, threshold_kind,
// template_used="default"}.
func TestWarningSender_HappyPath_DefaultTemplate(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := newFakeSettingsReader()
	// Settings sin custom template → cae al default.
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001, AutomuteMinutes: 10, WarnUserEnabled: true, WarnUserTemplate: nil}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	if err := w.SendWarning(context.Background(), msg, 2, WarningPreMute); err != nil {
		t.Fatalf("SendWarning: %v", err)
	}
	if len(tg.calls) != 1 {
		t.Fatalf("SendMessage calls = %d, want 1", len(tg.calls))
	}
	if tg.calls[0].chatID != -1001 {
		t.Errorf("chatID = %d, want -1001", tg.calls[0].chatID)
	}
	if !contains(tg.calls[0].text, "Juan") {
		t.Errorf("text sin nombre: %q", tg.calls[0].text)
	}
	if !contains(tg.calls[0].text, "10 min") {
		t.Errorf("text sin mute_minutes: %q", tg.calls[0].text)
	}
	if tg.calls[0].keyboard != nil {
		t.Errorf("keyboard = %v, want nil (warning unidireccional)", tg.calls[0].keyboard)
	}
	if tg.calls[0].preview != false {
		t.Errorf("preview = %v, want false", tg.calls[0].preview)
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	e := lg.entries[0]
	if e.Action != logs.ActionWarnUserSent {
		t.Errorf("Action = %q, want %q", e.Action, logs.ActionWarnUserSent)
	}
	if e.ActorID != nil {
		t.Errorf("ActorID = %v, want nil", e.ActorID)
	}
	if e.Status != logs.StatusSuccess {
		t.Errorf("Status = %q, want %q", e.Status, logs.StatusSuccess)
	}
	if e.Metadata["template_used"] != "default" {
		t.Errorf("template_used = %v, want \"default\"", e.Metadata["template_used"])
	}
	if e.Metadata["threshold_kind"] != "pre_mute" {
		t.Errorf("threshold_kind = %v, want \"pre_mute\"", e.Metadata["threshold_kind"])
	}
	if e.Metadata["warning_count"] != int16(2) {
		t.Errorf("warning_count = %v, want 2", e.Metadata["warning_count"])
	}
}

// TestWarningSender_HappyPath_CustomTemplate: template custom → log
// metadata {template_used="custom"}.
func TestWarningSender_HappyPath_CustomTemplate(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := newFakeSettingsReader()
	custom := "🚨 Custom warn para {nombre}: {count} strikes."
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001, AutomuteMinutes: 10, WarnUserEnabled: true, WarnUserTemplate: &custom}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Pedro", "")
	if err := w.SendWarning(context.Background(), msg, 3, WarningPreBan); err != nil {
		t.Fatalf("SendWarning: %v", err)
	}
	if tg.calls[0].text != "🚨 Custom warn para Pedro: 3 strikes." {
		t.Errorf("text = %q", tg.calls[0].text)
	}
	if lg.entries[0].Metadata["template_used"] != "custom" {
		t.Errorf("template_used = %v, want \"custom\"", lg.entries[0].Metadata["template_used"])
	}
	if lg.entries[0].Metadata["threshold_kind"] != "pre_ban" {
		t.Errorf("threshold_kind = %v, want \"pre_ban\"", lg.entries[0].Metadata["threshold_kind"])
	}
}

// TestWarningSender_PermissionDenied: bot member → log
// PERMISSION_DENIED, NO SendMessage.
func TestWarningSender_PermissionDenied(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusMember}, // bot removido
	}}
	sr := newFakeSettingsReader()
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	err := w.SendWarning(context.Background(), msg, 2, WarningPreMute)
	if err == nil {
		t.Fatal("SendWarning = nil, want error")
	}
	if len(tg.calls) != 0 {
		t.Errorf("SendMessage calls = %d, want 0 (no admin)", len(tg.calls))
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	if lg.entries[0].Status != logs.StatusPermissionDenied {
		t.Errorf("Status = %q, want %q", lg.entries[0].Status, logs.StatusPermissionDenied)
	}
}

// TestWarningSender_GroupNotFound: grupo eliminado → log NOT_FOUND,
// NO SendMessage.
func TestWarningSender_GroupNotFound(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{}} // vacio
	sr := newFakeSettingsReader()
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	if err := w.SendWarning(context.Background(), msg, 2, WarningPreMute); err == nil {
		t.Fatal("SendWarning = nil, want error")
	}
	if len(tg.calls) != 0 {
		t.Errorf("SendMessage calls = %d, want 0", len(tg.calls))
	}
	if lg.entries[0].Status != logs.StatusNotFound {
		t.Errorf("Status = %q, want %q", lg.entries[0].Status, logs.StatusNotFound)
	}
}

// TestWarningSender_TelegramError: telegram devuelve error generico →
// log TELEGRAM_ERROR, error propagado al caller.
func TestWarningSender_TelegramError(t *testing.T) {
	tg := &fakeTelegramMesseger{err: telegram.ErrTelegramUnavailable}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := newFakeSettingsReader()
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	err := w.SendWarning(context.Background(), msg, 2, WarningPreMute)
	if !errors.Is(err, telegram.ErrTelegramUnavailable) {
		t.Errorf("err = %v, want ErrTelegramUnavailable", err)
	}
	if len(tg.calls) != 1 {
		t.Errorf("SendMessage calls = %d, want 1 (se intento enviar)", len(tg.calls))
	}
	if lg.entries[0].Status != logs.StatusTelegramError {
		t.Errorf("Status = %q, want %q", lg.entries[0].Status, logs.StatusTelegramError)
	}
	if lg.entries[0].ErrorMessage == nil {
		t.Error("ErrorMessage = nil, want contiene mensaje del adapter")
	}
}

// TestWarningSender_TelegramPermissionDenied_MappedToStatus: el adapter
// devuelve ErrPermissionDenied → log mapeado a PERMISSION_DENIED.
func TestWarningSender_TelegramPermissionDenied_MappedToStatus(t *testing.T) {
	tg := &fakeTelegramMesseger{err: telegram.ErrPermissionDenied}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := newFakeSettingsReader()
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	err := w.SendWarning(context.Background(), msg, 2, WarningPreMute)
	if !errors.Is(err, telegram.ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
	if lg.entries[0].Status != logs.StatusPermissionDenied {
		t.Errorf("Status = %q, want %q", lg.entries[0].Status, logs.StatusPermissionDenied)
	}
}

// TestWarningSender_EmptyKind: kind vacio → error sin log ni send.
func TestWarningSender_EmptyKind(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := newFakeSettingsReader()
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	if err := w.SendWarning(context.Background(), msg, 2, ""); err == nil {
		t.Fatal("SendWarning = nil, want error (kind vacio)")
	}
	if len(tg.calls) != 0 {
		t.Errorf("SendMessage calls = %d, want 0", len(tg.calls))
	}
	if len(lg.entries) != 0 {
		t.Errorf("logs = %d, want 0 (no se intento)", len(lg.entries))
	}
}

// TestWarningSender_NilMsg: msg nil → error sin log ni send.
func TestWarningSender_NilMsg(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := newFakeSettingsReader()
	sr.rows[[2]int64{testTenantID, -1001}] = &Settings{TenantID: testTenantID, GroupID: -1001}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	if err := w.SendWarning(context.Background(), nil, 2, WarningPreMute); err == nil {
		t.Fatal("SendWarning = nil, want error (msg nil)")
	}
	if len(tg.calls) != 0 {
		t.Errorf("SendMessage calls = %d, want 0", len(tg.calls))
	}
	if len(lg.entries) != 0 {
		t.Errorf("logs = %d, want 0", len(lg.entries))
	}
}

// TestWarningSender_SettingsReadFails_UsesDefault: si GetSettings
// falla, usa defaults puros (log warn, no error fatal). SendMessage
// igual se llama con el template default.
func TestWarningSender_SettingsReadFails_UsesDefault(t *testing.T) {
	tg := &fakeTelegramMesseger{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	sr := &fakeSettingsReader{err: errors.New("db exploded")}
	w := NewWarningSender(tg, lg, sr, gr, nil, testTenantID)

	msg := makeMsg("Juan", "")
	if err := w.SendWarning(context.Background(), msg, 2, WarningPreMute); err != nil {
		t.Fatalf("SendWarning: %v", err)
	}
	if len(tg.calls) != 1 {
		t.Errorf("SendMessage calls = %d, want 1 (fallback a default)", len(tg.calls))
	}
	if !contains(tg.calls[0].text, "Juan") {
		t.Errorf("text sin nombre: %q", tg.calls[0].text)
	}
	if lg.entries[0].Metadata["template_used"] != "default" {
		t.Errorf("template_used = %v, want \"default\" (fallback)", lg.entries[0].Metadata["template_used"])
	}
}

// contains es un helper minimalista para evitar importar strings solo
// para Contains.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

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

// fakeTelegramActor implementa TelegramActor registrando las llamadas.
// Soporta inyectar un error fijo o por-llamada.
type fakeTelegramActor struct {
	mu        sync.Mutex
	muteCalls []muteCall
	banCalls  []banCall
	// err: si esta set, todas las llamadas devuelven este error.
	err error
	// nowOverride: si esta set, untilDate se ignora (tests verifican por
	// otra via). Default: usamos el reloj real del wrapper.
	nowOverride time.Time
}

type muteCall struct {
	chatID    int64
	userID    int64
	untilDate int64
}

type banCall struct {
	chatID         int64
	userID         int64
	untilDate      int64
	revokeMessages bool
}

func (f *fakeTelegramActor) MuteUser(_ context.Context, chatID, userID int64, untilDate int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.muteCalls = append(f.muteCalls, muteCall{chatID: chatID, userID: userID, untilDate: untilDate})
	return f.err
}

func (f *fakeTelegramActor) BanUser(_ context.Context, chatID, userID int64, untilDate int64, revokeMessages bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.banCalls = append(f.banCalls, banCall{chatID: chatID, userID: userID, untilDate: untilDate, revokeMessages: revokeMessages})
	return f.err
}

// fakeLogs implementa LogWriter capturando entries.
type fakeLogs struct {
	mu      sync.Mutex
	entries []*logs.Entry
	err     error
}

func (f *fakeLogs) Create(_ context.Context, e *logs.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.entries = append(f.entries, e)
	return nil
}

// fakeGroups implementa GroupReader con un mapa en memoria.
type fakeGroups struct {
	groups map[int64]*groups.Group
	err    error
}

func (f *fakeGroups) GetByTelegramID(_ context.Context, id int64) (*groups.Group, error) {
	if f.err != nil {
		return nil, f.err
	}
	g, ok := f.groups[id]
	if !ok {
		return nil, groups.ErrNotFound
	}
	return g, nil
}

// TestAutoActioner_Mute_HappyPath: bot admin, mute OK → log SUCCESS
// con ActorID=nil, metadata {rule_name, warning_count}, y tg.MuteUser
// llamado con untilDate ~= now + minutes*60.
func TestAutoActioner_Mute_HappyPath(t *testing.T) {
	tg := &fakeTelegramActor{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	a := NewAutoActioner(tg, lg, gr, nil)

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	wrapper := a.(*tgAutoActioner)
	wrapper.now = func() time.Time { return now }

	action := AutoAction{
		Kind:         AutoActionMute,
		GroupID:      -1001,
		UserID:       999,
		MinutesUntil: 10,
		RuleName:     "flood",
		WarningCount: 3,
	}
	if err := a.Execute(context.Background(), action); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(tg.muteCalls) != 1 {
		t.Fatalf("muteCalls = %d, want 1", len(tg.muteCalls))
	}
	wantUntil := now.Add(10 * time.Minute).Unix()
	if tg.muteCalls[0].untilDate != wantUntil {
		t.Errorf("untilDate = %d, want %d", tg.muteCalls[0].untilDate, wantUntil)
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	e := lg.entries[0]
	if e.ActorID != nil {
		t.Errorf("ActorID = %v, want nil (system event)", e.ActorID)
	}
	if e.Action != logs.ActionAutomuteUser {
		t.Errorf("Action = %q, want %q", e.Action, logs.ActionAutomuteUser)
	}
	if e.Status != logs.StatusSuccess {
		t.Errorf("Status = %q, want %q", e.Status, logs.StatusSuccess)
	}
	if e.Metadata["rule_name"] != "flood" {
		t.Errorf("metadata.rule_name = %v, want \"flood\"", e.Metadata["rule_name"])
	}
	if e.Metadata["warning_count"] != int16(3) {
		t.Errorf("metadata.warning_count = %v, want 3", e.Metadata["warning_count"])
	}
}

// TestAutoActioner_Ban_HappyPath: bot admin, ban OK → log SUCCESS,
// tg.BanUser llamado con untilDate=0 + revoke=true.
func TestAutoActioner_Ban_HappyPath(t *testing.T) {
	tg := &fakeTelegramActor{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	a := NewAutoActioner(tg, lg, gr, nil)

	action := AutoAction{
		Kind:         AutoActionBan,
		GroupID:      -1001,
		UserID:       999,
		RuleName:     "flood",
		WarningCount: 5,
	}
	if err := a.Execute(context.Background(), action); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(tg.banCalls) != 1 {
		t.Fatalf("banCalls = %d, want 1", len(tg.banCalls))
	}
	if tg.banCalls[0].untilDate != 0 || !tg.banCalls[0].revokeMessages {
		t.Errorf("banCalls[0] = %+v, want untilDate=0, revoke=true", tg.banCalls[0])
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	e := lg.entries[0]
	if e.Action != logs.ActionAutobanUser {
		t.Errorf("Action = %q, want %q", e.Action, logs.ActionAutobanUser)
	}
	if e.Status != logs.StatusSuccess {
		t.Errorf("Status = %q, want %q", e.Status, logs.StatusSuccess)
	}
}

// TestAutoActioner_BotDemoted_PermissionDenied: bot era admin al hit
// pero ya es member al dispatch → log PERMISSION_DENIED, NO se llama
// a Telegram.
func TestAutoActioner_BotDemoted_PermissionDenied(t *testing.T) {
	tg := &fakeTelegramActor{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusMember}, // bot removido
	}}
	a := NewAutoActioner(tg, lg, gr, nil)

	action := AutoAction{
		Kind:         AutoActionMute,
		GroupID:      -1001,
		UserID:       999,
		MinutesUntil: 10,
		RuleName:     "flood",
		WarningCount: 3,
	}
	if err := a.Execute(context.Background(), action); err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if len(tg.muteCalls) != 0 {
		t.Errorf("muteCalls = %d, want 0 (no se llamo a Telegram)", len(tg.muteCalls))
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	e := lg.entries[0]
	if e.Status != logs.StatusPermissionDenied {
		t.Errorf("Status = %q, want %q", e.Status, logs.StatusPermissionDenied)
	}
	if e.ActorID != nil {
		t.Errorf("ActorID = %v, want nil", e.ActorID)
	}
}

// TestAutoActioner_GroupNotFound: grupo borrado entre hit y dispatch
// → log NOT_FOUND, NO Telegram.
func TestAutoActioner_GroupNotFound(t *testing.T) {
	tg := &fakeTelegramActor{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{}}
	a := NewAutoActioner(tg, lg, gr, nil)

	action := AutoAction{
		Kind: AutoActionBan, GroupID: -1001, UserID: 999,
		RuleName: "flood", WarningCount: 5,
	}
	if err := a.Execute(context.Background(), action); err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if len(tg.banCalls) != 0 {
		t.Errorf("banCalls = %d, want 0", len(tg.banCalls))
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	if lg.entries[0].Status != logs.StatusNotFound {
		t.Errorf("Status = %q, want %q", lg.entries[0].Status, logs.StatusNotFound)
	}
}

// TestAutoActioner_TelegramError_LoggedAsError: bot admin, Telegram
// devuelve ErrPermissionDenied → log PERMISSION_DENIED, error
// propagado al caller.
func TestAutoActioner_TelegramError_LoggedAsError(t *testing.T) {
	tg := &fakeTelegramActor{err: telegram.ErrPermissionDenied}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	a := NewAutoActioner(tg, lg, gr, nil)

	action := AutoAction{
		Kind: AutoActionMute, GroupID: -1001, UserID: 999,
		MinutesUntil: 10, RuleName: "flood", WarningCount: 3,
	}
	err := a.Execute(context.Background(), action)
	if !errors.Is(err, telegram.ErrPermissionDenied) {
		t.Fatalf("Execute: err = %v, want ErrPermissionDenied", err)
	}
	if len(lg.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(lg.entries))
	}
	if lg.entries[0].Status != logs.StatusPermissionDenied {
		t.Errorf("Status = %q, want %q", lg.entries[0].Status, logs.StatusPermissionDenied)
	}
	if lg.entries[0].ErrorMessage == nil {
		t.Error("ErrorMessage = nil, want contiene mensaje del adapter")
	}
}

// TestAutoActioner_UnknownKind_Error: action.Kind desconocido → error
// sin log ni dispatch.
func TestAutoActioner_UnknownKind_Error(t *testing.T) {
	tg := &fakeTelegramActor{}
	lg := &fakeLogs{}
	gr := &fakeGroups{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}
	a := NewAutoActioner(tg, lg, gr, nil)

	action := AutoAction{
		Kind: AutoActionKind("invalid"), GroupID: -1001, UserID: 999,
		RuleName: "flood", WarningCount: 3,
	}
	if err := a.Execute(context.Background(), action); err == nil {
		t.Fatal("Execute: want error, got nil")
	}
	if len(tg.muteCalls) != 0 || len(tg.banCalls) != 0 {
		t.Errorf("se llamo a Telegram con kind invalido")
	}
}

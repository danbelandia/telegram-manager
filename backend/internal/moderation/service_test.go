package moderation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/joinrequests"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// fakeGroups implementa GroupPermissionReader con un mapa en memoria.
type fakeGroups struct {
	groups map[int64]*groups.Group
}

func (f *fakeGroups) GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error) {
	g, ok := f.groups[id]
	if !ok {
		return nil, groups.ErrNotFound
	}
	return g, nil
}

// fakeTelegram implementa TelegramActions registrando cada llamada.
type fakeTelegram struct {
	err      error // si no es nil, todas las acciones devuelven este error
	banned   [][3]int64
	unbanned []int64
	muted    [][3]int64
	unmuted  []int64
	deleted  []int64
	pinned   []int64
	locked   []int64
	unlocked []int64
	approved []int64
	rejected []int64
}

func (f *fakeTelegram) BanUser(ctx context.Context, chatID, userID, untilDate int64, revoke bool) error {
	f.banned = append(f.banned, [3]int64{chatID, userID, untilDate})
	return f.err
}
func (f *fakeTelegram) UnbanUser(ctx context.Context, chatID, userID int64) error {
	f.unbanned = append(f.unbanned, userID)
	return f.err
}
func (f *fakeTelegram) MuteUser(ctx context.Context, chatID, userID, untilDate int64) error {
	f.muted = append(f.muted, [3]int64{chatID, userID, untilDate})
	return f.err
}
func (f *fakeTelegram) UnmuteUser(ctx context.Context, chatID, userID int64) error {
	f.unmuted = append(f.unmuted, userID)
	return f.err
}
func (f *fakeTelegram) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	f.deleted = append(f.deleted, messageID)
	return f.err
}
func (f *fakeTelegram) PinMessage(ctx context.Context, chatID, messageID int64) error {
	f.pinned = append(f.pinned, messageID)
	return f.err
}
func (f *fakeTelegram) LockGroup(ctx context.Context, chatID int64) error {
	f.locked = append(f.locked, chatID)
	return f.err
}
func (f *fakeTelegram) UnlockGroup(ctx context.Context, chatID int64) error {
	f.unlocked = append(f.unlocked, chatID)
	return f.err
}
func (f *fakeTelegram) ApproveJoinRequest(ctx context.Context, chatID, userID int64) error {
	f.approved = append(f.approved, userID)
	return f.err
}
func (f *fakeTelegram) RejectJoinRequest(ctx context.Context, chatID, userID int64) error {
	f.rejected = append(f.rejected, userID)
	return f.err
}

// fakeLogs implementa LogWriter capturando entries.
type fakeLogs struct {
	entries []*logs.Entry
}

func (f *fakeLogs) Create(ctx context.Context, e *logs.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

// fakeRequests implementa RequestStore en memoria.
type fakeRequests struct {
	requests map[int64]*joinrequests.Request
	nextID   int64
}

func newFakeRequests() *fakeRequests {
	return &fakeRequests{requests: make(map[int64]*joinrequests.Request), nextID: 1}
}

func (f *fakeRequests) add(groupID, userID int64) *joinrequests.Request {
	r := &joinrequests.Request{
		ID:          f.nextID,
		GroupID:     groupID,
		UserID:      userID,
		Status:      joinrequests.StatusPending,
		RequestedAt: time.Now(),
	}
	f.nextID++
	f.requests[r.ID] = r
	return r
}

func (f *fakeRequests) GetByID(ctx context.Context, id int64) (*joinrequests.Request, error) {
	r, ok := f.requests[id]
	if !ok {
		return nil, joinrequests.ErrNotFound
	}
	return r, nil
}

func (f *fakeRequests) Resolve(ctx context.Context, id int64, status joinrequests.Status, decidedBy *int64) error {
	r, ok := f.requests[id]
	if !ok {
		return joinrequests.ErrNotFound
	}
	if r.Status != joinrequests.StatusPending {
		return joinrequests.ErrNotPending
	}
	r.Status = status
	r.DecidedBy = decidedBy
	return nil
}

func newService(t *testing.T, tg *fakeTelegram, reqs *fakeRequests, groupsMap map[int64]*groups.Group) (*Service, *fakeLogs) {
	t.Helper()
	logsFake := &fakeLogs{}
	svc := NewService(&fakeGroups{groups: groupsMap}, tg, reqs, logsFake)
	return svc, logsFake
}

func groupWithPerms(permissions map[string]bool) *groups.Group {
	return &groups.Group{
		TelegramID:     -1001,
		Title:          "MU Online Comunidad",
		Type:           "supergroup",
		BotStatus:      groups.StatusAdministrator,
		BotPermissions: permissions,
	}
}

func TestService_BanSuccess(t *testing.T) {
	tg := &fakeTelegram{}
	svc, logFake := newService(t, tg, nil, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_restrict_members": true}),
	})

	err := svc.Ban(context.Background(), 7, -1001, 456, 0, true)
	if err != nil {
		t.Fatalf("Ban() error: %v", err)
	}
	if len(tg.banned) != 1 || tg.banned[0][1] != 456 || tg.banned[0][2] != 0 {
		t.Errorf("banned = %v, want user 456 hasta 0", tg.banned)
	}
	if len(logFake.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(logFake.entries))
	}
	e := logFake.entries[0]
	if e.Action != logs.ActionBanUser || e.Status != logs.StatusSuccess {
		t.Errorf("log = %s/%s, want BAN_USER/SUCCESS", e.Action, e.Status)
	}
	if e.ActorID == nil || *e.ActorID != 7 {
		t.Errorf("actor_id = %v, want 7", e.ActorID)
	}
	if e.TargetUserID == nil || *e.TargetUserID != 456 {
		t.Errorf("target_user_id = %v, want 456", e.TargetUserID)
	}
}

func TestService_PermissionDeniedDoesNotCallTelegram(t *testing.T) {
	// Bot sin can_restrict_members.
	tg := &fakeTelegram{}
	svc, logFake := newService(t, tg, nil, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_delete_messages": true}),
	})

	err := svc.Ban(context.Background(), 7, -1001, 456, 0, true)
	if !errors.Is(err, ErrBotPermission) {
		t.Fatalf("Ban() error = %v, want ErrBotPermission", err)
	}
	if len(tg.banned) != 0 {
		t.Errorf("se llamo a Telegram sin permiso: %v", tg.banned)
	}
	if len(logFake.entries) != 1 {
		t.Fatalf("logs = %d, want 1 (fallo registrado)", len(logFake.entries))
	}
	e := logFake.entries[0]
	if e.Status != logs.StatusPermissionDenied {
		t.Errorf("status = %s, want PERMISSION_DENIED", e.Status)
	}
	if e.ErrorMessage == nil || *e.ErrorMessage == "" {
		t.Error("error_message vacio en log de permiso")
	}
}

func TestService_TelegramErrorLogged(t *testing.T) {
	tg := &fakeTelegram{err: telegram.ErrPermissionDenied}
	svc, logFake := newService(t, tg, nil, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_restrict_members": true}),
	})

	err := svc.Mute(context.Background(), 7, -1001, 456, 3600)
	if !errors.Is(err, telegram.ErrPermissionDenied) {
		t.Fatalf("Mute() error = %v, want ErrPermissionDenied del adapter", err)
	}
	if len(logFake.entries) != 1 {
		t.Fatalf("logs = %d, want 1", len(logFake.entries))
	}
	e := logFake.entries[0]
	if e.Status != logs.StatusPermissionDenied {
		t.Errorf("status = %s, want PERMISSION_DENIED (mapeo del error de Telegram)", e.Status)
	}
	if e.ErrorMessage == nil {
		t.Error("error_message ausente")
	}
}

func TestService_TelegramNotFoundLogged(t *testing.T) {
	tg := &fakeTelegram{err: telegram.ErrTelegramNotFound}
	svc, logFake := newService(t, tg, nil, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_delete_messages": true}),
	})

	err := svc.DeleteMessage(context.Background(), 7, -1001, 999)
	if !errors.Is(err, telegram.ErrTelegramNotFound) {
		t.Fatalf("DeleteMessage() error = %v, want ErrTelegramNotFound", err)
	}
	if logFake.entries[0].Status != logs.StatusNotFound {
		t.Errorf("status = %s, want NOT_FOUND", logFake.entries[0].Status)
	}
}

func TestService_GroupNotFoundNoLog(t *testing.T) {
	tg := &fakeTelegram{}
	svc, logFake := newService(t, tg, nil, map[int64]*groups.Group{})

	err := svc.Unban(context.Background(), 7, -999, 456)
	if !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("Unban() error = %v, want ErrGroupNotFound", err)
	}
	if len(tg.unbanned) != 0 {
		t.Errorf("se llamo a Telegram con grupo inexistente: %v", tg.unbanned)
	}
	if len(logFake.entries) != 0 {
		t.Errorf("logs = %d, want 0 (no hay grupo administrado que auditar)", len(logFake.entries))
	}
}

func TestService_SinPermisosConocidosRechaza(t *testing.T) {
	// BotPermissions nil (estado desconocido): la accion se rechaza
	// (principio de seguridad: mas seguro no habilitar).
	tg := &fakeTelegram{}
	svc, _ := newService(t, tg, nil, map[int64]*groups.Group{
		-1001: groupWithPerms(nil),
	})

	err := svc.Lock(context.Background(), 7, -1001)
	if !errors.Is(err, ErrBotPermission) {
		t.Fatalf("Lock() error = %v, want ErrBotPermission", err)
	}
	if len(tg.locked) != 0 {
		t.Errorf("se llamo a Telegram con permisos desconocidos: %v", tg.locked)
	}
}

func TestService_ApproveSuccess(t *testing.T) {
	reqs := newFakeRequests()
	req := reqs.add(-1001, 456)
	tg := &fakeTelegram{}
	svc, logFake := newService(t, tg, reqs, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_invite_users": true}),
	})

	err := svc.Approve(context.Background(), 7, -1001, req.ID)
	if err != nil {
		t.Fatalf("Approve() error: %v", err)
	}
	if len(tg.approved) != 1 || tg.approved[0] != 456 {
		t.Errorf("approved = %v, want user 456", tg.approved)
	}
	if req.Status != joinrequests.StatusApproved {
		t.Errorf("status = %s, want approved", req.Status)
	}
	if req.DecidedBy == nil || *req.DecidedBy != 7 {
		t.Errorf("decided_by = %v, want 7", req.DecidedBy)
	}
	if len(logFake.entries) != 1 || logFake.entries[0].Status != logs.StatusSuccess {
		t.Errorf("logs = %+v, want 1 SUCCESS", logFake.entries)
	}
}

func TestService_RejectRequiresCanInviteUsers(t *testing.T) {
	// Nota tasks 2.6: reject usa can_invite_users, no can_restrict.
	reqs := newFakeRequests()
	req := reqs.add(-1001, 456)
	tg := &fakeTelegram{}
	svc, _ := newService(t, tg, reqs, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_restrict_members": true}),
	})

	err := svc.Reject(context.Background(), 7, -1001, req.ID)
	if !errors.Is(err, ErrBotPermission) {
		t.Fatalf("Reject() error = %v, want ErrBotPermission (reject requiere can_invite_users)", err)
	}
	if len(tg.rejected) != 0 {
		t.Errorf("se rechazo sin permiso: %v", tg.rejected)
	}
}

func TestService_ApproveAlreadyDecidedNoTelegram(t *testing.T) {
	reqs := newFakeRequests()
	req := reqs.add(-1001, 456)
	req.Status = joinrequests.StatusApproved
	tg := &fakeTelegram{}
	svc, _ := newService(t, tg, reqs, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_invite_users": true}),
	})

	err := svc.Approve(context.Background(), 7, -1001, req.ID)
	if !errors.Is(err, ErrRequestAlreadyDecided) {
		t.Fatalf("Approve() error = %v, want ErrRequestAlreadyDecided", err)
	}
	if len(tg.approved) != 0 {
		t.Errorf("se llamo a Telegram con solicitud ya decidida: %v", tg.approved)
	}
}

func TestService_ApproveWrongGroup(t *testing.T) {
	reqs := newFakeRequests()
	req := reqs.add(-2002, 456) // solicitud de OTRO grupo
	tg := &fakeTelegram{}
	svc, _ := newService(t, tg, reqs, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_invite_users": true}),
	})

	err := svc.Approve(context.Background(), 7, -1001, req.ID)
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("Approve() error = %v, want ErrRequestNotFound (request de otro grupo)", err)
	}
	if len(tg.approved) != 0 {
		t.Errorf("se aprobo una solicitud de otro grupo: %v", tg.approved)
	}
}

func TestService_ApproveNotFound(t *testing.T) {
	reqs := newFakeRequests()
	tg := &fakeTelegram{}
	svc, _ := newService(t, tg, reqs, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_invite_users": true}),
	})

	err := svc.Approve(context.Background(), 7, -1001, 999)
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("Approve() error = %v, want ErrRequestNotFound", err)
	}
}

func TestService_ApproveTelegramErrorKeepsPending(t *testing.T) {
	reqs := newFakeRequests()
	req := reqs.add(-1001, 456)
	tg := &fakeTelegram{err: telegram.ErrPermissionDenied}
	svc, logFake := newService(t, tg, reqs, map[int64]*groups.Group{
		-1001: groupWithPerms(map[string]bool{"can_invite_users": true}),
	})

	err := svc.Approve(context.Background(), 7, -1001, req.ID)
	if !errors.Is(err, telegram.ErrPermissionDenied) {
		t.Fatalf("Approve() error = %v, want error de Telegram", err)
	}
	// La solicitud queda sin decidir.
	if req.Status != joinrequests.StatusPending {
		t.Errorf("status = %s, want pending (sin decidir ante error de Telegram)", req.Status)
	}
	if logFake.entries[0].Status != logs.StatusPermissionDenied {
		t.Errorf("log status = %s, want PERMISSION_DENIED", logFake.entries[0].Status)
	}
}

package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/telegram-manager/backend/internal/auth"
	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/joinrequests"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/moderation"
	"github.com/telegram-manager/backend/internal/telegram"
)

// fakePinger satisface pinger sin postgres.
type fakePinger struct{}

func (fakePinger) PingContext(ctx context.Context) error { return nil }

// fixedAuthenticator satisface authenticator devolviendo claims fijos:
// los tests de handlers no necesitan JWT real ni DB.
type fixedAuthenticator struct{ claims *auth.Claims }

func (f fixedAuthenticator) ParseAccess(token string) (*auth.Claims, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	return f.claims, nil
}

// testClaims es la identidad del admin de prueba (actor en logs).
var testClaims = &auth.Claims{
	Username: "admin",
	RegisteredClaims: jwt.RegisteredClaims{
		Subject: "1",
	},
}

// fakeGroupStore satisface groupStore.
type fakeGroupStore struct {
	list []groups.Group
	get  *groups.Group
	err  error
}

func (f *fakeGroupStore) List(ctx context.Context) ([]groups.Group, error) {
	return f.list, f.err
}

func (f *fakeGroupStore) GetByTelegramID(ctx context.Context, id int64) (*groups.Group, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.get == nil {
		return nil, groups.ErrNotFound
	}
	return f.get, nil
}

// fakeGroupUsers satisface groupUsersLookup.
type fakeGroupUsers struct {
	members []telegram.ChatMember
	member  *telegram.ChatMember
	err     error
}

func (f *fakeGroupUsers) GetChatAdministrators(ctx context.Context, chatID int64) ([]telegram.ChatMember, error) {
	return f.members, f.err
}

func (f *fakeGroupUsers) GetChatMember(ctx context.Context, chatID, userID int64) (telegram.ChatMember, error) {
	if f.member != nil {
		return *f.member, f.err
	}
	return telegram.ChatMember{}, f.err
}

// fakeModeration satisface moderationActions y registra llamadas para
// verificar el actor y los parametros.
type fakeModeration struct {
	err            error
	banParams      [4]int64 // actor, group, user, until
	approachParams [3]int64 // actor, group, request
	lockCalls      int
	rejectCalls    int
	banCalls       int
	approveCalls   int
	unmuteCalls    int
}

func (f *fakeModeration) Ban(ctx context.Context, actorID, groupID, userID int64, untilDate int64, revokeMessages bool) error {
	f.banCalls++
	f.banParams = [4]int64{actorID, groupID, userID, untilDate}
	return f.err
}

func (f *fakeModeration) Unban(ctx context.Context, actorID, groupID, userID int64) error {
	return f.err
}

func (f *fakeModeration) Mute(ctx context.Context, actorID, groupID, userID int64, untilDate int64) error {
	return f.err
}

func (f *fakeModeration) Unmute(ctx context.Context, actorID, groupID, userID int64) error {
	f.unmuteCalls++
	return f.err
}

func (f *fakeModeration) DeleteMessage(ctx context.Context, actorID, groupID, messageID int64) error {
	return f.err
}

func (f *fakeModeration) PinMessage(ctx context.Context, actorID, groupID, messageID int64) error {
	return f.err
}

func (f *fakeModeration) Lock(ctx context.Context, actorID, groupID int64) error {
	f.lockCalls++
	return f.err
}

func (f *fakeModeration) Unlock(ctx context.Context, actorID, groupID int64) error {
	return f.err
}

func (f *fakeModeration) Approve(ctx context.Context, actorID, groupID, requestID int64) error {
	f.approveCalls++
	f.approachParams = [3]int64{actorID, groupID, requestID}
	return f.err
}

func (f *fakeModeration) Reject(ctx context.Context, actorID, groupID, requestID int64) error {
	f.rejectCalls++
	return f.err
}

// fakeJoinRequestStore satisface joinRequestStore.
type fakeJoinRequestStore struct {
	requests []joinrequests.Request
	err      error
}

func (f *fakeJoinRequestStore) ListByGroup(ctx context.Context, groupID int64) ([]joinrequests.Request, error) {
	return f.requests, f.err
}

// fakeLogStore satisface logStore.
type fakeLogStore struct {
	entries []logs.Entry
	err     error
}

func (f *fakeLogStore) ListByGroup(ctx context.Context, groupID int64) ([]logs.Entry, error) {
	return f.entries, f.err
}

// buildModerationServer construye un Server con auth fija y todos los
// modulos del paso 10 habilitados (fakes inyectados).
func buildModerationServer(t *testing.T, mod *fakeModeration, opts ...Option) (*Server, *fakeModeration) {
	t.Helper()
	gs := &fakeGroupStore{}
	gu := &fakeGroupUsers{}
	jr := &fakeJoinRequestStore{}
	ls := &fakeLogStore{}

	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithGroups(gs, gu),
		WithModeration(mod),
		WithJoinRequests(jr, mod),
		WithLogs(ls),
	)
	for _, o := range opts {
		o(server)
	}
	return server, mod
}

var _ = http.StatusOK // mantener import si se reorganizan los tests

// TestModerationRoutes_RequireAuth: las rutas del paso 10 exigen acceso.
func TestModerationRoutes_RequireAuth(t *testing.T) {
	mod := &fakeModeration{}
	server, _ := buildModerationServer(t, mod)

	cases := []struct {
		method, path string
	}{
		{"GET", "/api/groups"},
		{"GET", "/api/groups/-100"},
		{"GET", "/api/groups/-100/users"},
		{"POST", "/api/groups/-100/users/42/ban"},
		{"POST", "/api/groups/-100/users/42/unban"},
		{"POST", "/api/groups/-100/users/42/mute"},
		{"POST", "/api/groups/-100/users/42/unmute"},
		{"POST", "/api/groups/-100/messages/7/delete"},
		{"POST", "/api/groups/-100/messages/7/pin"},
		{"POST", "/api/groups/-100/lock"},
		{"POST", "/api/groups/-100/unlock"},
		{"GET", "/api/groups/-100/join-requests"},
		{"POST", "/api/groups/-100/join-requests/9/approve"},
		{"POST", "/api/groups/-100/join-requests/9/reject"},
		{"GET", "/api/groups/-100/logs"},
	}
	for _, tc := range cases {
		rr := doRequest(server, tc.method, tc.path, "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: code = %d, want 401", tc.method, tc.path, rr.Code)
		}
	}
}

// TestModerationActions_PassActorAndIDs: ban recibe actor del claims y
// los ids del path.
func TestModerationActions_PassActorAndIDs(t *testing.T) {
	mod := &fakeModeration{}
	server, _ := buildModerationServer(t, mod)

	rr := doRequest(server, "POST", "/api/groups/-100123/users/42/ban", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("ban: code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if mod.banCalls != 1 {
		t.Fatalf("ban calls = %d, want 1", mod.banCalls)
	}
	if mod.banParams != [4]int64{1, -100123, 42, 0} {
		t.Errorf("ban params = %v, want [1 -100123 42 0]", mod.banParams)
	}
}

// TestModerationAction_Errors: mapeo de errores de dominio (seccion 18).
func TestModerationAction_Errors(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		modErr     error
		wantStatus int
	}{
		{"grupo no existe", "POST", "/api/groups/-1/users/2/ban", moderation.ErrGroupNotFound, http.StatusNotFound},
		{"bot sin permiso", "POST", "/api/groups/-1/lock", moderation.ErrBotPermission, http.StatusForbidden},
		{"telegram permission denied", "POST", "/api/groups/-1/users/2/ban", telegram.ErrPermissionDenied, http.StatusForbidden},
		{"telegram not found", "POST", "/api/groups/-1/users/2/ban", telegram.ErrTelegramNotFound, http.StatusNotFound},
		{"telegram unavailable", "POST", "/api/groups/-1/lock", telegram.ErrTelegramUnavailable, http.StatusBadGateway},
		{"telegram api error 400", "POST", "/api/groups/-1/lock", &telegram.TelegramAPIError{Code: 400, Description: "method is available only in supergroups"}, http.StatusBadGateway},
		{"solicitud ya decidida", "POST", "/api/groups/-1/join-requests/9/approve", moderation.ErrRequestAlreadyDecided, http.StatusConflict},
		{"solicitud no encontrada", "POST", "/api/groups/-1/join-requests/9/approve", moderation.ErrRequestNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod := &fakeModeration{err: tc.modErr}
			server, _ := buildModerationServer(t, mod)
			rr := doRequest(server, tc.method, tc.path, "", validToken)
			if rr.Code != tc.wantStatus {
				t.Errorf("code = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
}

// TestModerationAction_Validation: ids no numericos responden 400.
func TestModerationAction_Validation(t *testing.T) {
	mod := &fakeModeration{}
	server, _ := buildModerationServer(t, mod)

	cases := []string{
		"/api/groups/abc/users/2/ban",
		"/api/groups/-1/users/abc/mute",
		"/api/groups/-1/messages/abc/delete",
		"/api/groups/-1/join-requests/abc/approve",
	}
	for _, path := range cases {
		rr := doRequest(server, "POST", path, "", validToken)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", path, rr.Code)
		}
	}
}

// TestModerationAction_BadBody: body invalido responde 400 y no llama
// al servicio.
func TestModerationAction_BadBody(t *testing.T) {
	mod := &fakeModeration{}
	server, _ := buildModerationServer(t, mod)

	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/groups/-1/users/2/ban", `{"until_date":`},
		{"POST", "/api/groups/-1/users/2/ban", `{"until_date":"x"}`},
		{"POST", "/api/groups/-1/users/2/mute", `{"until_date":{"a":1}}`},
	} {
		rr := doRequest(server, tc.method, tc.path, tc.body, validToken)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s %s body=%s: code = %d, want 400", tc.method, tc.path, tc.body, rr.Code)
		}
	}
	if mod.banCalls != 0 {
		t.Errorf("ban calls = %d, want 0 (no debe llamar al servicio con body invalido)", mod.banCalls)
	}
}

// TestBan_UntilDateBody: el body opcional llega hasta el servicio.
func TestBan_UntilDateBody(t *testing.T) {
	mod := &fakeModeration{}
	server, _ := buildModerationServer(t, mod)

	rr := doRequest(server, "POST", "/api/groups/-100/users/42/ban",
		`{"until_date":1750000000,"revoke_messages":false}`, validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if mod.banParams != [4]int64{1, -100, 42, 1750000000} {
		t.Errorf("ban params = %v, quiero [1 -100 42 1750000000]", mod.banParams)
	}
}

// TestApproveJoinRequest_PassActorAndIDs.
func TestApproveJoinRequest_PassActorAndIDs(t *testing.T) {
	mod := &fakeModeration{}
	server, _ := buildModerationServer(t, mod)

	rr := doRequest(server, "POST", "/api/groups/-100/join-requests/9/approve", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if mod.approveCalls != 1 {
		t.Fatalf("approve calls = %d, want 1", mod.approveCalls)
	}
	if mod.approachParams != [3]int64{1, -100, 9} {
		t.Errorf("approve params = %v, quiero [1 -100 9]", mod.approachParams)
	}
}

const validToken = "token-del-test"

// TestListGroups_Data: listado con respuesta JSON poblada.
func TestListGroups_Data(t *testing.T) {
	gs := &fakeGroupStore{list: []groups.Group{{
		TelegramID: -100123,
		Title:      "MU Online Comunidad",
		Type:       "supergroup",
		BotStatus:  groups.StatusAdministrator,
	}}}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithGroups(gs, &fakeGroupUsers{}),
	)

	rr := doRequest(server, "GET", "/api/groups", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "MU Online Comunidad") {
		t.Errorf("body sin titulo del grupo: %s", rr.Body.String())
	}
}

// TestGetGroup_NotFound: grupo inexistente responde 404 con
// NOT_FOUND (grupo no existe en nuestra base).
func TestGetGroup_NotFound(t *testing.T) {
	gs := &fakeGroupStore{} // get == nil → ErrNotFound
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithGroups(gs, &fakeGroupUsers{}),
	)
	rr := doRequest(server, "GET", "/api/groups/-1", "", validToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestListGroupUsers_Lookup: ?userId= devuelve el miembro puntual.
func TestListGroupUsers_Lookup(t *testing.T) {
	gu := &fakeGroupUsers{member: &telegram.ChatMember{
		Status: "member",
		User:   &telegram.User{ID: 42, FirstName: "Juan", Username: "juanito"},
	}}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithGroups(&fakeGroupStore{}, gu),
	)

	rr := doRequest(server, "GET", "/api/groups/-100/users?userId=42", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"user_id":42`) {
		t.Errorf("body sin user_id 42: %s", rr.Body.String())
	}
}

// TestListGroupUsers_InvalidUserID: ?userId=no-numerico responde 400.
func TestListGroupUsers_InvalidUserID(t *testing.T) {
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithGroups(&fakeGroupStore{}, &fakeGroupUsers{}),
	)
	rr := doRequest(server, "GET", "/api/groups/-100/users?userId=abc", "", validToken)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestListGroupUsers_NoPermission: el bot no puede ver admins → 403.
func TestListGroupUsers_NoPermission(t *testing.T) {
	gu := &fakeGroupUsers{err: telegram.ErrPermissionDenied}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithGroups(&fakeGroupStore{}, gu),
	)
	rr := doRequest(server, "GET", "/api/groups/-100/users", "", validToken)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestListJoinRequests_Data: listado con data y estado.
func TestListJoinRequests_Data(t *testing.T) {
	jr := &fakeJoinRequestStore{requests: []joinrequests.Request{{
		ID:        9,
		GroupID:   -100,
		UserID:    42,
		Status:    joinrequests.StatusPending,
		FirstName: "Juan",
	}}}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithJoinRequests(jr, &fakeModeration{}),
	)

	rr := doRequest(server, "GET", "/api/groups/-100/join-requests", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"pending"`) {
		t.Errorf("body sin status pending: %s", rr.Body.String())
	}
}

// TestListGroupLogs_Data: listado de auditoria con accion.
func TestListGroupLogs_Data(t *testing.T) {
	ls := &fakeLogStore{entries: []logs.Entry{{
		ActorID: int64Ptr(1),
		GroupID: -100,
		Action:  logs.ActionBanUser,
		Status:  logs.StatusSuccess,
	}}}
	server := NewServer(fakePinger{}, botStatusStub{},
		WithAuth(nil, fixedAuthenticator{claims: testClaims}, false),
		WithLogs(ls),
	)

	rr := doRequest(server, "GET", "/api/groups/-100/logs", "", validToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"action":"BAN_USER"`) {
		t.Errorf("body sin BAN_USER: %s", rr.Body.String())
	}
}

func int64Ptr(v int64) *int64 { return &v }

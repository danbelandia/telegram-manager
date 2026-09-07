package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// moderationStubServer responde como la Bot API y captura el body JSON
// de cada request. Si handler no es nil, es el quien escribe la
// respuesta (como en newStubServer); el default es {ok:true,result:true}.
func moderationStubServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]string, *[]map[string]any) {
	t.Helper()
	var paths []string
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/bot") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		paths = append(paths, r.URL.Path)

		if body, err := io.ReadAll(r.Body); err == nil {
			var m map[string]any
			if err := json.Unmarshal(body, &m); err != nil {
				t.Errorf("request body no es JSON valido: %v (raw: %s)", err, body)
			} else {
				bodies = append(bodies, m)
			}
		}

		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	return srv, &paths, &bodies
}

func TestAdapterBanUser_Success(t *testing.T) {
	srv, paths, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	// Baneo temporal, sin revocar mensajes.
	err := adapter.BanUser(context.Background(), -100123, 456, 1700000000, false)
	if err != nil {
		t.Fatalf("BanUser() unexpected error: %v", err)
	}

	if len(*paths) != 1 || !strings.HasSuffix((*paths)[0], "/banChatMember") {
		t.Errorf("paths = %v, want 1 request a /banChatMember", *paths)
	}
	if len(*bodies) != 1 {
		t.Fatalf("bodies = %d, want 1", len(*bodies))
	}
	b := (*bodies)[0]
	if b["chat_id"] != float64(-100123) || b["user_id"] != float64(456) {
		t.Errorf("chat_id/user_id mal: %v", b)
	}
	if b["until_date"] != float64(1700000000) {
		t.Errorf("until_date = %v, want 1700000000", b["until_date"])
	}
	if b["revoke_messages"] != false {
		t.Errorf("revoke_messages = %v, want false (explicito, no omitido)", b["revoke_messages"])
	}
}

func TestAdapterBanUser_Permanent(t *testing.T) {
	srv, _, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	// untilDate=0 => baneo indefinido: hasta_date debe estar AUSENTE.
	err := adapter.BanUser(context.Background(), -100123, 456, 0, true)
	if err != nil {
		t.Fatalf("BanUser() unexpected error: %v", err)
	}
	b := (*bodies)[0]
	if _, ok := b["until_date"]; ok {
		t.Errorf("until_date presente con valor %v, want omitido (0 => indefinido)", b["until_date"])
	}
	if b["revoke_messages"] != true {
		t.Errorf("revoke_messages = %v, want true", b["revoke_messages"])
	}
}

func TestAdapterBanUser_PermissionDenied(t *testing.T) {
	srv, _, _ := moderationStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot is not a member of the channel chat"}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.BanUser(context.Background(), -100123, 456, 0, true)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("BanUser() error = %v, want ErrPermissionDenied", err)
	}
}

func TestAdapterBanUser_BadRequestNotFound(t *testing.T) {
	// 400 con description "not found" => ErrTelegramNotFound.
	srv, _, _ := moderationStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: user not found"}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.BanUser(context.Background(), -100123, 456, 0, true)
	if !errors.Is(err, ErrTelegramNotFound) {
		t.Fatalf("BanUser() error = %v, want ErrTelegramNotFound (400 user not found)", err)
	}
}

func TestAdapterBanUser_BadRequestRights(t *testing.T) {
	// 400 con "not enough rights" => ErrPermissionDenied.
	srv, _, _ := moderationStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: not enough rights in chat"}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.BanUser(context.Background(), -100123, 456, 0, true)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("BanUser() error = %v, want ErrPermissionDenied (400 not enough rights)", err)
	}
}

func TestAdapterMuteUser_PermissionsClosed(t *testing.T) {
	srv, paths, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.MuteUser(context.Background(), -100123, 456, 1700000000)
	if err != nil {
		t.Fatalf("MuteUser() unexpected error: %v", err)
	}

	if !strings.HasSuffix((*paths)[0], "/restrictChatMember") {
		t.Errorf("path = %s, want /restrictChatMember", (*paths)[0])
	}
	b := (*bodies)[0]
	if b["chat_id"] != float64(-100123) || b["user_id"] != float64(456) {
		t.Errorf("chat_id/user_id mal: %v", b)
	}
	if b["until_date"] != float64(1700000000) {
		t.Errorf("until_date = %v", b["until_date"])
	}

	perms, ok := b["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions ausente o mal tipado: %v", b["permissions"])
	}
	if perms["can_send_messages"].(bool) {
		t.Error("can_send_messages deberia ser false (mute cierra todo)")
	}
	// El mute cierra cada tipo de contenido.
	for _, key := range []string{"can_send_audios", "can_send_photos", "can_send_videos", "can_send_polls", "can_send_other_messages"} {
		if got, _ := perms[key].(bool); got {
			t.Errorf("%s = true, want false en mute", key)
		}
	}
}

func TestAdapterUnmuteUser_PermissionsOpen(t *testing.T) {
	srv, _, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.UnmuteUser(context.Background(), -100123, 456)
	if err != nil {
		t.Fatalf("UnmuteUser() unexpected error: %v", err)
	}

	b := (*bodies)[0]
	perms, ok := b["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions ausente o mal tipado: %v", b["permissions"])
	}
	if !perms["can_send_messages"].(bool) {
		t.Error("can_send_messages = false, want true en unmute")
	}
	if _, ok := b["until_date"]; ok {
		t.Errorf("until_date presente en unmute, want omitido (restriccion abierta indefinida)")
	}
}

func TestAdapterLockUnlockGroup(t *testing.T) {
	srv, paths, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	if err := adapter.LockGroup(context.Background(), -100123); err != nil {
		t.Fatalf("LockGroup() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[0], "/setChatPermissions") {
		t.Errorf("lock path = %s, want /setChatPermissions", (*paths)[0])
	}
	lockPerms := (*bodies)[0]["permissions"].(map[string]any)
	if lockPerms["can_send_messages"].(bool) {
		t.Error("lock: can_send_messages deberia ser false")
	}

	if err := adapter.UnlockGroup(context.Background(), -100123); err != nil {
		t.Fatalf("UnlockGroup() unexpected error: %v", err)
	}
	if len(*paths) != 2 {
		t.Fatalf("paths = %v, want 2 llamadas", *paths)
	}
	unlockPerms := (*bodies)[1]["permissions"].(map[string]any)
	if !unlockPerms["can_send_messages"].(bool) {
		t.Error("unlock: can_send_messages deberia ser true")
	}
}

func TestAdapterDeletePinMessage(t *testing.T) {
	srv, paths, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	if err := adapter.DeleteMessage(context.Background(), -100123, 77); err != nil {
		t.Fatalf("DeleteMessage() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[0], "/deleteMessage") {
		t.Errorf("path = %s, want /deleteMessage", (*paths)[0])
	}
	if (*bodies)[0]["message_id"] != float64(77) {
		t.Errorf("message_id = %v, want 77", (*bodies)[0]["message_id"])
	}

	if err := adapter.PinMessage(context.Background(), -100123, 77); err != nil {
		t.Fatalf("PinMessage() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[1], "/pinChatMessage") {
		t.Errorf("path = %s, want /pinChatMessage", (*paths)[1])
	}
}

func TestAdapterApproveRejectJoinRequest(t *testing.T) {
	srv, paths, bodies := moderationStubServer(t, nil)
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	if err := adapter.ApproveJoinRequest(context.Background(), -100123, 42); err != nil {
		t.Fatalf("ApproveJoinRequest() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[0], "/approveChatJoinRequest") {
		t.Errorf("path = %s, want /approveChatJoinRequest", (*paths)[0])
	}
	if (*bodies)[0]["user_id"] != float64(42) {
		t.Errorf("user_id = %v, want 42", (*bodies)[0]["user_id"])
	}

	if err := adapter.RejectJoinRequest(context.Background(), -100123, 42); err != nil {
		t.Fatalf("RejectJoinRequest() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[1], "/declineChatJoinRequest") {
		t.Errorf("path = %s, want /declineChatJoinRequest", (*paths)[1])
	}
}

func TestAdapterGetChatMember_DecodesResult(t *testing.T) {
	srv, paths, _ := moderationStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"status":"restricted","user":{"id":456,"first_name":"Juan"},"can_send_messages":false}}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	member, err := adapter.GetChatMember(context.Background(), -100123, 456)
	if err != nil {
		t.Fatalf("GetChatMember() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[0], "/getChatMember") {
		t.Errorf("path = %s, want /getChatMember", (*paths)[0])
	}
	if member.Status != MemberStatusRestricted || member.User == nil || member.User.ID != 456 {
		t.Errorf("member mal decodificado: %+v", member)
	}
	if member.CanSendMessages == nil || *member.CanSendMessages {
		t.Errorf("can_send_messages = %v, want false", member.CanSendMessages)
	}
}

func TestAdapterGetChatAdministrators_DecodesResult(t *testing.T) {
	srv, paths, _ := moderationStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"status":"creator","user":{"id":1,"first_name":"Owner"}},
			{"status":"administrator","user":{"id":2,"first_name":"Mod"},"can_restrict_members":true}
		]}`))
	})
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	admins, err := adapter.GetChatAdministrators(context.Background(), -100123)
	if err != nil {
		t.Fatalf("GetChatAdministrators() unexpected error: %v", err)
	}
	if !strings.HasSuffix((*paths)[0], "/getChatAdministrators") {
		t.Errorf("path = %s, want /getChatAdministrators", (*paths)[0])
	}
	if len(admins) != 2 {
		t.Fatalf("admins = %d, want 2", len(admins))
	}
	if admins[0].Status != MemberStatusCreator || admins[1].Status != MemberStatusAdministrator {
		t.Errorf("status mal: %s, %s", admins[0].Status, admins[1].Status)
	}
	if admins[1].CanRestrictMembers == nil || !*admins[1].CanRestrictMembers {
		t.Errorf("can_restrict_members = %v, want true", admins[1].CanRestrictMembers)
	}
}

func TestAdapterDoWithRetry_RateLimitThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.DeleteMessage(context.Background(), -100123, 1)
	if err != nil {
		t.Fatalf("DeleteMessage() tras 429 con retry_after: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (429 + reintento)", calls.Load())
	}
}

func TestAdapterDoWithRetry_RateLimitWaitsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	start := time.Now()
	err := adapter.PinMessage(context.Background(), -100123, 1)
	if err != nil {
		t.Fatalf("PinMessage() tras 429: %v", err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Errorf("no esperó retry_after=1s (duración %v)", elapsed)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestAdapterDoWithRetry_RateLimitNoRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests"}`))
	}))
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	// Sin parameters.retry_after no se reintenta a ciegas (§18.1).
	err := adapter.BanUser(context.Background(), -100123, 1, 0, true)
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("BanUser() error = %v, want *RateLimitError", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (sin retry_after no se reintenta)", calls.Load())
	}
}

func TestAdapterDoWithRetry_RateLimitExhausted(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":0}}`))
	}))
	defer srv.Close()

	adapter := NewAdapter("123456:test", WithBaseURL(srv.URL))

	err := adapter.MuteUser(context.Background(), -100123, 1, 0)
	if err == nil {
		t.Fatal("MuteUser() = nil, want error tras agotar reintentos")
	}
	if calls.Load() > maxRateLimitRetries+1 {
		t.Errorf("calls = %d, want <= %d (max reintentos + inicial)", calls.Load(), maxRateLimitRetries+1)
	}
}

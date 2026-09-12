package joinrequests

import (
	"context"
	"testing"

	"github.com/telegram-manager/backend/internal/telegram"
)

func TestHandleChatJoinRequest_FillsRequest(t *testing.T) {
	upd := &telegram.ChatJoinRequest{
		From: telegram.User{ID: 42, FirstName: "Juan"},
		Chat: telegram.Chat{ID: -1001, Type: "supergroup", Title: "MU Online"},
	}

	var r Request
	if err := HandleChatJoinRequest(context.Background(), upd, &r); err != nil {
		t.Fatalf("HandleChatJoinRequest() error: %v", err)
	}
	if r.GroupID != -1001 || r.UserID != 42 {
		t.Errorf("request = %+v, want group -1001 user 42", r)
	}
	if r.Status != StatusPending {
		t.Errorf("status = %s, want pending", r.Status)
	}
}

func TestHandleChatJoinRequest_NilNoop(t *testing.T) {
	var r Request
	if err := HandleChatJoinRequest(context.Background(), nil, &r); err != nil {
		t.Fatalf("HandleChatJoinRequest(nil) error: %v", err)
	}
	if r.GroupID != 0 || r.Status != "" {
		t.Errorf("request cambio con upd nil: %+v", r)
	}
}

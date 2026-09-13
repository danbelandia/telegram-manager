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

func TestHandleChatMember_MemberStatusRelevant(t *testing.T) {
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -1001, Type: "supergroup"},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusMember,
			User:   &telegram.User{ID: 42, FirstName: "Juan"},
		},
	}

	groupID, userID, relevant := HandleChatMember(upd)
	if !relevant {
		t.Fatal("HandleChatMember: want relevant=true for member status")
	}
	if groupID != -1001 {
		t.Errorf("groupID = %d, want -1001", groupID)
	}
	if userID != 42 {
		t.Errorf("userID = %d, want 42", userID)
	}
}

func TestHandleChatMember_KickedNotRelevant(t *testing.T) {
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -1001},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusKicked,
			User:   &telegram.User{ID: 42},
		},
	}

	_, _, relevant := HandleChatMember(upd)
	if relevant {
		t.Error("HandleChatMember: want relevant=false for kicked status")
	}
}

func TestHandleChatMember_LeftNotRelevant(t *testing.T) {
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -1001},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusLeft,
			User:   &telegram.User{ID: 42},
		},
	}

	_, _, relevant := HandleChatMember(upd)
	if relevant {
		t.Error("HandleChatMember: want relevant=false for left status")
	}
}

func TestHandleChatMember_RestrictedNotRelevant(t *testing.T) {
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -1001},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusRestricted,
			User:   &telegram.User{ID: 42},
		},
	}

	_, _, relevant := HandleChatMember(upd)
	if relevant {
		t.Error("HandleChatMember: want relevant=false for restricted status")
	}
}

func TestHandleChatMember_NilUpdateNotRelevant(t *testing.T) {
	_, _, relevant := HandleChatMember(nil)
	if relevant {
		t.Error("HandleChatMember(nil): want relevant=false")
	}
}

func TestHandleChatMember_NilUserNotRelevant(t *testing.T) {
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -1001},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusMember,
			User:   nil,
		},
	}

	_, _, relevant := HandleChatMember(upd)
	if relevant {
		t.Error("HandleChatMember: want relevant=false when user is nil")
	}
}

func TestHandleChatMember_NilChatNotRelevant(t *testing.T) {
	upd := &telegram.ChatMemberUpdated{
		Chat: nil,
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusMember,
			User:   &telegram.User{ID: 42},
		},
	}

	_, _, relevant := HandleChatMember(upd)
	if relevant {
		t.Error("HandleChatMember: want relevant=false when chat is nil")
	}
}

package groups

import (
	"context"
	"testing"

	"github.com/telegram-manager/backend/internal/telegram"
)

func boolPtr(b bool) *bool { return &b }

func TestHandleMyChatMember_IgnoresPrivate(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: 111, Type: "private"},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusMember,
		},
	}

	err := HandleMyChatMember(context.Background(), upd, &g)
	if err != nil {
		t.Fatalf("HandleMyChatMember(private) error = %v, want nil", err)
	}
	if g.TelegramID != 0 {
		t.Errorf("group llenado con chat private: %#v", g)
	}
}

func TestHandleMyChatMember_IgnoresNilChatOrMember(t *testing.T) {
	tests := []struct {
		name string
		upd  *telegram.ChatMemberUpdated
	}{
		{"chat nil", &telegram.ChatMemberUpdated{NewChatMember: telegram.ChatMember{}}},
		{"upd nil", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g Group
			if err := HandleMyChatMember(context.Background(), tt.upd, &g); err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if g.TelegramID != 0 {
				t.Errorf("group llenado con update ignorado: %#v", g)
			}
		})
	}
}

func TestHandleMyChatMember_MapsSupergroup(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -100123, Type: "supergroup", Title: "MU Online", Username: "mucomunidad"},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusAdministrator,
		},
	}

	if err := HandleMyChatMember(context.Background(), upd, &g); err != nil {
		t.Fatalf("error = %v", err)
	}
	if g.TelegramID != -100123 {
		t.Errorf("TelegramID = %d, want -100123", g.TelegramID)
	}
	if g.Title != "MU Online" {
		t.Errorf("Title = %q, want MU Online", g.Title)
	}
	if g.Type != "supergroup" {
		t.Errorf("Type = %q, want supergroup", g.Type)
	}
	if g.BotStatus != StatusAdministrator {
		t.Errorf("BotStatus = %q, want administrator", g.BotStatus)
	}
	if g.Username == nil || *g.Username != "mucomunidad" {
		t.Errorf("Username = %v, want mucomunidad", g.Username)
	}
}

func TestHandleMyChatMember_UsernameOptional(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -100456, Type: "group", Title: "Sin user"},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusMember,
		},
	}

	if err := HandleMyChatMember(context.Background(), upd, &g); err != nil {
		t.Fatalf("error = %v", err)
	}
	if g.Username != nil {
		t.Errorf("Username = %v, want nil", g.Username)
	}
}

func TestHandleMyChatMember_KeepsLeftStatus(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -100789, Type: "supergroup", Title: "Salida"},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusLeft,
		},
	}

	if err := HandleMyChatMember(context.Background(), upd, &g); err != nil {
		t.Fatalf("error = %v", err)
	}
	if g.BotStatus != StatusLeft {
		t.Errorf("BotStatus = %q, want left", g.BotStatus)
	}
	if g.BotPermissions != nil {
		t.Errorf("BotPermissions = %#v, want nil (no admin)", g.BotPermissions)
	}
}

func TestHandleMyChatMember_CopiesPresentPermissions(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -100321, Type: "supergroup", Title: "Admin"},
		NewChatMember: telegram.ChatMember{
			Status:             telegram.MemberStatusAdministrator,
			CanDeleteMessages:  boolPtr(true),
			CanRestrictMembers: boolPtr(true),
			CanPinMessages:     boolPtr(false),
			CanInviteUsers:     boolPtr(true),
			CanPromoteMembers:  boolPtr(false),
			CanChangeInfo:      boolPtr(true),
		},
	}

	if err := HandleMyChatMember(context.Background(), upd, &g); err != nil {
		t.Fatalf("error = %v", err)
	}
	want := map[string]bool{
		"can_delete_messages":  true,
		"can_restrict_members": true,
		"can_pin_messages":     false,
		"can_invite_users":     true,
		"can_promote_members":  false,
		"can_change_info":      true,
	}
	if len(g.BotPermissions) != len(want) {
		t.Fatalf("BotPermissions = %#v, want %#v", g.BotPermissions, want)
	}
	for k, v := range want {
		if g.BotPermissions[k] != v {
			t.Errorf("BotPermissions[%q] = %v, want %v", k, g.BotPermissions[k], v)
		}
	}
}

func TestHandleMyChatMember_NoPermissionsForMember(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -100654, Type: "supergroup", Title: "Miembro"},
		NewChatMember: telegram.ChatMember{
			Status: telegram.MemberStatusMember,
		},
	}

	if err := HandleMyChatMember(context.Background(), upd, &g); err != nil {
		t.Fatalf("error = %v", err)
	}
	if g.BotPermissions != nil {
		t.Errorf("BotPermissions = %#v, want nil", g.BotPermissions)
	}
	if g.BotStatus != StatusMember {
		t.Errorf("BotStatus = %q, want member", g.BotStatus)
	}
}

func TestHandleMyChatMember_NoAdminNoPermissions(t *testing.T) {
	var g Group
	upd := &telegram.ChatMemberUpdated{
		Chat: &telegram.Chat{ID: -100987, Type: "channel", Title: "Canal"},
		NewChatMember: telegram.ChatMember{
			Status:         telegram.MemberStatusAdministrator,
			CanPinMessages: boolPtr(true),
		},
	}

	if err := HandleMyChatMember(context.Background(), upd, &g); err != nil {
		t.Fatalf("error = %v", err)
	}
	if got := len(g.BotPermissions); got != 1 {
		t.Errorf("BotPermissions len = %d, want 1 (solo can_pin_messages presente)", got)
	}
}

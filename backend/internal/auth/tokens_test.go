package auth

import (
	"testing"
	"time"
)

func testAdmin() Admin {
	now := time.Now()
	return Admin{ID: 42, Username: "admin", TenantID: 7, CreatedAt: now}
}

func TestTokenManager_IssueAndParseAccess(t *testing.T) {
	m := NewTokenManager("test-secret-ojos-que-no-ven-corazon-que-no-siente-x")

	admin := testAdmin()
	tokenStr, err := m.IssueAccess(admin)
	if err != nil {
		t.Fatalf("issue access: %v", err)
	}

	claims, err := m.ParseAccess(tokenStr)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.Subject != "42" {
		t.Errorf("sub = %q, want 42", claims.Subject)
	}
	if claims.Username != "admin" {
		t.Errorf("username = %q, want admin", claims.Username)
	}
	if claims.TenantID != 7 {
		t.Errorf("tenant_id = %d, want 7", claims.TenantID)
	}
}

func TestTokenManager_AccessTTLIs15Min(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC) // futuro: no expira vs reloj del sistema
	m := &TokenManager{
		secret: []byte("test-secret-ojos-que-no-ven-corazon-que-no-siente-x"),
		now:    func() time.Time { return now },
	}

	admin := testAdmin()
	tokenStr, err := m.IssueAccess(admin)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := m.ParseAccess(tokenStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("exp nil")
	}
	wantExp := now.Add(15 * time.Minute)
	if !claims.ExpiresAt.Time.Equal(wantExp) {
		t.Errorf("exp = %v, want %v (15 min)", claims.ExpiresAt.Time, wantExp)
	}
}

func TestTokenManager_IssueAndParseRefresh(t *testing.T) {
	m := NewTokenManager("test-secret-ojos-que-no-ven-corazon-que-no-siente-x")

	tokenStr, err := m.IssueRefresh(testAdmin())
	if err != nil {
		t.Fatalf("issue refresh: %v", err)
	}
	if _, err := m.ParseRefresh(tokenStr); err != nil {
		t.Fatalf("parse refresh: %v", err)
	}
}

func TestTokenManager_ExpiredTokenRejected(t *testing.T) {
	secret := "test-secret-ojos-que-no-ven-corazon-que-no-siente-x"
	// Linea base en el pasado (2020): el exp resulta anterior al reloj
	// real del sistema, que es lo que jwt/v5 usa para validar.
	issued := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	m := &TokenManager{secret: []byte(secret), now: func() time.Time { return issued }}

	tokenStr, err := m.IssueAccess(testAdmin())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := m.ParseAccess(tokenStr); err == nil {
		t.Fatal("want error for expired token, got nil")
	}
}

func TestTokenManager_TamperedTokenRejected(t *testing.T) {
	m := NewTokenManager("test-secret-ojos-que-no-ven-corazon-que-no-siente-x")

	tokenStr, err := m.IssueAccess(testAdmin())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	tampered := tokenStr[:len(tokenStr)-2] + "xx"

	if _, err := m.ParseAccess(tampered); err == nil {
		t.Fatal("want error for tampered token, got nil")
	}
}

func TestTokenManager_WrongSecretRejected(t *testing.T) {
	issuer := NewTokenManager("secret-a-para-emitir-xxxxxxxxxxxxxxxxxxx")
	tokenStr, err := issuer.IssueAccess(testAdmin())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	verifier := NewTokenManager("secret-b-distinta-yyyyyyyyyyyyyyyyyyy")
	if _, err := verifier.ParseAccess(tokenStr); err == nil {
		t.Fatal("want error for wrong secret, got nil")
	}
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/telegram-manager/backend/internal/tenants"
)

// signupSetup arma SignupService con TG mockeado (conteo de llamadas,
// sin Bot API real) y crypter de fixture. Retorna el servicio, el
// contador de llamadas getMe y el crypter (para verificar el token
// cifrado en DB).
func signupSetup(t *testing.T, db *sql.DB, botErr error) (*SignupService, *int, *tenants.Crypter) {
	t.Helper()
	calls := 0
	validate := func(_ context.Context, token string) (string, error) {
		calls++
		if botErr != nil {
			return "", botErr
		}
		return "fixture_bot", nil
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypter, err := tenants.NewCrypter(key)
	if err != nil {
		t.Fatalf("crypter: %v", err)
	}
	svc := NewSignupService(db, tenants.NewRepository(db), NewRepository(db), crypter, validate)
	return svc, &calls, crypter
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestSignup_Validation(t *testing.T) {
	db := testDB(t)
	svc, calls, _ := signupSetup(t, db, nil)

	cases := []SignupInput{
		{},
		{Slug: "acme", Username: "u", Password: "secret123"},
		{Slug: "acme", Username: "u", Password: "corto", BotToken: "tok"},
		{Slug: "acme", Username: "", Password: "secret123", BotToken: "tok"},
	}
	for i, in := range cases {
		if _, err := svc.Signup(context.Background(), in); !errors.Is(err, ErrSignupValidation) {
			t.Errorf("case %d err = %v, want ErrSignupValidation", i, err)
		}
	}
	if *calls != 0 {
		t.Errorf("getMe calls = %d, want 0 (validacion previa, sin TG ni DB)", *calls)
	}
}

func TestSignup_SlugTakenNoTelegram(t *testing.T) {
	db := testDB(t)
	svc, calls, _ := signupSetup(t, db, nil)
	ctx := context.Background()

	if _, err := svc.Signup(ctx, SignupInput{Slug: "acme-dup", Username: "user1", Password: "secret123", BotToken: "tok-1"}); err != nil {
		t.Fatalf("first signup: %v", err)
	}
	before := *calls
	if _, err := svc.Signup(ctx, SignupInput{Slug: "acme-dup", Username: "user2", Password: "secret123", BotToken: "tok-2"}); !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("err = %v, want ErrSlugTaken", err)
	}
	if *calls != before {
		t.Error("slug duplicado llamo a Telegram: debe fallar antes (409 sin TG)")
	}
}

func TestSignup_UsernameTakenGlobal(t *testing.T) {
	db := testDB(t)
	svc, calls, _ := signupSetup(t, db, nil)
	ctx := context.Background()

	if _, err := svc.Signup(ctx, SignupInput{Slug: "acme-uq", Username: "juan", Password: "secret123", BotToken: "tok-1"}); err != nil {
		t.Fatalf("first signup: %v", err)
	}
	before := *calls
	// Mismo username, slug libre, otro tenant → 409 igual (Q1-a).
	if _, err := svc.Signup(ctx, SignupInput{Slug: "otro-uq", Username: "juan", Password: "secret123", BotToken: "tok-2"}); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("err = %v, want ErrUsernameTaken", err)
	}
	if *calls != before {
		t.Error("username duplicado llamo a Telegram: debe fallar antes")
	}
}

func TestSignup_InvalidTokenNoRows(t *testing.T) {
	db := testDB(t)
	svc, _, _ := signupSetup(t, db, errors.New("telegram: invalid token"))
	ctx := context.Background()

	tenantsBefore, adminsBefore := countRows(t, db, "tenants"), countRows(t, db, "admins")
	if _, err := svc.Signup(ctx, SignupInput{Slug: "acme-badtk", Username: "u1-badtk", Password: "secret123", BotToken: "bad"}); !errors.Is(err, ErrInvalidBotToken) {
		t.Fatalf("err = %v, want ErrInvalidBotToken", err)
	}
	if got := countRows(t, db, "tenants"); got != tenantsBefore {
		t.Errorf("tenants = %d, want %d (token invalido no persiste)", got, tenantsBefore)
	}
	if got := countRows(t, db, "admins"); got != adminsBefore {
		t.Errorf("admins = %d, want %d (token invalido no persiste)", got, adminsBefore)
	}
}

func TestSignup_HappyPath(t *testing.T) {
	db := testDB(t)
	svc, _, crypter := signupSetup(t, db, nil)
	ctx := context.Background()

	res, err := svc.Signup(ctx, SignupInput{Slug: "acme-happy", Username: "admin-acme", Password: "secret123", BotToken: "tok-fixture-1"})
	if err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if res.TenantSlug != "acme-happy" || res.AdminUsername != "admin-acme" {
		t.Errorf("res = %+v", res)
	}
	if res.TenantID == 0 || res.AdminID == 0 {
		t.Errorf("ids cero: %+v", res)
	}

	// Admin vinculado al tenant con hash bcrypt valido.
	admin, err := NewRepository(db).GetByUsername(ctx, "admin-acme")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if admin.TenantID != res.TenantID {
		t.Errorf("admin.tenant = %d, want %d", admin.TenantID, res.TenantID)
	}

	// Token cifrado en DB y recuperable (nunca en claro).
	var enc []byte
	var botUsername sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT bot_token_encrypted, bot_username FROM tenants WHERE id = $1`, res.TenantID,
	).Scan(&enc, &botUsername); err != nil {
		t.Fatalf("read tenant: %v", err)
	}
	if len(enc) == 0 {
		t.Fatal("token no persistido cifrado")
	}
	if strings.Contains(string(enc), "tok-fixture-1") {
		t.Fatal("token en claro en la columna cifrada")
	}
	plain, err := crypter.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plain) != "tok-fixture-1" {
		t.Error("el token descifrado no coincide")
	}
	if !botUsername.Valid || botUsername.String != "fixture_bot" {
		t.Errorf("bot_username = %v, want fixture_bot (del getMe)", botUsername)
	}
}

func TestSignup_NotConfigured(t *testing.T) {
	db := testDB(t)
	svc := NewSignupService(db, tenants.NewRepository(db), NewRepository(db), nil,
		func(_ context.Context, _ string) (string, error) { return "x", nil })

	if _, err := svc.Signup(context.Background(), SignupInput{Slug: "s", Username: "u", Password: "secret123", BotToken: "t"}); !errors.Is(err, ErrSignupNotConfigured) {
		t.Errorf("err = %v, want ErrSignupNotConfigured", err)
	}
}

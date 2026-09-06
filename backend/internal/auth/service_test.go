package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeStore es un administratorStore en memoria para los unit tests del
// service (evita Postgres; los handlers usan el real).
type fakeStore struct {
	byUsername map[string]Admin
	loginCalls int
}

func newFakeStore(admins ...Admin) *fakeStore {
	s := &fakeStore{byUsername: map[string]Admin{}}
	for _, a := range admins {
		s.byUsername[a.Username] = a
	}
	return s
}

func (s *fakeStore) GetByUsername(_ context.Context, username string) (Admin, error) {
	a, ok := s.byUsername[username]
	if !ok {
		return Admin{}, ErrNotFound
	}
	return a, nil
}

func (s *fakeStore) GetByID(_ context.Context, id int64) (Admin, error) {
	for _, a := range s.byUsername {
		if a.ID == id {
			return a, nil
		}
	}
	return Admin{}, ErrNotFound
}

func (s *fakeStore) UpdateLastLogin(_ context.Context, _ int64, _ time.Time) error {
	s.loginCalls++
	return nil
}

func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	return mustBcrypt(t, pw)
}

func TestService_Login_Success(t *testing.T) {
	admin := Admin{ID: 1, Username: "admin", PasswordHash: hashPassword(t, "secret123")}
	repo := newFakeStore(admin)
	svc := NewService(repo, NewTokenManager("test-secret-ojos-que-no-ven-corazon-que-no-siente-x"))

	res, err := svc.Login(context.Background(), "admin", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("want access and refresh tokens, got empty")
	}
	if repo.loginCalls != 1 {
		t.Errorf("last_login updates = %d, want 1", repo.loginCalls)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	admin := Admin{ID: 1, Username: "admin", PasswordHash: hashPassword(t, "secret123")}
	svc := NewService(newFakeStore(admin), NewTokenManager("test-secret-ojos-que-no-ven-corazon-que-no-siente-x"))

	_, err := svc.Login(context.Background(), "admin", "wrong")
	if !errors.Is(err, ErrCredentialInvalid) {
		t.Fatalf("err = %v, want ErrCredentialInvalid", err)
	}
}

func TestService_Login_UnknownUserSameError(t *testing.T) {
	// Mismo error que password incorrecto: no se revela si el username existe.
	svc := NewService(newFakeStore(Admin{ID: 1, Username: "admin", PasswordHash: hashPassword(t, "secret123")}), NewTokenManager("test-secret-x"))
	_, err := svc.Login(context.Background(), "noexiste", "secret123")
	if !errors.Is(err, ErrCredentialInvalid) {
		t.Fatalf("err = %v, want ErrCredentialInvalid", err)
	}
}

func TestService_Refresh_Success(t *testing.T) {
	admin := Admin{ID: 1, Username: "admin", PasswordHash: hashPassword(t, "secret123")}
	repo := newFakeStore(admin)
	svc := NewService(repo, NewTokenManager("test-secret-ojos-que-no-ven-corazon-que-no-siente-x"))

	res, err := svc.Login(context.Background(), "admin", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	access2, err := svc.Refresh(context.Background(), res.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if access2 == "" {
		t.Fatal("want non-empty new access token")
	}
}

func TestService_Refresh_InvalidToken(t *testing.T) {
	svc := NewService(newFakeStore(Admin{ID: 1, Username: "admin", PasswordHash: hashPassword(t, "x")}), NewTokenManager("test-secret-x"))
	if _, err := svc.Refresh(context.Background(), "not-a-jwt"); err == nil {
		t.Fatal("want error for invalid refresh, got nil")
	}
}

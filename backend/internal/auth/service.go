package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// administratorStore es la vista minima de datos que el service
// necesita sobre admins. Declarada donde se consume; *Repository y los
// fakes de test la satisfacen.
type administratorStore interface {
	GetByUsername(ctx context.Context, username string) (Admin, error)
	GetByID(ctx context.Context, id int64) (Admin, error)
	UpdateLastLogin(ctx context.Context, id int64, at time.Time) error
}

// Service contiene la logica de negocio de auth: login, refresh y
// logout. Depende del repositorio (identidad) y del TokenManager
// (emision). No conoce HTTP.
type Service struct {
	repo   administratorStore
	tokens *TokenManager
}

// NewService construye el servicio de auth.
func NewService(repo administratorStore, tokens *TokenManager) *Service {
	return &Service{repo: repo, tokens: tokens}
}

// LoginResult es el resultado de un login exitoso.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	Admin        Admin
}

// Login valida username+password, emite access (15 min) y refresh
// (7 dias), y registra last_login_at. Devuelve ErrCredentialInvalid
// para username inexistente O password incorrecto (mismo error: no
// revela cual fallo).
func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	admin, err := s.repo.GetByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		return LoginResult{}, ErrCredentialInvalid
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, ErrCredentialInvalid
	}

	access, err := s.tokens.IssueAccess(admin)
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: login: issue access: %w", err)
	}
	refresh, err := s.tokens.IssueRefresh(admin)
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: login: issue refresh: %w", err)
	}
	if err := s.repo.UpdateLastLogin(ctx, admin.ID, time.Now()); err != nil {
		// No falla el login por un problema de telemetria; solo se
		// propaga el error real. Se loguea en capa superior.
		return LoginResult{}, fmt.Errorf("auth: login: update last login: %w", err)
	}

	return LoginResult{AccessToken: access, RefreshToken: refresh, Admin: admin}, nil
}

// Refresh valida el refresh token stateless y emite un nuevo access
// token. El refresh no rota: sigue valido hasta expirar (7 dias).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return "", fmt.Errorf("auth: refresh: %w", err)
	}

	// Re-verifica que el admin siga existiendo (por si fue eliminado).
	id, err := parseSubjectID(claims.Subject)
	if err != nil {
		return "", fmt.Errorf("auth: refresh: %w", err)
	}
	admin, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("auth: refresh: %w", err)
	}

	return s.tokens.IssueAccess(admin)
}

// Logout no mantiene estado (tokens stateless): la sesion se cierra en
// el cliente borrando la cookie. Metodo presente para la intencion.
func (s *Service) Logout() {}

func parseSubjectID(subject string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(subject, "%d", &id); err != nil {
		return 0, fmt.Errorf("auth: invalid subject: %w", err)
	}
	return id, nil
}

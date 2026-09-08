package auth

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost es el costo de hashing (AGENTS.md 17.1: costo 12).
const bcryptCost = 12

// tenantEnsurer es la vista minima del repo de tenants que el seed
// necesita: crear el tenant `default` si falta. *tenants.Repository
// la satisface; los tests usan un fake.
type tenantEnsurer interface {
	EnsureDefault(ctx context.Context) (int64, error)
}

// EnsureInitialAdmin crea el primer admin al arrancar si la tabla esta
// vacia, usando username/password de env y adoptando el tenant
// `default` (slice 0). Idempotente: si ya hay al menos un admin, no
// hace nada (nunca duplica ni pisa passwords).
func EnsureInitialAdmin(ctx context.Context, repo *Repository, tenants tenantEnsurer, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("auth: ADMIN_USERNAME and ADMIN_PASSWORD are required for bootstrap")
	}

	count, err := repo.Count(ctx)
	if err != nil {
		return fmt.Errorf("auth: bootstrap: %w", err)
	}
	if count > 0 {
		return nil // ya hay admins; no crear ni modificar
	}

	tenantID, err := tenants.EnsureDefault(ctx)
	if err != nil {
		return fmt.Errorf("auth: bootstrap: ensure default tenant: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: bootstrap: hash password: %w", err)
	}
	if _, err := repo.Create(ctx, username, string(hash), tenantID); err != nil {
		return fmt.Errorf("auth: bootstrap: %w", err)
	}
	return nil
}

// mustBcrypt hashea una password para tests (falla el test si algo
// sale mal, en lugar de devolver error).
func mustBcrypt(t interface{ Fatalf(string, ...any) }, pw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(hash)
}

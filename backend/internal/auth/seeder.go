package auth

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost es el costo de hashing (AGENTS.md 17.1: costo 12).
const bcryptCost = 12

// EnsureInitialAdmin crea el primer admin al arrancar si la tabla esta
// vacia, usando username/password de env. Idempotente: si ya hay al
// menos un admin, no hace nada (nunca duplica ni pisa passwords).
func EnsureInitialAdmin(ctx context.Context, repo *Repository, username, password string) error {
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

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: bootstrap: hash password: %w", err)
	}
	if _, err := repo.Create(ctx, username, string(hash)); err != nil {
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

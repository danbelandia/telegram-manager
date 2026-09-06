package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registra el driver "pgx"
)

// TestDatabaseURLEnv es la variable de entorno que puede apuntar a una
// PostgreSQL dedicada para tests de integracion. Si no esta definida,
// los tests usan la base por-paquete de desarrollo (ver OpenTestDB).
const TestDatabaseURLEnv = "TEST_DATABASE_URL"

// OpenTestDB abre una PostgreSQL REAL para tests de integracion
// (convencion AGENTS 21 / backend-go-skill §8: no mockear la DB).
//
// name identifica a la suite (ej. "auth", "api", "groups"). Sin
// TEST_DATABASE_URL, cada suite usa su propia base
// telegram_manager_<name>, creada si no existe; asi los paquetes
// corren en paralelo (go test ./...) sin pisarse: antes, auth y api
// compartian la tabla admins en la misma base y se invalidaban entre
// tests.
//
// Con TEST_DATABASE_URL definida se usa esa (debe existir), y el
// caller decide si truncar.
func OpenTestDB(t interface {
	Helper()
	Fatalf(string, ...any)
	Skipf(string, ...any)
}, name string) (*sql.DB, error) {
	t.Helper()

	dsn := os.Getenv(TestDatabaseURLEnv)
	if dsn == "" {
		dsn = "postgres://telegram:telegram@localhost:5432/telegram_manager_" + name + "?sslmode=disable"
		if err := ensureDatabase(dsn); err != nil {
			return nil, fmt.Errorf("database: ensure test db %q: %w", name, err)
		}
	}

	db, err := Connect(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// ensureDatabase crea (si no existe) la base indicada en el DSN,
// conectandose a la base de mantenimiento "postgres" del cluster.
func ensureDatabase(dsn string) error {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	dbName := strings.TrimPrefix(parsed.Path, "/")
	if dbName == "" || dbName == "postgres" {
		return nil // nunca crear la base de mantenimiento
	}

	adminDSN := *parsed
	adminDSN.Path = "/postgres"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	admin, err := sql.Open("pgx", adminDSN.String())
	if err != nil {
		return fmt.Errorf("open admin conn: %w", err)
	}
	defer admin.Close()
	if err := admin.PingContext(ctx); err != nil {
		return fmt.Errorf("ping admin conn: %w", err)
	}

	var exists bool
	if err := admin.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists); err != nil {
		return fmt.Errorf("check db exists: %w", err)
	}
	if exists {
		return nil
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+dbName); err != nil {
		return fmt.Errorf("create db %q: %w", dbName, err)
	}
	return nil
}

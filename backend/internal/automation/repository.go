package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository persiste settings y warning_state en PostgreSQL. Es la
// unica capa que habla SQL para el dominio automation.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetSettings devuelve los settings del grupo. Si no existe fila,
// retorna ErrNotFound — el caller decide si crear defaults (la policy
// vive en Service.LoadOrCreateSettings para que sea testable).
func (r *Repository) GetSettings(ctx context.Context, groupID int64) (*Settings, error) {
	const q = `
SELECT group_id, enabled, anti_spam_enabled, anti_link_enabled,
       banned_words_enabled, flood_enabled,
       flood_messages, flood_seconds, warning_limit,
       automute_warnings, automute_minutes, autoban_warnings,
       warning_expire_days,
       warn_user_enabled, warn_user_template,
       updated_at
FROM group_moderation_settings
WHERE group_id = $1`

	s, err := scanSettings(r.db.QueryRowContext(ctx, q, groupID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("automation: get settings %d: %w", groupID, err)
	}
	return &s, nil
}

// UpsertSettings crea o reemplaza la fila de settings. ON CONFLICT
// (group_id) DO UPDATE toca todas las columnas (excepto updated_at que
// la DB setea via now()). El caller es responsable de los defaults
// antes de llamar (Service.LoadOrCreateSettings lo garantiza).
func (r *Repository) UpsertSettings(ctx context.Context, s *Settings) error {
	const q = `
INSERT INTO group_moderation_settings (
    group_id, enabled,
    anti_spam_enabled, anti_link_enabled, banned_words_enabled,
    flood_enabled,
    flood_messages, flood_seconds, warning_limit,
    automute_warnings, automute_minutes, autoban_warnings,
    warning_expire_days,
    warn_user_enabled, warn_user_template
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (group_id) DO UPDATE SET
    enabled               = EXCLUDED.enabled,
    anti_spam_enabled     = EXCLUDED.anti_spam_enabled,
    anti_link_enabled     = EXCLUDED.anti_link_enabled,
    banned_words_enabled  = EXCLUDED.banned_words_enabled,
    flood_enabled         = EXCLUDED.flood_enabled,
    flood_messages        = EXCLUDED.flood_messages,
    flood_seconds         = EXCLUDED.flood_seconds,
    warning_limit         = EXCLUDED.warning_limit,
    automute_warnings     = EXCLUDED.automute_warnings,
    automute_minutes      = EXCLUDED.automute_minutes,
    autoban_warnings      = EXCLUDED.autoban_warnings,
    warning_expire_days   = EXCLUDED.warning_expire_days,
    warn_user_enabled     = EXCLUDED.warn_user_enabled,
    warn_user_template    = EXCLUDED.warn_user_template,
    updated_at            = now()`

	_, err := r.db.ExecContext(ctx, q,
		s.GroupID, s.Enabled,
		s.AntiSpamEnabled, s.AntiLinkEnabled, s.BannedWordsEnabled,
		s.FloodEnabled,
		s.FloodMessages, s.FloodSeconds, s.WarningLimit,
		s.AutomuteWarnings, s.AutomuteMinutes, s.AutobanWarnings,
		s.WarningExpireDays,
		s.WarnUserEnabled, s.WarnUserTemplate,
	)
	if err != nil {
		return fmt.Errorf("automation: upsert settings %d: %w", s.GroupID, err)
	}
	return nil
}

// GetWarningState devuelve el warning_state de un (group, user). Si no
// existe fila, retorna ErrNotFound.
func (r *Repository) GetWarningState(ctx context.Context, groupID, userID int64) (*WarningState, error) {
	const q = `
SELECT group_id, user_id, warning_count, last_warning_at, last_action_at, expires_at
FROM user_warning_state
WHERE group_id = $1 AND user_id = $2`

	ws, err := scanWarningState(r.db.QueryRowContext(ctx, q, groupID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("automation: get warning state (%d,%d): %w", groupID, userID, err)
	}
	return &ws, nil
}

// UpsertWarningState crea o reemplaza la fila de warning_state con los
// valores exactos que el caller pasa (incluido WarningCount). El
// caller ya incremento el counter en memoria (despues del rule hit).
//
// Para el path de "primera vez" (counter en 0), el caller debe haber
// invocado LoadOrCreateWarningState (en service.go) que ya inserto la
// fila; aqui solo actualizamos.
func (r *Repository) UpsertWarningState(ctx context.Context, ws *WarningState) error {
	const q = `
INSERT INTO user_warning_state (group_id, user_id, warning_count, last_warning_at, last_action_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (group_id, user_id) DO UPDATE SET
    warning_count    = EXCLUDED.warning_count,
    last_warning_at  = EXCLUDED.last_warning_at,
    last_action_at   = EXCLUDED.last_action_at,
    expires_at       = EXCLUDED.expires_at`

	_, err := r.db.ExecContext(ctx, q,
		ws.GroupID, ws.UserID, ws.WarningCount,
		ws.LastWarningAt, ws.LastActionAt, ws.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("automation: upsert warning state (%d,%d): %w", ws.GroupID, ws.UserID, err)
	}
	return nil
}

// CreateWarningStateIfMissing inserta la fila con warning_count=0 si
// no existe. Es idempotente: si ya existe, no hace nada (no pisa el
// counter). Usado por Service.LoadOrCreateWarningState para arrancar
// el contador de un usuario nuevo.
func (r *Repository) CreateWarningStateIfMissing(ctx context.Context, groupID, userID int64) error {
	const q = `
INSERT INTO user_warning_state (group_id, user_id)
VALUES ($1, $2)
ON CONFLICT (group_id, user_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, q, groupID, userID)
	if err != nil {
		return fmt.Errorf("automation: create warning state (%d,%d): %w", groupID, userID, err)
	}
	return nil
}

// ListWarningStates devuelve todos los warning_state de un grupo (para
// dashboard de slice 3; slice 1 lo expone pero no lo usa en runtime).
func (r *Repository) ListWarningStates(ctx context.Context, groupID int64) ([]WarningState, error) {
	const q = `
SELECT group_id, user_id, warning_count, last_warning_at, last_action_at, expires_at
FROM user_warning_state
WHERE group_id = $1
ORDER BY warning_count DESC, user_id ASC`

	rows, err := r.db.QueryContext(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("automation: list warning states %d: %w", groupID, err)
	}
	defer rows.Close()

	out := make([]WarningState, 0)
	for rows.Next() {
		ws, err := scanWarningState(rows)
		if err != nil {
			return nil, fmt.Errorf("automation: list warning states %d: %w", groupID, err)
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("automation: list warning states %d: %w", groupID, err)
	}
	return out, nil
}

// ResetExpiredWarnings pone warning_count=0 y limpia timestamps para
// todas las filas cuya expires_at <= now(). Pensado para correr en
// background (futuro cron). No se invoca desde slice 1 — vive aca
// porque Service.HandleMessage lo necesita para validar el reset
// individual antes de incrementar el counter.
func (r *Repository) ResetExpiredWarnings(ctx context.Context, groupID int64, now time.Time) (int64, error) {
	const q = `
UPDATE user_warning_state
SET warning_count    = 0,
    last_warning_at  = NULL,
    last_action_at   = NULL,
    expires_at       = NULL
WHERE group_id = $1 AND expires_at IS NOT NULL AND expires_at <= $2`

	res, err := r.db.ExecContext(ctx, q, groupID, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("automation: reset expired %d: %w", groupID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("automation: reset expired %d: %w", groupID, err)
	}
	return n, nil
}

// --- banned_words (slice 2) ---

// ListBannedWords devuelve la lista ordenada alfabeticamente (lower-case,
// normalizada por el INSERT). Se usa desde el Service.HandleMessage
// para pre-cargar la lista por mensaje (1 query por mensaje cuando
// BannedWordsEnabled esta on). El matcher de la regla lowercases el
// texto para que coincida.
func (r *Repository) ListBannedWords(ctx context.Context, groupID int64) ([]string, error) {
	const q = `SELECT word FROM banned_words WHERE group_id = $1 ORDER BY word ASC`
	rows, err := r.db.QueryContext(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("automation: list banned words %d: %w", groupID, err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, fmt.Errorf("automation: scan banned word: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("automation: list banned words %d: %w", groupID, err)
	}
	return out, nil
}

// AddBannedWord inserta (idempotente via ON CONFLICT DO NOTHING). El
// handler normaliza `word` con LOWER() antes de llamar; aca solo
// persistimos. Devuelve nil si la fila ya existia (POST idempotente:
// spec REQ-8).
func (r *Repository) AddBannedWord(ctx context.Context, groupID int64, word string) error {
	const q = `
INSERT INTO banned_words (group_id, word) VALUES ($1, $2)
ON CONFLICT (group_id, word) DO NOTHING`
	if _, err := r.db.ExecContext(ctx, q, groupID, word); err != nil {
		return fmt.Errorf("automation: add banned word: %w", err)
	}
	return nil
}

// RemoveBannedWord borra la fila. Devuelve nil si no existia (DELETE
// idempotente: spec REQ-8). El handler normaliza `word` con LOWER().
func (r *Repository) RemoveBannedWord(ctx context.Context, groupID int64, word string) error {
	const q = `DELETE FROM banned_words WHERE group_id = $1 AND word = $2`
	if _, err := r.db.ExecContext(ctx, q, groupID, word); err != nil {
		return fmt.Errorf("automation: remove banned word: %w", err)
	}
	return nil
}

// --- link_allowlist (slice 2) ---

// ListLinkAllowlist devuelve la lista ordenada alfabeticamente (case
// preserved: el matcher lowercases en evaluacion, no en storage).
func (r *Repository) ListLinkAllowlist(ctx context.Context, groupID int64) ([]string, error) {
	const q = `SELECT domain FROM link_allowlist WHERE group_id = $1 ORDER BY domain ASC`
	rows, err := r.db.QueryContext(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("automation: list link allowlist %d: %w", groupID, err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("automation: scan link allowlist: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("automation: list link allowlist %d: %w", groupID, err)
	}
	return out, nil
}

// AddLinkAllowlist inserta (idempotente via ON CONFLICT DO NOTHING).
// Case preserved: no normalizamos el dominio (los hosts son
// case-insensitive en la practica pero el matcher lowercases en
// evaluacion).
func (r *Repository) AddLinkAllowlist(ctx context.Context, groupID int64, domain string) error {
	const q = `
INSERT INTO link_allowlist (group_id, domain) VALUES ($1, $2)
ON CONFLICT (group_id, domain) DO NOTHING`
	if _, err := r.db.ExecContext(ctx, q, groupID, domain); err != nil {
		return fmt.Errorf("automation: add link allowlist: %w", err)
	}
	return nil
}

// RemoveLinkAllowlist borra la fila. Devuelve nil si no existia
// (DELETE idempotente).
func (r *Repository) RemoveLinkAllowlist(ctx context.Context, groupID int64, domain string) error {
	const q = `DELETE FROM link_allowlist WHERE group_id = $1 AND domain = $2`
	if _, err := r.db.ExecContext(ctx, q, groupID, domain); err != nil {
		return fmt.Errorf("automation: remove link allowlist: %w", err)
	}
	return nil
}

// rowScanner es la vista minima de fila que el scanner necesita.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSettings(row rowScanner) (Settings, error) {
	var s Settings
	err := row.Scan(
		&s.GroupID, &s.Enabled,
		&s.AntiSpamEnabled, &s.AntiLinkEnabled, &s.BannedWordsEnabled,
		&s.FloodEnabled,
		&s.FloodMessages, &s.FloodSeconds, &s.WarningLimit,
		&s.AutomuteWarnings, &s.AutomuteMinutes, &s.AutobanWarnings,
		&s.WarningExpireDays,
		&s.WarnUserEnabled, &s.WarnUserTemplate,
		&s.UpdatedAt,
	)
	if err != nil {
		return Settings{}, err
	}
	return s, nil
}

func scanWarningState(row rowScanner) (WarningState, error) {
	var ws WarningState
	err := row.Scan(
		&ws.GroupID, &ws.UserID, &ws.WarningCount,
		&ws.LastWarningAt, &ws.LastActionAt, &ws.ExpiresAt,
	)
	if err != nil {
		return WarningState{}, err
	}
	return ws, nil
}

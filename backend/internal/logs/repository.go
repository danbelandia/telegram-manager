package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository persiste logs de auditoria en PostgreSQL.
type Repository struct {
	db *sql.DB
}

// NewRepository construye el repositorio sobre un pool existente.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserta un log. Metadata nil se guarda como JSONB NULL.
// El caller setea Entry.TenantID (el servicio lo toma del tenant en
// contexto/claims; 0 solo en paths legacy que migran).
func (r *Repository) Create(ctx context.Context, e *Entry) error {
	const q = `
INSERT INTO logs (tenant_id, actor_id, group_id, action, target_user_id, metadata, status, error_message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	meta, err := marshalMetadata(e.Metadata)
	if err != nil {
		return fmt.Errorf("logs: create %s: %w", e.Action, err)
	}

	_, err = r.db.ExecContext(ctx, q,
		e.TenantID, e.ActorID, e.GroupID, e.Action, e.TargetUserID, meta, string(e.Status), e.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("logs: create %s: %w", e.Action, err)
	}
	return nil
}

// ListByGroup devuelve los logs del tenant para un grupo, del mas
// reciente al mas antiguo (vista /groups/:id/logs del panel).
func (r *Repository) ListByGroup(ctx context.Context, tenantID, groupID int64) ([]Entry, error) {
	const q = `
SELECT id, tenant_id, actor_id, group_id, action, target_user_id, metadata, status, error_message, created_at
FROM logs
WHERE tenant_id = $1 AND group_id = $2
ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, tenantID, groupID)
	if err != nil {
		return nil, fmt.Errorf("logs: list %d: %w", groupID, err)
	}
	defer rows.Close()

	entries := make([]Entry, 0)
	for rows.Next() {
		var (
			e        Entry
			metaJSON []byte
		)
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.ActorID, &e.GroupID, &e.Action, &e.TargetUserID,
			&metaJSON, &e.Status, &e.ErrorMessage, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("logs: list %d: %w", groupID, err)
		}
		if metaJSON != nil {
			if err := json.Unmarshal(metaJSON, &e.Metadata); err != nil {
				return nil, fmt.Errorf("logs: scan metadata: %w", err)
			}
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("logs: list %d: %w", groupID, err)
	}
	return entries, nil
}

// marshalMetadata convierte el mapa a JSONB listo para Insert.
// nil → SQL NULL.
func marshalMetadata(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return b, nil
}

// CountByActionAndGroup agrega logs por action en una ventana temporal
// para un grupo. Usado por el handler GET .../automation/stats del
// dashboard de slice 3 (Fase 3). 1 roundtrip para los 3 actions
// (RULE_TRIGGERED, AUTOMUTE_USER, AUTOBAN_USER) gracias a `action =
// ANY($2)` con array bounded.
//
// Retorna map[action]count con todos los actions que tengan count > 0
// en la ventana (los actions sin ocurrencias NO aparecen en el map).
// El caller rellena los contadores faltantes con 0 antes de responder
// al panel (ej. `{rule_triggered: 0, automute: 0, autoban: 0}`).
func (r *Repository) CountByActionAndGroup(ctx context.Context, tenantID, groupID int64, actions []string, since time.Time) (map[string]int, error) {
	const q = `
SELECT action, COUNT(*)
FROM logs
WHERE tenant_id = $1 AND group_id = $2 AND action = ANY($3) AND created_at >= $4
GROUP BY action`

	rows, err := r.db.QueryContext(ctx, q, tenantID, groupID, actions, since.UTC())
	if err != nil {
		return nil, fmt.Errorf("logs: count actions %d: %w", groupID, err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		action, count, err := scanCount(rows)
		if err != nil {
			return nil, fmt.Errorf("logs: count actions %d: scan: %w", groupID, err)
		}
		out[action] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("logs: count actions %d: %w", groupID, err)
	}
	return out, nil
}

// scanCount desempaqueta la fila (action, count) de
// CountByActionAndGroup. Reutilizable si futuros reportes piden
// agregaciones similares.
func scanCount(row rowScanner) (string, int, error) {
	var action string
	var count int
	if err := row.Scan(&action, &count); err != nil {
		return "", 0, err
	}
	return action, count, nil
}

// rowScanner es la vista minima que scanCount necesita (compatible
// con *sql.Rows y *sql.Row).
type rowScanner interface {
	Scan(dest ...any) error
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/telegram-manager/backend/internal/tenants"
)

// --- Interfaces (vistas minimas consumidas por los handlers) ---

type adminTenantsLister interface {
	ListAll(ctx context.Context) ([]tenants.Tenant, error)
}

type adminTenantsGetter interface {
	GetByID(ctx context.Context, id int64) (*tenants.Tenant, error)
}

type adminTenantsUpdater interface {
	UpdateLicense(ctx context.Context, id int64, in tenants.LicenseUpdate) error
}

// --- Request / Response types ---

type updateTenantLicenseRequest struct {
	Status         *string `json:"status"`
	Plan           *string `json:"plan"`
	TrialEndsAt    *string `json:"trial_ends_at"`    // RFC3339 o null
	ExpiresAt      *string `json:"expires_at"`        // RFC3339 o null
	MaxGroups      *int    `json:"max_groups"`
	MaxMessagesDay *int    `json:"max_messages_day"`
}

type tenantListItem struct {
	ID             int64   `json:"id"`
	Slug           string  `json:"slug"`
	Plan           string  `json:"plan"`
	Status         string  `json:"status"`
	TrialEndsAt    *string `json:"trial_ends_at"`
	ExpiresAt      *string `json:"expires_at"`
	MaxGroups      int     `json:"max_groups"`
	MaxMessagesDay int     `json:"max_messages_day"`
	BotUsername     *string `json:"bot_username"`
	CreatedAt      string  `json:"created_at"`
}

type tenantDetailItem struct {
	ID             int64   `json:"id"`
	Slug           string  `json:"slug"`
	Plan           string  `json:"plan"`
	Status         string  `json:"status"`
	TrialEndsAt    *string `json:"trial_ends_at"`
	ExpiresAt      *string `json:"expires_at"`
	MaxGroups      int     `json:"max_groups"`
	MaxMessagesDay int     `json:"max_messages_day"`
	BotUsername     *string `json:"bot_username"`
	CreatedAt      string  `json:"created_at"`
}

// --- Status transition validation ---

// validTransitions define las transiciones de status permitidas.
var validTransitions = map[string][]string{
	"trial":   {"active", "suspended", "expired"},
	"active":  {"suspended"},
	"suspended": {"active"},
	"expired": {"active"},
}

func isValidTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

// --- Handlers ---

// handleAdminListTenants lista todos los tenants (super-admin only).
// GET /api/admin/tenants
func (s *Server) handleAdminListTenants(w http.ResponseWriter, r *http.Request) {
	tenantsList, err := s.adminTenantsRepo.ListAll(r.Context())
	if err != nil {
		slog.Error("admin: list tenants", "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar los tenants")
		return
	}

	items := make([]tenantListItem, len(tenantsList))
	for i, t := range tenantsList {
		items[i] = tenantToListItem(t)
	}
	respond(w, http.StatusOK, items)
}

// handleAdminGetTenant devuelve el detalle de un tenant.
// GET /api/admin/tenants/{id}
func (s *Server) handleAdminGetTenant(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id invalido")
		return
	}

	t, err := s.adminTenantsGetter.GetByID(r.Context(), id)
	if errors.Is(err, tenants.ErrNotFound) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "tenant no encontrado")
		return
	}
	if err != nil {
		slog.Error("admin: get tenant", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el tenant")
		return
	}

	item := tenantDetailItem{
		ID:             t.ID,
		Slug:           t.Slug,
		Plan:           t.Plan,
		Status:         t.Status,
		MaxGroups:      t.MaxGroups,
		MaxMessagesDay: t.MaxMessagesDay,
		BotUsername:     t.BotUsername,
		CreatedAt:      t.CreatedAt.Format(time.RFC3339),
	}
	if t.TrialEndsAt != nil {
		s := t.TrialEndsAt.Format(time.RFC3339)
		item.TrialEndsAt = &s
	}
	if t.ExpiresAt != nil {
		s := t.ExpiresAt.Format(time.RFC3339)
		item.ExpiresAt = &s
	}
	respond(w, http.StatusOK, item)
}

// handleAdminUpdateTenant actualiza los campos de licencia de un tenant.
// PUT /api/admin/tenants/{id}
func (s *Server) handleAdminUpdateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id invalido")
		return
	}

	var req updateTenantLicenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body invalido")
		return
	}

	// Validar transicion de status si se esta cambiando.
	if req.Status != nil {
		current, err := s.adminTenantsGetter.GetByID(r.Context(), id)
		if errors.Is(err, tenants.ErrNotFound) {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "tenant no encontrado")
			return
		}
		if err != nil {
			slog.Error("admin: update tenant (get current)", "id", id, "error", err)
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el tenant")
			return
		}
		if !isValidTransition(current.Status, *req.Status) {
			respondError(w, http.StatusBadRequest, "INVALID_TRANSITION",
				fmt.Sprintf("no se puede cambiar de '%s' a '%s'", current.Status, *req.Status))
			return
		}
	}

	update := tenants.LicenseUpdate{
		Status:         req.Status,
		Plan:           req.Plan,
		MaxGroups:      req.MaxGroups,
		MaxMessagesDay: req.MaxMessagesDay,
	}

	// Parsear fechas RFC3339 si se proveen.
	if req.TrialEndsAt != nil {
		if *req.TrialEndsAt == "" {
			update.TrialEndsAt = &time.Time{} // null → set to zero
		} else {
			t, err := time.Parse(time.RFC3339, *req.TrialEndsAt)
			if err != nil {
				respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "trial_ends_at formato invalido (usa RFC3339)")
				return
			}
			update.TrialEndsAt = &t
		}
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			update.ExpiresAt = &time.Time{} // null → set to zero
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "expires_at formato invalido (usa RFC3339)")
				return
			}
			update.ExpiresAt = &t
		}
	}

	if err := s.adminTenantsUpdater.UpdateLicense(r.Context(), id, update); err != nil {
		slog.Error("admin: update tenant", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo actualizar el tenant")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "updated"})
}

// tenantToListItem convierte un Tenant a tenantListItem con fechas RFC3339.
func tenantToListItem(t tenants.Tenant) tenantListItem {
	item := tenantListItem{
		ID:             t.ID,
		Slug:           t.Slug,
		Plan:           t.Plan,
		Status:         t.Status,
		MaxGroups:      t.MaxGroups,
		MaxMessagesDay: t.MaxMessagesDay,
		BotUsername:     t.BotUsername,
		CreatedAt:      t.CreatedAt.Format(time.RFC3339),
	}
	if t.TrialEndsAt != nil {
		s := t.TrialEndsAt.Format(time.RFC3339)
		item.TrialEndsAt = &s
	}
	if t.ExpiresAt != nil {
		s := t.ExpiresAt.Format(time.RFC3339)
		item.ExpiresAt = &s
	}
	return item
}

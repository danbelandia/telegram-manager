package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/telegram"
	"github.com/telegram-manager/backend/internal/users"
)

// groupStore es la vista minima del repositorio de grupos que los
// handlers necesitan (lado consumidor). Scopeado por tenant (slice 0):
// grupo ajeno → groups.ErrNotFound → 404 NOT_FOUND (D9, no revela
// existencia).
type groupStore interface {
	ListByTenant(ctx context.Context, tenantID int64) ([]groups.Group, error)
	GetByTenant(ctx context.Context, tenantID, telegramID int64) (*groups.Group, error)
}

// groupUsersLookup es la vista minima del Service de Telegram para
// listar usuarios de un grupo: admins y lookup puntual (la Bot API NO
// permite listar miembros; AGENTS.md §7 y decision P2).
type groupUsersLookup interface {
	GetChatMember(ctx context.Context, chatID, userID int64) (telegram.ChatMember, error)
	GetChatAdministrators(ctx context.Context, chatID int64) ([]telegram.ChatMember, error)
}

// tenantUserLookup es la vista minima del repositorio de users para el
// scope de tenant (slice 0, T9/Q3-a): users es global, el alcance se
// resuelve via join con groups/join_requests del tenant.
type tenantUserLookup interface {
	GetByTenant(ctx context.Context, tenantID, userID int64) (*users.User, error)
}

// groupResponse es la vista JSON de un grupo para el panel.
type groupResponse struct {
	ID             int64           `json:"id"`
	TelegramID     int64           `json:"telegram_id"`
	Title          string          `json:"title"`
	Username       *string         `json:"username"`
	Type           string          `json:"type"`
	MemberCount    *int64          `json:"member_count"`
	BotStatus      string          `json:"bot_status"`
	BotPermissions map[string]bool `json:"bot_permissions"`
}

func toGroupResponse(g *groups.Group) groupResponse {
	return groupResponse{
		ID:             g.ID,
		TelegramID:     g.TelegramID,
		Title:          g.Title,
		Username:       g.Username,
		Type:           g.Type,
		MemberCount:    g.MemberCount,
		BotStatus:      string(g.BotStatus),
		BotPermissions: g.BotPermissions,
	}
}

// handleListGroups responde GET /api/groups: solo los grupos del
// tenant de los claims (iso-listados).
func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	if s.groups == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de grupos no habilitado")
		return
	}
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	list, err := s.groups.ListByTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar los grupos")
		return
	}
	out := make([]groupResponse, 0, len(list))
	for i := range list {
		out = append(out, toGroupResponse(&list[i]))
	}
	respond(w, http.StatusOK, out)
}

// handleGetGroup responde GET /api/groups/:id. Grupo ajeno →
// NOT_FOUND (no 403, para no revelar existencia).
func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	if s.groups == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de grupos no habilitado")
		return
	}
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	g, err := s.groups.GetByTenant(r.Context(), tenantID, id)
	if errors.Is(err, groups.ErrNotFound) {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
		return
	}
	respond(w, http.StatusOK, toGroupResponse(g))
}

// handleListGroupUsers responde GET /api/groups/:id/users (decision P2):
// admins del grupo; con ?userId= hace un lookup puntual
// (getChatMember). La Bot API no expone la lista completa de miembros.
//
// Slice 0: ownership del grupo primero (ajeno → NOT_FOUND sin llamar a
// Telegram); el lookup puntual ademas verifica presencia del usuario
// en el tenant (users global via join, T9): usuario solo de otro
// tenant → NOT_FOUND.
func (s *Server) handleListGroupUsers(w http.ResponseWriter, r *http.Request) {
	if s.groups == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de grupos no habilitado")
		return
	}
	tenantID, ok := tenantIDFromClaims(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.groups.GetByTenant(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, groups.ErrNotFound) {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "grupo no encontrado")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo obtener el grupo")
		return
	}
	lookup, ok := s.resolveGroupUsers(tenantID)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "modulo de grupos no habilitado")
		return
	}

	// Lookup puntual de un miembro especifico.
	if raw := r.URL.Query().Get("userId"); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "userId debe ser un entero valido")
			return
		}
		if s.tenantUsers != nil {
			if _, err := s.tenantUsers.GetByTenant(r.Context(), tenantID, userID); err != nil {
				if errors.Is(err, users.ErrNotFound) {
					respondError(w, http.StatusNotFound, "NOT_FOUND", "el usuario no es miembro del grupo")
					return
				}
				respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo consultar el miembro")
				return
			}
		}
		member, err := lookup.GetChatMember(r.Context(), id, userID)
		if errors.Is(err, telegram.ErrTelegramNotFound) {
			respondError(w, http.StatusNotFound, "NOT_FOUND", "el usuario no es miembro del grupo")
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudo consultar el miembro")
			return
		}
		respond(w, http.StatusOK, toMemberResponse(member))
		return
	}

	admins, err := lookup.GetChatAdministrators(r.Context(), id)
	if err != nil {
		if errors.Is(err, telegram.ErrPermissionDenied) {
			respondError(w, http.StatusForbidden, "PERMISSION_DENIED", "el bot no puede listar administradores del grupo")
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no se pudieron listar los administradores")
		return
	}
	out := make([]memberResponse, 0, len(admins))
	for _, m := range admins {
		out = append(out, toMemberResponse(m))
	}
	respond(w, http.StatusOK, out)
}

// memberResponse es la vista JSON de un miembro/administrador.
type memberResponse struct {
	UserID             int64   `json:"user_id"`
	FirstName          string  `json:"first_name"`
	Username           *string `json:"username"`
	Status             string  `json:"status"`
	CanRestrictMembers *bool   `json:"can_restrict_members"`
	CanDeleteMessages  *bool   `json:"can_delete_messages"`
	CanPinMessages     *bool   `json:"can_pin_messages"`
	CanInviteUsers     *bool   `json:"can_invite_users"`
}

func toMemberResponse(m telegram.ChatMember) memberResponse {
	out := memberResponse{
		Status: m.Status,
	}
	if m.User != nil {
		out.UserID = m.User.ID
		out.FirstName = m.User.FirstName
		if m.User.Username != "" {
			out.Username = &m.User.Username
		}
	}
	out.CanRestrictMembers = m.CanRestrictMembers
	out.CanDeleteMessages = m.CanDeleteMessages
	out.CanPinMessages = m.CanPinMessages
	out.CanInviteUsers = m.CanInviteUsers
	return out
}

// pathID parsea un path param como int64 (design D10): ids de Telegram
// son int64; un valor invalido responde VALIDATION_ERROR.
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id invalido")
		return 0, false
	}
	return id, true
}

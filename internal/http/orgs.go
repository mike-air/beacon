package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/auth"
	"beacon/internal/jobs"
)

// Org handlers: create an org, list the caller's orgs, add/list members.
//
// Course mapping: Chapter 7 — multi-tenancy; Chapter 17 — RBAC (adding members
// is owner/admin-only, gated by requireRole on the route).

type createOrgRequest struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
}

type addMemberRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=owner admin member"`
}

// handleCreateOrg creates an org owned by the caller. POST /v1/orgs.
func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	var req createOrgRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	org, err := s.orgs.CreateOrg(r.Context(), userID, req.Name)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

// handleListOrgs lists the orgs the caller belongs to. GET /v1/orgs.
func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	list, err := s.orgs.ListForUser(r.Context(), userID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	// This was the one list endpoint in the API that answered a bare
	// {"items": [...]} instead of listResponse. Every client then needs a
	// special case for exactly one URL, and the ones that do not have it fail
	// on a shape they had no reason to expect. One envelope, everywhere.
	//
	// The slice is paged here rather than in SQL because ListForUser returns
	// every org a single user belongs to — a handful of rows, already loaded.
	// Paging it in the handler keeps the response shape honest without a
	// query change that would buy nothing.
	limit, offset := parsePaging(r)
	if offset > len(list) {
		offset = len(list)
	}
	end := offset + limit
	if end > len(list) {
		end = len(list)
	}
	writeJSON(w, http.StatusOK, listResponse{
		Items:  list[offset:end],
		Limit:  limit,
		Offset: offset,
	})
}

// handleAddMember adds a member to an org. POST /v1/orgs/{orgID}/members.
// Owner/admin-only (enforced by requireRole on the route AND by the service).
func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	actorRole, _ := auth.RoleFrom(r.Context())

	var req addMemberRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	m, err := s.orgs.AddMember(r.Context(), orgID, actorRole, req.Email, req.Role)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	// "You were added to {org}" email — enqueued, never inline (Ch 23).
	if err := s.jobs.Enqueue(r.Context(), jobs.KindSendEmail, jobs.SendEmailPayload{
		To:       req.Email,
		Subject:  "You were added to an organization on Beacon",
		HTMLBody: "<p>You were added to a Beacon organization. Sign in to get started.</p>",
		TextBody: "You were added to a Beacon organization. Sign in to get started.",
	}); err != nil {
		s.logger.Error("add member: enqueue notification email", "err", err)
	}

	writeJSON(w, http.StatusCreated, m)
}

// handleListMembers lists an org's members. GET /v1/orgs/{orgID}/members.
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	members, err := s.orgs.Members(r.Context(), orgID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": members})
}

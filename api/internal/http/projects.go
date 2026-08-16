package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/auth"
	"beacon/internal/flags"
)

// Project handlers: CRUD under /v1/orgs/{orgID}/projects. Every call is scoped
// to {orgID}, which requireOrg has already confirmed the caller belongs to.
//
// Course mapping: Chapter 7 — org scoping; Chapter 13 — pagination; Chapter 21
// — service layer.

type projectRequest struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
}

// handleCreateProject creates a project in the org. POST .../projects.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var req projectRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	p, err := s.projects.Create(r.Context(), orgID, req.Name)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// handleListProjects lists an org's projects, paginated. GET .../projects.
//
// This is Chapters 31 and 32's worked example, and the two layer in a specific
// order. The FLAG decides whether the new board is reachable at all. The
// EXPERIMENT splits the users the flag let through. Get that order backwards
// and you run an experiment on people who cannot see the thing you're testing.
//
// Note where the checks live: in the handler. Repositories never check flags —
// a query that behaves differently depending on a boolean somewhere else is a
// bug you will spend a day finding.
//
// [verbatim ch31 + ch32's projects_handler excerpts, merged; the chapters show
// them one at a time on the same handler.]
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	userID, _ := auth.UserIDFrom(r.Context())

	if !s.flags.Enabled(r.Context(), FlagNewBoardUI, flags.Subject{UserID: userID, OrgID: orgID}) {
		s.listProjectsV1(w, r) // the old code path, still here, still tested
		return
	}

	switch variant := s.experiments.VariantFor(r.Context(), FlagNewBoardUI, userID); variant {
	case "treatment":
		s.listProjectsV2(w, r)
	case "control", "":
		s.listProjectsV1(w, r)
	default:
		s.logger.Warn("unknown variant", "exp", FlagNewBoardUI, "variant", variant)
		s.listProjectsV1(w, r) // fail safe — old code path
	}
}

// FlagNewBoardUI names the flag and the experiment. They share a key on purpose:
// the flag gates the feature, the experiment measures it, and there is exactly
// one thing being talked about.
//
// Every flag needs an owner and an expiry, or the codebase silts up with dead
// branches nobody dares delete. Owner: platform. Expiry: delete this flag, both
// code paths' fork, and listProjectsV1 once v2 is the only board.
const FlagNewBoardUI = "new_board_ui"

// listProjectsV1 is the shipped behaviour: a flat paginated list.
func (s *Server) listProjectsV1(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	limit, offset := parsePaging(r)
	list, err := s.projects.List(r.Context(), orgID, limit, offset)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Items: list, Limit: limit, Offset: offset})
}

// listProjectsV2 is the new board's shape: the same projects, plus the board
// metadata the v2 client needs, and a marker so a response can be traced back
// to the branch that produced it.
func (s *Server) listProjectsV2(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	limit, offset := parsePaging(r)
	list, err := s.projects.List(r.Context(), orgID, limit, offset)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		listResponse
		Board string `json:"board"`
	}{
		listResponse: listResponse{Items: list, Limit: limit, Offset: offset},
		Board:        "v2",
	})
}

// handleGetProject returns one project. GET .../projects/{projectID}.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "projectID")
	p, err := s.projects.Get(r.Context(), orgID, id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	// Ch 28, layer one. `private` because this response is scoped to a member of
	// one org and a shared cache must never hand it to anyone else; the ETag
	// (id + updated_at, so it changes exactly when the row does) lets a client
	// revalidate for the price of a header instead of the whole body.
	setCacheHeaders(w, cachePrivate, 30*time.Second)
	setETag(r.Context(), etagFor(p.ID, p.UpdatedAt.UTC().Format(time.RFC3339Nano)))

	writeJSON(w, http.StatusOK, p)
}

// handleUpdateProject renames a project. PATCH .../projects/{projectID}.
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "projectID")
	var req projectRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	p, err := s.projects.Update(r.Context(), orgID, id, req.Name)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handleDeleteProject deletes a project. DELETE .../projects/{projectID}.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "projectID")
	if err := s.projects.Delete(r.Context(), orgID, id); err != nil {
		s.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	limit, offset := parsePaging(r)
	list, err := s.projects.List(r.Context(), orgID, limit, offset)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Items: list, Limit: limit, Offset: offset})
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

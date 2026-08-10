package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/auth"
)

// Task + comment handlers, under /v1/orgs/{orgID}/projects/{projectID}/tasks.
// Every call is scoped to {orgID}.
//
// Course mapping: Chapter 11 — status validation; Chapter 13 — pagination +
// status filter; Chapter 21 — service layer.

type createTaskRequest struct {
	Title    string  `json:"title"    validate:"required,min=1,max=200"`
	Status   string  `json:"status"   validate:"omitempty,oneof=todo in_progress done"`
	Position float64 `json:"position"`
}

type updateTaskRequest struct {
	Title    string  `json:"title"    validate:"required,min=1,max=200"`
	Status   string  `json:"status"   validate:"required,oneof=todo in_progress done"`
	Position float64 `json:"position"`
}

type createCommentRequest struct {
	Body string `json:"body" validate:"required,min=1,max=10000"`
}

// handleCreateTask creates a task in the project. POST .../tasks.
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	projectID := chi.URLParam(r, "projectID")
	var req createTaskRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	t, err := s.tasks.Create(r.Context(), orgID, projectID, req.Title, req.Status, req.Position)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.notifyTaskEvent(r.Context(), orgID, eventTaskCreated, t)
	writeJSON(w, http.StatusCreated, t)
}

// handleListTasks lists a project's tasks, paginated, with an optional
// ?status= filter. GET .../tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	projectID := chi.URLParam(r, "projectID")
	limit, offset := parsePaging(r)
	status := r.URL.Query().Get("status")

	list, err := s.tasks.List(r.Context(), orgID, projectID, status, limit, offset)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Items: list, Limit: limit, Offset: offset})
}

// handleGetTask returns one task. GET .../tasks/{taskID}.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "taskID")
	t, err := s.tasks.Get(r.Context(), orgID, id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// handleUpdateTask updates a task. PATCH .../tasks/{taskID}.
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "taskID")
	var req updateTaskRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	t, err := s.tasks.Update(r.Context(), orgID, id, req.Title, req.Status, req.Position)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.notifyTaskEvent(r.Context(), orgID, eventTaskUpdated, t)
	writeJSON(w, http.StatusOK, t)
}

// handleDeleteTask deletes a task. DELETE .../tasks/{taskID}.
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "taskID")
	if err := s.tasks.Delete(r.Context(), orgID, id); err != nil {
		s.handleError(w, r, err)
		return
	}
	s.notifyTaskEvent(r.Context(), orgID, eventTaskDeleted, map[string]string{"id": id, "org_id": orgID})
	w.WriteHeader(http.StatusNoContent)
}

// handleCreateComment adds a comment to a task. POST .../tasks/{taskID}/comments.
func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	taskID := chi.URLParam(r, "taskID")
	userID, _ := auth.UserIDFrom(r.Context())

	var req createCommentRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	c, err := s.tasks.AddComment(r.Context(), orgID, taskID, userID, req.Body)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// handleListComments lists a task's comments. GET .../tasks/{taskID}/comments.
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	taskID := chi.URLParam(r, "taskID")
	list, err := s.tasks.Comments(r.Context(), orgID, taskID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

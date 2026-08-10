package http

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"

	"beacon/internal/attachments"
	"beacon/internal/auth"
	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/storage"
	"beacon/internal/tasks"
	"beacon/internal/users"
	"beacon/internal/webhooks"
)

// handleError is the one translator between domain errors and HTTP responses.
// Every handler funnels failures through here. It keeps the existing
// errorEnvelope shape from response.go and never leaks an unknown error's text
// to the client — those become an opaque 500, logged in full.
//
// Course mapping: Chapter 12 — error handling (typed errors → JSON envelope).
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	// Validation: malformed body → 400, failed field rules → 422.
	var bodyErr *bodyError
	if errors.As(err, &bodyErr) {
		writeError(w, http.StatusBadRequest, "invalid_request", bodyErr.msg)
		return
	}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		writeJSON(w, http.StatusUnprocessableEntity, errorEnvelope{
			Error: errorBody{
				Code:    "validation_failed",
				Message: "request failed validation",
				Fields:  flatten(err),
			},
		})
		return
	}

	switch {
	// 404 — not found.
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, http.StatusNotFound, "task_not_found", "task not found")
	case errors.Is(err, projects.ErrNotFound):
		writeError(w, http.StatusNotFound, "project_not_found", "project not found")
	case errors.Is(err, orgs.ErrNotFound):
		writeError(w, http.StatusNotFound, "org_not_found", "organization not found")
	case errors.Is(err, orgs.ErrUserNotFound), errors.Is(err, users.ErrNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, attachments.ErrNotFound):
		writeError(w, http.StatusNotFound, "attachment_not_found", "attachment not found")
	case errors.Is(err, webhooks.ErrNotFound):
		writeError(w, http.StatusNotFound, "webhook_not_found", "webhook not found")

	// 501 — feature configured off (storage).
	case errors.Is(err, storage.ErrDisabled):
		writeError(w, http.StatusNotImplemented, "storage_disabled", "file storage is not configured on this server")

	// 403 — forbidden / not a member.
	case errors.Is(err, orgs.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "you don't have access to this resource")
	case errors.Is(err, orgs.ErrNotMember):
		writeError(w, http.StatusForbidden, "not_member", "you are not a member of this organization")

	// 409 — conflict.
	case errors.Is(err, users.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email already registered")
	case errors.Is(err, orgs.ErrAlreadyMember):
		writeError(w, http.StatusConflict, "already_member", "user is already a member")

	// 422 — semantic validation.
	case errors.Is(err, tasks.ErrInvalidStatus):
		writeError(w, http.StatusUnprocessableEntity, "invalid_status", "status must be one of: todo, in_progress, done")

	// 401 — bad credentials / token.
	case errors.Is(err, users.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, auth.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")

	default:
		s.logger.Error("request failed", "err", err, "path", r.URL.Path)
		writeError(w, http.StatusInternalServerError, "internal_error", "something went wrong on our end")
	}
}

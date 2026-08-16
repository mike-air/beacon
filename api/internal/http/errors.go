package http

import (
	"context"
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
// classify maps a domain error to the HTTP status, stable code and message a
// client should see.
//
// It exists so the chi path and the huma path cannot drift. handleError writes
// its result to a ResponseWriter; humaError returns it as an error for huma to
// render. If this mapping lived in only one of them, the two halves of the
// service would eventually answer the same failure differently, and a client
// would have to know which handler it happened to reach.
//
// The bool reports whether the error was recognised. An unrecognised error is
// a bug, not a client mistake, so both callers log it and answer 500 without
// leaking what it said.
func classify(err error) (status int, code, message string, fields []fieldError, known bool) {
	// Validation: malformed body -> 400, failed field rules -> 422.
	var bodyErr *bodyError
	if errors.As(err, &bodyErr) {
		return http.StatusBadRequest, "invalid_request", bodyErr.msg, nil, true
	}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		return http.StatusUnprocessableEntity, "validation_failed",
			"request failed validation", flatten(err), true
	}

	switch {
	// 404 — not found.
	case errors.Is(err, tasks.ErrNotFound):
		return http.StatusNotFound, "task_not_found", "task not found", nil, true
	case errors.Is(err, projects.ErrNotFound):
		return http.StatusNotFound, "project_not_found", "project not found", nil, true
	case errors.Is(err, orgs.ErrNotFound):
		return http.StatusNotFound, "org_not_found", "organization not found", nil, true
	case errors.Is(err, orgs.ErrUserNotFound), errors.Is(err, users.ErrNotFound):
		return http.StatusNotFound, "user_not_found", "user not found", nil, true
	case errors.Is(err, attachments.ErrNotFound):
		return http.StatusNotFound, "attachment_not_found", "attachment not found", nil, true
	case errors.Is(err, webhooks.ErrNotFound):
		return http.StatusNotFound, "webhook_not_found", "webhook not found", nil, true

	// 501 — feature configured off (storage).
	case errors.Is(err, storage.ErrDisabled):
		return http.StatusNotImplemented, "storage_disabled",
			"file storage is not configured on this server", nil, true

	// 403 — forbidden / not a member.
	case errors.Is(err, orgs.ErrForbidden):
		return http.StatusForbidden, "forbidden", "you don't have access to this resource", nil, true
	case errors.Is(err, orgs.ErrNotMember):
		return http.StatusForbidden, "not_member",
			"you are not a member of this organization", nil, true

	// 409 — conflict.
	case errors.Is(err, users.ErrEmailTaken):
		return http.StatusConflict, "email_taken", "email already registered", nil, true
	case errors.Is(err, orgs.ErrAlreadyMember):
		return http.StatusConflict, "already_member", "user is already a member", nil, true

	// 422 — semantic validation.
	case errors.Is(err, tasks.ErrInvalidStatus):
		return http.StatusUnprocessableEntity, "invalid_status",
			"status must be one of: todo, in_progress, done", nil, true

	// 401 — bad credentials / token.
	case errors.Is(err, users.ErrInvalidCredentials):
		return http.StatusUnauthorized, "invalid_credentials", "invalid email or password", nil, true
	case errors.Is(err, auth.ErrInvalidToken):
		return http.StatusUnauthorized, "unauthenticated", "authentication required", nil, true
	}

	return http.StatusInternalServerError, "internal_error",
		"something went wrong on our end", nil, false
}

// handleError is the chi path: classify, then write.
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message, fields, known := classify(err)
	if !known {
		s.logger.Error("request failed", "err", err, "path", r.URL.Path)
	}
	if len(fields) > 0 {
		writeJSON(w, status, errorEnvelope{
			Error: errorBody{Code: code, Message: message, Fields: fields},
		})
		return
	}
	writeError(w, status, code, message)
}

// humaError is the huma path: classify, then return. huma renders it through
// the same envelope, because humaError satisfies huma.StatusError.
func (s *Server) asHumaError(ctx context.Context, err error) error {
	status, code, message, fields, known := classify(err)
	if !known {
		s.logger.ErrorContext(ctx, "request failed", "err", err)
	}
	return &humaError{
		status: status,
		Body:   errorBody{Code: code, Message: message, Fields: fields},
	}
}

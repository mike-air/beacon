// Unit tests for the web layer's pure helpers — no database, no router.
//
// Course mapping: Chapter 38 — unit tests. Covers pagination parsing
// (defaults/caps/clamping), the handleError sentinel→status mapping, and the
// validator flatten helper, all in-package so unexported helpers are reachable.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beacon/internal/auth"
	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/storage"
	"beacon/internal/tasks"
	"beacon/internal/users"
)

func TestParsePaging(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "", defaultLimit, 0},
		{"explicit values", "?limit=5&offset=10", 5, 10},
		{"limit capped at max", "?limit=1000", maxLimit, 0},
		{"zero limit falls back", "?limit=0", defaultLimit, 0},
		{"negative limit falls back", "?limit=-3", defaultLimit, 0},
		{"garbage limit falls back", "?limit=abc", defaultLimit, 0},
		{"negative offset ignored", "?offset=-5", defaultLimit, 0},
		{"garbage offset ignored", "?offset=xyz", defaultLimit, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/things"+tt.query, nil)
			limit, offset := parsePaging(r)
			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

// testServer is a Server with only the fields handleError touches (the logger),
// enough to map errors to responses without any database wiring.
func testServer() *Server {
	return &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestHandleErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantBody string // expected error.code in the envelope
	}{
		{"task not found", tasks.ErrNotFound, http.StatusNotFound, "task_not_found"},
		{"project not found", projects.ErrNotFound, http.StatusNotFound, "project_not_found"},
		{"org not found", orgs.ErrNotFound, http.StatusNotFound, "org_not_found"},
		{"user not found", users.ErrNotFound, http.StatusNotFound, "user_not_found"},
		{"storage disabled", storage.ErrDisabled, http.StatusNotImplemented, "storage_disabled"},
		{"forbidden", orgs.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"not member", orgs.ErrNotMember, http.StatusForbidden, "not_member"},
		{"email taken", users.ErrEmailTaken, http.StatusConflict, "email_taken"},
		{"already member", orgs.ErrAlreadyMember, http.StatusConflict, "already_member"},
		{"invalid status", tasks.ErrInvalidStatus, http.StatusUnprocessableEntity, "invalid_status"},
		{"invalid credentials", users.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
		{"invalid token", auth.ErrInvalidToken, http.StatusUnauthorized, "unauthenticated"},
		{"body error → 400", &bodyError{msg: "bad json"}, http.StatusBadRequest, "invalid_request"},
		{"unknown → 500", errors.New("boom"), http.StatusInternalServerError, "internal_error"},
	}
	s := testServer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
			s.handleError(rec, r, tt.err)

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode envelope: %v (body=%s)", err, rec.Body.String())
			}
			if env.Error.Code != tt.wantBody {
				t.Errorf("error.code = %q, want %q", env.Error.Code, tt.wantBody)
			}
		})
	}
}

// TestHandleErrorWrappedSentinel confirms the mapping uses errors.Is, so a
// wrapped sentinel (the form repos actually return, via fmt.Errorf %w) still
// maps correctly.
func TestHandleErrorWrappedSentinel(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
	wrapped := fmt.Errorf("tasks.GetByID: %w", tasks.ErrNotFound)
	s.handleError(rec, r, wrapped)
	if rec.Code != http.StatusNotFound {
		t.Errorf("wrapped ErrNotFound mapped to %d, want 404", rec.Code)
	}
}

func TestValidationFlattenViaHandleError(t *testing.T) {
	// Feed decodeAndValidate a body that violates a field rule, then push the
	// resulting validator error through handleError and assert the 422 envelope
	// carries flattened field detail.
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/x", strings.NewReader(`{"email":"not-an-email"}`))
	var dst body
	err := decodeAndValidate(r, &dst)
	if err == nil {
		t.Fatal("expected validation error for a bad email")
	}

	s := testServer()
	rec := httptest.NewRecorder()
	s.handleError(rec, r, err)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Error.Code != "validation_failed" {
		t.Errorf("error.code = %q, want validation_failed", env.Error.Code)
	}
	if len(env.Error.Fields) == 0 {
		t.Fatal("expected flattened fields, got none")
	}
	if env.Error.Fields[0].Field != "email" {
		t.Errorf("field = %q, want email", env.Error.Fields[0].Field)
	}
}

package http

import (
	"net/http"

	"beacon/internal/auth"
	"beacon/internal/jobs"
)

// Auth handlers: signup, login, and the authenticated /me lookup.
//
// Course mapping: Chapter 15 — signup hashes the password; Chapter 16 — login
// issues a JWT.

type signupRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Name     string `json:"name"     validate:"max=200"`
	Password string `json:"password" validate:"required,min=12,max=200"`
}

type loginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

// handleSignup creates a user. POST /v1/auth/signup.
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	user, err := s.users.Signup(r.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	// Welcome email — enqueued, never sent inline (Ch 23, the queue boundary).
	if err := s.jobs.Enqueue(r.Context(), jobs.KindSendEmail, jobs.SendEmailPayload{
		To:       user.Email,
		Subject:  "Welcome to Beacon",
		HTMLBody: "<p>Welcome to Beacon! Your account is ready.</p>",
		TextBody: "Welcome to Beacon! Your account is ready.",
	}); err != nil {
		// Best-effort: the account exists; a missed welcome email isn't fatal.
		s.logger.Error("signup: enqueue welcome email", "err", err)
	}

	writeJSON(w, http.StatusCreated, user)
}

// handleLogin verifies credentials and returns a token. POST /v1/auth/login.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	user, token, err := s.users.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token, User: user})
}

// handleMe returns the authenticated user. GET /v1/me (behind requireAuth).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	user, err := s.users.GetByID(r.Context(), userID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

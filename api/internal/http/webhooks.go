package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Webhook handlers, under /v1/orgs/{orgID}/webhooks. Owner/admin only (gated by
// requireRole on the route). Registering returns the generated secret once so
// the customer can configure their receiver.
//
// Course mapping: Chapter 24 — outgoing webhooks (register/list/delete; delivery
// + signing happen in the jobs worker).

type registerWebhookRequest struct {
	URL    string   `json:"url"    validate:"required,url,max=2000"`
	Events []string `json:"events" validate:"omitempty,dive,max=100"`
}

// handleRegisterWebhook registers a webhook for the org. POST .../webhooks.
func (s *Server) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	var req registerWebhookRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	hook, err := s.webhooks.Register(r.Context(), orgID, req.URL, req.Events)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, hook)
}

// handleListWebhooks lists the org's webhooks. GET .../webhooks.
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	list, err := s.webhooks.List(r.Context(), orgID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeList(w, r, list)
}

// handleDeleteWebhook removes a webhook. DELETE .../webhooks/{webhookID}.
func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	id := chi.URLParam(r, "webhookID")
	if err := s.webhooks.Delete(r.Context(), orgID, id); err != nil {
		s.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

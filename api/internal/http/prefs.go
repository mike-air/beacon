package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/language"

	"beacon/internal/auth"
	"beacon/internal/db"
	"beacon/internal/i18n"
)

// Chapter 33's endpoints: read and write the two preferences that decide how
// everything else is rendered.
//
// [glue: the chapter describes the columns and the cascade but does not print a
// handler for them. These are the smallest handlers that make the cascade
// testable end to end.]

type prefsRequest struct {
	// Locale is a BCP-47 tag ("de", "pt-BR"). Empty clears the preference and
	// hands the decision back to the org default and Accept-Language.
	Locale string `json:"locale" validate:"omitempty,max=35"`
	// Timezone is an IANA name ("Europe/Berlin"), never an offset.
	Timezone string `json:"timezone" validate:"omitempty,max=64"`
}

type prefsResponse struct {
	Locale   string `json:"locale"`
	Timezone string `json:"timezone"`
	// Resolved is what the cascade actually decided for THIS request, which is
	// not always what is stored — that is the whole point of a cascade.
	Resolved string `json:"resolved_locale"`
	// The same instant, rendered three ways, so the effect is visible.
	NowUTC   time.Time `json:"now_utc"`
	NowLocal string    `json:"now_local"`
	Greeting string    `json:"greeting"`
	Price    string    `json:"example_price"`
}

// handleGetPrefs returns the caller's preferences and shows what they produce.
// GET /v1/me/preferences
func (s *Server) handleGetPrefs(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	uid, err := uuid.Parse(userID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	row, err := db.New(s.pool).GetUserPreferences(r.Context(), db.GetUserPreferencesParams{ID: uid})
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	tag := i18n.LocaleFromCtx(r.Context())
	tz := i18n.TimezoneFromCtx(r.Context())
	// Stored in UTC (Chapter 33's rule), converted only here, for a human.
	now := time.Now().UTC()

	writeJSON(w, http.StatusOK, prefsResponse{
		Locale:   row.Locale,
		Timezone: row.Timezone,
		Resolved: tag.String(),
		NowUTC:   now,
		NowLocal: i18n.FormatTime(now, tz, tag),
		Greeting: i18n.PrinterFor(tag).Sprintf("Welcome to Beacon"),
		// 1900 minor units of USD. Not 19.00, which is a float and therefore
		// not a price.
		Price: i18n.FormatMoney(1900, "USD"),
	})
}

// handleSetPrefs stores the caller's locale and timezone.
// PATCH /v1/me/preferences
func (s *Server) handleSetPrefs(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFrom(r.Context())
	uid, err := uuid.Parse(userID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	var req prefsRequest
	if err := decodeAndValidate(r, &req); err != nil {
		s.handleError(w, r, err)
		return
	}
	// Reject nonsense at the boundary rather than storing it and failing to
	// parse it on every future request.
	if req.Locale != "" {
		if _, err := language.Parse(req.Locale); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_locale",
				"locale must be a BCP-47 language tag, e.g. \"de\" or \"pt-BR\"")
			return
		}
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_timezone",
			"timezone must be an IANA name, e.g. \"Europe/Berlin\"")
		return
	}

	if err := db.New(s.pool).SetUserPreferences(r.Context(), db.SetUserPreferencesParams{
		ID: uid, Locale: req.Locale, Timezone: req.Timezone,
	}); err != nil {
		s.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package http

import (
	"context"
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
// preferences builds the response body: what is stored, plus what the cascade
// actually resolved for THIS request, plus the same instant rendered three
// ways so the effect of the choice is visible rather than asserted.
//
// Shared by the chi and huma paths so they cannot answer differently.
func (s *Server) preferences(ctx context.Context) (prefsResponse, error) {
	userID, _ := auth.UserIDFrom(ctx)
	uid, err := uuid.Parse(userID)
	if err != nil {
		return prefsResponse{}, err
	}
	row, err := db.New(s.pool).GetUserPreferences(ctx, db.GetUserPreferencesParams{ID: uid})
	if err != nil {
		return prefsResponse{}, err
	}

	tag := i18n.LocaleFromCtx(ctx)
	tz := i18n.TimezoneFromCtx(ctx)
	// Stored in UTC, converted only here, only for a human to read.
	now := time.Now().UTC()

	return prefsResponse{
		Locale:   row.Locale,
		Timezone: row.Timezone,
		Resolved: tag.String(),
		NowUTC:   now,
		NowLocal: i18n.FormatTime(now, tz, tag),
		Greeting: i18n.PrinterFor(tag).Sprintf("Welcome to Beacon"),
		// 1900 minor units of USD. Not 19.00, which is a float and therefore
		// not a price.
		Price: i18n.FormatMoney(1900, "USD"),
	}, nil
}

// savePreferences validates and stores locale and timezone.
//
// Both are rejected at the boundary rather than stored and re-parsed on every
// future request. A bad locale saved once is a bad locale parsed forever.
func (s *Server) savePreferences(ctx context.Context, locale, timezone string) error {
	userID, _ := auth.UserIDFrom(ctx)
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	if locale != "" {
		if _, err := language.Parse(locale); err != nil {
			return &bodyError{msg: `locale must be a BCP-47 language tag, e.g. "de" or "pt-BR"`}
		}
	}
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return &bodyError{msg: `timezone must be an IANA name, e.g. "Europe/Berlin"`}
	}
	return db.New(s.pool).SetUserPreferences(ctx, db.SetUserPreferencesParams{
		ID: uid, Locale: locale, Timezone: timezone,
	})
}

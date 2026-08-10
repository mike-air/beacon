package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"beacon/internal/auth"
	"beacon/internal/db"
	"beacon/internal/i18n"
)

// localeMiddleware resolves the caller's locale once and puts it in the
// context, so no handler ever has to think about it again.
//
// [ch33's LocaleMiddleware, with the preference load added: the chapter's
// version can call i18n.LocaleFrom directly because its auth.User already
// carries Locale and OrgDefaultLocale on the JWT claims. This repo's token
// carries only the user ID (Chapter 16), so the row is read here.]
//
// One database read per request is a real cost. It is bounded by the Chapter 28
// cache in front of memberships and by the fact that this is a single indexed
// row; if it showed up in a profile, the fix would be to put the locale on the
// token and accept that a locale change takes effect at the next login.
func (s *Server) localeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if userID, ok := auth.UserIDFrom(ctx); ok {
			if uid, err := uuid.Parse(userID); err == nil {
				// The org is only known on org-scoped routes; elsewhere the
				// LEFT JOIN simply finds nothing and the cascade skips a step.
				var orgID uuid.UUID
				if raw := chi.URLParam(r, "orgID"); raw != "" {
					if oid, err := uuid.Parse(raw); err == nil {
						orgID = oid
					}
				}
				if row, err := db.New(s.pool).GetUserPreferences(ctx, db.GetUserPreferencesParams{
					ID: uid, ID_2: orgID,
				}); err == nil {
					ctx = i18n.WithPrefs(ctx, i18n.Prefs{
						UserLocale: row.Locale,
						OrgLocale:  row.OrgDefaultLocale,
						Timezone:   row.Timezone,
					})
				}
			}
		}

		tag := i18n.LocaleFrom(ctx, r)
		ctx = i18n.WithLocale(ctx, tag)
		// Content-Language tells caches and clients which language they got.
		// Without it, the Chapter 28 CDN layer would happily serve a German
		// response to an English reader.
		w.Header().Set("Content-Language", tag.String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

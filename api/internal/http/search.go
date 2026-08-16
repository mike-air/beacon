package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"beacon/internal/search"
)

// handleSearch runs a search scoped to one org. GET /v1/orgs/{orgID}/search?q=
//
// requireOrg has already established that the caller is a member of {orgID},
// and the org id goes straight into the query's WHERE clause, so the tenancy
// boundary is enforced twice: once in the middleware, once in the SQL.
//
// Course mapping: Chapter 29 — full-text search; Chapter 30 — the same endpoint
// once Meilisearch is in front of it. Note that the handler cannot tell which
// engine answered; that is the point of putting the fallback in the service.
//
// [verbatim ch29's handler, with the auth check dropped because this repo does
// it in middleware (requireOrg) rather than inline, and this repo's parsePaging
// instead of the chapter's paginate.]
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")

	q := r.URL.Query().Get("q")
	if len(q) < 2 {
		writeError(w, http.StatusUnprocessableEntity, "query_too_short",
			"search query must be at least 2 characters")
		return
	}
	// A 4,000-character query would be turned into a 4,000-lexeme tsquery and
	// walk the whole posting list. Truncate rather than reject: nobody typed
	// 200 meaningful characters into a search box.
	if len(q) > 200 {
		q = q[:200]
	}

	limit, offset := parsePaging(r)

	res, err := s.search.Search(r.Context(), search.SearchInput{
		OrgID:  orgID,
		Query:  q,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

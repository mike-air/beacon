package http

// Organisations, members and search.
//
// The organisation is the tenancy boundary, so every operation below /v1/orgs
// carries orgScoped or orgAdmin — proven membership, and for the admin ones a
// role that ranks at or above admin. The server enforces it; the client hides
// the buttons. Both, because one of them is a convenience and the other is the
// actual control.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/auth"
	"beacon/internal/jobs"
	"beacon/internal/orgs"
	"beacon/internal/search"
)

type CreateOrgInput struct {
	IdempotencyHeader
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"200" required:"true"`
	}
}

type OrgOutput struct {
	Status int
	Body   orgs.Org
}

type ListOrgsInput struct{ Paging }

type ListOrgsOutput struct {
	Body ListBody[orgs.OrgWithRole]
}

type ListMembersInput struct {
	OrgPath
	Paging
}

type ListMembersOutput struct {
	Body ListBody[orgs.Member]
}

type AddMemberInput struct {
	OrgPath
	IdempotencyHeader
	Body struct {
		Email string `json:"email" format:"email" required:"true"`
		Role  string `json:"role" required:"true" enum:"owner,admin,member"`
	}
}

type AddMemberOutput struct {
	Status int
	Body   orgs.Membership
}

type SearchInput struct {
	OrgPath
	Paging
	// Two characters is the server's floor; below it the query is rejected
	// rather than run, because a one-character tsquery matches most of the
	// index and costs the same as a table scan.
	Q string `query:"q" required:"true" minLength:"2" doc:"At least 2 characters. Longer than 200 is truncated, not rejected."`
}

type SearchOutput struct {
	Body search.SearchResult
}

func (s *Server) registerOrgs(api huma.API, g gates) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-org",
		Method:        http.MethodPost,
		Path:          "/v1/orgs",
		Summary:       "Create an organisation",
		Description:   "The caller becomes its owner.",
		Tags:          []string{"orgs"},
		Security:      []map[string][]string{{"bearerAuth": {}}},
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.authed,
	}, func(ctx context.Context, in *CreateOrgInput) (*OrgOutput, error) {
		userID, _ := auth.UserIDFrom(ctx)
		org, err := s.orgs.CreateOrg(ctx, userID, in.Body.Name)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &OrgOutput{Status: http.StatusCreated, Body: org}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-orgs",
		Method:      http.MethodGet,
		Path:        "/v1/orgs",
		Summary:     "Organisations the caller belongs to, each with their role",
		Tags:        []string{"orgs"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.authed,
	}, func(ctx context.Context, in *ListOrgsInput) (*ListOrgsOutput, error) {
		userID, _ := auth.UserIDFrom(ctx)
		list, err := s.orgs.ListForUser(ctx, userID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ListOrgsOutput{Body: page(list, in.Limit, in.Offset)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-members",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/members",
		Summary:     "Members of the organisation",
		Tags:        []string{"members"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *ListMembersInput) (*ListMembersOutput, error) {
		list, err := s.orgs.Members(ctx, in.OrgID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &ListMembersOutput{Body: page(list, in.Limit, in.Offset)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "add-member",
		Method:        http.MethodPost,
		Path:          "/v1/orgs/{orgID}/members",
		Summary:       "Add an existing user to the organisation",
		Description:   "Beacon adds accounts that already exist; it does not email invitations.",
		Tags:          []string{"members"},
		Security:      []map[string][]string{{"bearerAuth": {}}},
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.orgAdmin,
	}, func(ctx context.Context, in *AddMemberInput) (*AddMemberOutput, error) {
		actorRole, _ := auth.RoleFrom(ctx)
		m, err := s.orgs.AddMember(ctx, in.OrgID, actorRole, in.Body.Email, in.Body.Role)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}

		// Enqueued, never inline: a slow mail server must not make adding a
		// colleague slow, and a broken one must not make it fail.
		if err := s.jobs.Enqueue(ctx, jobs.KindSendEmail, jobs.SendEmailPayload{
			To:       in.Body.Email,
			Subject:  "You were added to an organization on Beacon",
			HTMLBody: "<p>You were added to a Beacon organization. Sign in to get started.</p>",
			TextBody: "You were added to a Beacon organization. Sign in to get started.",
		}); err != nil {
			s.logger.Error("add member: enqueue notification email", "err", err)
		}
		return &AddMemberOutput{Status: http.StatusCreated, Body: m}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "search",
		Method:      http.MethodGet,
		Path:        "/v1/orgs/{orgID}/search",
		Summary:     "Search every entity in the organisation",
		Description: "`source` reports which engine answered. `postgres` means the " +
			"Meilisearch fallback fired and results will be noticeably worse — " +
			"a client that hides that is hiding a live incident.",
		Tags:        []string{"search"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.orgScoped,
	}, func(ctx context.Context, in *SearchInput) (*SearchOutput, error) {
		q := in.Q
		// Truncate rather than reject. Nobody typed 200 meaningful characters
		// into a search box, and a 4,000-lexeme tsquery walks the whole
		// posting list.
		if len(q) > 200 {
			q = q[:200]
		}
		res, err := s.search.Search(ctx, search.SearchInput{
			OrgID: in.OrgID, Query: q, Limit: in.Limit, Offset: in.Offset,
		})
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &SearchOutput{Body: res}, nil
	})
}

// page slices an already-loaded result set into the shared list envelope.
//
// Used by the endpoints whose result sets are inherently small — a user's
// organisations, an organisation's members. Those queries load everything
// anyway, so paging here keeps the response shape honest without pretending
// the database did work it did not do. Endpoints that can return unbounded
// rows page in SQL and pass their already-limited slice straight through.
func page[T any](items []T, limit, offset int) ListBody[T] {
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return ListBody[T]{Items: items[offset:end], Limit: limit, Offset: offset}
}

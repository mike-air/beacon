package http

// The /v1/me operations.
//
// This file is the pattern every other operation file follows, so it is worth
// reading once slowly.
//
// A handler takes a typed Input and returns a typed Output. Those two structs
// ARE the OpenAPI document: path parameters, query string, request body,
// response body and status are all read off them when the operation is
// registered. There is no second place where the shape is written down, which
// is the entire reason for this design.
//
// The service layer below is untouched. s.users.GetByID neither knows nor
// cares that the shape of its caller changed.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/auth"
	"beacon/internal/users"
)

// MeOutput is the response. `Body` is special to huma: it is the field that
// gets serialized. Any other exported field becomes a response header.
type MeOutput struct {
	Body users.User
}

type PreferencesOutput struct {
	Body prefsResponse
}

type SetPreferencesInput struct {
	IdempotencyHeader
	Body struct {
		// A BCP-47 tag ("de", "pt-BR"). Empty clears the preference and hands
		// the decision back to the org default and Accept-Language.
		Locale string `json:"locale" maxLength:"35"`
		// An IANA name ("Europe/Berlin"), never an offset — an offset is a
		// fact about one instant, and a zone is the rule that produced it.
		Timezone string `json:"timezone" maxLength:"64"`
	}
}

func (s *Server) registerMe(api huma.API, g gates) {
	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/v1/me",
		Summary:     "The authenticated user",
		Tags:        []string{"me"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.authed,
	}, func(ctx context.Context, _ *struct{}) (*MeOutput, error) {
		userID, _ := auth.UserIDFrom(ctx)
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &MeOutput{Body: user}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-preferences",
		Method:      http.MethodGet,
		Path:        "/v1/me/preferences",
		Summary:     "Locale and timezone, and what the cascade resolved for this request",
		Tags:        []string{"me"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.authed,
	}, func(ctx context.Context, _ *struct{}) (*PreferencesOutput, error) {
		body, err := s.preferences(ctx)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &PreferencesOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-preferences",
		Method:      http.MethodPatch,
		Path:        "/v1/me/preferences",
		Summary:     "Set locale and timezone",
		Tags:        []string{"me"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: g.authed,
	}, func(ctx context.Context, in *SetPreferencesInput) (*PreferencesOutput, error) {
		if err := s.savePreferences(ctx, in.Body.Locale, in.Body.Timezone); err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		body, err := s.preferences(ctx)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &PreferencesOutput{Body: body}, nil
	})
}

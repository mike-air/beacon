package http

// The /v1/me operations, converted to huma.
//
// This file is the pattern every other operation follows, so it is worth
// reading once carefully.
//
// A handler now takes a typed Input and returns a typed Output. Those two
// structs ARE the OpenAPI document: the path parameters, the query string,
// the request body, the response body and the status code are all read off
// them by huma when the operation is registered. There is no second place
// where the shape is written down, which is the whole reason for the change.
//
// The service layer below is untouched. `s.users.GetByID` neither knows nor
// cares that its caller changed shape.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/auth"
	"beacon/internal/users"
)

// MeOutput is the response. The `Body` field is special to huma: it is the
// thing that gets serialized. Anything else on the struct becomes a header.
type MeOutput struct {
	Body users.User
}

// PreferencesInput carries no parameters; the caller is identified by their
// token, which the auth gate has already resolved into a context value.
type PreferencesInput struct{}

type PreferencesOutput struct {
	Body prefsResponse
}

type SetPreferencesInput struct {
	Body prefsRequest
}

// registerMe wires the three /v1/me operations.
//
// Each one lists its own gates. `authed` means: a valid bearer token, inside
// the org's rate limit, with the locale resolved. Reading that line tells you
// what protects the operation without scrolling anywhere.
func (s *Server) registerMe(api huma.API, authed huma.Middlewares) {
	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/v1/me",
		Summary:     "The authenticated user",
		Tags:        []string{"me"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
		Middlewares: authed,
	}, func(ctx context.Context, _ *struct{}) (*MeOutput, error) {
		userID, _ := auth.UserIDFrom(ctx)
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		return &MeOutput{Body: user}, nil
	})
}

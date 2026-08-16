package http

// The public operations: signup and login.
//
// These are the only two routes with no bearer token, so they are the only two
// gated by IP rather than by tenant. The limit is deliberately tight — five a
// minute — because nobody legitimately fires hundreds of logins a minute from
// one address, and the cost of being wrong about that is somebody's password
// being guessed.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"beacon/internal/jobs"
	"beacon/internal/users"
)

type SignupInput struct {
	Body struct {
		Email    string `json:"email" format:"email" required:"true"`
		Name     string `json:"name" maxLength:"200"`
		Password string `json:"password" minLength:"12" maxLength:"200" required:"true"`
	}
}

// SignupOutput returns the user and NOT a token. Signing up and signing in are
// two calls on purpose: the client that pretends otherwise ends up with a
// session it never actually verified.
type SignupOutput struct {
	Status int
	Body   users.User
}

type LoginInput struct {
	Body struct {
		Email    string `json:"email" format:"email" required:"true"`
		Password string `json:"password" required:"true"`
	}
}

type LoginOutput struct {
	Body struct {
		Token string     `json:"token" doc:"A bearer JWT. There is no refresh endpoint."`
		User  users.User `json:"user"`
	}
}

func (s *Server) registerAuth(api huma.API, g gates) {
	huma.Register(api, huma.Operation{
		OperationID:   "signup",
		Method:        http.MethodPost,
		Path:          "/v1/auth/signup",
		Summary:       "Create an account",
		Description:   "Returns the user. It does not return a token — call login next.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   g.public,
	}, func(ctx context.Context, in *SignupInput) (*SignupOutput, error) {
		user, err := s.users.Signup(ctx, in.Body.Email, in.Body.Name, in.Body.Password)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}

		// Enqueued, never sent inline. A slow SMTP server must not be able to
		// make signing up slow, and a broken one must not make it fail.
		if err := s.jobs.Enqueue(ctx, jobs.KindSendEmail, jobs.SendEmailPayload{
			To:       user.Email,
			Subject:  "Welcome to Beacon",
			HTMLBody: "<p>Welcome to Beacon! Your account is ready.</p>",
			TextBody: "Welcome to Beacon! Your account is ready.",
		}); err != nil {
			// Best effort: the account exists, and a missed welcome email is
			// not worth failing the request the user is waiting on.
			s.logger.Error("signup: enqueue welcome email", "err", err)
		}
		return &SignupOutput{Status: http.StatusCreated, Body: user}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/v1/auth/login",
		Summary:     "Exchange credentials for a token",
		Tags:        []string{"auth"},
		Middlewares: g.public,
	}, func(ctx context.Context, in *LoginInput) (*LoginOutput, error) {
		user, token, err := s.users.Login(ctx, in.Body.Email, in.Body.Password)
		if err != nil {
			return nil, s.asHumaError(ctx, err)
		}
		out := &LoginOutput{}
		out.Body.Token = token
		out.Body.User = user
		return out, nil
	})
}

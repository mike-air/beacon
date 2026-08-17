package http

// The OpenAPI layer.
//
// Every route in this service is declared through huma, which reads the Go
// input and output structs and writes openapi.json from them. That document
// generates the TypeScript SDK, so the chain from a Go field to a compile
// error in the web app has no hand-written link in it.
//
// Two things here exist purely to keep that machinery from changing Beacon's
// observable behaviour:
//
//   1. huma's default error body is RFC 7807 problem+json. Beacon's clients
//      read {"error":{"code","message","fields"}} and branch on `code`.
//      newErrorEnvelope keeps the shape Beacon has always had.
//
//   2. huma stamps a "$schema" property onto every response body by default.
//      It is a genuinely useful feature and it would appear in every parsed
//      object in the client, so it is turned off.

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// apiTitle and apiVersion appear in the emitted document and, from there, in
// the generated SDK's package metadata.
const (
	apiTitle   = "Beacon API"
	apiVersion = "1.0.0"
)

// humaError renders Beacon's error envelope for failures huma produces
// itself — a bad path parameter, an unparseable body, a 404 from the router.
//
// Without it, a request that failed inside huma would answer in RFC 7807
// problem+json and a request that failed inside a handler would answer in
// Beacon's envelope, and every client would have to understand both.
//
// It cannot simply reuse response.go's errorEnvelope: huma.StatusError
// requires an Error() METHOD, and that type has an Error FIELD. So this
// type carries the same JSON shape and satisfies the interface separately.
type humaError struct {
	status int
	Body   errorBody `json:"error"`
}

func (e *humaError) GetStatus() int { return e.status }

// Error satisfies both `error` and huma.StatusError. The string form is for
// logs; the JSON body is what a client reads.
func (e *humaError) Error() string { return e.Body.Message }

// newErrorEnvelope is installed as huma.NewError. huma hands it a status, a
// message and any per-field detail it collected while validating the request
// against the spec; the field detail is what a form needs to mark the right
// input, so it is carried across rather than flattened into the message.
func newErrorEnvelope(status int, msg string, errs ...error) huma.StatusError {
	env := &humaError{
		status: status,
		Body: errorBody{
			Code:    codeForStatus(status),
			Message: msg,
		},
	}
	for _, err := range errs {
		var detail *huma.ErrorDetail
		if ok := asErrorDetail(err, &detail); ok {
			env.Body.Fields = append(env.Body.Fields, fieldError{
				Field:   detail.Location,
				Rule:    "invalid",
				Message: detail.Message,
			})
			continue
		}
		// An error huma did not annotate with a location still deserves to be
		// seen; append it to the message rather than dropping it silently.
		if err != nil {
			env.Body.Message += "; " + err.Error()
		}
	}
	return env
}

func asErrorDetail(err error, out **huma.ErrorDetail) bool {
	d, ok := err.(*huma.ErrorDetail)
	if ok {
		*out = d
	}
	return ok
}

// codeForStatus gives huma-generated failures the same stable `code` values
// Beacon's own handlers use, so a client can branch on one vocabulary.
func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthenticated"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusNotImplemented:
		return "not_implemented"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "error"
	}
}

// writeHumaError writes an already-classified error (from asHumaError)
// straight onto the response, for the one place that needs it: gates, which
// run before an operation and so cannot return (Output, error) the way a
// handler does — huma.WriteErr is not an option for them.
//
// This exists because huma.WriteErr cannot carry a domain-specific code.
// It funnels through huma.NewError (= newErrorEnvelope above), which only
// ever has a status and a message to work with, so codeForStatus's generic
// per-status default is all it can produce. That is wrong whenever classify()
// has a more specific answer — "you are not a member of this organization"
// must read code: "not_member", not the generic "forbidden" every other 403
// gets, because the web client's toast text branches on exactly that string.
// A gate that wants classify()'s answer has to write the body itself.
func writeHumaError(ctx huma.Context, err error) {
	he, ok := err.(*humaError)
	if !ok {
		// Should not happen — asHumaError always returns *humaError — but a
		// 500 is the correct fallback for an error this function does not
		// recognise. See classify()'s own default arm for the same reasoning.
		he = &humaError{status: http.StatusInternalServerError, Body: errorBody{
			Code: "internal_error", Message: "something went wrong on our end",
		}}
	}
	ctx.SetHeader("Content-Type", "application/json")
	ctx.SetStatus(he.status)
	_ = json.NewEncoder(ctx.BodyWriter()).Encode(he)
}

// newHumaAPI wraps an existing chi router. chi stays the router: every
// middleware already mounted on it keeps running, and chi still parses path
// parameters. What huma adds is the operation registry and the document.
func newHumaAPI(r chi.Router) huma.API {
	cfg := huma.DefaultConfig(apiTitle, apiVersion)

	// Drop the "$schema" property huma adds to every response.
	//
	// DefaultConfig installs a SchemaLinkTransformer through CreateHooks, and
	// that one object does two separate things: it appends a response
	// Transformer (which puts "$schema" in the body) AND an OnAddOperation
	// hook (which declares "$schema" as a property of the schema itself).
	// Clearing Transformers alone removes it from responses and leaves it in
	// the document — which is worse than either extreme, because the generated
	// client would then carry a field the server never sends.
	//
	// The hook has not run yet at this point; it runs when the adapter builds
	// the API. So the removal has to happen here, before that.
	//
	// The feature is a genuine convenience for a human exploring the API by
	// hand. It is pure noise for a generated client, where it would appear on
	// every parsed object.
	cfg.CreateHooks = nil
	cfg.Transformers = nil

	cfg.Info.Description = "A multi-tenant task board. " +
		"This document is emitted from the Go handlers; it is never edited by hand."

	// One security scheme, matching what requireAuth actually accepts.
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description: "The token from POST /v1/auth/login. " +
				"There is no refresh endpoint; when it expires, sign in again.",
		},
	}

	return humachi.New(r, cfg)
}

func init() {
	// Installed once, globally, because huma looks it up by package variable
	// rather than per-API. Doing it in init keeps it impossible to construct
	// an API that answers in the wrong error shape.
	huma.NewError = newErrorEnvelope
}

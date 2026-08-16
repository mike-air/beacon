package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

// One shared, goroutine-safe validator built once at startup. It reports field
// names using their json tag so error messages match what the client sent.
//
// Course mapping: Chapter 11 — request validation.
var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}

const maxBodyBytes = 1 << 20 // 1 MiB

// decodeAndValidate reads a single JSON object from the request body into dst,
// rejecting unknown fields and oversized bodies, then runs the struct-tag
// validation. It returns a *bodyError for shape problems (→ 400) or
// validator.ValidationErrors for field-rule failures (→ 422).
func decodeAndValidate(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return &bodyError{msg: friendlyDecodeError(err)}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &bodyError{msg: "body must contain a single JSON object"}
	}
	return validate.Struct(dst)
}

// bodyError marks errors from the request body itself (not field rules).
type bodyError struct{ msg string }

func (b *bodyError) Error() string { return b.msg }

func friendlyDecodeError(err error) string {
	var syn *json.SyntaxError
	var ute *json.UnmarshalTypeError
	var mbe *http.MaxBytesError
	switch {
	case errors.As(err, &syn):
		return "request body contains malformed JSON"
	case errors.As(err, &ute):
		return "field " + ute.Field + ": wrong type"
	case errors.As(err, &mbe):
		return "request body too large"
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		return "unknown field in request body"
	case errors.Is(err, io.EOF):
		return "request body is empty"
	}
	return "could not parse request body"
}

// flatten turns validator's error type into a flat list for the wire.
type fieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func flatten(err error) []fieldError {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return nil
	}
	out := make([]fieldError, 0, len(verrs))
	for _, fe := range verrs {
		out = append(out, fieldError{Field: fe.Field(), Rule: fe.Tag(), Message: humanMessage(fe)})
	}
	return out
}

func humanMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "this field is required"
	case "min":
		return "must be at least " + fe.Param() + " characters"
	case "max":
		return "must be at most " + fe.Param() + " characters"
	case "oneof":
		return "must be one of: " + fe.Param()
	case "email":
		return "must be a valid email address"
	case "uuid":
		return "must be a valid UUID"
	}
	return "failed " + fe.Tag() + " check"
}

// Pagination (Chapter 13). Offset-based, kept simple per the project spec.
const (
	defaultLimit = 20
	maxLimit     = 100
)

// parsePaging reads ?limit= and ?offset= with sane caps. Bad values fall back
// to defaults rather than erroring — list endpoints stay forgiving.
func parsePaging(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			offset = n
		}
	}
	return limit, offset
}

// writeList answers with the one list envelope, paging an already-loaded
// slice.
//
// Five of the nine list endpoints used to answer a bare {"items": [...]}
// instead — so a client needed a special case per URL, and the ones written
// against the documented shape broke on the undocumented one. Routing every
// list through here means a new endpoint gets the right shape by default
// rather than by remembering.
//
// The slice is paged here rather than in SQL for the endpoints whose result
// sets are inherently small (a user's orgs, an org's members, a task's
// comments and files, an org's webhooks). Those queries load everything
// anyway; paging in the handler makes the response honest without pretending
// the database did work it did not do. The endpoints that CAN return
// unbounded rows — projects, tasks, search — page in SQL, and pass their
// already-limited slice straight through.
func writeList[T any](w http.ResponseWriter, r *http.Request, items []T) {
	limit, offset := parsePaging(r)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	writeJSON(w, http.StatusOK, listResponse{
		Items:  items[offset:end],
		Limit:  limit,
		Offset: offset,
	})
}

// listResponse is the shape every paginated list endpoint returns.
type listResponse struct {
	Items  any `json:"items"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

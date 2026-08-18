package http

// Shapes every operation shares.
//
// These are the types the OpenAPI document is built from, so a change here
// changes the generated SDK and, one compile later, the web app. That is the
// intent: there is exactly one definition of what a paged list looks like.

import "beacon/internal/orgs"

// ListBody is the envelope every list endpoint returns.
//
// Five of Beacon's nine list endpoints once answered a bare {"items": [...]}
// while the other four answered this, so a client needed a special case per
// URL. Making it generic means a new list endpoint cannot get the shape wrong
// without saying so in its type.
type ListBody[T any] struct {
	// nullable:"false" because listBody guarantees a non-nil slice. huma
	// infers nullability from the Go type — every []T can be nil — so without
	// this the contract advertises a null the server never sends.
	Items  []T `json:"items" nullable:"false"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// listBody builds the envelope, and exists for one reason: a nil Go slice
// marshals to JSON null.
//
// Without this, an empty page answers {"items": null} and a non-empty one
// answers {"items": [...]}, so every client has to handle two spellings of
// "nothing here" — and the ones that do not crash on the first empty board a
// user opens. Constructing the envelope through a function makes the
// normalisation impossible to forget, which a struct literal does not.
func listBody[T any](items []T, limit, offset int) ListBody[T] {
	if items == nil {
		items = []T{}
	}
	return ListBody[T]{Items: items, Limit: limit, Offset: offset}
}

// Paging is embedded in any input that reads a page. The defaults and the
// maximum land in the spec, so the SDK and the server agree on the bounds
// without either restating them.
type Paging struct {
	Limit  int `query:"limit" default:"20" minimum:"1" maximum:"100" doc:"How many rows to return"`
	Offset int `query:"offset" default:"0" minimum:"0" doc:"How many rows to skip"`
}

// OrgPath is embedded in every organisation-scoped input.
type OrgPath struct {
	OrgID string `path:"orgID" format:"uuid" doc:"The organisation"`
}

// ProjectPath extends OrgPath for project-scoped routes.
type ProjectPath struct {
	OrgPath
	ProjectID string `path:"projectID" format:"uuid" doc:"The project"`
}

// TaskPath extends ProjectPath for task-scoped routes.
type TaskPath struct {
	ProjectPath
	TaskID string `path:"taskID" format:"uuid" doc:"The task"`
}

// CommentPath extends TaskPath for the two routes that address one comment.
type CommentPath struct {
	TaskPath
	CommentID string `path:"commentID" format:"uuid" doc:"The comment"`
}

// IdempotencyHeader is embedded in every mutating input.
//
// Declaring it here is what puts the header in the OpenAPI document, which is
// what makes it appear in the generated SDK. Beacon has always honoured the
// header; until it was written down, no generated client knew to send it.
type IdempotencyHeader struct {
	IdempotencyKey string `header:"Idempotency-Key" required:"false" doc:"Repeating a request with the same key replays the original response instead of performing the write twice."`
}

// roleValues is the enum the spec advertises for a membership role. It is
// derived from the domain package rather than restated, so adding a role in
// one place cannot leave the contract describing the old set.
var roleValues = []any{orgs.RoleOwner, orgs.RoleAdmin, orgs.RoleMember}

// Typed context keys that carry the authenticated user ID and their per-org
// role through a request. The keys are private zero-size types nothing outside
// this package can construct — no string collisions, no accidental overwrites.
//
// Course mapping: Chapter 16 — user context; Chapter 17 — the per-membership
// role that RBAC checks against.
package auth

import "context"

type userIDKey struct{}
type roleKey struct{}

// WithUserID returns a copy of ctx carrying the authenticated user ID.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

// UserIDFrom returns the authenticated user ID and whether one was present.
func UserIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey{}).(string)
	return id, ok
}

// WithRole returns a copy of ctx carrying the caller's role in the active org.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey{}, role)
}

// RoleFrom returns the caller's role in the active org and whether one was set.
func RoleFrom(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleKey{}).(string)
	return role, ok
}

// UserIDContextKey and RoleContextKey expose the private key types so a
// middleware layer that builds contexts through someone else's API — huma's
// WithValue, which takes a key rather than a whole context — can write the
// same values WithUserID and WithRole write.
//
// The keys stay unexported struct types, so no other package can collide with
// them by using the same string. Only the ability to name them is shared.
func UserIDContextKey() any { return userIDKey{} }
func RoleContextKey() any   { return roleKey{} }

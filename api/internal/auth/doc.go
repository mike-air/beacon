// Package auth owns credentials: argon2id password hashing, JWT issue and
// verify, and the typed context keys that carry the authenticated user and
// their org role through a request. Domain packages and the HTTP layer both
// lean on it, but it imports neither — it is a leaf, and it stays one.
//
//	password.go  argon2id hash and verify        Chapter 15
//	token.go     JWT issue and parse             Chapter 16
//	context.go   the typed context keys          Chapters 16–17
//
// # The two decisions worth knowing
//
// There is no session table. A signed access token IS the proof — verifying a
// signature is arithmetic, so a request costs no database round trip to
// authenticate. The price is that a token cannot be revoked before it
// expires, which is why the TTL is short.
//
// The context keys are private zero-size types nothing outside this package
// can construct. A string key would let any package in the process write
// "user_id" into a context and be believed.
//
// # Deviations from the course
//
// The course stores password parameters in Config and rehashes lazily on
// login. This keeps one fixed OWASP-recommended parameter set; the encoded
// hash still records its own parameters, so rotation can be added later
// without invalidating a single stored password.
//
// The course also builds database-backed refresh-token rotation. This has one
// access token (IssueToken/ParseToken) and says so rather than pretending
// otherwise — READING.md lists it as work left to do.
package auth

// Integration tests for the users repository + service against a real Postgres.
//
// Course mapping: Chapter 39 — integration tests. ENV-GATED: skips unless
// TEST_DATABASE_URL is set (see internal/testsupport; we use that instead of
// testcontainers to keep the go1.23 pin). No build tag — this compiles in the
// normal build and just skips locally.
package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"beacon/internal/auth"
	"beacon/internal/testsupport"
	"beacon/internal/users"
)

func TestSignupAndLogin(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	const secret = "test-secret"
	svc := users.NewService(users.NewRepo(pool), secret, time.Hour)

	// Signup creates a user.
	u, err := svc.Signup(ctx, "ada@example.com", "Ada", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if u.ID == "" || u.Email != "ada@example.com" {
		t.Fatalf("Signup returned %+v", u)
	}
	if u.PasswordHash == "" {
		t.Error("Signup did not store a password hash")
	}

	// Duplicate email (case-insensitive) → ErrEmailTaken.
	if _, err := svc.Signup(ctx, "ADA@example.com", "Ada2", "another good password!!"); !errors.Is(err, users.ErrEmailTaken) {
		t.Fatalf("duplicate signup err = %v, want ErrEmailTaken", err)
	}

	// Login verifies the password and issues a token.
	got, token, err := svc.Login(ctx, "ada@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("Login user ID = %q, want %q", got.ID, u.ID)
	}
	subject, err := auth.ParseToken(secret, token)
	if err != nil {
		t.Fatalf("issued token did not parse: %v", err)
	}
	if subject != u.ID {
		t.Errorf("token subject = %q, want %q", subject, u.ID)
	}

	// Wrong password → ErrInvalidCredentials.
	if _, _, err := svc.Login(ctx, "ada@example.com", "wrong"); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("wrong-password login err = %v, want ErrInvalidCredentials", err)
	}
	// Unknown email → ErrInvalidCredentials (no account enumeration).
	if _, _, err := svc.Login(ctx, "nobody@example.com", "whatever"); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("unknown-email login err = %v, want ErrInvalidCredentials", err)
	}
}

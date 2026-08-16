package auth

import (
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	const pw = "correct horse battery staple"

	encoded, err := Hash(pw)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := Verify(pw, encoded)
	if err != nil {
		t.Fatalf("Verify (right password): %v", err)
	}
	if !ok {
		t.Fatal("Verify returned false for the correct password")
	}

	ok, err = Verify("wrong password", encoded)
	if err != nil {
		t.Fatalf("Verify (wrong password): %v", err)
	}
	if ok {
		t.Fatal("Verify returned true for a wrong password")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	const (
		secret = "test-secret"
		userID = "11111111-1111-1111-1111-111111111111"
	)

	tok, err := IssueToken(secret, userID, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	got, err := ParseToken(secret, tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if got != userID {
		t.Fatalf("ParseToken = %q, want %q", got, userID)
	}

	if _, err := ParseToken("other-secret", tok); err == nil {
		t.Fatal("ParseToken accepted a token signed with a different secret")
	}
}

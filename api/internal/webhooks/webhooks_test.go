package webhooks

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/db"
)

// Course mapping: Chapter 24 — HMAC-SHA256 signing. A receiver re-signs the body
// with the shared secret and compares; we verify our Sign/Verify round-trips and
// rejects tampering.
func TestSignVerifyRoundTrip(t *testing.T) {
	secret := "shhh-super-secret"
	body := []byte(`{"event":"task.created","data":{"id":"abc"}}`)

	sig := Sign(secret, body)
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("signature should be prefixed sha256=, got %q", sig)
	}
	if !Verify(secret, body, sig) {
		t.Fatal("Verify rejected a signature it should accept")
	}
}

func TestVerifyRejectsTamperedBody(t *testing.T) {
	secret := "shhh"
	body := []byte(`{"amount":10}`)
	sig := Sign(secret, body)

	tampered := []byte(`{"amount":1000000}`)
	if Verify(secret, tampered, sig) {
		t.Fatal("Verify accepted a tampered body")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	body := []byte(`{"x":1}`)
	sig := Sign("right-secret", body)

	if Verify("wrong-secret", body, sig) {
		t.Fatal("Verify accepted a signature made with a different secret")
	}
}

func TestSignIsDeterministic(t *testing.T) {
	secret := "k"
	body := []byte("payload")
	if Sign(secret, body) != Sign(secret, body) {
		t.Fatal("Sign should be deterministic for the same input")
	}
}

// TestSecretIsRedactedExceptOnCreate pins the rule the UI states out loud:
// "It is shown once. Beacon cannot show it again."
//
// It did not used to be true. toWebhook copied the secret through, so listing
// webhooks returned the full HMAC key for every one of them on every page load
// of the settings screen.
func TestSecretIsRedactedExceptOnCreate(t *testing.T) {
	row := db.Webhook{
		ID:     uuid.New(),
		OrgID:  uuid.New(),
		Url:    "https://example.com/hook",
		Secret: "a-real-signing-secret",
		Events: []string{"task.created"},
		Active: true,
	}

	if got := toWebhook(row).Secret; got != "" {
		t.Errorf("toWebhook leaked the secret: %q", got)
	}
	if got := toWebhookWithSecret(row).Secret; got != row.Secret {
		t.Errorf("create must return the secret in full, got %q", got)
	}
}

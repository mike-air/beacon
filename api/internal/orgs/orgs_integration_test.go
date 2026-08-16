// Integration tests for the orgs repository + service against a real Postgres.
//
// Course mapping: Chapter 39 — integration tests. ENV-GATED: skips unless
// TEST_DATABASE_URL is set (testsupport instead of testcontainers). No build tag.
package orgs_test

import (
	"context"
	"errors"
	"testing"

	"beacon/internal/orgs"
	"beacon/internal/testsupport"
	"beacon/internal/users"
)

// newUser creates a user directly through the repo and returns its ID. The
// password hash is a placeholder — these tests exercise orgs, not auth.
func newUser(t *testing.T, repo *users.Repo, email string) string {
	t.Helper()
	u, err := repo.Create(context.Background(), email, "Test", "hash-placeholder")
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u.ID
}

func TestCreateOrgMakesOwner(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	usersRepo := users.NewRepo(pool)
	orgsRepo := orgs.NewRepo(pool)
	svc := orgs.NewService(orgsRepo, pool)

	ownerID := newUser(t, usersRepo, "owner@example.com")

	org, err := svc.CreateOrg(ctx, ownerID, "Acme")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	if org.ID == "" || org.Name != "Acme" {
		t.Fatalf("CreateOrg returned %+v", org)
	}

	// The creator is an owner — in the same transaction.
	m, err := orgsRepo.GetMembership(ctx, ownerID, org.ID)
	if err != nil {
		t.Fatalf("GetMembership: %v", err)
	}
	if m.Role != orgs.RoleOwner {
		t.Errorf("creator role = %q, want owner", m.Role)
	}

	// ListForUser returns the org with the role.
	list, err := svc.ListForUser(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(list) != 1 || list[0].ID != org.ID || list[0].Role != orgs.RoleOwner {
		t.Fatalf("ListForUser = %+v, want one owned org", list)
	}
}

func TestAddMemberRBAC(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	usersRepo := users.NewRepo(pool)
	svc := orgs.NewService(orgs.NewRepo(pool), pool)

	ownerID := newUser(t, usersRepo, "owner@example.com")
	_ = newUser(t, usersRepo, "bob@example.com")
	_ = newUser(t, usersRepo, "carol@example.com")

	org, err := svc.CreateOrg(ctx, ownerID, "Acme")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	// A member-rank actor cannot add anyone.
	if _, err := svc.AddMember(ctx, org.ID, orgs.RoleMember, "bob@example.com", orgs.RoleMember); !errors.Is(err, orgs.ErrForbidden) {
		t.Fatalf("member adding member: err = %v, want ErrForbidden", err)
	}

	// An admin actor may add a member.
	if _, err := svc.AddMember(ctx, org.ID, orgs.RoleAdmin, "bob@example.com", orgs.RoleMember); err != nil {
		t.Fatalf("admin adding member: %v", err)
	}

	// Only an owner may mint another owner — an admin cannot.
	if _, err := svc.AddMember(ctx, org.ID, orgs.RoleAdmin, "carol@example.com", orgs.RoleOwner); !errors.Is(err, orgs.ErrForbidden) {
		t.Fatalf("admin granting owner: err = %v, want ErrForbidden", err)
	}
	if _, err := svc.AddMember(ctx, org.ID, orgs.RoleOwner, "carol@example.com", orgs.RoleOwner); err != nil {
		t.Fatalf("owner granting owner: %v", err)
	}

	// Adding the same user twice → ErrAlreadyMember.
	if _, err := svc.AddMember(ctx, org.ID, orgs.RoleOwner, "bob@example.com", orgs.RoleMember); !errors.Is(err, orgs.ErrAlreadyMember) {
		t.Fatalf("duplicate add: err = %v, want ErrAlreadyMember", err)
	}

	// Adding an unknown email → ErrUserNotFound.
	if _, err := svc.AddMember(ctx, org.ID, orgs.RoleOwner, "ghost@example.com", orgs.RoleMember); !errors.Is(err, orgs.ErrUserNotFound) {
		t.Fatalf("add unknown user: err = %v, want ErrUserNotFound", err)
	}

	// Members lists everyone (owner + bob + carol).
	members, err := svc.Members(ctx, org.ID)
	if err != nil {
		t.Fatalf("Members: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("member count = %d, want 3", len(members))
	}
}

func TestGetMembershipNotMember(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	usersRepo := users.NewRepo(pool)
	orgsRepo := orgs.NewRepo(pool)
	svc := orgs.NewService(orgsRepo, pool)

	ownerID := newUser(t, usersRepo, "owner@example.com")
	strangerID := newUser(t, usersRepo, "stranger@example.com")
	org, err := svc.CreateOrg(ctx, ownerID, "Acme")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	if _, err := orgsRepo.GetMembership(ctx, strangerID, org.ID); !errors.Is(err, orgs.ErrNotMember) {
		t.Fatalf("stranger membership err = %v, want ErrNotMember", err)
	}

	// Stranger sees no orgs.
	list, err := svc.ListForUser(ctx, strangerID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("stranger ListForUser = %+v, want empty", list)
	}
}

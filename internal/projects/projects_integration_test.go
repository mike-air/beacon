// Integration tests for the projects repository against a real Postgres.
//
// Course mapping: Chapter 39 — integration tests. ENV-GATED: skips unless
// TEST_DATABASE_URL is set (testsupport instead of testcontainers). No build tag.
package projects_test

import (
	"context"
	"errors"
	"testing"

	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/testsupport"
	"beacon/internal/users"
)

func TestProjectCRUD(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	ownerID := mustUser(t, users.NewRepo(pool), "owner@example.com")
	org, err := orgs.NewService(orgs.NewRepo(pool), pool).CreateOrg(ctx, ownerID, "Acme")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	svc := projects.NewService(projects.NewRepo(pool))

	// Create.
	p, err := svc.Create(ctx, org.ID, "Website")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" || p.OrgID != org.ID || p.Name != "Website" {
		t.Fatalf("Create returned %+v", p)
	}

	// Get.
	got, err := svc.Get(ctx, org.ID, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("Get ID = %q, want %q", got.ID, p.ID)
	}

	// Create a second project, then list with pagination.
	if _, err := svc.Create(ctx, org.ID, "Mobile"); err != nil {
		t.Fatalf("Create second: %v", err)
	}
	all, err := svc.List(ctx, org.ID, 20, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List returned %d, want 2", len(all))
	}
	page, err := svc.List(ctx, org.ID, 1, 0)
	if err != nil {
		t.Fatalf("List(limit=1): %v", err)
	}
	if len(page) != 1 {
		t.Errorf("List(limit=1) returned %d, want 1", len(page))
	}

	// Update.
	updated, err := svc.Update(ctx, org.ID, p.ID, "Website v2")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Website v2" {
		t.Errorf("Update name = %q, want Website v2", updated.Name)
	}

	// Delete, then confirm it's gone.
	if err := svc.Delete(ctx, org.ID, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, org.ID, p.ID); !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrNotFound", err)
	}
	// Deleting again → ErrNotFound.
	if err := svc.Delete(ctx, org.ID, p.ID); !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("double delete: err = %v, want ErrNotFound", err)
	}
}

// mustUser creates a user and returns its ID.
func mustUser(t *testing.T, repo *users.Repo, email string) string {
	t.Helper()
	u, err := repo.Create(context.Background(), email, "Test", "hash")
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u.ID
}

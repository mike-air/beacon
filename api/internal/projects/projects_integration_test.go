// Integration tests for the projects repository against a real Postgres.
//
// Course mapping: Chapter 39 — integration tests. ENV-GATED: skips unless
// TEST_DATABASE_URL is set (testsupport instead of testcontainers). No build tag.
package projects_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/db"
	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/tasks"
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

	svc := projects.NewService(projects.NewRepo(pool), pool)

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
	if err := svc.Delete(ctx, org.ID, ownerID, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, org.ID, p.ID); !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrNotFound", err)
	}
	// Deleting again → ErrNotFound. The row is still there, but deleted_at is
	// already set, and softDelete's WHERE clause only matches a live row.
	if err := svc.Delete(ctx, org.ID, ownerID, p.ID); !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("double delete: err = %v, want ErrNotFound", err)
	}
}

// TestProjectDeleteIsSoftCascadingAndAudited is Chapter 10's actual point,
// and it asserts all three halves of it:
//
//   - the row SURVIVES, which is what makes the delete soft. Every service
//     read filters deleted_at IS NULL, so proving this needs a query that
//     does not — that is what the raw SELECT below is for.
//   - the delete CASCADES to tasks. The ON DELETE CASCADE foreign key that
//     used to do this for free does nothing now, because from the database's
//     point of view the project row never went anywhere.
//   - it is AUDITED, in the same transaction, naming who did it.
func TestProjectDeleteIsSoftCascadingAndAudited(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	ownerID := mustUser(t, users.NewRepo(pool), "owner@example.com")
	org, err := orgs.NewService(orgs.NewRepo(pool), pool).CreateOrg(ctx, ownerID, "Acme")
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}

	projSvc := projects.NewService(projects.NewRepo(pool), pool)
	taskSvc := tasks.NewService(tasks.NewRepo(pool), pool)

	p, err := projSvc.Create(ctx, org.ID, "Website")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}
	tk, err := taskSvc.Create(ctx, org.ID, p.ID, "Ship it", "todo", 1000)
	if err != nil {
		t.Fatalf("Create task: %v", err)
	}

	if err := projSvc.Delete(ctx, org.ID, ownerID, p.ID); err != nil {
		t.Fatalf("Delete project: %v", err)
	}

	// Soft: the row is still physically present, with deleted_at set.
	pid, err := uuid.Parse(p.ID)
	if err != nil {
		t.Fatalf("parse project id: %v", err)
	}
	var deletedAt *string
	if err := pool.QueryRow(ctx,
		"SELECT deleted_at::text FROM projects WHERE id = $1", pid,
	).Scan(&deletedAt); err != nil {
		t.Fatalf("the project row is gone — a soft delete must not remove it: %v", err)
	}
	if deletedAt == nil {
		t.Error("project row survived but deleted_at is NULL; the delete did not happen")
	}

	// Cascading: the task went with it.
	if _, err := taskSvc.Get(ctx, org.ID, tk.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Fatalf("task after project delete: err = %v, want ErrNotFound", err)
	}

	// Audited: exactly one entry, naming the right actor, action and resource.
	oid, err := uuid.Parse(org.ID)
	if err != nil {
		t.Fatalf("parse org id: %v", err)
	}
	entries, err := db.New(pool).ListAuditLog(ctx, db.ListAuditLogParams{
		OrgID: oid, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit log has %d entries, want 1: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.ActorID.String() != ownerID {
		t.Errorf("audit actor = %s, want %s", e.ActorID, ownerID)
	}
	if e.Action != "project.deleted" {
		t.Errorf("audit action = %q, want project.deleted", e.Action)
	}
	if e.ResourceType != "project" {
		t.Errorf("audit resource_type = %q, want project", e.ResourceType)
	}
	if e.ResourceID.String() != p.ID {
		t.Errorf("audit resource_id = %s, want %s", e.ResourceID, p.ID)
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

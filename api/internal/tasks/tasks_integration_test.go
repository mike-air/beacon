// Integration tests for tasks + the cross-tenant security guarantee, against a
// real Postgres.
//
// Course mapping: Chapter 39 — integration tests. ENV-GATED: skips unless
// TEST_DATABASE_URL is set (testsupport instead of testcontainers). No build tag.
//
// The showcase test is TestCrossTenantIsolation: two orgs, two users, and the
// assertion that user B's repo calls cannot read, update, or delete org A's
// project or task. Org scoping holds at the DB layer (every query carries
// org_id), so the boundary is a property of the SQL, not of any handler.
package tasks_test

import (
	"context"
	"errors"
	"testing"

	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/tasks"
	"beacon/internal/testsupport"
	"beacon/internal/users"
)

// tenant bundles everything one org's tests need: its ID and a seeded project.
type tenant struct {
	orgID     string
	projectID string
	// ownerID is the user who created the org. Chapter 10's deletes take an
	// actor id for the audit entry, so a tenant has to remember who it is.
	ownerID string
}

func mkUser(t *testing.T, repo *users.Repo, email string) string {
	t.Helper()
	u, err := repo.Create(context.Background(), email, "Test", "hash")
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u.ID
}

func mkTenant(t *testing.T, orgSvc *orgs.Service, projRepo *projects.Repo, usersRepo *users.Repo, email, orgName string) tenant {
	t.Helper()
	ctx := context.Background()
	uid := mkUser(t, usersRepo, email)
	org, err := orgSvc.CreateOrg(ctx, uid, orgName)
	if err != nil {
		t.Fatalf("CreateOrg(%s): %v", orgName, err)
	}
	p, err := projRepo.Create(ctx, org.ID, orgName+" project")
	if err != nil {
		t.Fatalf("Create project for %s: %v", orgName, err)
	}
	return tenant{orgID: org.ID, projectID: p.ID, ownerID: uid}
}

func TestTaskCRUD(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	orgSvc := orgs.NewService(orgs.NewRepo(pool), pool)
	projRepo := projects.NewRepo(pool)
	usersRepo := users.NewRepo(pool)
	svc := tasks.NewService(tasks.NewRepo(pool), pool)

	a := mkTenant(t, orgSvc, projRepo, usersRepo, "owner@a.test", "Alpha")

	// Create with default status.
	tk, err := svc.Create(ctx, a.orgID, a.projectID, "Ship it", "", 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tk.Status != tasks.StatusTodo {
		t.Errorf("default status = %q, want todo", tk.Status)
	}

	// Invalid status is rejected.
	if _, err := svc.Create(ctx, a.orgID, a.projectID, "Bad", "nope", 0); !errors.Is(err, tasks.ErrInvalidStatus) {
		t.Fatalf("invalid status: err = %v, want ErrInvalidStatus", err)
	}

	// List + status filter.
	if _, err := svc.Create(ctx, a.orgID, a.projectID, "Second", tasks.StatusInProgress, 2); err != nil {
		t.Fatalf("Create second: %v", err)
	}
	all, err := svc.List(ctx, a.orgID, a.projectID, "", 20, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List = %d, want 2", len(all))
	}
	inProg, err := svc.List(ctx, a.orgID, a.projectID, tasks.StatusInProgress, 20, 0)
	if err != nil {
		t.Fatalf("List(filter): %v", err)
	}
	if len(inProg) != 1 {
		t.Errorf("List(in_progress) = %d, want 1", len(inProg))
	}

	// Update.
	upd, err := svc.Update(ctx, a.orgID, tk.ID, "Ship it now", tasks.StatusDone, 3)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if upd.Status != tasks.StatusDone || upd.Title != "Ship it now" {
		t.Errorf("Update = %+v", upd)
	}

	// Delete + confirm gone.
	if err := svc.Delete(ctx, a.orgID, a.ownerID, tk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, a.orgID, tk.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Fatalf("Get after delete: err = %v, want ErrNotFound", err)
	}
}

// TestCrossTenantIsolation is the showcase: user B (org B) must not be able to
// read, update, or delete org A's project or task. The scope lives in every
// WHERE clause, so a wrong-org id is indistinguishable from a missing row —
// ErrNotFound, never another tenant's data.
func TestCrossTenantIsolation(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	orgSvc := orgs.NewService(orgs.NewRepo(pool), pool)
	projRepo := projects.NewRepo(pool)
	usersRepo := users.NewRepo(pool)
	projSvc := projects.NewService(projRepo, pool)
	taskSvc := tasks.NewService(tasks.NewRepo(pool), pool)

	a := mkTenant(t, orgSvc, projRepo, usersRepo, "owner@a.test", "Alpha")
	b := mkTenant(t, orgSvc, projRepo, usersRepo, "owner@b.test", "Bravo")

	// Org A owns a task on its project.
	aTask, err := taskSvc.Create(ctx, a.orgID, a.projectID, "A secret", tasks.StatusTodo, 1)
	if err != nil {
		t.Fatalf("create A task: %v", err)
	}

	// --- Projects: B cannot reach A's project ---
	if _, err := projSvc.Get(ctx, b.orgID, a.projectID); !errors.Is(err, projects.ErrNotFound) {
		t.Errorf("B reading A's project: err = %v, want ErrNotFound", err)
	}
	if _, err := projSvc.Update(ctx, b.orgID, a.projectID, "Hijacked"); !errors.Is(err, projects.ErrNotFound) {
		t.Errorf("B updating A's project: err = %v, want ErrNotFound", err)
	}
	if err := projSvc.Delete(ctx, b.orgID, b.ownerID, a.projectID); !errors.Is(err, projects.ErrNotFound) {
		t.Errorf("B deleting A's project: err = %v, want ErrNotFound", err)
	}

	// --- Tasks: B cannot reach A's task ---
	if _, err := taskSvc.Get(ctx, b.orgID, aTask.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("B reading A's task: err = %v, want ErrNotFound", err)
	}
	if _, err := taskSvc.Update(ctx, b.orgID, aTask.ID, "Hijacked", tasks.StatusDone, 9); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("B updating A's task: err = %v, want ErrNotFound", err)
	}
	if err := taskSvc.Delete(ctx, b.orgID, b.ownerID, aTask.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("B deleting A's task: err = %v, want ErrNotFound", err)
	}

	// --- Create: B cannot write INTO A's project ---
	//
	// This case was missing until bulk import went in, and it was the one the
	// boundary actually failed. Read, update and delete were all covered
	// above; create was not, and CreateTask's old VALUES form checked that
	// org_id and project_id each existed without ever checking they belonged
	// together. B could therefore create a task carrying B's org_id and A's
	// project_id. No data leaked — every read here is org-scoped, so neither
	// side saw the other's rows — but one tenant could write rows referencing
	// another's project, and could tell a real project id from a fake one by
	// whether the call succeeded.
	if _, err := taskSvc.Create(ctx, b.orgID, a.projectID, "Planted", tasks.StatusTodo, 1); !errors.Is(err, tasks.ErrProjectNotFound) {
		t.Errorf("B creating a task in A's project: err = %v, want ErrProjectNotFound", err)
	}
	// The bulk path must not be a way around the same boundary.
	planted := []tasks.ImportRow{{Title: "Planted in bulk", Status: tasks.StatusTodo}}
	if _, err := taskSvc.Import(ctx, b.orgID, a.projectID, b.ownerID, planted); !errors.Is(err, tasks.ErrProjectNotFound) {
		t.Errorf("B importing into A's project: err = %v, want ErrProjectNotFound", err)
	}

	// --- The boundary held: A's data is untouched ---
	stillThere, err := taskSvc.Get(ctx, a.orgID, aTask.ID)
	if err != nil {
		t.Fatalf("A's task should survive B's attempts: %v", err)
	}
	if stillThere.Title != "A secret" || stillThere.Status != tasks.StatusTodo {
		t.Errorf("A's task was mutated across the tenant boundary: %+v", stillThere)
	}

	// B listing its own (empty) project returns nothing of A's.
	bTasks, err := taskSvc.List(ctx, b.orgID, b.projectID, "", 20, 0)
	if err != nil {
		t.Fatalf("B list own tasks: %v", err)
	}
	if len(bTasks) != 0 {
		t.Errorf("B's project has %d tasks, want 0 (no leakage from A)", len(bTasks))
	}
}

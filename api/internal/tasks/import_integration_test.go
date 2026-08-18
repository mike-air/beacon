// Integration tests for bulk import, against a real Postgres.
//
// ENV-GATED like every other integration test here: skips unless
// TEST_DATABASE_URL is set. The parsing half is unit-tested in import_test.go;
// what needs a database is the promise that gives this feature its shape —
// that a failed import writes NOTHING. A mock cannot prove a rollback.
package tasks_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/tasks"
	"beacon/internal/testsupport"
	"beacon/internal/users"
)

func importFixture(t *testing.T) (*tasks.Service, tenant, context.Context) {
	t.Helper()
	pool := testsupport.NewTestPool(t)
	orgSvc := orgs.NewService(orgs.NewRepo(pool), pool)
	svc := tasks.NewService(tasks.NewRepo(pool), pool)
	tn := mkTenant(t, orgSvc, projects.NewRepo(pool), users.NewRepo(pool),
		"importer@test.test", "Importers")
	return svc, tn, context.Background()
}

func TestImportCreatesEveryRow(t *testing.T) {
	svc, tn, ctx := importFixture(t)

	rows, rowErrs, err := tasks.ParseCSV(strings.NewReader(
		"title,status\nFirst,todo\nSecond,in_progress\nThird,done\n",
	))
	if err != nil || len(rowErrs) > 0 {
		t.Fatalf("ParseCSV: %v / %v", err, rowErrs)
	}

	created, err := svc.Import(ctx, tn.orgID, tn.projectID, tn.ownerID, rows)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("created = %d, want 3", len(created))
	}

	got, err := svc.List(ctx, tn.orgID, tn.projectID, "", 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("listed = %d, want 3", len(got))
	}

	// List orders by position, so the CSV's order must survive the round trip.
	// An import that scrambles the user's ordering is a bug they notice
	// immediately and cannot fix except by hand.
	for i, want := range []string{"First", "Second", "Third"} {
		if got[i].Title != want {
			t.Errorf("task %d = %q, want %q (CSV order should be preserved)", i, got[i].Title, want)
		}
	}
	if got[1].Status != tasks.StatusInProgress {
		t.Errorf("status = %q, want in_progress", got[1].Status)
	}
}

// THE test for this feature. A row that passes ParseCSV but is rejected by the
// database must leave the project exactly as it was — not partly imported.
func TestImportIsAllOrNothing(t *testing.T) {
	svc, tn, ctx := importFixture(t)

	// Seed one real task so the assertion is "unchanged", not merely "empty".
	if _, err := svc.Create(ctx, tn.orgID, tn.projectID, "Existing", tasks.StatusTodo, 500); err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// A status the parser would have caught, injected directly to simulate the
	// case the parser cannot: a row that only the database rejects. Import
	// validates statuses itself, so this exercises the same guard the DB's
	// CHECK constraint would, without depending on which one fires first.
	bad := []tasks.ImportRow{
		{Title: "Good one", Status: tasks.StatusTodo},
		{Title: "Good two", Status: tasks.StatusDone},
		{Title: "Bad", Status: "not_a_status"},
	}

	if _, err := svc.Import(ctx, tn.orgID, tn.projectID, tn.ownerID, bad); err == nil {
		t.Fatal("Import should fail when a row is invalid")
	}

	got, err := svc.List(ctx, tn.orgID, tn.projectID, "", 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Existing" {
		t.Fatalf("after a failed import the project holds %d tasks (%+v); "+
			"want only the pre-existing one — the transaction did not roll back", len(got), got)
	}
}

// Imported tasks go after whatever is already on the board, in file order.
// Appending rather than interleaving is what makes an import feel like
// "add these", which is what the user asked for.
func TestImportAppendsAfterExistingTasks(t *testing.T) {
	svc, tn, ctx := importFixture(t)

	first, err := svc.Create(ctx, tn.orgID, tn.projectID, "Already here", tasks.StatusTodo, 5000)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	rows, _, err := tasks.ParseCSV(strings.NewReader("title\nImported A\nImported B\n"))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	created, err := svc.Import(ctx, tn.orgID, tn.projectID, tn.ownerID, rows)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	for i, c := range created {
		if c.Position <= first.Position {
			t.Errorf("imported task %d position %v is not after the existing %v",
				i, c.Position, first.Position)
		}
	}
	if created[0].Position >= created[1].Position {
		t.Errorf("imported positions %v, %v are not in file order",
			created[0].Position, created[1].Position)
	}
}

// Tenant scoping is not weakened by the bulk path: an import names an org, and
// a project in another org must not accept rows just because the caller has a
// valid session somewhere.
func TestImportRespectsOrgScope(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	orgSvc := orgs.NewService(orgs.NewRepo(pool), pool)
	projRepo := projects.NewRepo(pool)
	usersRepo := users.NewRepo(pool)
	svc := tasks.NewService(tasks.NewRepo(pool), pool)

	a := mkTenant(t, orgSvc, projRepo, usersRepo, "a@scope.test", "Alpha")
	b := mkTenant(t, orgSvc, projRepo, usersRepo, "b@scope.test", "Beta")

	rows := []tasks.ImportRow{{Title: "Should not land", Status: tasks.StatusTodo}}

	// Org A's id with org B's project. Before CreateTask became an
	// INSERT ... SELECT this SUCCEEDED — the two foreign keys were each valid
	// on their own and nothing checked they belonged together. It is now
	// ErrProjectNotFound, the same answer a project that truly does not exist
	// gets, so the endpoint cannot be used to probe for real project ids.
	if _, err := svc.Import(ctx, a.orgID, b.projectID, a.ownerID, rows); !errors.Is(err, tasks.ErrProjectNotFound) {
		t.Fatalf("cross-org Import: err = %v, want ErrProjectNotFound", err)
	}

	got, err := svc.List(ctx, b.orgID, b.projectID, "", 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("org B's project holds %d tasks; a cross-org import wrote into it", len(got))
	}
}

func TestImportRejectsOversizeBatch(t *testing.T) {
	svc, tn, ctx := importFixture(t)

	rows := make([]tasks.ImportRow, tasks.MaxImportRows+1)
	for i := range rows {
		rows[i] = tasks.ImportRow{Title: fmt.Sprintf("task %d", i), Status: tasks.StatusTodo}
	}
	if _, err := svc.Import(ctx, tn.orgID, tn.projectID, tn.ownerID, rows); err == nil {
		t.Fatal("Import should reject a batch over MaxImportRows")
	}
}

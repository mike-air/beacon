// Integration tests for editing and deleting comments, against a real Postgres.
//
// ENV-GATED like the rest: skips unless TEST_DATABASE_URL is set.
//
// The permission matrix IS the feature, so it is tested as a matrix rather
// than as a happy path with a couple of negatives bolted on. Two rules, and
// they differ on purpose:
//
//	edit   — the author, and nobody else. Not admins. An admin rewriting
//	         somebody's words under their name has no legitimate use.
//	delete — the author, OR an org admin/owner. Moderation has to be possible
//	         by someone other than whoever posted the thing.
package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/orgs"
	"beacon/internal/projects"
	"beacon/internal/tasks"
	"beacon/internal/testsupport"
	"beacon/internal/users"
)

// commentFixture builds one org with a task and three people in it: the
// comment's author, another plain member, and an admin.
type commentFixture struct {
	// pool is the fixture's OWN database. Tests that query tables directly
	// must use THIS pool: NewTestPool creates a fresh database per call, so a
	// test that calls it again gets a different, empty one — which is exactly
	// how the search-index assertion below first failed, reading a table in a
	// database the comment was never written to.
	pool      *pgxpool.Pool
	svc       *tasks.Service
	orgID     string
	taskID    string
	commentID string
	authorID  string
	memberID  string
	adminID   string
	ctx       context.Context
}

func newCommentFixture(t *testing.T) commentFixture {
	t.Helper()
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	orgSvc := orgs.NewService(orgs.NewRepo(pool), pool)
	projRepo := projects.NewRepo(pool)
	usersRepo := users.NewRepo(pool)
	svc := tasks.NewService(tasks.NewRepo(pool), pool)

	owner := mkTenant(t, orgSvc, projRepo, usersRepo, "author@c.test", "Commenters")

	// AddMember looks the user up by email, so each has to exist first. The
	// actor role is the owner's, because only owners and admins may add.
	memberID := mkUser(t, usersRepo, "member@c.test")
	if _, err := orgSvc.AddMember(ctx, owner.orgID, orgs.RoleOwner, "member@c.test", orgs.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}
	adminID := mkUser(t, usersRepo, "admin@c.test")
	if _, err := orgSvc.AddMember(ctx, owner.orgID, orgs.RoleOwner, "admin@c.test", orgs.RoleAdmin); err != nil {
		t.Fatalf("add admin: %v", err)
	}

	task, err := svc.Create(ctx, owner.orgID, owner.projectID, "Discuss me", tasks.StatusTodo, 1)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	c, err := svc.AddComment(ctx, owner.orgID, task.ID, owner.ownerID, "the original text")
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}

	return commentFixture{
		pool: pool,
		svc:  svc, orgID: owner.orgID, taskID: task.ID, commentID: c.ID,
		authorID: owner.ownerID, memberID: memberID, adminID: adminID, ctx: ctx,
	}
}

func TestCommentAuthorCanEditOwn(t *testing.T) {
	f := newCommentFixture(t)

	updated, err := f.svc.UpdateComment(f.ctx, f.orgID, f.commentID, f.authorID, "the corrected text")
	if err != nil {
		t.Fatalf("UpdateComment: %v", err)
	}
	if updated.Body != "the corrected text" {
		t.Errorf("body = %q, want the new text", updated.Body)
	}
	if !updated.Edited {
		t.Error("Edited = false after an edit; a reader has no way to tell the text changed")
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Error("updated_at is not after created_at")
	}

	// And it is the stored row that changed, not just the returned struct.
	list, err := f.svc.Comments(f.ctx, f.orgID, f.taskID)
	if err != nil {
		t.Fatalf("Comments: %v", err)
	}
	if len(list) != 1 || list[0].Body != "the corrected text" {
		t.Fatalf("list = %+v, want the edited body", list)
	}
	if !list[0].Edited {
		t.Error("Edited = false when re-read from the database")
	}
}

// A freshly posted comment must not claim to have been edited — created_at and
// updated_at are separate column defaults, so this is a real risk, not a
// theoretical one.
func TestFreshCommentIsNotMarkedEdited(t *testing.T) {
	f := newCommentFixture(t)

	list, err := f.svc.Comments(f.ctx, f.orgID, f.taskID)
	if err != nil {
		t.Fatalf("Comments: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("comments = %d, want 1", len(list))
	}
	if list[0].Edited {
		t.Error("a comment that was never edited is marked Edited")
	}
}

func TestCommentEditIsAuthorOnly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor func(commentFixture) string
	}{
		// An admin is included deliberately: elevated rights must NOT extend
		// to putting words in somebody else's mouth.
		{"another member", func(f commentFixture) string { return f.memberID }},
		{"an org admin", func(f commentFixture) string { return f.adminID }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommentFixture(t)
			_, err := f.svc.UpdateComment(f.ctx, f.orgID, f.commentID, tc.actor(f), "hijacked")
			if !errors.Is(err, tasks.ErrNotCommentAuthor) {
				t.Fatalf("err = %v, want ErrNotCommentAuthor", err)
			}

			list, _ := f.svc.Comments(f.ctx, f.orgID, f.taskID)
			if len(list) != 1 || list[0].Body != "the original text" {
				t.Errorf("comment body is now %+v; the rejected edit was written anyway", list)
			}
		})
	}
}

func TestCommentDeletePermissions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		actor   func(commentFixture) string
		role    string
		allowed bool
	}{
		{"the author", func(f commentFixture) string { return f.authorID }, orgs.RoleOwner, true},
		{"an org admin", func(f commentFixture) string { return f.adminID }, orgs.RoleAdmin, true},
		{"another member", func(f commentFixture) string { return f.memberID }, orgs.RoleMember, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommentFixture(t)
			err := f.svc.DeleteComment(f.ctx, f.orgID, f.commentID, tc.actor(f), tc.role)

			if tc.allowed {
				if err != nil {
					t.Fatalf("DeleteComment: %v", err)
				}
				list, _ := f.svc.Comments(f.ctx, f.orgID, f.taskID)
				if len(list) != 0 {
					t.Errorf("comments = %d after delete, want 0", len(list))
				}
				return
			}

			if !errors.Is(err, tasks.ErrCannotDeleteComment) {
				t.Fatalf("err = %v, want ErrCannotDeleteComment", err)
			}
			list, _ := f.svc.Comments(f.ctx, f.orgID, f.taskID)
			if len(list) != 1 {
				t.Errorf("comments = %d, want the comment still there", len(list))
			}
		})
	}
}

// A deleted comment is gone from every read path, and deleting it twice does
// not succeed the second time — otherwise a double-click writes two audit
// entries for one removal.
func TestDeletedCommentIsGoneAndNotDeletableTwice(t *testing.T) {
	f := newCommentFixture(t)

	if err := f.svc.DeleteComment(f.ctx, f.orgID, f.commentID, f.authorID, orgs.RoleOwner); err != nil {
		t.Fatalf("first delete: %v", err)
	}

	err := f.svc.DeleteComment(f.ctx, f.orgID, f.commentID, f.authorID, orgs.RoleOwner)
	if !errors.Is(err, tasks.ErrCommentNotFound) {
		t.Errorf("second delete: err = %v, want ErrCommentNotFound", err)
	}

	if _, err := f.svc.UpdateComment(f.ctx, f.orgID, f.commentID, f.authorID, "resurrect"); !errors.Is(err, tasks.ErrCommentNotFound) {
		t.Errorf("editing a deleted comment: err = %v, want ErrCommentNotFound", err)
	}
}

// The tenant boundary again, on the two new verbs. Org B must not be able to
// touch org A's comment even holding its id.
func TestCommentEditDeleteRespectOrgScope(t *testing.T) {
	pool := testsupport.NewTestPool(t)
	ctx := context.Background()

	orgSvc := orgs.NewService(orgs.NewRepo(pool), pool)
	projRepo := projects.NewRepo(pool)
	usersRepo := users.NewRepo(pool)
	svc := tasks.NewService(tasks.NewRepo(pool), pool)

	a := mkTenant(t, orgSvc, projRepo, usersRepo, "a@cs.test", "Alpha")
	b := mkTenant(t, orgSvc, projRepo, usersRepo, "b@cs.test", "Bravo")

	task, err := svc.Create(ctx, a.orgID, a.projectID, "A's task", tasks.StatusTodo, 1)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	c, err := svc.AddComment(ctx, a.orgID, task.ID, a.ownerID, "A's words")
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}

	// B is the OWNER of its own org, so this proves the org scope holds rather
	// than the role check happening to reject a low-privilege caller.
	if _, err := svc.UpdateComment(ctx, b.orgID, c.ID, b.ownerID, "hijacked"); !errors.Is(err, tasks.ErrCommentNotFound) {
		t.Errorf("B editing A's comment: err = %v, want ErrCommentNotFound", err)
	}
	if err := svc.DeleteComment(ctx, b.orgID, c.ID, b.ownerID, orgs.RoleOwner); !errors.Is(err, tasks.ErrCommentNotFound) {
		t.Errorf("B deleting A's comment: err = %v, want ErrCommentNotFound", err)
	}

	list, err := svc.Comments(ctx, a.orgID, task.ID)
	if err != nil {
		t.Fatalf("Comments: %v", err)
	}
	if len(list) != 1 || list[0].Body != "A's words" {
		t.Fatalf("A's comment is %+v; B reached across the boundary", list)
	}
}

// Deleting a comment must also remove it from search. This is the regression
// test for the gap 0011 closed: the search triggers fired on DELETE, soft
// delete is an UPDATE, so a deleted row stayed indexed and findable.
func TestDeletedCommentLeavesTheSearchIndex(t *testing.T) {
	f := newCommentFixture(t)

	// A word unlikely to appear anywhere else, so the count is unambiguous.
	needle := "zqxjkv"
	c, err := f.svc.AddComment(f.ctx, f.orgID, f.taskID, f.authorID, "contains "+needle+" exactly once")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	indexed := func() int {
		t.Helper()
		var n int
		if err := f.pool.QueryRow(f.ctx,
			`SELECT count(*) FROM search_index WHERE entity_kind = 'comment' AND entity_id = $1`,
			c.ID,
		).Scan(&n); err != nil {
			t.Fatalf("count search_index: %v", err)
		}
		return n
	}

	if got := indexed(); got != 1 {
		t.Fatalf("indexed rows after posting = %d, want 1 — the fixture cannot test a removal that never happened", got)
	}

	if err := f.svc.DeleteComment(f.ctx, f.orgID, c.ID, f.authorID, orgs.RoleOwner); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}

	if got := indexed(); got != 0 {
		t.Errorf("indexed rows after delete = %d, want 0 — a deleted comment is still findable by search", got)
	}
}

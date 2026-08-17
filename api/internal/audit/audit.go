// Package audit is Chapter 10's second half — a record of who deleted what,
// and when. It owns exactly one table and two operations: write an entry,
// list an org's entries.
//
// The write is deliberately NOT best-effort the way notify.go's webhook and
// SSE fan-out is. Those are told about something that already happened and
// happen to fail quietly if the telling doesn't land — the user's write
// already succeeded, and a dropped webhook is a retry away from correct. An
// audit entry is different: its entire job is to be trustworthy evidence
// that a delete happened, which is worthless if it can silently not exist.
// So Write takes a *db.Queries that the caller has already bound to a
// transaction (Queries.WithTx), and is called in the SAME transaction as the
// soft delete it describes — see internal/projects, internal/tasks and
// internal/webhooks's Delete methods. Either both land or neither does.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"beacon/internal/db"
)

// Entry is one row: an actor did something to a resource, inside one org.
type Entry struct {
	OrgID        string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
}

// Write appends one entry. q must already be bound to the transaction the
// entry needs to commit or roll back with — see the package doc for why.
func Write(ctx context.Context, q *db.Queries, e Entry) error {
	orgID, err := uuid.Parse(e.OrgID)
	if err != nil {
		return fmt.Errorf("audit.Write: parse org id: %w", err)
	}
	actorID, err := uuid.Parse(e.ActorID)
	if err != nil {
		return fmt.Errorf("audit.Write: parse actor id: %w", err)
	}
	resourceID, err := uuid.Parse(e.ResourceID)
	if err != nil {
		return fmt.Errorf("audit.Write: parse resource id: %w", err)
	}
	if err := q.InsertAuditEntry(ctx, db.InsertAuditEntryParams{
		OrgID:        orgID,
		ActorID:      actorID,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   resourceID,
	}); err != nil {
		return fmt.Errorf("audit.Write: %w", err)
	}
	return nil
}

// Logged is one entry as read back — every id crosses the package boundary
// as a string, the same convention every domain package here follows.
type Logged struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func toLogged(a db.AuditLog) Logged {
	return Logged{
		ID:           a.ID.String(),
		OrgID:        a.OrgID.String(),
		ActorID:      a.ActorID.String(),
		Action:       a.Action,
		ResourceType: a.ResourceType,
		ResourceID:   a.ResourceID.String(),
		CreatedAt:    a.CreatedAt,
	}
}

// List returns an org's audit entries, newest first.
func List(ctx context.Context, q *db.Queries, orgID string, limit, offset int) ([]Logged, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("audit.List: parse org id: %w", err)
	}
	rows, err := q.ListAuditLog(ctx, db.ListAuditLogParams{
		OrgID:  oid,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("audit.List: %w", err)
	}
	out := make([]Logged, len(rows))
	for i, r := range rows {
		out[i] = toLogged(r)
	}
	return out, nil
}

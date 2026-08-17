// Chapter 30's write path. A handler never touches Meilisearch. Instead the
// service transaction that writes the row also enqueues a reindex job, in the
// same commit, so either both happen or neither does. A worker picks it up a
// moment later, reads the canonical row out of Postgres, and pushes it to
// Meili. Typical lag: under a second.
//
// [verbatim ch30] with one adaptation: the chapter's job types are River
// structs (Kind() string, InsertOpts()). This repo runs its own Postgres-backed
// queue (see internal/jobs and its DEVIATION note from Chapter 26), so the
// same two jobs are expressed as this queue's kind + JSON payload + handler.

package search

import (
	"context"
	"encoding/json"
	"fmt"
)

// Job kinds, registered on the worker in the wiring step.
const (
	KindReindex    = "search_reindex"
	KindReindexAll = "search_reindex_all"
)

// ReindexArgs identifies one entity to push into Meili.
type ReindexArgs struct {
	Kind     string `json:"kind"` // "task", "project", "comment"
	EntityID string `json:"entity_id"`
}

// ReindexAllArgs streams everything (or one org) from Postgres into Meili.
type ReindexAllArgs struct {
	OrgID string `json:"org_id,omitempty"` // empty = all orgs
}

// ReindexHandler returns the handler for a single-entity reindex.
func ReindexHandler(svc *Service) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var args ReindexArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return fmt.Errorf("search.reindex: bad payload: %w", err)
		}
		return svc.IndexOne(ctx, args.Kind, args.EntityID)
	}
}

// ReindexAllHandler returns the handler for a full rebuild, in batches of 500.
func ReindexAllHandler(svc *Service) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var args ReindexAllArgs
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &args); err != nil {
				return fmt.Errorf("search.reindex_all: bad payload: %w", err)
			}
		}
		n, err := svc.ReindexInBatches(ctx, args.OrgID, 500)
		if err != nil {
			return err
		}
		svc.log.Info("search reindex complete", "documents", n, "org_id", args.OrgID)
		return nil
	}
}

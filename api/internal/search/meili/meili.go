// Chapter 30 — the Meilisearch client.
//
// Everything here is configuration and one-way writes. The index settings are
// applied once at startup and are idempotent, so a redeploy is safe. There is
// no code path from a handler into this package: documents arrive only from the
// reindex worker, which read them from Postgres first.
//
// [verbatim ch30] for New and EnsureIndex; Upsert, DeleteOne, Search and the
// Request/Response mapping are the glue the chapter's service calls imply.
package meili

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"beacon/internal/search"
)

type Client struct {
	c       meilisearch.ServiceManager
	idx     string
	timeout time.Duration
}

// New constructs a Meili client. The index is the document collection
// inside Meilisearch we'll read and write to. For Beacon we use a single
// "beacon" index with org_id as a filterable attribute.
func New(url, apiKey, index string) *Client {
	c := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
	return &Client{c: c, idx: index, timeout: 3 * time.Second}
}

// EnsureIndex creates the index and configures its settings if not already
// done. Idempotent. Called once at startup.
func (c *Client) EnsureIndex(ctx context.Context) error {
	settings := &meilisearch.Settings{
		// Order is weight: a hit in the title outranks a hit in the body.
		SearchableAttributes: []string{"title", "body"},
		// Meili will refuse to filter on an attribute that isn't declared here,
		// which is a good failure — it turns "we forgot the tenant filter" into
		// an error instead of a leak.
		FilterableAttributes: []string{"org_id", "kind"},
		SortableAttributes:   []string{"updated_at"},
		RankingRules: []string{
			"words", "typo", "proximity", "attribute", "sort", "exactness",
		},
		TypoTolerance: &meilisearch.TypoTolerance{
			Enabled: true,
			MinWordSizeForTypos: meilisearch.MinWordSizeForTypos{
				OneTypo: 4, TwoTypos: 8,
			},
		},
	}

	task, err := c.c.Index(c.idx).UpdateSettingsWithContext(ctx, settings)
	if err != nil {
		return fmt.Errorf("meili settings: %w", err)
	}
	// Meili applies settings asynchronously. Wait for it, or the first search
	// after boot runs against an index with no filterable attributes and fails
	// in a way that looks like a bug in our code.
	return c.awaitTask(ctx, task.TaskUID, "settings")
}

// Upsert adds or replaces documents. Meili keys on the "id" field, which we set
// to "kind:uuid", so the same entity always lands in the same document.
func (c *Client) Upsert(ctx context.Context, docs []search.Document) error {
	if len(docs) == 0 {
		return nil
	}
	// The primary key is named explicitly. Meili tries to infer it from any
	// field ending in "id", and our documents have both `id` and `entity_id`,
	// so inference fails and every add is rejected.
	primaryKey := "id"
	task, err := c.c.Index(c.idx).AddDocumentsWithContext(ctx, docs, &meilisearch.DocumentOptions{PrimaryKey: &primaryKey})
	if err != nil {
		return fmt.Errorf("meili upsert: %w", err)
	}
	return c.awaitTask(ctx, task.TaskUID, "upsert")
}

// awaitTask waits for an async Meili task AND checks that it succeeded.
//
// WaitForTask returns a nil error when the task FAILED — the error is a field
// on the task, not a Go error. Without this check the worker marks the job done
// while Meilisearch has quietly indexed nothing, and the only symptom is an
// empty search page. Fail loudly instead: the job retries, and a Meili that
// stays broken shows up as a dead job rather than as silence.
func (c *Client) awaitTask(ctx context.Context, uid int64, what string) error {
	task, err := c.c.WaitForTaskWithContext(ctx, uid, 50*time.Millisecond)
	if err != nil {
		return fmt.Errorf("meili %s task: %w", what, err)
	}
	if task.Status != meilisearch.TaskStatusSucceeded {
		msg := string(task.Status)
		if task.Error.Message != "" {
			msg = task.Error.Message
		}
		return fmt.Errorf("meili %s task %d: %s", what, uid, msg)
	}
	return nil
}

func (c *Client) DeleteOne(ctx context.Context, id string) error {
	task, err := c.c.Index(c.idx).DeleteDocumentWithContext(ctx, id, nil)
	if err != nil {
		return fmt.Errorf("meili delete: %w", err)
	}
	return c.awaitTask(ctx, task.TaskUID, "delete")
}

func (c *Client) Search(ctx context.Context, req search.Request) (search.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	res, err := c.c.Index(c.idx).SearchWithContext(ctx, req.Query, &meilisearch.SearchRequest{
		Filter:                req.Filter,
		Limit:                 int64(req.Limit),
		Offset:                int64(req.Offset),
		AttributesToHighlight: req.AttributesToHighlight,
		HighlightPreTag:       "<mark>",
		HighlightPostTag:      "</mark>",
	})
	if err != nil {
		return search.Response{}, fmt.Errorf("meili search: %w", err)
	}

	out := search.Response{Hits: make([]search.RawHit, 0, len(res.Hits))}
	for _, raw := range res.Hits {
		// meilisearch.Hit is map[string]json.RawMessage, so each field is
		// decoded on demand rather than through a fixed struct — which is what
		// lets one index hold three different entity kinds.
		m := make(map[string]any, len(raw))
		for k, v := range raw {
			var val any
			if err := json.Unmarshal(v, &val); err == nil {
				m[k] = val
			}
		}
		out.Hits = append(out.Hits, search.RawHit{
			Kind:     str(m["kind"]),
			EntityID: str(m["entity_id"]),
			Fields:   m,
		})
	}
	return out, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// Package search is Chapters 29 and 30, in that order and for that reason.
//
// Chapter 29: Postgres already has a search engine. A tsvector column, a GIN
// index, and the @@ operator give you stemming, stopwords, ranking, and
// tenant-scoped results with no new service to run, no second datastore to
// keep in sync, and no 3am page from a machine you forgot you owned.
//
// Chapter 30: you graduate to a real engine when a customer names a case
// Postgres can't do — typo tolerance, per-document language, faceted filters —
// and not one day earlier. When you do, the rule that keeps you sane is:
// Postgres stays authoritative. Data flows Postgres → Meili, one direction,
// and if Meili is down or wrong, the fallback is the Chapter 29 path, so users
// see different ranking rather than zero results.
//
// A finding from actually running both engines against the same data, which is
// worth having before you decide the graduation is free: the two are not
// ordered, they trade.
//
//	query "verifying", document "Verify the whole thing"
//	  Postgres  1 hit   — it stems, so verifying and verify are one word
//	  Meili     0 hits  — it does not stem; it tolerates typos instead
//
//	query "authentcation", document "Authentication Rewrite"
//	  Postgres  0 hits  — a misspelling is simply a different lexeme
//	  Meili     1 hit   — one typo at 8+ characters is within tolerance
//
// So switching Meili on does not strictly improve search; it changes which
// queries work. The fallback in Search() hides that: the same user typing the
// same word gets a different answer depending on whether Meili is up. Know
// that before you turn it on, and prefer to turn it on because a customer named
// the case, not because a search engine sounds more serious than a database.
//
// [verbatim ch29 + ch30 for the service shape, Search, searchMeili, IndexOne
// and the fallback; the constructors and the pgx plumbing are the glue the
// chapters imply.]
package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/db"
)

// ErrEntityGone means the row disappeared between the write that enqueued the
// reindex and the worker getting round to it. Not an error worth retrying —
// the correct response is to delete the document from Meili and move on.
var ErrEntityGone = errors.New("search: entity no longer exists")

// ErrQueryTooShort guards the index against one-character queries, which match
// half the corpus and cost a full scan of the GIN posting lists.
var ErrQueryTooShort = errors.New("search: query must be at least 2 characters")

// Hit is one search result, whichever engine produced it.
type Hit struct {
	Kind    string  `json:"kind"`
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Rank    float32 `json:"rank,omitempty"`
}

type SearchInput struct {
	OrgID  string
	Query  string
	Limit  int
	Offset int
}

type SearchResult struct {
	Hits []Hit `json:"hits"`
	// Source is "postgres" or "meili". It exists so an operator reading a
	// response can tell instantly whether the fallback fired.
	Source string `json:"source"`
}

// Document is the shape pushed to Meilisearch. Its ID is "kind_uuid" so one
// index can hold every entity type without collisions.
//
// The separator is an underscore, not a colon, because a Meilisearch document
// identifier may only contain letters, digits, hyphens and underscores. A colon
// is rejected at index time — and since Meili reports that as a failed task
// rather than a failed request, it is the kind of mistake that looks like an
// empty search page rather than an error.
type Document struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	Kind      string `json:"kind"`
	EntityID  string `json:"entity_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	UpdatedAt int64  `json:"updated_at"`
}

// Indexer is the narrow surface the service needs from Meilisearch, so the
// service can be constructed and tested without one.
type Indexer interface {
	EnsureIndex(ctx context.Context) error
	Upsert(ctx context.Context, docs []Document) error
	DeleteOne(ctx context.Context, id string) error
	Search(ctx context.Context, req Request) (Response, error)
}

type Service struct {
	queries *db.Queries
	log     *slog.Logger

	meili        Indexer
	meiliEnabled bool
}

func NewService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{queries: db.New(pool), log: log}
}

// WithMeili turns on the Chapter 30 path. Without it, Search is Chapter 29 and
// nothing else — which is a complete, working product.
func (s *Service) WithMeili(m Indexer) *Service {
	s.meili = m
	s.meiliEnabled = m != nil
	return s
}

func (s *Service) MeiliEnabled() bool { return s.meiliEnabled }

// docID builds the Meilisearch primary key for one entity.
func docID(kind, entityID string) string { return kind + "_" + entityID }

func validate(in SearchInput) error {
	if len([]rune(in.Query)) < 2 {
		return ErrQueryTooShort
	}
	return nil
}

// Search is Chapter 30's read path: try the fast engine, fall back to the one
// that is always there. A Meili failure is a warning in the log and a slightly
// different ordering for the user — never an error page.
func (s *Service) Search(ctx context.Context, in SearchInput) (SearchResult, error) {
	if err := validate(in); err != nil {
		return SearchResult{}, err
	}

	if s.meiliEnabled {
		out, err := s.searchMeili(ctx, in)
		if err == nil {
			return out, nil
		}
		// Meili failed. Log and fall back.
		s.log.Warn("meili failed, falling back to postgres", "err", err)
	}
	return s.searchPostgres(ctx, in)
}

// searchPostgres is Chapter 29's implementation, unchanged by Chapter 30.
func (s *Service) searchPostgres(ctx context.Context, in SearchInput) (SearchResult, error) {
	orgID, err := uuid.Parse(in.OrgID)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search: parse org id: %w", err)
	}

	rows, err := s.queries.SearchOrg(ctx, db.SearchOrgParams{
		OrganizationID: orgID,
		PlaintoTsquery: in.Query,
		Limit:          int32(in.Limit),
		Offset:         int32(in.Offset),
	})
	if err != nil {
		return SearchResult{}, fmt.Errorf("search: %w", err)
	}

	out := SearchResult{Hits: make([]Hit, 0, len(rows)), Source: "postgres"}
	for _, r := range rows {
		out.Hits = append(out.Hits, Hit{
			Kind:    r.EntityKind,
			ID:      r.EntityID.String(),
			Title:   r.Title,
			Snippet: string(r.Snippet),
			Rank:    r.Rank,
		})
	}
	return out, nil
}

func (s *Service) searchMeili(ctx context.Context, in SearchInput) (SearchResult, error) {
	res, err := s.meili.Search(ctx, Request{
		Query: in.Query,
		// Meilisearch enforces no tenancy of its own. Every single query is
		// filtered by org_id, and that filter is not optional.
		Filter:                fmt.Sprintf("org_id = %q", in.OrgID),
		Limit:                 in.Limit,
		Offset:                in.Offset,
		AttributesToHighlight: []string{"title", "body"},
	})
	if err != nil {
		return SearchResult{}, err
	}

	hits := make([]Hit, 0, len(res.Hits))
	for _, h := range res.Hits {
		hits = append(hits, Hit{
			Kind:    h.Kind,
			ID:      h.EntityID,
			Title:   h.Highlighted("title"),
			Snippet: h.Highlighted("body"),
		})
	}
	return SearchResult{Hits: hits, Source: "meili"}, nil
}

// IndexOne pushes one entity's current state into Meili. Called by the reindex
// worker, never by a handler — a handler that wrote to Meili directly would be
// making Meili authoritative, which is the mistake this design exists to avoid.
func (s *Service) IndexOne(ctx context.Context, kind, entityID string) error {
	if !s.meiliEnabled {
		return nil // feature off — Postgres FTS only
	}

	doc, err := s.loadDocument(ctx, kind, entityID)
	if err != nil {
		if errors.Is(err, ErrEntityGone) {
			// entity was deleted before we got here — delete from Meili too
			return s.meili.DeleteOne(ctx, docID(kind, entityID))
		}
		return err
	}

	return s.meili.Upsert(ctx, []Document{doc})
}

func (s *Service) loadDocument(ctx context.Context, kind, entityID string) (Document, error) {
	id, err := uuid.Parse(entityID)
	if err != nil {
		return Document{}, fmt.Errorf("search: parse entity id: %w", err)
	}
	row, err := s.queries.GetSearchDocument(ctx, db.GetSearchDocumentParams{
		EntityKind: kind,
		EntityID:   id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Document{}, ErrEntityGone
		}
		return Document{}, fmt.Errorf("search: load document: %w", err)
	}
	return Document{
		ID:        docID(kind, entityID),
		OrgID:     row.OrganizationID.String(),
		Kind:      row.EntityKind,
		EntityID:  row.EntityID.String(),
		Title:     row.Title,
		Body:      row.Body,
		UpdatedAt: row.UpdatedAt.Unix(),
	}, nil
}

// ReindexInBatches streams the whole index (or one org's slice of it) from
// Postgres into Meili in pages. Used after a settings change, a Meili restore,
// or the first time the engine is switched on.
func (s *Service) ReindexInBatches(ctx context.Context, orgID string, batch int) (int, error) {
	if !s.meiliEnabled {
		return 0, nil
	}
	if err := s.meili.EnsureIndex(ctx); err != nil {
		return 0, err
	}

	var filter *uuid.UUID
	if orgID != "" {
		id, err := uuid.Parse(orgID)
		if err != nil {
			return 0, fmt.Errorf("search: parse org id: %w", err)
		}
		filter = &id
	}

	total := 0
	after := uuid.UUID{} // the zero UUID sorts first, so this starts at the top
	for {
		rows, err := s.queries.ListSearchDocuments(ctx, db.ListSearchDocumentsParams{
			OrgID: filter,
			ID:    after,
			Limit: int32(batch),
		})
		if err != nil {
			return total, fmt.Errorf("search: list documents: %w", err)
		}
		if len(rows) == 0 {
			return total, nil
		}

		docs := make([]Document, 0, len(rows))
		for _, r := range rows {
			docs = append(docs, Document{
				ID:        docID(r.EntityKind, r.EntityID.String()),
				OrgID:     r.OrganizationID.String(),
				Kind:      r.EntityKind,
				EntityID:  r.EntityID.String(),
				Title:     r.Title,
				Body:      r.Body,
				UpdatedAt: r.UpdatedAt.Unix(),
			})
			after = r.ID
		}
		if err := s.meili.Upsert(ctx, docs); err != nil {
			return total, err
		}
		total += len(docs)
	}
}

-- Chapter 29 — the tenant-scoped full-text query.
--
-- Four functions carry it. plainto_tsquery turns the user's words into a
-- tsquery safely (never glue strings into to_tsquery — that is SQL injection in
-- a disguise). @@ is the match operator. ts_rank scores using the A/B/C weights
-- the trigger set. ts_headline returns the matching slice of the body wrapped
-- in <mark>, which is what makes a result look like a real search result.
--
-- The WHERE clause starts with organization_id. That ordering is not
-- cosmetic — it is the reason one tenant can never see another's rows.
--
-- [verbatim ch29]

-- name: SearchOrg :many
SELECT
    si.entity_kind,
    si.entity_id,
    si.title,
    ts_headline('english', si.body, plainto_tsquery('english', $2),
                'StartSel=<mark>, StopSel=</mark>, MaxWords=25, MinWords=10') AS snippet,
    ts_rank(si.search_vector, plainto_tsquery('english', $2)) AS rank
FROM search_index si
WHERE si.organization_id = $1
  AND si.search_vector @@ plainto_tsquery('english', $2)
ORDER BY rank DESC, si.updated_at DESC
LIMIT $3 OFFSET $4;

-- name: GetSearchDocument :one
-- Chapter 30's write path loads the canonical row out of Postgres before
-- pushing it to Meilisearch. Postgres is authoritative; Meili is a copy.
SELECT
    si.organization_id,
    si.entity_kind,
    si.entity_id,
    si.title,
    si.body,
    si.updated_at
FROM search_index si
WHERE si.entity_kind = $1 AND si.entity_id = $2;

-- name: ListSearchDocuments :many
-- The batched full reindex (Chapter 30). Keyset pagination on the primary key
-- so a reindex of a large org doesn't OFFSET its way into a slow crawl.
SELECT
    si.id,
    si.organization_id,
    si.entity_kind,
    si.entity_id,
    si.title,
    si.body,
    si.updated_at
FROM search_index si
WHERE (sqlc.narg('org_id')::uuid IS NULL OR si.organization_id = sqlc.narg('org_id')::uuid)
  AND si.id > $1
ORDER BY si.id
LIMIT $2;

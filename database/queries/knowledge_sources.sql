-- name: CreateKnowledgeSource :one
-- CreateKnowledgeSource adds a source, idle until its first run.
INSERT INTO knowledge_sources (id, workspace_id, type, name, mode, targets, help_center_id, config, include,
    exclude, credentials_encrypted, schedule, next_sync_at, created_by_member_id, created_at, updated_at)
VALUES (@id, @workspace_id, @type, @name, @mode, @targets, sqlc.narg(help_center_id), @config, @include, @exclude,
    sqlc.narg(credentials_encrypted), @schedule, sqlc.narg(next_sync_at), sqlc.narg(created_by_member_id), @now, @now)
RETURNING *;

-- name: UpdateKnowledgeSource :one
-- UpdateKnowledgeSource changes what a source reads and where it goes.
UPDATE knowledge_sources
SET name = @name, targets = @targets, help_center_id = sqlc.narg(help_center_id), config = @config,
    include = @include, exclude = @exclude, schedule = @schedule, next_sync_at = sqlc.narg(next_sync_at),
    credentials_encrypted = coalesce(sqlc.narg(credentials_encrypted), credentials_encrypted), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindKnowledgeSource :one
-- FindKnowledgeSource returns one source of the workspace.
SELECT * FROM knowledge_sources WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockKnowledgeSource :one
-- LockKnowledgeSource returns one source, locked for a change of state.
SELECT * FROM knowledge_sources WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListKnowledgeSources :many
-- ListKnowledgeSources returns the workspace's sources, oldest first.
SELECT * FROM knowledge_sources WHERE workspace_id = @workspace_id ORDER BY created_at, id;

-- name: DeleteKnowledgeSource :execrows
-- DeleteKnowledgeSource removes a source with its runs, documents and chunks.
DELETE FROM knowledge_sources WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetKnowledgeSourceState :exec
-- SetKnowledgeSourceState records where a source stands after a change.
UPDATE knowledge_sources
SET status = @status, last_error_code = sqlc.narg(last_error_code), last_error = sqlc.narg(last_error),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishKnowledgeSource :exec
-- FinishKnowledgeSource records the end of a run on its source.
UPDATE knowledge_sources
SET status = @status, last_error_code = sqlc.narg(last_error_code), last_error = sqlc.narg(last_error),
    last_synced_at = @now::timestamptz,
    last_success_at = CASE WHEN @succeeded::boolean THEN @now::timestamptz ELSE last_success_at END,
    next_sync_at = sqlc.narg(next_sync_at),
    document_count = (SELECT count(*) FROM knowledge_documents d
        WHERE d.workspace_id = @workspace_id AND d.source_id = @id AND d.status = 'active'),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListDueKnowledgeSources :many
-- ListDueKnowledgeSources finds the workspace's sources whose scheduled
-- run is due.
SELECT workspace_id, id FROM knowledge_sources
WHERE workspace_id = @workspace_id AND status IN ('idle', 'error') AND schedule <> 'manual' AND next_sync_at <= @now
ORDER BY next_sync_at
LIMIT @row_limit;

-- name: CreateKnowledgeSyncRun :one
-- CreateKnowledgeSyncRun starts a run of a source.
INSERT INTO knowledge_sync_runs (id, workspace_id, source_id, trigger, status, full_run, started_at, created_at,
    updated_at)
VALUES (@id, @workspace_id, @source_id, @trigger, 'running', @full_run, @now, @now, @now)
RETURNING *;

-- name: FindKnowledgeSyncRun :one
-- FindKnowledgeSyncRun returns one run of the workspace.
SELECT * FROM knowledge_sync_runs WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindActiveKnowledgeSyncRun :one
-- FindActiveKnowledgeSyncRun returns the source's run in progress.
SELECT * FROM knowledge_sync_runs
WHERE workspace_id = @workspace_id AND source_id = @source_id AND status IN ('queued', 'running')
ORDER BY created_at DESC
LIMIT 1;

-- name: ListKnowledgeSyncRuns :many
-- ListKnowledgeSyncRuns returns a source's runs, newest first.
SELECT * FROM knowledge_sync_runs
WHERE workspace_id = @workspace_id AND source_id = @source_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: AddKnowledgeSyncWork :one
-- AddKnowledgeSyncWork counts documents queued for fetching and bumps
-- the run's counters by the deltas given.
UPDATE knowledge_sync_runs r
SET pending = r.pending + @pending::int,
    stats = (SELECT coalesce(jsonb_object_agg(k, coalesce((r.stats->>k)::bigint, 0)
        + coalesce((@deltas::jsonb->>k)::bigint, 0)), '{}'::jsonb)
        FROM (SELECT jsonb_object_keys(r.stats) AS k UNION SELECT jsonb_object_keys(@deltas::jsonb)) keys),
    updated_at = @now
WHERE r.workspace_id = @workspace_id AND r.id = @id
RETURNING r.*;

-- name: MarkKnowledgeSyncDiscovered :one
-- MarkKnowledgeSyncDiscovered records that discovery reached the end.
UPDATE knowledge_sync_runs SET discovered = true, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: EndKnowledgeSyncRun :one
-- EndKnowledgeSyncRun closes a run once; a closed run stays as it is.
UPDATE knowledge_sync_runs
SET status = @status, error_code = sqlc.narg(error_code), error = sqlc.narg(error), finished_at = @now,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('queued', 'running')
RETURNING *;

-- name: PruneKnowledgeSyncRuns :exec
-- PruneKnowledgeSyncRuns keeps the source's newest runs only.
DELETE FROM knowledge_sync_runs r
WHERE r.workspace_id = @workspace_id AND r.source_id = @source_id AND r.id NOT IN (
    SELECT k.id FROM knowledge_sync_runs k
    WHERE k.workspace_id = @workspace_id AND k.source_id = @source_id
    ORDER BY k.created_at DESC, k.id DESC
    LIMIT @keep
);

-- name: FindKnowledgeDocumentByExternalID :one
-- FindKnowledgeDocumentByExternalID returns a source's document by the
-- id its driver knows it by.
SELECT * FROM knowledge_documents
WHERE workspace_id = @workspace_id AND source_id = @source_id AND external_id = @external_id;

-- name: FindKnowledgeDocument :one
-- FindKnowledgeDocument returns one document of the workspace.
SELECT * FROM knowledge_documents WHERE workspace_id = @workspace_id AND id = @id;

-- name: SeeKnowledgeDocument :one
-- SeeKnowledgeDocument records a document the run found, new or known,
-- and returns it; a document the run already saw returns no row.
INSERT INTO knowledge_documents (id, workspace_id, source_id, external_id, url, path, title, version,
    last_seen_run_id, created_at, updated_at)
VALUES (@id, @workspace_id, @source_id, @external_id, sqlc.narg(url), @path, @title, '', @run_id, @now, @now)
ON CONFLICT (source_id, external_id) DO UPDATE
SET last_seen_run_id = EXCLUDED.last_seen_run_id, url = coalesce(EXCLUDED.url, knowledge_documents.url),
    path = CASE WHEN EXCLUDED.path = '' THEN knowledge_documents.path ELSE EXCLUDED.path END
WHERE knowledge_documents.last_seen_run_id IS DISTINCT FROM EXCLUDED.last_seen_run_id
RETURNING *;

-- name: ExcludeKnowledgeDocument :exec
-- ExcludeKnowledgeDocument keeps a document out of the index, with why.
UPDATE knowledge_documents
SET status = 'excluded', excluded_by = @excluded_by, error_code = NULL, error = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: IncludeKnowledgeDocument :exec
-- IncludeKnowledgeDocument lets an excluded document be fetched again.
UPDATE knowledge_documents
SET status = 'pending', excluded_by = NULL, version = '', content_hash = '', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailKnowledgeDocument :exec
-- FailKnowledgeDocument records why a document could not be read.
UPDATE knowledge_documents
SET status = CASE WHEN status = 'active' THEN 'active' ELSE 'error' END, error_code = @error_code,
    error = @error, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: StoreKnowledgeDocument :one
-- StoreKnowledgeDocument keeps what a fetch read and makes it live.
UPDATE knowledge_documents
SET title = @title, url = sqlc.narg(url), path = @path, locale = @locale, translation_of = sqlc.narg(translation_of),
    collection = @collection, version = @version, content_markdown = @content_markdown,
    content_hash = @content_hash, visibility = @visibility, draft = @draft, assets = @assets, status = 'active',
    excluded_by = NULL,
    error_code = NULL, error = NULL, fetched_at = @now, indexed_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchKnowledgeDocument :exec
-- TouchKnowledgeDocument records an unchanged fetch.
UPDATE knowledge_documents
SET version = @version, status = 'active', error_code = NULL, error = NULL, fetched_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RemoveKnowledgeDocument :exec
-- RemoveKnowledgeDocument marks a document gone from its source.
UPDATE knowledge_documents SET status = 'removed', updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: RemoveUnseenKnowledgeDocuments :many
-- RemoveUnseenKnowledgeDocuments drops the documents a complete run no
-- longer saw, members' exclusions aside, and returns them.
UPDATE knowledge_documents
SET status = 'removed', updated_at = @now
WHERE workspace_id = @workspace_id AND source_id = @source_id AND status IN ('pending', 'active', 'error')
  AND (last_seen_run_id IS NULL OR last_seen_run_id <> @run_id)
RETURNING id;

-- name: ListKnowledgeDocuments :many
-- ListKnowledgeDocuments pages through a source's documents, filtered by
-- status and title.
SELECT id, source_id, external_id, url, path, title, locale, status, excluded_by, error_code, error,
    linked_article_id, fetched_at, indexed_at, updated_at,
    (SELECT count(*) FROM knowledge_document_chunks c WHERE c.document_id = d.id)::int AS chunk_count
FROM knowledge_documents d
WHERE d.workspace_id = @workspace_id AND d.source_id = @source_id
  AND (cardinality(@statuses::text[]) = 0 OR d.status = ANY (@statuses::text[]))
  AND (@query::text = '' OR d.title ILIKE '%' || @query::text || '%' OR d.path ILIKE '%' || @query::text || '%')
  AND (sqlc.narg(after_path)::text IS NULL OR (d.path, d.id) > (sqlc.narg(after_path)::text, @after_id::uuid))
ORDER BY d.path, d.id
LIMIT @row_limit;

-- name: CountKnowledgeDocumentsByStatus :many
-- CountKnowledgeDocumentsByStatus counts a source's documents by status.
SELECT status, count(*)::int AS total FROM knowledge_documents
WHERE workspace_id = @workspace_id AND source_id = @source_id
GROUP BY status;

-- name: ListKnowledgeDocumentsToImport :many
-- ListKnowledgeDocumentsToImport returns a source's live documents whose
-- text changed since it last became an article.
SELECT * FROM knowledge_documents
WHERE workspace_id = @workspace_id AND source_id = @source_id AND status = 'active'
  AND (imported_hash IS NULL OR imported_hash <> content_hash)
ORDER BY path, id
LIMIT @row_limit;

-- name: LinkKnowledgeDocumentArticle :exec
-- LinkKnowledgeDocumentArticle records the article a document became.
UPDATE knowledge_documents SET linked_article_id = @article_id, imported_hash = @imported_hash, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteKnowledgeDocumentChunks :exec
-- DeleteKnowledgeDocumentChunks drops a document's chunks.
DELETE FROM knowledge_document_chunks WHERE workspace_id = @workspace_id AND document_id = @document_id;

-- name: DeleteRemovedKnowledgeDocumentChunks :exec
-- DeleteRemovedKnowledgeDocumentChunks drops the chunks of a source's
-- documents that are no longer live.
DELETE FROM knowledge_document_chunks c
USING knowledge_documents d
WHERE c.document_id = d.id AND d.workspace_id = @workspace_id AND d.source_id = @source_id
  AND d.status IN ('removed', 'excluded');

-- name: InsertKnowledgeDocumentChunk :exec
-- InsertKnowledgeDocumentChunk stores one chunk of a document.
INSERT INTO knowledge_document_chunks (id, workspace_id, document_id, source_id, ordinal, title, heading_path, body,
    visibility, embed_hash, created_at)
VALUES (@id, @workspace_id, @document_id, @source_id, @ordinal, @title, @heading_path, @body, @visibility,
    @embed_hash, @now)
ON CONFLICT DO NOTHING;

-- name: ListDocumentEmbedHashes :many
-- ListDocumentEmbedHashes lists the hashes of a document's chunks with
-- no vector of the model.
SELECT DISTINCT c.embed_hash FROM knowledge_document_chunks c
WHERE c.workspace_id = @workspace_id AND c.document_id = @document_id
  AND NOT EXISTS (
      SELECT 1 FROM knowledge_embeddings e
      WHERE e.workspace_id = @workspace_id AND e.model_key = @model_key AND e.embed_hash = c.embed_hash
  )
ORDER BY 1;

-- name: SearchKnowledgeSourceDocuments :many
-- SearchKnowledgeSourceDocuments finds the live documents of sources
-- feeding the target, matching any of the query's words, best first.
SELECT d.id, d.source_id, s.type AS source_type, d.title, d.url, d.content_markdown, d.content_hash, d.visibility,
    d.locale, d.indexed_at, ts_rank_cd(d.search, to_tsquery('dimmi_search', @query::text))::real AS rank
FROM knowledge_documents d
JOIN knowledge_sources s ON s.id = d.source_id
WHERE d.workspace_id = @workspace_id AND d.status = 'active' AND s.mode = 'sync' AND @target::text = ANY (s.targets)
  AND d.search @@ to_tsquery('dimmi_search', @query::text)
  AND d.visibility = ANY (@visibilities::text[])
  AND (cardinality(@source_ids::uuid[]) = 0 OR d.source_id = ANY (@source_ids::uuid[]))
ORDER BY rank DESC, d.id
LIMIT @row_limit;

-- name: CountKnowledgeSourceDocuments :one
-- CountKnowledgeSourceDocuments counts the live documents of sources
-- feeding the target.
SELECT count(*) FROM knowledge_documents d
JOIN knowledge_sources s ON s.id = d.source_id
WHERE d.workspace_id = @workspace_id AND d.status = 'active' AND s.mode = 'sync' AND @target::text = ANY (s.targets);

-- name: FindKnowledgeSourceDocumentForReading :one
-- FindKnowledgeSourceDocumentForReading returns one live document as the
-- agent reads it.
SELECT d.id, d.source_id, s.type AS source_type, d.title, d.url, d.content_markdown, d.visibility, d.locale,
    d.indexed_at
FROM knowledge_documents d
JOIN knowledge_sources s ON s.id = d.source_id
WHERE d.workspace_id = @workspace_id AND d.id = @id AND d.status = 'active' AND s.mode = 'sync'
  AND d.visibility = ANY (@visibilities::text[]);

-- name: SearchKnowledgeDocumentChunksVector :many
-- SearchKnowledgeDocumentChunksVector ranks the live chunks of sources
-- feeding the target by cosine distance to the query vector.
SELECT c.id, c.document_id, c.source_id, s.type AS source_type, c.title, c.heading_path, c.body, c.visibility,
    d.url, d.locale, d.indexed_at, (e.embedding <=> @embedding::halfvec)::float8 AS distance
FROM knowledge_document_chunks c
JOIN knowledge_documents d ON d.id = c.document_id
JOIN knowledge_sources s ON s.id = c.source_id
JOIN knowledge_embeddings e
    ON e.workspace_id = c.workspace_id AND e.model_key = @model_key AND e.embed_hash = c.embed_hash
WHERE c.workspace_id = @workspace_id AND d.status = 'active' AND s.mode = 'sync' AND @target::text = ANY (s.targets)
  AND c.visibility = ANY (@visibilities::text[])
  AND (cardinality(@hashes::text[]) = 0 OR c.embed_hash = ANY (@hashes::text[]))
ORDER BY distance, c.id
LIMIT @row_limit;

-- name: ListKnowledgeAlertRecipients :many
-- ListKnowledgeAlertRecipients returns the active owners and admins who
-- hear about a source that stopped syncing.
SELECT sqlc.embed(u) FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.role IN ('owner', 'admin') AND m.status = 'active'
ORDER BY m.created_at, m.id;

-- name: FindCollectionForImport :one
-- FindCollectionForImport finds a live collection by name under a
-- parent, so an import reuses the collections it made before.
SELECT id FROM collections
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::uuid AND lower(name) = lower(@name::text)
ORDER BY created_at
LIMIT 1;

-- name: ListHelpCenterLocales :many
-- ListHelpCenterLocales lists the workspace's live help centers with
-- their language, for imports that place translations.
SELECT id, locale FROM help_centers WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY created_at;

-- name: UpsertCollectionRedirect :exec
-- UpsertCollectionRedirect points an old address at an imported
-- collection.
INSERT INTO help_center_redirects (workspace_id, help_center_id, from_path, target_type, target_collection_id, source,
    created_at, updated_at)
VALUES (@workspace_id, @help_center_id, @from_path, 'collection', @collection_id, 'import', @now, @now)
ON CONFLICT (help_center_id, from_path) DO NOTHING;

-- name: LockKnowledgeSyncRun :one
-- LockKnowledgeSyncRun returns a run, locked, so the writes of one run's
-- jobs take turns and never deadlock over the documents they share.
SELECT * FROM knowledge_sync_runs WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

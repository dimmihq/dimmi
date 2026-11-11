-- name: FindKnowledgeSettings :one
-- FindKnowledgeSettings returns the workspace's embedding binding.
SELECT * FROM knowledge_settings WHERE workspace_id = @workspace_id;

-- name: InsertKnowledgeSettings :exec
-- InsertKnowledgeSettings creates the workspace's binding unless it
-- exists, with the model it starts building, if any.
INSERT INTO knowledge_settings (workspace_id, pending_model_key, pending_key_id, embeddings_status,
    reembed_started_at, created_at, updated_at)
VALUES (@workspace_id, sqlc.narg(pending_model_key), sqlc.narg(pending_key_id), @embeddings_status,
    sqlc.narg(started_at), @now, @now)
ON CONFLICT (workspace_id) DO NOTHING;

-- name: SetPendingEmbeddingModel :one
-- SetPendingEmbeddingModel starts building a model while the active one
-- keeps serving searches.
UPDATE knowledge_settings
SET pending_model_key = @pending_model_key, pending_key_id = sqlc.narg(pending_key_id),
    embeddings_status = CASE WHEN active_model_key IS NULL THEN 'building' ELSE embeddings_status END,
    embeddings_error = NULL, reembed_total = 0, reembed_done = 0, reembed_failed = 0,
    reembed_started_at = @now::timestamptz, updated_at = @now
WHERE workspace_id = @workspace_id
RETURNING *;

-- name: UpdateReembedProgress :exec
-- UpdateReembedProgress records how far the pending model has come.
UPDATE knowledge_settings
SET reembed_total = @total, reembed_done = @done, reembed_failed = @failed, updated_at = @now
WHERE workspace_id = @workspace_id AND pending_model_key = @model_key::text;

-- name: SwitchEmbeddingModel :exec
-- SwitchEmbeddingModel makes the fully built pending model the active
-- one, keeping the old one's name for the rollback window.
UPDATE knowledge_settings
SET previous_model_key = active_model_key, active_model_key = pending_model_key,
    embedding_key_id = pending_key_id, pending_model_key = NULL, pending_key_id = NULL,
    embeddings_status = 'ready', embeddings_error = NULL, switched_at = @now::timestamptz, updated_at = @now
WHERE workspace_id = @workspace_id AND pending_model_key = @model_key::text;


-- name: SetEmbeddingsStatus :exec
-- SetEmbeddingsStatus records whether the bound embedder works.
UPDATE knowledge_settings
SET embeddings_status = @embeddings_status, embeddings_error = sqlc.narg(embeddings_error), updated_at = @now
WHERE workspace_id = @workspace_id;

-- name: FindLiveArticleForIndex :one
-- FindLiveArticleForIndex returns the article's live published version,
-- as the index chunks it.
SELECT a.id, a.workspace_id, a.help_center_id, v.id AS version_id, v.title, v.body_markdown, v.visibility
FROM articles a
JOIN article_versions v ON v.id = a.published_version_id
JOIN help_centers h ON h.id = a.help_center_id
WHERE a.workspace_id = @workspace_id AND a.id = @id AND a.deleted_at IS NULL AND a.status = 'published'
  AND h.deleted_at IS NULL;

-- name: DeleteStaleKnowledgeChunks :exec
-- DeleteStaleKnowledgeChunks removes the article's chunks of every
-- version but the one kept, all of them when none is.
DELETE FROM knowledge_chunks
WHERE workspace_id = @workspace_id AND article_id = @article_id
  AND (sqlc.narg(keep_version_id)::uuid IS NULL OR article_version_id <> sqlc.narg(keep_version_id)::uuid);

-- name: CountKnowledgeChunksForVersion :one
-- CountKnowledgeChunksForVersion counts the chunks of one version.
SELECT count(*) FROM knowledge_chunks WHERE workspace_id = @workspace_id AND article_version_id = @version_id;

-- name: InsertKnowledgeChunk :exec
-- InsertKnowledgeChunk stores one chunk; a chunk already stored stays.
INSERT INTO knowledge_chunks (id, workspace_id, article_id, article_version_id, help_center_id, ordinal, title,
    heading_path, body, visibility, embed_hash, created_at)
VALUES (@id, @workspace_id, @article_id, @article_version_id, @help_center_id, @ordinal, @title,
    @heading_path, @body, @visibility, @embed_hash, @now)
ON CONFLICT DO NOTHING;

-- name: ListUnindexedArticles :many
-- ListUnindexedArticles lists live published articles whose version has
-- no chunks yet.
SELECT a.id FROM articles a
JOIN help_centers h ON h.id = a.help_center_id
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL AND a.status = 'published'
  AND h.deleted_at IS NULL AND a.published_version_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM knowledge_chunks c WHERE c.article_version_id = a.published_version_id)
ORDER BY a.id
LIMIT @row_limit;

-- name: ListMissingEmbedHashes :many
-- ListMissingEmbedHashes lists the hashes of chunks and conversations
-- with no vector of the model and no recorded failure.
SELECT DISTINCT s.embed_hash::text AS embed_hash FROM (
    SELECT c.embed_hash FROM knowledge_chunks c WHERE c.workspace_id = @workspace_id
    UNION
    SELECT dc.embed_hash FROM knowledge_document_chunks dc WHERE dc.workspace_id = @workspace_id
    UNION
    SELECT k.embed_hash FROM knowledge_conversations k WHERE k.workspace_id = @workspace_id
) s
WHERE NOT EXISTS (
    SELECT 1 FROM knowledge_embeddings e
    WHERE e.workspace_id = @workspace_id AND e.model_key = @model_key AND e.embed_hash = s.embed_hash
) AND NOT EXISTS (
    SELECT 1 FROM knowledge_embedding_failures f
    WHERE f.workspace_id = @workspace_id AND f.model_key = @model_key AND f.embed_hash = s.embed_hash
)
ORDER BY 1
LIMIT @row_limit;

-- name: ListArticleEmbedHashes :many
-- ListArticleEmbedHashes lists the hashes of one article's chunks with
-- no vector of the model.
SELECT DISTINCT c.embed_hash FROM knowledge_chunks c
WHERE c.workspace_id = @workspace_id AND c.article_id = @article_id
  AND NOT EXISTS (
      SELECT 1 FROM knowledge_embeddings e
      WHERE e.workspace_id = @workspace_id AND e.model_key = @model_key AND e.embed_hash = c.embed_hash
  )
ORDER BY 1;

-- name: CountEmbeddingCoverage :one
-- CountEmbeddingCoverage counts the distinct chunk hashes, those with a
-- vector of the model and those the provider refused.
SELECT count(*)::int AS total,
    count(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM knowledge_embeddings e
        WHERE e.workspace_id = @workspace_id AND e.model_key = @model_key AND e.embed_hash = h.embed_hash
    ))::int AS embedded,
    count(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM knowledge_embedding_failures f
        WHERE f.workspace_id = @workspace_id AND f.model_key = @model_key AND f.embed_hash = h.embed_hash
    ))::int AS failed
FROM (
    SELECT embed_hash FROM knowledge_chunks WHERE workspace_id = @workspace_id
    UNION
    SELECT embed_hash FROM knowledge_document_chunks WHERE workspace_id = @workspace_id
) h;

-- name: ListChunkEmbedInputs :many
-- ListChunkEmbedInputs returns one chunk per hash, to build its input.
SELECT DISTINCT ON (embed_hash) embed_hash::text AS embed_hash, title::text AS title,
    heading_path::text[] AS heading_path, body::text AS body
FROM (
    SELECT a.embed_hash, a.title, a.heading_path, a.body, a.id FROM knowledge_chunks a
    WHERE a.workspace_id = @workspace_id AND a.embed_hash = ANY (@hashes::text[])
    UNION ALL
    SELECT b.embed_hash, b.title, b.heading_path, b.body, b.id FROM knowledge_document_chunks b
    WHERE b.workspace_id = @workspace_id AND b.embed_hash = ANY (@hashes::text[])
) c
ORDER BY embed_hash, id;

-- name: ListConversationEmbedInputs :many
-- ListConversationEmbedInputs returns the conversation texts per hash.
SELECT DISTINCT ON (embed_hash) embed_hash, body FROM knowledge_conversations
WHERE workspace_id = @workspace_id AND embed_hash = ANY (@hashes::text[])
ORDER BY embed_hash, conversation_id;

-- name: ListEmbeddedHashes :many
-- ListEmbeddedHashes returns the hashes that already have a vector of
-- the model.
SELECT embed_hash FROM knowledge_embeddings
WHERE workspace_id = @workspace_id AND model_key = @model_key AND embed_hash = ANY (@hashes::text[]);

-- name: InsertKnowledgeEmbedding :exec
-- InsertKnowledgeEmbedding stores one vector; a vector already stored
-- stays, since the same input always gives the same vector.
INSERT INTO knowledge_embeddings (workspace_id, model_key, embed_hash, dims, embedding, created_at)
VALUES (@workspace_id, @model_key, @embed_hash, @dims, @embedding::halfvec, @now)
ON CONFLICT DO NOTHING;

-- name: RecordEmbeddingFailure :exec
-- RecordEmbeddingFailure remembers an input the provider refused.
INSERT INTO knowledge_embedding_failures (workspace_id, model_key, embed_hash, last_error, updated_at)
VALUES (@workspace_id, @model_key, @embed_hash, @last_error, @now)
ON CONFLICT (workspace_id, model_key, embed_hash)
DO UPDATE SET attempts = knowledge_embedding_failures.attempts + 1, last_error = EXCLUDED.last_error,
    updated_at = EXCLUDED.updated_at;

-- name: ClearEmbeddingFailures :exec
-- ClearEmbeddingFailures forgets failures of inputs about to be retried.
DELETE FROM knowledge_embedding_failures
WHERE workspace_id = @workspace_id AND model_key = @model_key AND embed_hash = ANY (@hashes::text[]);

-- name: ListFailedEmbedHashes :many
-- ListFailedEmbedHashes lists refused inputs worth another try.
SELECT embed_hash FROM knowledge_embedding_failures
WHERE workspace_id = @workspace_id AND model_key = @model_key AND attempts < @max_attempts
ORDER BY embed_hash
LIMIT @row_limit;

-- name: PurgeUnusedEmbeddings :execrows
-- PurgeUnusedEmbeddings deletes a batch of vectors of models the
-- workspace no longer keeps.
DELETE FROM knowledge_embeddings d
WHERE d.workspace_id = @workspace_id AND (d.model_key, d.embed_hash) IN (
    SELECT i.model_key, i.embed_hash FROM knowledge_embeddings i
    WHERE i.workspace_id = @workspace_id AND i.model_key <> ALL (@keep::text[])
    LIMIT @batch
);

-- name: PurgeUnusedEmbeddingFailures :exec
-- PurgeUnusedEmbeddingFailures forgets failures of models no longer kept.
DELETE FROM knowledge_embedding_failures WHERE workspace_id = @workspace_id AND model_key <> ALL (@keep::text[]);

-- name: CountKnowledgeChunks :one
-- CountKnowledgeChunks counts the workspace's chunks, of articles and
-- of source documents.
SELECT (SELECT count(*) FROM knowledge_chunks c WHERE c.workspace_id = @workspace_id)
    + (SELECT count(*) FROM knowledge_document_chunks d WHERE d.workspace_id = @workspace_id);

-- name: SearchKnowledgeChunksVector :many
-- SearchKnowledgeChunksVector ranks the live chunks by cosine distance
-- of their vector of the model to the query vector, nearest first,
-- optionally only those with the given hashes.
SELECT c.id, c.article_id, c.help_center_id, c.title, c.heading_path, c.body, c.visibility, h.subdomain, v.slug,
    v.created_at, (e.embedding <=> @embedding::halfvec)::float8 AS distance
FROM knowledge_chunks c
JOIN articles a ON a.id = c.article_id AND a.published_version_id = c.article_version_id
JOIN article_versions v ON v.id = c.article_version_id
JOIN help_centers h ON h.id = c.help_center_id
JOIN knowledge_embeddings e
    ON e.workspace_id = c.workspace_id AND e.model_key = @model_key AND e.embed_hash = c.embed_hash
WHERE c.workspace_id = @workspace_id AND a.status = 'published' AND a.deleted_at IS NULL AND h.deleted_at IS NULL
  AND c.visibility = ANY (@visibilities::text[])
  AND (cardinality(@hashes::text[]) = 0 OR c.embed_hash = ANY (@hashes::text[]))
ORDER BY distance, c.id
LIMIT @row_limit;

-- name: SearchKnowledgeEmbeddingsANN1024 :many
-- SearchKnowledgeEmbeddingsANN1024 finds the nearest 1024-dimension
-- vectors of the model through the HNSW index.
SELECT e.embed_hash FROM knowledge_embeddings e
WHERE e.dims = 1024 AND e.workspace_id = @workspace_id AND e.model_key = @model_key
ORDER BY (e.embedding::halfvec(1024)) <=> (@embedding::halfvec)::halfvec(1024)
LIMIT @row_limit;

-- name: FindConversationForEmbedding :one
-- FindConversationForEmbedding returns what a closed conversation is
-- remembered by: its subject, the customer's first message and the
-- team's last reply.
SELECT c.id, c.workspace_id, c.subject, c.closed_at,
    coalesce((SELECT m.body FROM messages m
     WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
       AND m.author_type = 'contact' AND m.deleted_at IS NULL
     ORDER BY m.created_at, m.id LIMIT 1), '')::text AS first_message,
    coalesce((SELECT m.body FROM messages m
     WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
       AND m.author_type IN ('member', 'agent') AND m.deleted_at IS NULL
     ORDER BY m.created_at DESC, m.id DESC LIMIT 1), '')::text AS last_reply
FROM conversations c
WHERE c.workspace_id = @workspace_id AND c.id = @id AND c.status = 'closed' AND c.deleted_at IS NULL;

-- name: UpsertKnowledgeConversation :exec
-- UpsertKnowledgeConversation stores a closed conversation's text.
INSERT INTO knowledge_conversations (conversation_id, workspace_id, body, embed_hash, closed_at, updated_at)
VALUES (@conversation_id, @workspace_id, @body, @embed_hash, @closed_at, @now)
ON CONFLICT (conversation_id)
DO UPDATE SET body = EXCLUDED.body, embed_hash = EXCLUDED.embed_hash, closed_at = EXCLUDED.closed_at,
    updated_at = EXCLUDED.updated_at;

-- name: FindKnowledgeConversation :one
-- FindKnowledgeConversation returns a conversation's stored text.
SELECT * FROM knowledge_conversations WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id;

-- name: SearchKnowledgeConversationsFTS :many
-- SearchKnowledgeConversationsFTS ranks the closed conversations the
-- reader may see by the words they share with the query, best first.
SELECT k.conversation_id, k.body, k.closed_at, c.number, c.subject
FROM knowledge_conversations k
JOIN conversations c ON c.id = k.conversation_id
WHERE k.workspace_id = @workspace_id AND k.conversation_id <> @exclude_id AND c.deleted_at IS NULL
  AND k.search @@ to_tsquery('dimmi_search', @query::text)
  AND dimmi_conversation_visible(
      c.assignee_member_id, c.assignee_team_id, c.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
ORDER BY ts_rank_cd(k.search, to_tsquery('dimmi_search', @query::text)) DESC, k.conversation_id
LIMIT @row_limit;

-- name: SearchKnowledgeConversationsVectorBySelf :many
-- SearchKnowledgeConversationsVectorBySelf ranks the other closed
-- conversations the reader may see by cosine distance of their vector of the model to the
-- given conversation's.
SELECT k.conversation_id, k.body, k.closed_at, c.number, c.subject,
    (e.embedding <=> own.embedding)::float8 AS distance
FROM knowledge_conversations me
JOIN knowledge_embeddings own
    ON own.workspace_id = me.workspace_id AND own.model_key = @model_key AND own.embed_hash = me.embed_hash
JOIN knowledge_conversations k ON k.workspace_id = me.workspace_id AND k.conversation_id <> me.conversation_id
JOIN conversations c ON c.id = k.conversation_id
JOIN knowledge_embeddings e
    ON e.workspace_id = k.workspace_id AND e.model_key = @model_key AND e.embed_hash = k.embed_hash
WHERE me.workspace_id = @workspace_id AND me.conversation_id = @conversation_id AND c.deleted_at IS NULL
  AND dimmi_conversation_visible(
      c.assignee_member_id, c.assignee_team_id, c.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
ORDER BY distance, k.conversation_id
LIMIT @row_limit;

-- name: AddKnowledgeUsage :exec
-- AddKnowledgeUsage adds one batch of embedding or one crawl to the
-- month's tally of a source, or of the workspace's own articles and
-- conversations when the source is null.
INSERT INTO knowledge_usage_periods (
    workspace_id, period_start, source_id, kind, model_key, input_tokens, estimated_tokens,
    browser_ms, requests, cost_micros, unpriced, created_at, updated_at
) VALUES (
    @workspace_id, @period_start::date, sqlc.narg(source_id)::uuid, @kind, @model_key, @input_tokens,
    @estimated_tokens, @browser_ms, @requests, @cost_micros, @unpriced, @now::timestamptz, @now::timestamptz
)
ON CONFLICT (
    workspace_id, period_start, kind, model_key,
    COALESCE(source_id, '00000000-0000-0000-0000-000000000000'::uuid)
) DO UPDATE SET
    input_tokens = knowledge_usage_periods.input_tokens + excluded.input_tokens,
    estimated_tokens = knowledge_usage_periods.estimated_tokens + excluded.estimated_tokens,
    browser_ms = knowledge_usage_periods.browser_ms + excluded.browser_ms,
    requests = knowledge_usage_periods.requests + excluded.requests,
    cost_micros = knowledge_usage_periods.cost_micros + excluded.cost_micros,
    unpriced = knowledge_usage_periods.unpriced OR excluded.unpriced,
    updated_at = excluded.updated_at;

-- name: ListKnowledgeUsage :many
-- ListKnowledgeUsage sums a month's knowledge usage per source and
-- kind, with the source's name while it still exists.
SELECT
    u.source_id,
    coalesce(s.name, '')::text AS source_name,
    coalesce(s.type, '')::text AS source_type,
    u.kind,
    sum(u.input_tokens)::bigint AS input_tokens,
    sum(u.estimated_tokens)::bigint AS estimated_tokens,
    sum(u.browser_ms)::bigint AS browser_ms,
    sum(u.requests)::bigint AS requests,
    sum(u.cost_micros)::bigint AS cost_micros,
    bool_or(u.unpriced) AS unpriced
FROM knowledge_usage_periods u
LEFT JOIN knowledge_sources s ON s.id = u.source_id AND s.workspace_id = u.workspace_id
WHERE u.workspace_id = @workspace_id AND u.period_start = @period_start::date
GROUP BY u.source_id, s.name, s.type, u.kind
ORDER BY sum(u.cost_micros) DESC, u.kind;

-- name: ListEmbedHashSources :many
-- ListEmbedHashSources names the source each hash's chunks come from,
-- so an embedding batch is charged to the sources it served.
SELECT
    c.embed_hash,
    (array_agg(c.source_id ORDER BY c.source_id))[1]::uuid AS source_id
FROM knowledge_document_chunks c
WHERE c.workspace_id = @workspace_id AND c.embed_hash = ANY(@hashes::text[])
GROUP BY c.embed_hash;

-- name: SetRunDraft :exec
-- SetRunDraft keeps the answer the agent drafted for a teammate to send.
UPDATE ai_runs SET draft_body = @draft_body, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordRunFeedback :execrows
-- RecordRunFeedback records the verdict a teammate or customer gave on
-- a run's answer, and when.
UPDATE ai_runs SET feedback = @feedback, feedback_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetRunShadowAgreed :exec
-- SetRunShadowAgreed records whether a draft agreed with the reply the
-- teammate sent.
UPDATE ai_runs SET shadow_agreed = @shadow_agreed, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindUnjudgedDraft :one
-- FindUnjudgedDraft returns the conversation's newest draft nobody
-- compared with a teammate's reply yet.
SELECT * FROM ai_runs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND outcome = 'drafted'
  AND draft_body IS NOT NULL AND shadow_agreed IS NULL
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ShadowScore :one
-- ShadowScore counts, among the latest judged drafts of conversations
-- whose thread keys start with the prefix, how many agreed with the
-- teammate's reply.
SELECT count(*) FILTER (WHERE judged.shadow_agreed)::bigint AS agreed, count(*)::bigint AS judged
FROM (
    SELECT r.shadow_agreed FROM ai_runs r
    JOIN conversations c ON c.workspace_id = r.workspace_id AND c.id = r.conversation_id
    WHERE r.workspace_id = @workspace_id AND r.outcome = 'drafted' AND r.shadow_agreed IS NOT NULL
      AND c.external_thread_key LIKE @key_prefix::text || '%'
    ORDER BY r.created_at DESC LIMIT @row_limit
) AS judged;

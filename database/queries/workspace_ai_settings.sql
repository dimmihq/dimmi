-- name: FindWorkspaceAISettings :one
-- FindWorkspaceAISettings returns the workspace's AI settings.
SELECT * FROM workspace_ai_settings WHERE workspace_id = @workspace_id;

-- name: EnsureWorkspaceAISettings :one
-- EnsureWorkspaceAISettings makes the workspace's AI settings at their
-- defaults when missing, and returns them either way.
WITH made AS (
    INSERT INTO workspace_ai_settings (workspace_id, monthly_spend_cap_micros, created_at, updated_at)
    VALUES (@workspace_id, sqlc.narg(monthly_spend_cap_micros), @now, @now)
    ON CONFLICT (workspace_id) DO NOTHING
    RETURNING *
)
SELECT * FROM made
UNION ALL
SELECT * FROM workspace_ai_settings WHERE workspace_id = @workspace_id AND NOT EXISTS (SELECT 1 FROM made);

-- name: UpdateWorkspaceAISettings :one
-- UpdateWorkspaceAISettings replaces the workspace's AI settings.
UPDATE workspace_ai_settings SET
    answer_key_id = sqlc.narg(answer_key_id), answer_model = sqlc.narg(answer_model),
    classifier_key_id = sqlc.narg(classifier_key_id), classifier_model = sqlc.narg(classifier_model),
    fallback_key_id = sqlc.narg(fallback_key_id), fallback_model = sqlc.narg(fallback_model),
    monthly_spend_cap_micros = sqlc.narg(monthly_spend_cap_micros), updated_at = @now
WHERE workspace_id = @workspace_id
RETURNING *;

-- name: KeyUsedByWorkspaceAISettings :one
-- KeyUsedByWorkspaceAISettings reports whether the workspace's defaults
-- name a connection.
SELECT EXISTS (
    SELECT 1 FROM workspace_ai_settings
    WHERE workspace_id = @workspace_id
        AND (answer_key_id = @key_id OR classifier_key_id = @key_id OR fallback_key_id = @key_id)
)::boolean;

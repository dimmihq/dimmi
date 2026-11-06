-- name: ListRedactionRules :many
-- ListRedactionRules returns the workspace's own rules in the order they
-- apply.
SELECT * FROM redaction_rules
WHERE workspace_id = @workspace_id
ORDER BY position, id;

-- name: ListEnabledRedactionRules :many
-- ListEnabledRedactionRules returns the rules a write applies, in order.
SELECT * FROM redaction_rules
WHERE workspace_id = @workspace_id AND enabled
ORDER BY position, id;

-- name: CountRedactionRules :one
SELECT count(*) FROM redaction_rules WHERE workspace_id = @workspace_id;

-- name: FindRedactionRule :one
SELECT * FROM redaction_rules WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateRedactionRule :one
-- CreateRedactionRule adds a rule after the workspace's others.
INSERT INTO redaction_rules (workspace_id, name, pattern, replacement, enabled, position, created_at, updated_at)
VALUES (
    @workspace_id, @name, @pattern, @replacement, @enabled,
    (SELECT coalesce(max(position) + 1, 0) FROM redaction_rules WHERE workspace_id = @workspace_id),
    @now, @now
)
RETURNING *;

-- name: UpdateRedactionRule :one
UPDATE redaction_rules
SET name = @name, pattern = @pattern, replacement = @replacement, enabled = @enabled, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MoveRedactionRule :exec
UPDATE redaction_rules SET position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteRedactionRule :execrows
DELETE FROM redaction_rules WHERE workspace_id = @workspace_id AND id = @id;

-- name: GetRedactionSettings :one
-- GetRedactionSettings returns the workspace's built-in rule switches,
-- an empty object when it never changed them.
SELECT coalesce(settings -> 'redaction', '{}'::jsonb)::jsonb AS redaction
FROM workspaces WHERE id = @workspace_id;

-- name: SetRedactionSettings :exec
UPDATE workspaces SET settings = jsonb_set(settings, '{redaction}', @redaction::jsonb), updated_at = @now
WHERE id = @workspace_id;

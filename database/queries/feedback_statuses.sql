-- name: ListFeedbackStatuses :many
-- ListFeedbackStatuses returns the workspace's statuses in its order.
SELECT * FROM feedback_statuses WHERE workspace_id = @workspace_id ORDER BY position, created_at, id;

-- name: SeedFeedbackStatus :exec
-- SeedFeedbackStatus stores a built-in status for a workspace that has
-- none yet.
INSERT INTO feedback_statuses (workspace_id, key, name, color, category, roadmap, position)
VALUES (@workspace_id, @key, @name, @color, @category, @roadmap, @position)
ON CONFLICT (workspace_id, key) DO NOTHING;

-- name: CreateFeedbackStatus :one
-- CreateFeedbackStatus stores a new status after the others.
INSERT INTO feedback_statuses (workspace_id, key, name, color, category, roadmap, position, created_at, updated_at)
VALUES (@workspace_id, @key, @name, @color, @category, @roadmap,
    (SELECT coalesce(max(position) + 1, 0) FROM feedback_statuses WHERE workspace_id = @workspace_id), @now, @now)
ON CONFLICT (workspace_id, key) DO NOTHING
RETURNING *;

-- name: UpdateFeedbackStatus :one
-- UpdateFeedbackStatus renames, recolors or recategorizes a status.
UPDATE feedback_statuses SET name = @name, color = @color, category = @category, roadmap = @roadmap,
    updated_at = @now
WHERE workspace_id = @workspace_id AND key = @key
RETURNING *;

-- name: DeleteFeedbackStatus :exec
-- DeleteFeedbackStatus removes a status.
DELETE FROM feedback_statuses WHERE workspace_id = @workspace_id AND key = @key;

-- name: SaveFeedbackStatusOrder :exec
-- SaveFeedbackStatusOrder stores the statuses in the order given.
UPDATE feedback_statuses f SET position = (s.ord - 1)::integer, updated_at = @now
FROM unnest(@keys::text[]) WITH ORDINALITY AS s(key, ord)
WHERE f.workspace_id = @workspace_id AND f.key = s.key;

-- name: MoveFeedbackPostsToStatus :exec
-- MoveFeedbackPostsToStatus moves every request in a status, deleted
-- ones included, to another.
UPDATE feedback_posts SET status = @target, updated_at = @now
WHERE workspace_id = @workspace_id AND status = @source;

-- name: ListFeedbackStatusPositions :many
-- ListFeedbackStatusPositions is the workspace's saved status order.
SELECT status FROM feedback_status_positions WHERE workspace_id = @workspace_id ORDER BY position;

-- name: SaveFeedbackStatusPositions :exec
-- SaveFeedbackStatusPositions stores the statuses in the order given.
INSERT INTO feedback_status_positions (workspace_id, status, position)
SELECT @workspace_id, s.status, (s.ord - 1)::smallint
FROM unnest(@statuses::text[]) WITH ORDINALITY AS s(status, ord)
ON CONFLICT (workspace_id, status) DO UPDATE SET position = excluded.position;

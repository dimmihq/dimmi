-- name: ListSnoozeLists :many
-- ListSnoozeLists returns the workspace's snooze list and its teams'.
SELECT * FROM snooze_lists WHERE workspace_id = @workspace_id ORDER BY team_id NULLS FIRST;

-- name: SnoozeListsFor :many
-- SnoozeListsFor returns the workspace's list and the team's, when the
-- team has one, the team's first.
SELECT * FROM snooze_lists
WHERE workspace_id = @workspace_id AND (team_id IS NULL OR team_id = sqlc.narg(team_id)::uuid)
ORDER BY team_id NULLS LAST;

-- name: SaveSnoozeList :one
-- SaveSnoozeList writes the options of the workspace's or a team's list.
INSERT INTO snooze_lists (workspace_id, team_id, options, updated_at)
VALUES (@workspace_id, sqlc.narg(team_id), @options, @now)
ON CONFLICT (workspace_id, team_id) DO UPDATE SET options = excluded.options, updated_at = excluded.updated_at
RETURNING *;

-- name: DeleteSnoozeList :exec
-- DeleteSnoozeList removes a list, so it inherits again.
DELETE FROM snooze_lists WHERE workspace_id = @workspace_id AND team_id IS NOT DISTINCT FROM sqlc.narg(team_id)::uuid;

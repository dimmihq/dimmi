-- name: RemoveMemberFromTeams :exec
-- RemoveMemberFromTeams takes the member out of every team of the
-- workspace.
DELETE FROM team_members WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id;

-- name: AddMemberToTeams :exec
-- AddMemberToTeams puts the member in each of the teams that still
-- exists in the workspace; teams deleted meanwhile are skipped, and a
-- team the member is already in stays as it is.
INSERT INTO team_members (team_id, workspace_id, workspace_member_id, created_at)
SELECT t.id, t.workspace_id, @workspace_member_id::uuid, @now::timestamptz
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.id = ANY (@team_ids::uuid[]) AND t.deleted_at IS NULL
ON CONFLICT DO NOTHING;

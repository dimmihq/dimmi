-- name: RemoveMemberFromTeams :exec
-- RemoveMemberFromTeams takes the member out of every team of the
-- workspace.
DELETE FROM team_members WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id;

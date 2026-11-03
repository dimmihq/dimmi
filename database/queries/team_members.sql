-- name: RemoveMemberFromTeams :many
-- RemoveMemberFromTeams takes the member out of every team of the
-- workspace and returns the teams they left.
DELETE FROM team_members WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id
RETURNING team_id;

-- name: AddTeamMember :execrows
-- AddTeamMember puts the member in the team, taking the workspace from
-- the team row so the two can never differ. A member already in the
-- team adds no row.
INSERT INTO team_members (team_id, workspace_id, workspace_member_id, created_at)
SELECT t.id, t.workspace_id, @workspace_member_id, @now
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.id = @team_id
ON CONFLICT (team_id, workspace_member_id) DO NOTHING;

-- name: RemoveTeamMember :execrows
-- RemoveTeamMember takes the member out of the team.
DELETE FROM team_members
WHERE workspace_id = @workspace_id AND team_id = @team_id AND workspace_member_id = @workspace_member_id;

-- name: ListTeamMembers :many
-- ListTeamMembers returns the members of the teams with their accounts,
-- by name.
SELECT tm.team_id, sqlc.embed(m), sqlc.embed(u)
FROM team_members tm
JOIN workspace_members m ON m.id = tm.workspace_member_id
JOIN users u ON u.id = m.user_id
WHERE tm.workspace_id = @workspace_id AND tm.team_id = ANY (@team_ids::uuid[])
ORDER BY lower(u.name), m.id;

-- name: ListMemberTeams :many
-- ListMemberTeams returns the live teams of the members, by name.
SELECT tm.workspace_member_id, sqlc.embed(t)
FROM team_members tm
JOIN teams t ON t.id = tm.team_id
WHERE tm.workspace_id = @workspace_id AND tm.workspace_member_id = ANY (@member_ids::uuid[]) AND t.deleted_at IS NULL
ORDER BY lower(t.name), t.id;

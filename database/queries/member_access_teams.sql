-- name: ListMemberAccessTeamIDs :many
-- ListMemberAccessTeamIDs returns the teams the member's conversation
-- access names.
SELECT team_id FROM member_access_teams
WHERE workspace_id = @workspace_id AND member_id = @member_id
ORDER BY team_id;

-- name: ListAccessTeamIDsOfMembers :many
-- ListAccessTeamIDsOfMembers returns the teams the conversation access of
-- each of the members names.
SELECT member_id, team_id FROM member_access_teams
WHERE workspace_id = @workspace_id AND member_id = ANY (@member_ids::uuid[])
ORDER BY member_id, team_id;

-- name: ListRestrictedMembers :many
-- ListRestrictedMembers returns the workspace's active members who do not
-- see every conversation, usually none.
SELECT * FROM workspace_members
WHERE workspace_id = @workspace_id AND status = 'active' AND conversation_access <> 'all'
ORDER BY id;

-- name: ClearMemberAccessTeams :exec
-- ClearMemberAccessTeams empties the list of teams of the member's
-- conversation access.
DELETE FROM member_access_teams WHERE workspace_id = @workspace_id AND member_id = @member_id;

-- name: AddMemberAccessTeams :exec
-- AddMemberAccessTeams puts the teams on the list of the member's
-- conversation access.
INSERT INTO member_access_teams (workspace_id, member_id, team_id, created_at)
SELECT @workspace_id, @member_id, team_id, @now FROM unnest(@team_ids::uuid[]) AS team_id;

-- name: SetMemberConversationAccess :one
-- SetMemberConversationAccess changes which conversations the member sees.
UPDATE workspace_members
SET conversation_access = @conversation_access, conversation_access_unassigned = @conversation_access_unassigned,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

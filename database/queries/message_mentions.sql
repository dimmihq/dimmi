-- name: FindMentionedMembers :many
-- FindMentionedMembers returns the workspace's active members among the
-- ids, with their names, so a note mentions only teammates who exist.
SELECT wm.id, u.name
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = @workspace_id AND wm.id = ANY (@ids::uuid[]) AND wm.status = 'active'
ORDER BY wm.id;

-- name: FindMentionedTeams :many
-- FindMentionedTeams returns the workspace's live teams among the ids,
-- with their names.
SELECT id, name FROM teams
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL
ORDER BY id;

-- name: ListActiveTeamMembers :many
-- ListActiveTeamMembers returns the active members of the teams, each
-- with the team that brought them in.
SELECT tm.team_id, tm.workspace_member_id
FROM team_members tm
JOIN workspace_members wm ON wm.id = tm.workspace_member_id
WHERE tm.workspace_id = @workspace_id AND tm.team_id = ANY (@team_ids::uuid[]) AND wm.status = 'active'
ORDER BY tm.team_id, tm.workspace_member_id;

-- name: AddMessageMention :execrows
-- AddMessageMention records that the note mentions the member, directly
-- or through a team, and reports whether the mention is new.
INSERT INTO message_mentions (
    workspace_id, message_id, conversation_id, workspace_member_id, via_team_id, created_at
) VALUES (@workspace_id, @message_id, @conversation_id, @member_id, sqlc.narg(via_team_id), @now)
ON CONFLICT (message_id, workspace_member_id) DO NOTHING;

-- name: RemoveMessageMentionsExcept :exec
-- RemoveMessageMentionsExcept drops the note's mentions of members it no
-- longer names.
DELETE FROM message_mentions
WHERE workspace_id = @workspace_id AND message_id = @message_id
  AND NOT (workspace_member_id = ANY (@keep::uuid[]));

-- name: CountUnreadMentions :one
-- CountUnreadMentions counts the conversations whose notes mention the
-- member after what they last read. Being mentioned lets them read the
-- conversation, so no other access rule applies.
SELECT count(DISTINCT mm.conversation_id)::bigint
FROM message_mentions mm
JOIN conversations c ON c.workspace_id = mm.workspace_id AND c.id = mm.conversation_id
WHERE mm.workspace_id = @workspace_id
  AND mm.workspace_member_id = @member_id
  AND c.deleted_at IS NULL
  AND NOT c.spam
  AND mm.message_id > coalesce((
      SELECT r.last_read_message_id FROM conversation_reads r
      WHERE r.workspace_id = mm.workspace_id AND r.conversation_id = mm.conversation_id
        AND r.workspace_member_id = @member_id
  ), '00000000-0000-0000-0000-000000000000'::uuid);

-- name: ListMentionableMembers :many
-- ListMentionableMembers returns up to the limit of the workspace's active
-- members whose name starts a word with the text, by name, each saying
-- whether they may see the conversation with that holder.
SELECT wm.id, u.name, u.avatar_path, wm.availability,
    dimmi_conversation_visible(
        sqlc.narg(assignee_member_id)::uuid, sqlc.narg(assignee_team_id)::uuid, wm.conversation_access, wm.id,
        coalesce((
            SELECT array_agg(a.team_id) FROM member_access_teams a
            WHERE a.workspace_id = wm.workspace_id AND a.member_id = wm.id
        ), '{}'::uuid[]),
        wm.conversation_access_unassigned
    )::boolean AS can_see
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = @workspace_id AND wm.status = 'active'
  AND (@query::text = '' OR u.name ILIKE @query::text || '%' OR u.name ILIKE '% ' || @query::text || '%')
ORDER BY u.name, wm.id
LIMIT @row_limit::bigint;

-- name: ListMentionableTeams :many
-- ListMentionableTeams returns up to the limit of the workspace's live
-- teams whose name starts a word with the text, by name, with how many
-- active members each has.
SELECT t.id, t.name, (
    SELECT count(*) FROM team_members tm
    JOIN workspace_members wm ON wm.id = tm.workspace_member_id
    WHERE tm.workspace_id = t.workspace_id AND tm.team_id = t.id AND wm.status = 'active'
)::bigint AS member_count
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL
  AND (@query::text = '' OR t.name ILIKE @query::text || '%' OR t.name ILIKE '% ' || @query::text || '%')
ORDER BY t.name, t.id
LIMIT @row_limit::bigint;

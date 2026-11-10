-- name: CreateChannelMemberLink :one
-- CreateChannelMemberLink links a chat app user to a teammate, unless
-- either already has a link on the connection.
INSERT INTO channel_member_links (
    workspace_id, channel_connection_id, workspace_member_id, external_user_id, source, created_at, updated_at
)
VALUES (
    @workspace_id, @channel_connection_id, @workspace_member_id, @external_user_id, @source, @now::timestamptz,
    @now::timestamptz
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: DeleteChannelMemberLinks :execrows
-- DeleteChannelMemberLinks removes the links of the user or the teammate
-- on the connection, before a stronger link replaces them.
DELETE FROM channel_member_links
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND (external_user_id = @external_user_id OR workspace_member_id = @workspace_member_id);

-- name: FindChannelMemberLink :one
-- FindChannelMemberLink returns the active teammate linked to the chat
-- app user, with their name.
SELECT l.*, u.name AS member_name
FROM channel_member_links AS l
JOIN workspace_members AS m ON m.id = l.workspace_member_id AND m.workspace_id = l.workspace_id
JOIN users AS u ON u.id = m.user_id
WHERE l.workspace_id = @workspace_id AND l.channel_connection_id = @channel_connection_id
  AND l.external_user_id = @external_user_id AND m.status = 'active';

-- name: FindChannelMemberLinkByMember :one
-- FindChannelMemberLinkByMember returns the chat app user linked to the
-- teammate.
SELECT * FROM channel_member_links
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND workspace_member_id = @workspace_member_id;

-- name: FindActiveMemberByVerifiedEmail :one
-- FindActiveMemberByVerifiedEmail returns the workspace's active member
-- whose verified sign-in address is the email.
SELECT m.id, u.name
FROM workspace_members AS m
JOIN users AS u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.status = 'active' AND u.email_verified_at IS NOT NULL
  AND lower(u.email) = lower(@email::text);

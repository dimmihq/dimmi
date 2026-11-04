-- name: FindInvitationByToken :one
-- FindInvitationByToken returns the invitation the token digest belongs
-- to, in whichever workspace. It runs through a security definer
-- function, since the invitee is not a member of that workspace yet.
SELECT
    i.id::uuid AS id,
    i.email::citext AS email,
    i.status::text AS status,
    i.expires_at::timestamptz AS expires_at
FROM auth_find_invitation(@token_digest) AS i;

-- name: CountPendingInvitationsForEmail :one
-- CountPendingInvitationsForEmail counts the invitations to the address
-- still waiting for an answer and not expired, across workspaces.
SELECT count(*)
FROM auth_pending_invitations_for_email(@email::citext) AS i
WHERE i.expires_at::timestamptz > @now::timestamptz;

-- name: PreviewInvitation :one
-- PreviewInvitation returns what an invitation link invites to: the
-- workspace, who sent it, the role, the address and where it stands. It
-- runs through a security definer function, since the holder of the link
-- belongs to no workspace yet, and finds nothing for a workspace in the
-- trash.
SELECT
    i.id::uuid AS id,
    i.workspace_id::uuid AS workspace_id,
    i.workspace_name::text AS workspace_name,
    coalesce(i.inviter_name, '')::text AS inviter_name,
    i.email::citext AS email,
    i.role::text AS role,
    i.status::text AS status,
    i.expires_at::timestamptz AS expires_at
FROM auth_preview_invitation(@token_digest) AS i;

-- name: CreateInvitation :one
-- CreateInvitation stores a pending invitation to the address. A second
-- pending invitation to the same address in the workspace fails on
-- invitations_workspace_id_email_pending_key.
INSERT INTO invitations (
    workspace_id, email, role, team_ids, invited_by_member_id, token_hash, expires_at, last_sent_at, created_at,
    updated_at
) VALUES (
    @workspace_id, @email, @role, coalesce(@team_ids::uuid[], '{}'), @invited_by_member_id, @token_hash, @expires_at, @now, @now, @now
)
RETURNING *;

-- name: LockPendingInvitationForEmail :one
-- LockPendingInvitationForEmail returns the workspace's pending
-- invitation to the address, expired or not, and holds it until the
-- transaction ends.
SELECT * FROM invitations
WHERE workspace_id = @workspace_id AND email = @email AND status = 'pending'
FOR UPDATE;

-- name: LockInvitation :one
-- LockInvitation returns the invitation and holds it until the
-- transaction ends.
SELECT * FROM invitations WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: SendInvitationAgain :one
-- SendInvitationAgain gives the invitation a new link, valid until the
-- new expiry, with the role and teams given, and counts the sending.
-- The old link stops working.
UPDATE invitations
SET token_hash = @token_hash, expires_at = @expires_at, last_sent_at = @now, send_count = send_count + 1,
    role = @role, team_ids = coalesce(@team_ids::uuid[], '{}'), status = 'pending', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RevokeInvitation :one
-- RevokeInvitation withdraws the invitation, recording who withdrew it.
UPDATE invitations
SET status = 'revoked', revoked_at = @now::timestamptz, revoked_by_member_id = sqlc.narg(revoked_by)::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RevokePendingInvitations :execrows
-- RevokePendingInvitations withdraws every pending invitation of the
-- workspace.
UPDATE invitations
SET status = 'revoked', revoked_at = @now::timestamptz, revoked_by_member_id = sqlc.narg(revoked_by)::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND status = 'pending';

-- name: RevokeMemberInvitations :many
-- RevokeMemberInvitations withdraws every pending invitation the member
-- sent, recording who withdrew them, and returns which.
UPDATE invitations
SET status = 'revoked', revoked_at = @now::timestamptz, revoked_by_member_id = @revoked_by::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND invited_by_member_id = @invited_by AND status = 'pending'
RETURNING id;

-- name: AcceptInvitation :one
-- AcceptInvitation records that the user accepted the invitation.
UPDATE invitations
SET status = 'accepted', accepted_at = @now::timestamptz, accepted_by_user_id = @user_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListInvitations :many
-- ListInvitations returns a page of the workspace's invitations with the
-- name of who sent each, newest first, narrowed to the statuses asked
-- for. A pending invitation past its expiry reads as expired.
SELECT
    sqlc.embed(i),
    (CASE WHEN i.status = 'pending' AND i.expires_at <= @now::timestamptz THEN 'expired' ELSE i.status END)::text
        AS current_status,
    coalesce(u.name, '')::text AS inviter_name
FROM invitations i
LEFT JOIN workspace_members m ON m.id = i.invited_by_member_id AND m.workspace_id = i.workspace_id
LEFT JOIN users u ON u.id = m.user_id
WHERE i.workspace_id = @workspace_id
  AND (CASE WHEN i.status = 'pending' AND i.expires_at <= @now::timestamptz THEN 'expired' ELSE i.status END)
      = ANY (@statuses::text[])
ORDER BY i.created_at DESC, i.id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: FindInvitationWithInviter :one
-- FindInvitationWithInviter returns the invitation with the name of who
-- sent it.
SELECT sqlc.embed(i), coalesce(u.name, '')::text AS inviter_name
FROM invitations i
LEFT JOIN workspace_members m ON m.id = i.invited_by_member_id AND m.workspace_id = i.workspace_id
LEFT JOIN users u ON u.id = m.user_id
WHERE i.workspace_id = @workspace_id AND i.id = @id;

-- name: ListExistingTeams :many
-- ListExistingTeams returns which of the team ids name live teams of the
-- workspace.
SELECT id FROM teams WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

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

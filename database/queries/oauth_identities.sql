-- name: FindOAuthIdentity :one
-- FindOAuthIdentity returns the identity of the provider's account,
-- whichever user linked it.
SELECT * FROM oauth_identities WHERE provider = @provider AND provider_user_id = @provider_user_id;

-- name: FindUserOAuthIdentity :one
-- FindUserOAuthIdentity returns one of the user's identities. An identity
-- of another user is not found.
SELECT * FROM oauth_identities WHERE id = @id AND user_id = @user_id;

-- name: ListUserOAuthIdentities :many
-- ListUserOAuthIdentities returns the user's identities, oldest first.
SELECT * FROM oauth_identities WHERE user_id = @user_id ORDER BY created_at, id;

-- name: CreateOAuthIdentity :one
-- CreateOAuthIdentity links the provider's account to the user. A
-- provider account links to one user, and a user links one account per
-- provider, so a second link fails on the unique keys.
INSERT INTO oauth_identities (
    user_id, provider, provider_user_id, email, email_verified, username, avatar_url, hosted_domain,
    last_login_at, created_at, updated_at
)
VALUES (
    @user_id, @provider, @provider_user_id, @email, @email_verified, @username, @avatar_url, @hosted_domain,
    @last_login_at, @now, @now
)
RETURNING *;

-- name: TouchOAuthIdentity :exec
-- TouchOAuthIdentity keeps what the provider reports about its account
-- current, for display, and records a sign-in with it when there was one.
UPDATE oauth_identities
SET email = @email, email_verified = @email_verified, username = @username, avatar_url = @avatar_url,
    hosted_domain = @hosted_domain, last_login_at = COALESCE(@last_login_at, last_login_at),
    updated_at = @now::timestamptz
WHERE id = @id;

-- name: DeleteOAuthIdentity :execrows
-- DeleteOAuthIdentity unlinks one of the user's identities and reports
-- whether it was still there.
DELETE FROM oauth_identities WHERE id = @id AND user_id = @user_id;

-- name: DeleteUserOAuthIdentities :execrows
-- DeleteUserOAuthIdentities unlinks every identity of the user and
-- returns how many there were, when the owner of the inbox takes back
-- an account whose links a stranger may have made.
DELETE FROM oauth_identities WHERE user_id = @user_id;

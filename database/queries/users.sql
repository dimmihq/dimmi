-- name: CreateUser :one
-- CreateUser stores a new account. The email is unique without regard to
-- case, so a second account for the same address fails on
-- users_email_key.
INSERT INTO users (name, email, password_hash, email_verified_at, locale, timezone, created_via, created_at, updated_at)
VALUES (@name, @email, @password_hash, @email_verified_at, @locale, @timezone, @created_via, @now, @now)
RETURNING *;

-- name: FindUser :one
-- FindUser returns the account with the id, deleted or not.
SELECT * FROM users WHERE id = @id;

-- name: FindUserByEmail :one
-- FindUserByEmail returns the live account with the address. A deleted
-- account no longer owns its address, so it is never found here.
SELECT * FROM users WHERE email = @email AND deleted_at IS NULL;

-- name: UpdateUserPasswordHash :exec
-- UpdateUserPasswordHash replaces the stored hash with one made from the
-- same password, such as after the hashing cost was raised.
UPDATE users SET password_hash = @password_hash, updated_at = @now WHERE id = @id;

-- name: UpdateUserTwoFactorConfirmedAt :exec
-- UpdateUserTwoFactorConfirmedAt records whether a second factor protects
-- the account: the time the first one was activated, or NULL once the
-- last one is gone.
UPDATE users SET two_factor_confirmed_at = @two_factor_confirmed_at, updated_at = @now WHERE id = @id;

-- name: UpdateUserDeletedAt :exec
-- UpdateUserDeletedAt marks the account deleted. Its sessions and tokens
-- stop working at once, since every guard loads the account.
UPDATE users SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz WHERE id = @id;

-- name: MarkUserEmailVerified :exec
-- MarkUserEmailVerified records that the user proved they read mail at
-- their address. An email proven earlier keeps its first proof.
UPDATE users
SET email_verified_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id AND email_verified_at IS NULL;

-- name: UpdateUserPassword :exec
-- UpdateUserPassword sets a new password, or removes it with a NULL hash,
-- and records when the password last changed.
UPDATE users
SET password_hash = @password_hash, password_changed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id;

-- name: RevokeUserTokens :one
-- RevokeUserTokens revokes every live access token the user holds, in
-- every workspace, and returns how many it revoked. It runs through a
-- security definer function, since tokens of many workspaces are hidden
-- until one is chosen.
SELECT auth_revoke_user_tokens(@user_id, @now)::bigint AS revoked;

-- name: LockUser :one
-- LockUser returns the account and holds it until the transaction ends,
-- so two changes to how it signs in cannot both pass a check that only
-- one of them may.
SELECT * FROM users WHERE id = @id FOR UPDATE;

-- name: SetUserAvatarPath :execrows
-- SetUserAvatarPath gives an account without an avatar the stored image.
-- An avatar the user set in the meantime is kept.
UPDATE users SET avatar_path = @avatar_path, updated_at = @now::timestamptz WHERE id = @id AND avatar_path IS NULL;

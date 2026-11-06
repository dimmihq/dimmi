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

-- name: UpdateUserProfile :one
-- UpdateUserProfile replaces what a person says about themselves: their
-- name, language, time zone and the preferences they changed.
UPDATE users
SET name = @name, locale = @locale, timezone = @timezone, preferences = @preferences,
    updated_at = @now::timestamptz
WHERE id = @id
RETURNING *;

-- name: ReplaceUserAvatarPath :one
-- ReplaceUserAvatarPath gives the account the stored image, or none with a
-- NULL path, whatever it had before.
UPDATE users SET avatar_path = @avatar_path, updated_at = @now::timestamptz WHERE id = @id RETURNING *;

-- name: UpdateUserEmail :one
-- UpdateUserEmail moves the account to an address its owner just proved.
-- It fails on users_email_key when another account took the address first.
UPDATE users
SET email = @email, email_verified_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id
RETURNING *;

-- name: AnonymizeUser :exec
-- AnonymizeUser leaves only a nameless row behind, so what the person wrote
-- keeps an author while nothing about them remains. The placeholder address
-- is unique and can never receive mail, and frees the old one for anyone.
UPDATE users
SET name = @name, email = @email, password_hash = NULL, two_factor_confirmed_at = NULL, avatar_path = NULL,
    locale = NULL, timezone = NULL, deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id;

-- name: CountUserLiveTokens :one
-- CountUserLiveTokens counts the access tokens that still work for the user,
-- in every workspace. It runs through a security definer function, since
-- tokens of many workspaces are hidden until one is chosen.
SELECT auth_count_user_tokens(@user_id, @now)::bigint AS remaining;

-- name: CountLiveUsers :one
-- CountLiveUsers counts the accounts that were not deleted, which tells a
-- fresh install from one that already has people.
SELECT count(*) FROM users WHERE deleted_at IS NULL;

-- name: ListLiveUsers :many
-- ListLiveUsers returns the accounts that were not deleted, oldest first,
-- each with the kinds of its active second factors, for the operator.
SELECT sqlc.embed(u),
    coalesce((
        SELECT array_agg(DISTINCT f.type ORDER BY f.type) FROM user_auth_factors f
        WHERE f.user_id = u.id AND f.status = 'active' AND f.type <> 'recovery_code'
    ), '{}')::text[] AS factor_types
FROM users u
WHERE u.deleted_at IS NULL
ORDER BY u.created_at, u.id;

-- name: AddUserRiskFlags :exec
-- AddUserRiskFlags marks the account with the flags, keeping each once.
UPDATE users
SET risk_flags = ARRAY(SELECT DISTINCT unnest(risk_flags || @flags::text[]) ORDER BY 1), updated_at = @now
WHERE id = @id;

-- name: SetUserManagedBy :one
-- SetUserManagedBy hands the account to the organization whose identity
-- provider vouches for it, and takes the name it reports when it sent
-- one.
UPDATE users
SET managed_by_organization_id = @organization_id, name = COALESCE(sqlc.narg('name'), name), updated_at = @now
WHERE id = @id
RETURNING *;

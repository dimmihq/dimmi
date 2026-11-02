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

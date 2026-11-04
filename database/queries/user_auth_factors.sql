-- name: CreateAuthFactor :one
-- CreateAuthFactor stores a new factor of the user. The id is chosen by
-- the caller, since the factor's secret is sealed bound to it.
INSERT INTO user_auth_factors (
    id, user_id, type, name, status, secret_encrypted, credential, is_preferred, confirmed_at, created_at, updated_at
)
VALUES (
    @id, @user_id, @type, @name, @status, @secret_encrypted, @credential, @is_preferred, @confirmed_at, @now, @now
)
RETURNING *;

-- name: ListUserAuthFactors :many
-- ListUserAuthFactors returns the user's active factors and the pending
-- ones started since the cutoff, preferred first, then oldest first.
SELECT * FROM user_auth_factors
WHERE user_id = @user_id AND (status = 'active' OR created_at > @pending_since)
ORDER BY is_preferred DESC, created_at, id;

-- name: LockUserAuthFactors :many
-- LockUserAuthFactors returns every factor of the user and holds them
-- until the transaction ends, so changes to them happen one at a time.
SELECT * FROM user_auth_factors
WHERE user_id = @user_id
ORDER BY is_preferred DESC, created_at, id
FOR UPDATE;

-- name: LockUserAuthFactor :one
-- LockUserAuthFactor returns one of the user's factors and holds it until
-- the transaction ends. A factor of another user is not found.
SELECT * FROM user_auth_factors WHERE id = @id AND user_id = @user_id FOR UPDATE;

-- name: ListActiveAuthFactors :many
-- ListActiveAuthFactors returns the user's active factors, preferred
-- first, then oldest first.
SELECT * FROM user_auth_factors
WHERE user_id = @user_id AND status = 'active'
ORDER BY is_preferred DESC, created_at, id;

-- name: ActivateAuthFactor :one
-- ActivateAuthFactor turns a pending factor active, keeping what its
-- first answer left: the credential, the digest it is found by, the
-- step or sign counter used, and whether it is now the preferred one.
UPDATE user_auth_factors
SET status = 'active', confirmed_at = @now::timestamptz, credential = @credential, lookup_hash = @lookup_hash,
    last_counter = @last_counter, is_preferred = @is_preferred, updated_at = @now
WHERE id = @id
RETURNING *;

-- name: RecordAuthFactorUse :exec
-- RecordAuthFactorUse keeps what checking a code changed on the factor:
-- its sealed secret, its credential and its last step.
UPDATE user_auth_factors
SET secret_encrypted = @secret_encrypted, credential = @credential, last_counter = @last_counter,
    last_used_at = @now::timestamptz, updated_at = @now
WHERE id = @id;

-- name: RenameAuthFactor :one
-- RenameAuthFactor gives the factor a new name.
UPDATE user_auth_factors SET name = @name, updated_at = @now WHERE id = @id RETURNING *;

-- name: UnpreferAuthFactors :exec
-- UnpreferAuthFactors clears the user's preferred factor, before another
-- one becomes preferred.
UPDATE user_auth_factors SET is_preferred = false, updated_at = @now WHERE user_id = @user_id AND is_preferred;

-- name: PreferAuthFactor :one
-- PreferAuthFactor makes the factor the user's preferred one.
UPDATE user_auth_factors SET is_preferred = true, updated_at = @now WHERE id = @id RETURNING *;

-- name: DeleteAuthFactor :exec
-- DeleteAuthFactor removes one of the user's factors.
DELETE FROM user_auth_factors WHERE id = @id AND user_id = @user_id;

-- name: DeleteAuthFactorsOfType :exec
-- DeleteAuthFactorsOfType removes the user's factors of the type in the
-- status, such as an unfinished enrollment a new one replaces.
DELETE FROM user_auth_factors WHERE user_id = @user_id AND type = @type AND status = @status;

-- name: DeleteUserAuthFactors :execrows
-- DeleteUserAuthFactors removes every factor of the user and reports how
-- many there were.
DELETE FROM user_auth_factors WHERE user_id = @user_id;

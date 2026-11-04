-- name: CreateOneTimeToken :one
-- CreateOneTimeToken stores a new short-lived secret: only the digest of
-- its token and, for secrets with a code, of its code. The id is chosen
-- by the caller, since the code's digest is bound to it.
INSERT INTO one_time_tokens (
    id, purpose, user_id, email, token_hash, code_hash, data, ip, user_agent, expires_at, created_at
)
VALUES (@id, @purpose, @user_id, @email, @token_hash, @code_hash, @data, @ip, @user_agent, @expires_at, @now)
RETURNING *;

-- name: FindOneTimeToken :one
-- FindOneTimeToken returns the secret with the id and purpose, spent or
-- not, so a caller can tell a used link from an unknown one.
SELECT * FROM one_time_tokens WHERE id = @id AND purpose = @purpose;

-- name: FindOneTimeTokenByHash :one
-- FindOneTimeTokenByHash returns the secret whose token has the digest,
-- spent or not.
SELECT * FROM one_time_tokens WHERE token_hash = @token_hash AND purpose = @purpose;

-- name: ListLiveOneTimeTokens :many
-- ListLiveOneTimeTokens returns the secrets of the purpose sent to the
-- address that can still be used, newest first.
SELECT * FROM one_time_tokens
WHERE email = @email AND purpose = @purpose AND consumed_at IS NULL AND expires_at > @now
  AND attempts < @max_attempts
ORDER BY created_at DESC, id DESC;

-- name: SpendOneTimeToken :one
-- SpendOneTimeToken uses the secret up, only when it is still unspent,
-- unexpired and not burned by wrong guesses. Two requests racing for one
-- secret cannot both get a row back.
UPDATE one_time_tokens
SET consumed_at = @now::timestamptz
WHERE id = @id AND consumed_at IS NULL AND expires_at > @now::timestamptz AND attempts < @max_attempts
RETURNING *;

-- name: CountOneTimeTokenAttempt :one
-- CountOneTimeTokenAttempt adds a wrong guess to the secret and returns
-- how many it has had.
UPDATE one_time_tokens SET attempts = attempts + 1 WHERE id = @id RETURNING attempts;

-- name: ExpireOlderOneTimeTokens :exec
-- ExpireOlderOneTimeTokens ends every live secret of the purpose sent to
-- the address except the newest ones kept, so only a few can be guessed
-- at once.
UPDATE one_time_tokens
SET expires_at = @now
WHERE id IN (
    SELECT t.id FROM one_time_tokens AS t
    WHERE t.email = @email AND t.purpose = @purpose AND t.consumed_at IS NULL AND t.expires_at > @now
    ORDER BY t.created_at DESC, t.id DESC
    OFFSET @keep
);

-- name: ExpireUserOneTimeTokens :exec
-- ExpireUserOneTimeTokens ends every live secret of the user, once the
-- address they were mailed to stops being the account's.
UPDATE one_time_tokens SET expires_at = @now
WHERE user_id = @user_id AND consumed_at IS NULL AND expires_at > @now;

-- name: DeleteUserOneTimeTokens :exec
-- DeleteUserOneTimeTokens removes every secret of a deleted account.
DELETE FROM one_time_tokens WHERE user_id = @user_id;

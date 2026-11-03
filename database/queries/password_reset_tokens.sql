-- name: ReplacePasswordResetToken :one
-- ReplacePasswordResetToken gives the user a new reset under a new id,
-- replacing the one they had, so an earlier link stops working at once.
INSERT INTO password_reset_tokens (id, user_id, token_hash, requested_ip, attempts, expires_at, created_at)
VALUES (@id, @user_id, @token_hash, @requested_ip, 0, @expires_at, @now)
ON CONFLICT (user_id) DO UPDATE
SET id = EXCLUDED.id, token_hash = EXCLUDED.token_hash, requested_ip = EXCLUDED.requested_ip,
    attempts = 0, expires_at = EXCLUDED.expires_at, created_at = EXCLUDED.created_at
RETURNING *;

-- name: FindPasswordResetToken :one
-- FindPasswordResetToken returns the reset with the id.
SELECT * FROM password_reset_tokens WHERE id = @id;

-- name: CountPasswordResetAttempt :one
-- CountPasswordResetAttempt adds a wrong guess to the reset and returns
-- how many it has had.
UPDATE password_reset_tokens SET attempts = attempts + 1 WHERE id = @id RETURNING attempts;

-- name: DeletePasswordResetToken :execrows
-- DeletePasswordResetToken removes the reset, once used or burned. It
-- reports whether it was still there, so two requests racing for one
-- reset cannot both use it.
DELETE FROM password_reset_tokens WHERE id = @id;

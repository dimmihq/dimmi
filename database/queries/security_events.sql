-- name: CreateSecurityEvent :exec
-- CreateSecurityEvent adds an entry to the user's own security log.
INSERT INTO security_events (user_id, type, ip, user_agent, device_label, metadata, created_at)
VALUES (@user_id, @type, @ip, @user_agent, @device_label, @metadata, @now);

-- name: HasSignedInFromDevice :one
-- HasSignedInFromDevice says whether the user ever signed in successfully
-- from the browser holding the device cookie with this id.
SELECT EXISTS (
    SELECT 1 FROM security_events
    WHERE user_id = @user_id AND type = 'login.succeeded' AND metadata ->> 'device_id' = @device_id::text
);

-- name: ListUserSecurityEvents :many
-- ListUserSecurityEvents returns a page of the user's security log,
-- newest first, after the cursor position when one is given.
SELECT * FROM security_events
WHERE user_id = @user_id
  AND (
      sqlc.narg(cursor_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT @row_limit::bigint;

-- name: PruneSecurityEvents :execrows
-- PruneSecurityEvents deletes up to a batch of security log entries, of
-- every user, written before the cutoff, oldest first.
DELETE FROM security_events
WHERE id IN (
    SELECT e.id FROM security_events AS e WHERE e.created_at < @cutoff ORDER BY e.created_at LIMIT @batch::bigint
);

-- name: FindSignInHistory :one
-- FindSignInHistory says whether the user ever signed in before and
-- whether the device with this id signed in as them since the time, to
-- tell a sign-in from a new device.
SELECT
    EXISTS (SELECT 1 FROM security_events AS e WHERE e.user_id = @user_id AND e.type = 'login.succeeded')::boolean
        AS signed_in_before,
    EXISTS (
        SELECT 1 FROM security_events AS e
        WHERE e.user_id = @user_id AND e.type = 'login.succeeded' AND e.created_at >= @since
          AND e.metadata ->> 'device_id' = @device_id::text
    )::boolean AS known_device;

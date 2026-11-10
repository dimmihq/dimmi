-- name: EnsureChannelSocketSession :exec
-- EnsureChannelSocketSession makes the lease row of a socket exist.
INSERT INTO channel_socket_sessions (kind, key, created_at, updated_at)
VALUES (@kind, @key, @now::timestamptz, @now::timestamptz)
ON CONFLICT (kind, key) DO NOTHING;

-- name: AcquireChannelSocketLease :one
-- AcquireChannelSocketLease takes the lease of a socket nobody holds, or
-- whose holder stopped renewing, or renews it for its holder. Another
-- live holder returns no row.
UPDATE channel_socket_sessions
SET owner = @owner, lease_expires_at = @expires_at::timestamptz,
    connected_at = CASE WHEN owner = @owner THEN connected_at ELSE @now::timestamptz END,
    updated_at = @now::timestamptz
WHERE kind = @kind AND key = @key
  AND (owner IS NULL OR owner = @owner OR lease_expires_at IS NULL OR lease_expires_at < @now::timestamptz)
RETURNING *;

-- name: ReleaseChannelSocketLease :exec
-- ReleaseChannelSocketLease gives a lease up so another replica takes it
-- at once, recording why the socket ended.
UPDATE channel_socket_sessions
SET owner = NULL, lease_expires_at = NULL, last_error = sqlc.narg(last_error), updated_at = @now::timestamptz
WHERE kind = @kind AND key = @key AND owner = @owner;

-- name: RecordChannelSocketEvent :exec
-- RecordChannelSocketEvent records when the socket last delivered.
UPDATE channel_socket_sessions SET last_event_at = @now::timestamptz
WHERE kind = @kind AND key = @key AND owner = @owner;

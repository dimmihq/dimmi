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
-- ListUserSecurityEvents returns the user's latest security log entries.
SELECT * FROM security_events WHERE user_id = @user_id ORDER BY created_at DESC, id DESC LIMIT @row_limit;

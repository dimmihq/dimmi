-- name: ProveWidgetSessionEmail :one
-- ProveWidgetSessionEmail records that the session's visitor typed the
-- code mailed to the address.
UPDATE widget_sessions SET verified_email = @email, email_verified_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ProveContactEmail :one
-- ProveContactEmail sets the lead's address to the one its visitor
-- proved and marks it verified.
UPDATE contacts SET email = @email, email_verified = true, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

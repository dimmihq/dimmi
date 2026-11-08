-- name: FindReaderByEmail :one
-- FindReaderByEmail returns who a help center reader who proved an
-- address is: a customer the team's product named with it before any
-- lead, then the one with a verified address, then the latest seen.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND email = @email AND deleted_at IS NULL AND merged_into_id IS NULL
ORDER BY (kind = 'user') DESC, email_verified DESC, last_seen_at DESC NULLS LAST
LIMIT 1;

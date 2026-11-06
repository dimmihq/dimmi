-- name: ResolveWidgetSession :one
-- ResolveWidgetSession finds whose session a token hash names, before any
-- workspace is known.
SELECT workspace_id::uuid, id::uuid FROM widget_resolve_session(@hash);

-- name: CreateWidgetSession :one
-- CreateWidgetSession stores a visitor's new session.
INSERT INTO widget_sessions (
    workspace_id, widget_settings_id, contact_id, token_hash, kind, test, expires_at, bot_score, pow_solved_at, origin,
    user_agent, ip_prefix, locale, timezone, current_page, recent_pages, context, last_seen_at, created_at, updated_at
) VALUES (
    @workspace_id, @widget_settings_id, @contact_id, @token_hash, @kind, @test, @expires_at, @bot_score, @pow_solved_at,
    @origin, @user_agent, @ip_prefix, @locale, @timezone, @current_page, @recent_pages, @context, @now, @now, @now
)
RETURNING *;

-- name: FindWidgetSession :one
-- FindWidgetSession returns the session with the id in the workspace.
SELECT * FROM widget_sessions WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateWidgetSessionActivity :exec
-- UpdateWidgetSessionActivity notes a visitor was active and slides the
-- session's expiry.
UPDATE widget_sessions SET expires_at = @expires_at, last_seen_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RevokeWidgetSession :execrows
-- RevokeWidgetSession ends a live session for the reason, and any session
-- handed over from it.
UPDATE widget_sessions SET revoked_at = @now::timestamptz, revoke_reason = @reason::text, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND revoked_at IS NULL;

-- name: RevokeWidgetSessionsForContact :many
-- RevokeWidgetSessionsForContact ends every live session of the contact.
UPDATE widget_sessions SET revoked_at = @now::timestamptz, revoke_reason = @reason::text, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND revoked_at IS NULL
RETURNING id, contact_id;

-- name: RevokeWidgetSessionsForWidget :many
-- RevokeWidgetSessionsForWidget ends every live session of a deleted widget.
UPDATE widget_sessions SET revoked_at = @now::timestamptz, revoke_reason = @reason::text, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND widget_settings_id = @widget_settings_id AND revoked_at IS NULL
RETURNING id, contact_id;

-- name: RepointWidgetSessions :exec
-- RepointWidgetSessions moves a merged lead's live sessions to the contact
-- that survived.
UPDATE widget_sessions SET contact_id = @into_contact_id, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND revoked_at IS NULL;

-- name: ScrubWidgetSessionsForContact :exec
-- ScrubWidgetSessionsForContact clears what a contact's sessions remember
-- about them.
UPDATE widget_sessions SET
    current_page = NULL, recent_pages = '[]', context = '{}', user_agent = NULL, ip_prefix = NULL, origin = NULL,
    updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: UpdateWidgetSessionPage :exec
-- UpdateWidgetSessionPage records where the visitor is now.
UPDATE widget_sessions SET current_page = @current_page, recent_pages = @recent_pages, context = @context, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RaiseWidgetSessionBotScore :exec
-- RaiseWidgetSessionBotScore adds to a session's bot score, which never
-- passes one hundred.
UPDATE widget_sessions SET bot_score = LEAST(100, bot_score + @points::smallint), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListWidgetSessionsDueForPrune :many
-- ListWidgetSessionsDueForPrune returns a batch of sessions over the cutoff.
SELECT id, contact_id FROM widget_sessions
WHERE workspace_id = @workspace_id AND (revoked_at < @cutoff::timestamptz OR expires_at < @cutoff::timestamptz)
ORDER BY id LIMIT @batch;

-- name: DeleteWidgetSessions :execrows
-- DeleteWidgetSessions deletes the sessions with the ids.
DELETE FROM widget_sessions WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: CreateNotificationDestination :one
-- CreateNotificationDestination stores a new Slack destination of the
-- workspace or of one member.
INSERT INTO notification_destinations (
    workspace_id, scope, workspace_member_id, type, integration_installation_id, name, config, triggers, inbox_ids,
    include_ai_handled, show_previews, digests, digest_time, next_digest_at, created_by_member_id, created_at,
    updated_at
) VALUES (
    @workspace_id, @scope, sqlc.narg('workspace_member_id'), @type, @integration_installation_id, @name, @config,
    @triggers::text[], sqlc.narg('inbox_ids')::uuid[], @include_ai_handled, @show_previews, @digests::text[],
    @digest_time, sqlc.narg('next_digest_at'), sqlc.narg('created_by_member_id'), @now, @now
)
RETURNING *;

-- name: FindNotificationDestination :one
-- FindNotificationDestination returns one destination of the workspace.
SELECT * FROM notification_destinations WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListNotificationDestinations :many
-- ListNotificationDestinations returns the workspace's destinations and
-- the member's own, the workspace's first, oldest first.
SELECT * FROM notification_destinations
WHERE workspace_id = @workspace_id AND (scope = 'workspace' OR workspace_member_id = @member_id::uuid)
ORDER BY (scope = 'member'), created_at, id;

-- name: CountWorkspaceNotificationDestinations :one
-- CountWorkspaceNotificationDestinations counts the workspace's own
-- destinations, which are capped.
SELECT count(*) FROM notification_destinations WHERE workspace_id = @workspace_id AND scope = 'workspace';

-- name: ListActiveChannelDestinations :many
-- ListActiveChannelDestinations returns the workspace's working Slack
-- channel destinations.
SELECT * FROM notification_destinations
WHERE workspace_id = @workspace_id AND scope = 'workspace' AND type = 'slack_channel' AND status = 'active'
ORDER BY created_at, id;

-- name: FindMemberSlackDestination :one
-- FindMemberSlackDestination returns the member's working Slack direct
-- message destination.
SELECT * FROM notification_destinations
WHERE workspace_id = @workspace_id AND workspace_member_id = @member_id::uuid AND type = 'slack_dm' AND status = 'active';

-- name: UpdateNotificationDestination :one
-- UpdateNotificationDestination replaces what a destination hears about
-- and when its digests go.
UPDATE notification_destinations
SET name = @name, triggers = @triggers::text[], inbox_ids = sqlc.narg('inbox_ids')::uuid[],
    include_ai_handled = @include_ai_handled, show_previews = @show_previews, digests = @digests::text[],
    digest_time = @digest_time, next_digest_at = sqlc.narg('next_digest_at'), status = @status::text,
    last_error_code = CASE WHEN @status::text = 'active' THEN NULL ELSE last_error_code END,
    last_error = CASE WHEN @status::text = 'active' THEN NULL ELSE last_error END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteNotificationDestination :execrows
-- DeleteNotificationDestination removes one destination.
DELETE FROM notification_destinations WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetNotificationDestinationConfig :exec
-- SetNotificationDestinationConfig merges facts learned while sending,
-- such as a direct message's channel, into the destination's config.
UPDATE notification_destinations
SET config = config || @config, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordNotificationDestinationDelivery :exec
-- RecordNotificationDestinationDelivery records that a post went out.
UPDATE notification_destinations
SET last_delivered_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailNotificationDestination :exec
-- FailNotificationDestination stops a destination with the reason.
UPDATE notification_destinations
SET status = 'error', last_error_code = @code::text, last_error = @message::text, last_error_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailInstallationNotificationDestinations :execrows
-- FailInstallationNotificationDestinations stops every destination of an
-- installation that is gone.
UPDATE notification_destinations
SET status = 'error', last_error_code = @code::text, last_error = @message::text, last_error_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND integration_installation_id = @integration_installation_id
  AND status = 'active';

-- name: ScheduleNotificationDigest :exec
-- ScheduleNotificationDigest sets when the destination's next digest is
-- due.
UPDATE notification_destinations
SET next_digest_at = sqlc.narg('next_digest_at'), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListNotificationDigestsDue :many
-- ListNotificationDigestsDue returns the destinations whose digest is
-- due, across every workspace, the longest overdue first.
SELECT r.workspace_id::uuid AS workspace_id, r.notification_destination_id::uuid AS notification_destination_id
FROM notification_digests_due(@due_before::timestamptz, @max_rows::integer) AS r;

-- name: DigestStandup :many
-- DigestStandup returns each active member's open queue: the open
-- conversations they hold and how many of those wait on the team.
SELECT m.id AS member_id, u.name AS member_name,
    count(c.id)::bigint AS open_count,
    count(c.id) FILTER (WHERE c.waiting_since IS NOT NULL)::bigint AS waiting_count
FROM workspace_members AS m
JOIN users AS u ON u.id = m.user_id
LEFT JOIN conversations AS c ON c.workspace_id = m.workspace_id AND c.assignee_member_id = m.id
    AND c.status = 'open' AND c.deleted_at IS NULL AND NOT c.spam AND NOT c.is_test
WHERE m.workspace_id = @workspace_id AND m.status = 'active'
GROUP BY m.id, u.name
ORDER BY open_count DESC, u.name, m.id;

-- name: DigestUnassigned :one
-- DigestUnassigned counts the open conversations nobody holds.
SELECT count(*)::bigint FROM conversations
WHERE workspace_id = @workspace_id AND status = 'open' AND assignee_member_id IS NULL AND deleted_at IS NULL
  AND NOT spam AND NOT is_test;

-- name: DigestSummary :one
-- DigestSummary sums up a window: conversations opened and closed, the
-- median time to the first reply, how many the agent answered alone and
-- how many it handed to the team.
SELECT
    count(*) FILTER (WHERE created_at >= @since::timestamptz AND created_at < @until::timestamptz)::bigint AS opened,
    count(*) FILTER (WHERE closed_at >= @since::timestamptz AND closed_at < @until::timestamptz)::bigint AS closed,
    coalesce(percentile_cont(0.5) WITHIN GROUP (
        ORDER BY extract(epoch FROM first_reply_at - coalesce(first_customer_message_at, created_at))
    ) FILTER (WHERE first_reply_at >= @since::timestamptz AND first_reply_at < @until::timestamptz), 0)::float8
        AS median_first_reply_seconds,
    count(*) FILTER (
        WHERE created_at >= @since::timestamptz AND created_at < @until::timestamptz AND ai_reply_count > 0
          AND ai_handed_off_at IS NULL
    )::bigint AS agent_answered,
    count(*) FILTER (
        WHERE ai_handed_off_at >= @since::timestamptz AND ai_handed_off_at < @until::timestamptz
    )::bigint AS handed_off
FROM conversations
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND NOT spam AND NOT is_test;

-- name: DigestOldestUnanswered :one
-- DigestOldestUnanswered returns the open conversation that has waited
-- longest without any reply.
SELECT id, number, subject FROM conversations
WHERE workspace_id = @workspace_id AND status = 'open' AND first_reply_at IS NULL AND deleted_at IS NULL
  AND NOT spam AND NOT is_test
ORDER BY coalesce(first_customer_message_at, created_at), id
LIMIT 1;

-- name: DigestTopTags :many
-- DigestTopTags returns the tags most conversations started in the
-- window carry, the most used first.
SELECT t.name::text AS name, count(*)::bigint AS uses
FROM conversation_tag AS ct
JOIN tags AS t ON t.id = ct.tag_id
JOIN conversations AS c ON c.id = ct.conversation_id
WHERE ct.workspace_id = @workspace_id AND c.created_at >= @since::timestamptz AND c.created_at < @until::timestamptz
  AND c.deleted_at IS NULL AND NOT c.spam
GROUP BY t.name
ORDER BY uses DESC, t.name
LIMIT @row_limit::integer;

-- name: DigestSubjects :many
-- DigestSubjects returns the subjects of conversations started in the
-- window, newest first.
SELECT subject::text AS subject FROM conversations
WHERE workspace_id = @workspace_id AND created_at >= @since::timestamptz AND created_at < @until::timestamptz
  AND deleted_at IS NULL AND NOT spam AND NOT is_test AND subject IS NOT NULL AND subject <> ''
ORDER BY created_at DESC, id
LIMIT @row_limit::integer;

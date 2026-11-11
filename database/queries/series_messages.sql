-- name: AddSeriesRecipients :many
-- AddSeriesRecipients records who one visit of a series message step
-- reaches, each once per visit, and returns the rows to send.
INSERT INTO broadcast_recipients (workspace_id, broadcast_id, series_enrollment_id, series_step_id, contact_id,
    company_id, room_id, address, status, reason, reply_token, due_at, created_at, updated_at)
SELECT @workspace_id::uuid, @broadcast_id::uuid, @series_enrollment_id::uuid, @series_step_id::uuid,
    NULLIF(unnest(@contact_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid),
    NULLIF(unnest(@company_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid),
    NULLIF(unnest(@room_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid), unnest(@addresses::text[]),
    unnest(@statuses::text[]), NULLIF(unnest(@reasons::text[]), ''), unnest(@reply_tokens::text[]),
    @now::timestamptz, @now::timestamptz, @now::timestamptz
ON CONFLICT DO NOTHING
RETURNING id, status, reason;

-- name: SeriesStepDeliveries :one
-- SeriesStepDeliveries counts how the sends of one step visit stand:
-- still queued, gone out, or never going.
SELECT count(*) FILTER (WHERE status = 'queued') AS queued,
    count(*) FILTER (WHERE status IN ('sent', 'delivered')) AS sent,
    count(*) FILTER (WHERE status NOT IN ('queued', 'sent', 'delivered')) AS failed,
    coalesce(string_agg(DISTINCT reason, ',') FILTER (WHERE status NOT IN ('queued', 'sent', 'delivered')), '')::text AS reasons
FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND series_step_id = @series_step_id;

-- name: SeriesEnrollmentEngagement :one
-- SeriesEnrollmentEngagement says whether a traveler replied to or
-- unsubscribed from any message of this journey.
SELECT coalesce(bool_or(replied_at IS NOT NULL), false)::boolean AS replied,
    coalesce(bool_or(unsubscribed_at IS NOT NULL), false)::boolean AS unsubscribed
FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND series_enrollment_id = @series_enrollment_id;

-- name: SeriesNodeEngagement :one
-- SeriesNodeEngagement says whether the traveler opened, clicked or
-- replied to what one step of this journey sent them.
SELECT coalesce(bool_or(r.opened_at IS NOT NULL OR r.clicked_at IS NOT NULL), false)::boolean AS opened,
    coalesce(bool_or(r.clicked_at IS NOT NULL), false)::boolean AS clicked,
    coalesce(bool_or(r.replied_at IS NOT NULL), false)::boolean AS replied
FROM broadcast_recipients r
JOIN series_steps s ON s.id = r.series_step_id
WHERE r.workspace_id = @workspace_id AND r.series_enrollment_id = @series_enrollment_id AND s.node_id = @node_id;

-- name: WithdrawSeriesRecipients :execrows
-- WithdrawSeriesRecipients keeps the unsent messages of an enrollment
-- that left the series from ever going out.
UPDATE broadcast_recipients SET status = 'skipped', reason = 'left_series', updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND series_enrollment_id = @series_enrollment_id AND status = 'queued';

-- name: ListCompanySeriesContacts :many
-- ListCompanySeriesContacts returns the people of a company a message
-- step can pick from.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND company_id = @company_id AND deleted_at IS NULL AND blocked_at IS NULL
ORDER BY created_at, id LIMIT 200;

-- name: ListCompanySeriesRooms :many
-- ListCompanySeriesRooms returns the customer Slack channels of a
-- company.
SELECT cr.* FROM channel_rooms cr
JOIN channel_connections cc ON cc.id = cr.channel_connection_id
WHERE cr.workspace_id = @workspace_id AND cc.type = 'slack' AND cc.deleted_at IS NULL
  AND cr.mode IN ('shared', 'community') AND cr.company_id = @company_id
ORDER BY cr.name, cr.id;

-- name: SetSeriesContactAttribute :exec
-- SetSeriesContactAttribute writes one custom attribute of a contact.
UPDATE contacts SET custom_attributes = custom_attributes || jsonb_build_object(@key::text, @value::jsonb),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetSeriesCompanyAttribute :exec
-- SetSeriesCompanyAttribute writes one custom attribute of a company.
UPDATE companies SET custom_attributes = custom_attributes || jsonb_build_object(@key::text, @value::jsonb),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetSeriesTopicChoice :exec
-- SetSeriesTopicChoice subscribes a contact to a topic, or takes them
-- off it.
INSERT INTO contact_topic_subscriptions (workspace_id, contact_id, topic_id, subscribed, source, updated_at)
VALUES (@workspace_id, @contact_id, @topic_id, @subscribed, 'teammate', @now::timestamptz)
ON CONFLICT (workspace_id, contact_id, topic_id) DO UPDATE SET subscribed = EXCLUDED.subscribed,
    source = EXCLUDED.source, updated_at = EXCLUDED.updated_at;

-- name: ContactEventSince :one
-- ContactEventSince says whether the contact did the event since the
-- time.
SELECT EXISTS (
    SELECT 1 FROM contact_event_summaries s
    JOIN event_definitions d ON d.id = s.event_definition_id
    WHERE s.workspace_id = @workspace_id AND s.contact_id = ANY (@contact_ids::uuid[]) AND d.name = @name
      AND s.last_at >= @since::timestamptz
)::boolean;

-- name: FindMemberContact :one
-- FindMemberContact returns the contact with a teammate's email, so the
-- teammate can try a series on themselves.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND lower(email::text) = lower(@email::text) AND deleted_at IS NULL
ORDER BY created_at LIMIT 1;

-- name: ListDueSeriesRecipients :many
-- ListDueSeriesRecipients returns the series messages still held after
-- their time, whose retry may have been lost.
SELECT id FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND series_step_id IS NOT NULL AND status = 'queued'
  AND due_at < @before::timestamptz
ORDER BY due_at LIMIT 1000;

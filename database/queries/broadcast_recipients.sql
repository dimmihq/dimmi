-- name: AddBroadcastRecipients :execrows
-- AddBroadcastRecipients freezes one page of the audience into
-- recipients. A contact or room already frozen is left as it is, so
-- freezing can resume after a crash. A nil id stands for none.
INSERT INTO broadcast_recipients (workspace_id, broadcast_id, contact_id, company_id, room_id, address, status,
    reason, reply_token, due_at, created_at, updated_at)
SELECT @workspace_id::uuid, @broadcast_id::uuid, NULLIF(unnest(@contact_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid), NULLIF(unnest(@company_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid),
    NULLIF(unnest(@room_ids::uuid[]), '00000000-0000-0000-0000-000000000000'::uuid), unnest(@addresses::text[]), unnest(@statuses::text[]),
    NULLIF(unnest(@reasons::text[]), ''), unnest(@reply_tokens::text[]), unnest(@due_ats::timestamptz[]),
    @now::timestamptz, @now::timestamptz
ON CONFLICT DO NOTHING;

-- name: LeaseDueBroadcastRecipients :many
-- LeaseDueBroadcastRecipients takes the next due recipients and moves
-- them out of reach for the lease, so a lost send job comes back on its
-- own while a running one is not sent twice.
UPDATE broadcast_recipients SET due_at = @lease_until::timestamptz, attempts = attempts + 1, updated_at = @now::timestamptz
WHERE id IN (
    SELECT q.id FROM broadcast_recipients q
    WHERE q.workspace_id = @workspace_id AND q.broadcast_id = @broadcast_id AND q.status = 'queued' AND q.due_at <= @now::timestamptz
    ORDER BY q.due_at, q.id
    LIMIT @row_limit
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: NextBroadcastRecipientDue :one
-- NextBroadcastRecipientDue returns when the next queued recipient is
-- due, or null when none is queued.
SELECT min(due_at)::timestamptz FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND status = 'queued';

-- name: FindBroadcastRecipient :one
-- FindBroadcastRecipient returns one recipient.
SELECT * FROM broadcast_recipients WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkBroadcastRecipientSent :one
-- MarkBroadcastRecipientSent records that a recipient's message went out
-- and was accepted; a bounce later takes the delivery back.
UPDATE broadcast_recipients SET status = 'sent', reason = NULL, delivered_at = @now::timestamptz,
    message_id_header = sqlc.narg(message_id_header),
    outbound_email_id = sqlc.narg(outbound_email_id), external_ref = sqlc.narg(external_ref),
    conversation_id = COALESCE(sqlc.narg(conversation_id), conversation_id), sent_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status = 'queued'
RETURNING *;

-- name: MarkBroadcastRecipientFailed :one
-- MarkBroadcastRecipientFailed records why a recipient was not reached.
UPDATE broadcast_recipients SET status = @status, reason = @reason, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('queued', 'sent', 'delivered')
RETURNING *;

-- name: RetryBroadcastRecipientAt :exec
-- RetryBroadcastRecipientAt puts a recipient back in the queue for later.
UPDATE broadcast_recipients SET due_at = @due_at::timestamptz, reason = sqlc.narg(reason), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status = 'queued';

-- name: SkipQueuedBroadcastRecipients :execrows
-- SkipQueuedBroadcastRecipients takes everyone still queued off a
-- stopped broadcast.
UPDATE broadcast_recipients SET status = 'skipped', reason = @reason, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND status = 'queued';

-- name: MarkBroadcastRecipientDelivered :exec
-- MarkBroadcastRecipientDelivered records the first sign a message
-- arrived.
UPDATE broadcast_recipients SET status = CASE WHEN status = 'sent' THEN 'delivered' ELSE status END,
    delivered_at = COALESCE(delivered_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('sent', 'delivered');

-- name: MarkBroadcastRecipientOpened :one
-- MarkBroadcastRecipientOpened records the first open; an open proves
-- delivery too.
UPDATE broadcast_recipients SET status = CASE WHEN status = 'sent' THEN 'delivered' ELSE status END,
    delivered_at = COALESCE(delivered_at, @now::timestamptz), opened_at = COALESCE(opened_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('sent', 'delivered')
RETURNING *;

-- name: MarkBroadcastRecipientClicked :one
-- MarkBroadcastRecipientClicked records the first click; a click proves
-- the open.
UPDATE broadcast_recipients SET status = CASE WHEN status = 'sent' THEN 'delivered' ELSE status END,
    delivered_at = COALESCE(delivered_at, @now::timestamptz), opened_at = COALESCE(opened_at, @now::timestamptz),
    clicked_at = COALESCE(clicked_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('sent', 'delivered')
RETURNING *;

-- name: MarkBroadcastRecipientReplied :one
-- MarkBroadcastRecipientReplied records the conversation a reply opened.
UPDATE broadcast_recipients SET status = CASE WHEN status = 'sent' THEN 'delivered' ELSE status END,
    delivered_at = COALESCE(delivered_at, @now::timestamptz), opened_at = COALESCE(opened_at, @now::timestamptz),
    replied_at = COALESCE(replied_at, @now::timestamptz), conversation_id = COALESCE(conversation_id, sqlc.narg(conversation_id)),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CountBroadcastRecipientReaction :exec
-- CountBroadcastRecipientReaction counts a reaction added or taken away.
UPDATE broadcast_recipients SET reactions = greatest(reactions + @delta::int, 0),
    reacted_at = COALESCE(reacted_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkBroadcastRecipientUnsubscribed :exec
-- MarkBroadcastRecipientUnsubscribed records that the recipient left.
UPDATE broadcast_recipients SET unsubscribed_at = COALESCE(unsubscribed_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkBroadcastRecipientConverted :exec
-- MarkBroadcastRecipientConverted records that the recipient reached the
-- goal.
UPDATE broadcast_recipients SET converted_at = COALESCE(converted_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetBroadcastRecipientExternalRef :exec
-- SetBroadcastRecipientExternalRef records where an edited message now
-- lives, or that it was recalled.
UPDATE broadcast_recipients SET external_ref = sqlc.narg(external_ref), status = @status, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindBroadcastRecipientByReplyToken :one
-- FindBroadcastRecipientByReplyToken returns the recipient a reply
-- token names.
SELECT * FROM broadcast_recipients WHERE workspace_id = @workspace_id AND reply_token = @reply_token;

-- name: FindBroadcastRecipientByMessageID :one
-- FindBroadcastRecipientByMessageID returns the recipient an email's
-- Message-ID was sent to.
SELECT * FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND message_id_header = ANY (@message_ids::text[])
LIMIT 1;

-- name: FindBroadcastRecipientByRef :one
-- FindBroadcastRecipientByRef returns the recipient of a Slack post.
SELECT * FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND room_id = @room_id AND external_ref = @external_ref;

-- name: FindBroadcastRecipientByOutboundEmail :one
-- FindBroadcastRecipientByOutboundEmail returns the recipient a sent
-- email was for.
SELECT * FROM broadcast_recipients WHERE workspace_id = @workspace_id AND outbound_email_id = @outbound_email_id;

-- name: FindBroadcastRecipientByConversation :one
-- FindBroadcastRecipientByConversation returns the recipient whose
-- broadcast started the conversation.
SELECT * FROM broadcast_recipients WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id LIMIT 1;

-- name: LastBroadcastSentToContact :one
-- LastBroadcastSentToContact returns when the contact last got a
-- broadcast that counts toward the frequency cap.
SELECT max(r.sent_at)::timestamptz FROM broadcast_recipients r
JOIN broadcasts b ON b.id = r.broadcast_id
WHERE r.workspace_id = @workspace_id AND r.contact_id = @contact_id AND r.sent_at IS NOT NULL
  AND b.service = false AND b.skip_frequency_cap = false AND r.broadcast_id <> @broadcast_id;

-- name: TallyBroadcastRecipients :one
-- TallyBroadcastRecipients counts what happened to a broadcast's
-- recipients.
SELECT
    count(*) AS total,
    count(*) FILTER (WHERE status = 'queued') AS queued,
    count(*) FILTER (WHERE sent_at IS NOT NULL) AS sent,
    count(*) FILTER (WHERE delivered_at IS NOT NULL AND status NOT IN ('bounced', 'complained', 'failed')) AS delivered,
    count(*) FILTER (WHERE opened_at IS NOT NULL) AS opened,
    count(*) FILTER (WHERE clicked_at IS NOT NULL) AS clicked,
    count(*) FILTER (WHERE replied_at IS NOT NULL) AS replied,
    count(*) FILTER (WHERE reacted_at IS NOT NULL) AS reacted,
    coalesce(sum(reactions), 0)::bigint AS reactions,
    count(*) FILTER (WHERE unsubscribed_at IS NOT NULL) AS unsubscribed,
    count(*) FILTER (WHERE converted_at IS NOT NULL) AS converted,
    count(*) FILTER (WHERE status = 'failed') AS failed,
    count(*) FILTER (WHERE status = 'bounced') AS bounced,
    count(*) FILTER (WHERE status = 'complained') AS complained,
    count(*) FILTER (WHERE status = 'suppressed') AS suppressed,
    count(*) FILTER (WHERE status = 'skipped') AS skipped,
    count(*) FILTER (WHERE status = 'recalled') AS recalled
FROM broadcast_recipients WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id;

-- name: CountBroadcastSkipReasons :many
-- CountBroadcastSkipReasons counts why recipients were left out.
SELECT coalesce(reason, '')::text AS reason, count(*) AS n FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id
  AND status IN ('skipped', 'suppressed', 'failed', 'bounced', 'complained')
GROUP BY reason ORDER BY n DESC;

-- name: ListBroadcastRecipientPage :many
-- ListBroadcastRecipientPage returns a page of recipients with who they
-- are, narrowed by status and company.
SELECT r.*, coalesce(c.name, '')::text AS contact_name, coalesce(c.email, '')::text AS contact_email,
    coalesce(co.name, '')::text AS company_name, coalesce(cr.name, '')::text AS room_name
FROM broadcast_recipients r
LEFT JOIN contacts c ON c.id = r.contact_id
LEFT JOIN companies co ON co.id = r.company_id
LEFT JOIN channel_rooms cr ON cr.id = r.room_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id
  AND (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status)::text
      OR (sqlc.narg(status)::text = 'opened' AND r.opened_at IS NOT NULL)
      OR (sqlc.narg(status)::text = 'clicked' AND r.clicked_at IS NOT NULL)
      OR (sqlc.narg(status)::text = 'replied' AND r.replied_at IS NOT NULL)
      OR (sqlc.narg(status)::text = 'unsubscribed' AND r.unsubscribed_at IS NOT NULL))
  AND (sqlc.narg(company_id)::uuid IS NULL OR r.company_id = sqlc.narg(company_id)::uuid)
ORDER BY r.updated_at DESC, r.id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: BroadcastCompanyReport :many
-- BroadcastCompanyReport rolls a broadcast's results up per company.
SELECT r.company_id, coalesce(co.name, '')::text AS company_name,
    count(*) AS total,
    count(*) FILTER (WHERE r.delivered_at IS NOT NULL AND r.status NOT IN ('bounced', 'complained', 'failed'))
        AS delivered,
    count(*) FILTER (WHERE r.opened_at IS NOT NULL) AS opened,
    count(*) FILTER (WHERE r.clicked_at IS NOT NULL) AS clicked,
    count(*) FILTER (WHERE r.replied_at IS NOT NULL) AS replied,
    coalesce(sum(r.reactions), 0)::bigint AS reactions,
    count(*) FILTER (WHERE r.status IN ('failed', 'bounced', 'complained')) AS failed
FROM broadcast_recipients r
LEFT JOIN companies co ON co.id = r.company_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id AND r.company_id IS NOT NULL
GROUP BY r.company_id, co.name
ORDER BY replied DESC, opened DESC, total DESC
LIMIT 200;

-- name: ListDeliveredBroadcastRecipients :many
-- ListDeliveredBroadcastRecipients returns recipients reached since a
-- time who have not converted, for goal checks.
SELECT * FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND sent_at IS NOT NULL AND converted_at IS NULL
  AND contact_id IS NOT NULL AND id > @after
ORDER BY id LIMIT 500;

-- name: ListSentSlackBroadcastRecipients :many
-- ListSentSlackBroadcastRecipients returns the Slack posts a broadcast
-- made, to edit or recall them.
SELECT * FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id AND room_id IS NOT NULL
  AND external_ref IS NOT NULL AND status IN ('sent', 'delivered')
ORDER BY id;

-- name: SuppressedEmailsAmong :many
-- SuppressedEmailsAmong returns which of the addresses must not be
-- mailed now.
SELECT DISTINCT lower(email::text)::text AS email FROM email_suppressions
WHERE workspace_id = @workspace_id AND lower(email::text) = ANY (@emails::text[]) AND scope = 'all'
  AND (expires_at IS NULL OR expires_at > @now::timestamptz);

-- name: MessengerContactsAmong :many
-- MessengerContactsAmong returns which of the contacts have used the
-- Messenger, so a message there can reach them.
SELECT DISTINCT contact_id FROM widget_sessions
WHERE workspace_id = @workspace_id AND contact_id = ANY (@contact_ids::uuid[]);

-- name: TopicChoicesAmong :many
-- TopicChoicesAmong returns the choices the contacts made about a topic.
SELECT contact_id, subscribed FROM contact_topic_subscriptions
WHERE workspace_id = @workspace_id AND topic_id = @topic_id AND contact_id = ANY (@contact_ids::uuid[]);

-- name: OptedOutAmong :many
-- OptedOutAmong returns which of the contacts left all broadcasts.
SELECT contact_id FROM broadcast_opt_outs
WHERE workspace_id = @workspace_id AND contact_id = ANY (@contact_ids::uuid[]);

-- name: LastBroadcastSentAmong :many
-- LastBroadcastSentAmong returns when each of the contacts last got a
-- broadcast that counts toward the frequency cap.
SELECT r.contact_id, max(r.sent_at)::timestamptz AS last_sent FROM broadcast_recipients r
JOIN broadcasts b ON b.id = r.broadcast_id
WHERE r.workspace_id = @workspace_id AND r.contact_id = ANY (@contact_ids::uuid[]) AND r.sent_at IS NOT NULL
  AND b.service = false AND b.skip_frequency_cap = false AND r.broadcast_id <> @broadcast_id
GROUP BY r.contact_id;

-- name: ListBroadcastRooms :many
-- ListBroadcastRooms returns the Slack channels a broadcast can post to:
-- the ones picked, or every customer channel when none are.
SELECT cr.* FROM channel_rooms cr
JOIN channel_connections cc ON cc.id = cr.channel_connection_id
WHERE cr.workspace_id = @workspace_id AND cc.type = 'slack' AND cc.deleted_at IS NULL
  AND cr.mode IN ('shared', 'community')
  AND (cardinality(@room_ids::uuid[]) = 0 OR cr.id = ANY (@room_ids::uuid[]))
ORDER BY cr.name, cr.id;

-- name: FindContactWidgetConnection :one
-- FindContactWidgetConnection returns the Messenger channel the contact
-- used most recently.
SELECT cc.* FROM widget_sessions s
JOIN widget_settings ws ON ws.id = s.widget_settings_id
JOIN channel_connections cc ON cc.id = ws.channel_connection_id
WHERE s.workspace_id = @workspace_id AND s.contact_id = @contact_id AND s.test = false AND cc.deleted_at IS NULL
ORDER BY s.created_at DESC
LIMIT 1;

-- name: SetConversationAIOff :exec
-- SetConversationAIOff keeps the AI agent out of a conversation a
-- broadcast started, when the broadcast leaves replies to the team.
UPDATE conversations SET ai_state = 'handed_off', updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: LastBroadcastRecipientChange :one
-- LastBroadcastRecipientChange returns when any of a broadcast's
-- recipients last changed.
SELECT coalesce(max(updated_at), 'epoch'::timestamptz)::timestamptz FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND broadcast_id = @broadcast_id;

-- name: ConvertBroadcastRecipients :execrows
-- ConvertBroadcastRecipients marks converted every recipient who sent the
-- goal's event after the broadcast reached them and within the window.
UPDATE broadcast_recipients r SET converted_at = s.last_at, updated_at = @now::timestamptz
FROM contact_event_summaries s
JOIN event_definitions e ON e.id = s.event_definition_id
WHERE r.workspace_id = @workspace_id AND r.broadcast_id = @broadcast_id AND r.converted_at IS NULL
  AND r.sent_at IS NOT NULL AND s.workspace_id = r.workspace_id AND s.contact_id = r.contact_id
  AND e.name = @event::text AND s.last_at > r.sent_at
  AND s.last_at <= r.sent_at + make_interval(days => @days::int);

-- name: StampBroadcastConversationClosed :one
-- StampBroadcastConversationClosed dates the close of a conversation a
-- broadcast opened closed, so an answer reopens it rather than starting
-- another.
UPDATE conversations SET closed_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status = 'closed'
RETURNING *;

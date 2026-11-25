-- name: EndChannelConnection :one
-- EndChannelConnection disconnects a connection whose app was removed:
-- its secrets go at once and it remembers when it ended, the first time
-- only, so what it brought in is deleted once the grace period is over.
UPDATE channel_connections
SET status = 'disconnected', last_error = sqlc.narg(last_error), last_error_at = @now::timestamptz,
    credentials_encrypted = NULL, ended_at = coalesce(ended_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListEndedSlackConnections :many
-- ListEndedSlackConnections returns the workspace's Slack connections
-- that ended before the cutoff and still hold what Slack brought in,
-- the oldest first.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND type = 'slack' AND status = 'disconnected'
  AND ended_at <= @cutoff::timestamptz AND data_purged_at IS NULL
ORDER BY ended_at, id;

-- name: ClaimEndedSlackConnection :one
-- ClaimEndedSlackConnection retires an ended connection as its purge
-- starts, unless it was connected again meanwhile. Retired, it is never
-- brought back: Slack connected again makes a new connection.
UPDATE channel_connections
SET deleted_at = coalesce(deleted_at, @now::timestamptz), external_id = NULL, credentials_encrypted = NULL,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND type = 'slack' AND status = 'disconnected'
  AND ended_at <= @cutoff::timestamptz AND data_purged_at IS NULL
RETURNING *;

-- name: FinishSlackConnectionPurge :one
-- FinishSlackConnectionPurge forgets what the connection knew of its
-- Slack team and app and records that its data is gone.
UPDATE channel_connections
SET name = 'Slack',
    config = jsonb_strip_nulls(jsonb_build_object(
        'mode', config -> 'mode', 'inbound_transport', config -> 'inbound_transport'
    )),
    data_purged_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND data_purged_at IS NULL
RETURNING *;

-- name: ListConnectionConversations :many
-- ListConnectionConversations returns a batch of the conversations that
-- came in on the connection, after the one given.
SELECT id FROM conversations
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id AND id > @after::uuid
ORDER BY id
LIMIT @row_limit;

-- name: ListConversationContacts :many
-- ListConversationContacts returns the customers of the conversations,
-- the placeholder left out.
SELECT DISTINCT c.contact_id FROM conversations c
JOIN contacts k ON k.id = c.contact_id AND k.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.id = ANY (@ids::uuid[]) AND NOT k.placeholder;

-- name: ListSlackOnlyContacts :many
-- ListSlackOnlyContacts returns which of the contacts exist only because
-- of the Slack connection: known by a Slack user alone, never named by
-- the workspace's own systems, and with no conversation anywhere else.
SELECT k.id FROM contacts k
WHERE k.workspace_id = @workspace_id AND k.id = ANY (@ids::uuid[]) AND NOT k.placeholder AND k.external_id IS NULL
  AND EXISTS (
      SELECT 1 FROM contact_identities i
      WHERE i.workspace_id = k.workspace_id AND i.contact_id = k.id AND i.type = 'slack_user'
  )
  AND NOT EXISTS (
      SELECT 1 FROM contact_identities i
      WHERE i.workspace_id = k.workspace_id AND i.contact_id = k.id AND i.type <> 'slack_user'
  )
  AND NOT EXISTS (
      SELECT 1 FROM conversations o
      WHERE o.workspace_id = k.workspace_id AND o.contact_id = k.id
        AND o.channel_connection_id <> @channel_connection_id
  )
ORDER BY k.id;

-- name: ListSlackConversationFiles :many
-- ListSlackConversationFiles returns the stored files of the messages
-- Slack brought into the conversations: the customer's, and a
-- teammate's written in Slack. A reply written in dimmi keeps its files.
SELECT a.id, a.disk, a.path FROM attachments a
JOIN messages m ON m.id = a.message_id AND m.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id AND m.conversation_id = ANY (@ids::uuid[])
  AND (m.author_type = 'contact' OR m.dedupe_key IS NOT NULL);

-- name: DeleteConversationAttachments :execrows
-- DeleteConversationAttachments deletes the files' rows of the messages
-- Slack brought into the conversations.
DELETE FROM attachments a
USING messages m
WHERE a.workspace_id = @workspace_id AND m.id = a.message_id AND m.workspace_id = a.workspace_id
  AND m.conversation_id = ANY (@ids::uuid[])
  AND (m.author_type = 'contact' OR m.dedupe_key IS NOT NULL);

-- name: DeleteConversationReactions :execrows
DELETE FROM message_reactions r
USING messages m
WHERE r.workspace_id = @workspace_id AND m.id = r.message_id AND m.workspace_id = r.workspace_id
  AND m.conversation_id = ANY (@ids::uuid[]);

-- name: WipeConversationMessages :execrows
-- WipeConversationMessages empties the messages Slack brought into the
-- conversations, the customer's and a teammate's written in Slack, and
-- forgets where on the chat app every message was. A reply written in
-- dimmi keeps its words: it is the team's own work, not Slack's data.
-- The rows stay, so the timeline and the counts built on it hold.
UPDATE messages
SET body = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN NULL ELSE body END,
    body_full = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN NULL ELSE body_full END,
    body_html = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN NULL ELSE body_html END,
    metadata = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN '{}' ELSE metadata END,
    deleted_at = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL
        THEN coalesce(deleted_at, @now::timestamptz) ELSE deleted_at END,
    channel_message_id = NULL, dedupe_key = NULL, delivery_error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[])
  AND (
      channel_message_id IS NOT NULL OR dedupe_key IS NOT NULL OR delivery_error IS NOT NULL
      OR (author_type = 'contact'
          AND (body IS NOT NULL OR body_full IS NOT NULL OR body_html IS NOT NULL OR metadata <> '{}'))
  );

-- name: EmptyPurgedConversations :execrows
-- EmptyPurgedConversations clears the subject, the source, the thread
-- and the agent's memory of the conversations and gives them to the
-- placeholder, as an erased customer's are.
UPDATE conversations
SET contact_id = @placeholder_id, subject = NULL, source = '{}', ai_memory = NULL, external_thread_key = NULL,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[])
  AND (
      contact_id <> @placeholder_id OR subject IS NOT NULL OR source <> '{}' OR ai_memory IS NOT NULL
      OR external_thread_key IS NOT NULL
  );

-- name: HandOverContactsConversations :execrows
-- HandOverContactsConversations gives every conversation of the contacts
-- to the placeholder, so the contacts can be deleted.
UPDATE conversations SET contact_id = @placeholder_id, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = ANY (@contact_ids::uuid[]);

-- name: DeleteSlackOnlyContacts :execrows
DELETE FROM contacts WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND NOT placeholder;

-- name: DeleteContactsSlackIdentities :execrows
-- DeleteContactsSlackIdentities forgets which Slack users the contacts
-- are, unless another Slack connection that still holds its data has
-- conversations with them.
DELETE FROM contact_identities i
WHERE i.workspace_id = @workspace_id AND i.type = 'slack_user' AND i.contact_id = ANY (@contact_ids::uuid[])
  AND NOT EXISTS (
      SELECT 1 FROM conversations c
      JOIN channel_connections cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
      WHERE c.workspace_id = i.workspace_id AND c.contact_id = i.contact_id AND cc.type = 'slack'
        AND cc.id <> @channel_connection_id AND cc.data_purged_at IS NULL
  );

-- name: ListConnectionStrayMessages :many
-- ListConnectionStrayMessages returns a batch of the messages that
-- travelled on the connection and still hold something of Slack's,
-- wherever their conversation came from, such as after a merge.
SELECT id FROM messages
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND (
      channel_message_id IS NOT NULL OR dedupe_key IS NOT NULL OR delivery_error IS NOT NULL
      OR (author_type = 'contact'
          AND (body IS NOT NULL OR body_full IS NOT NULL OR body_html IS NOT NULL OR metadata <> '{}'))
  )
ORDER BY id
LIMIT @row_limit;

-- name: ListSlackMessageFiles :many
-- ListSlackMessageFiles returns the stored files of those of the
-- messages that Slack brought in.
SELECT a.id, a.disk, a.path FROM attachments a
JOIN messages m ON m.id = a.message_id AND m.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id AND a.message_id = ANY (@message_ids::uuid[])
  AND (m.author_type = 'contact' OR m.dedupe_key IS NOT NULL);

-- name: DeleteMessagesAttachments :execrows
-- DeleteMessagesAttachments deletes the files' rows of those of the
-- messages that Slack brought in.
DELETE FROM attachments a
USING messages m
WHERE a.workspace_id = @workspace_id AND a.message_id = ANY (@message_ids::uuid[])
  AND m.id = a.message_id AND m.workspace_id = a.workspace_id
  AND (m.author_type = 'contact' OR m.dedupe_key IS NOT NULL);

-- name: WipeMessages :execrows
-- WipeMessages empties the messages, as WipeConversationMessages does.
UPDATE messages
SET body = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN NULL ELSE body END,
    body_full = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN NULL ELSE body_full END,
    body_html = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN NULL ELSE body_html END,
    metadata = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL THEN '{}' ELSE metadata END,
    deleted_at = CASE WHEN author_type = 'contact' OR dedupe_key IS NOT NULL
        THEN coalesce(deleted_at, @now::timestamptz) ELSE deleted_at END,
    channel_message_id = NULL, dedupe_key = NULL, delivery_error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: DeleteConnectionInboundEvents :execrows
-- DeleteConnectionInboundEvents deletes a batch of the deliveries kept
-- from the connection.
DELETE FROM channel_inbound_events
WHERE id IN (
    SELECT e.id FROM channel_inbound_events e
    WHERE e.workspace_id = @workspace_id AND e.channel_connection_id = @channel_connection_id
    LIMIT @row_limit
);

-- name: DeleteConnectionMessageRefs :execrows
-- DeleteConnectionMessageRefs deletes a batch of the chat app's ids of
-- the connection's messages.
DELETE FROM channel_message_refs
WHERE id IN (
    SELECT r.id FROM channel_message_refs r
    WHERE r.workspace_id = @workspace_id AND r.channel_connection_id = @channel_connection_id
    LIMIT @row_limit
);

-- name: DeleteConnectionConversationPosts :execrows
-- DeleteConnectionConversationPosts deletes a batch of the triage posts,
-- discussions and drafts the team's Slack held for conversations.
DELETE FROM channel_conversation_posts
WHERE id IN (
    SELECT p.id FROM channel_conversation_posts p
    WHERE p.workspace_id = @workspace_id AND p.channel_connection_id = @channel_connection_id
    LIMIT @row_limit
);

-- name: DeleteConnectionMemberLinks :execrows
DELETE FROM channel_member_links
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id;

-- name: DeleteConnectionOperatorThreads :execrows
-- DeleteConnectionOperatorThreads deletes the talks with Operator that
-- happened in the connection's Slack, and with them which Slack threads
-- they were.
DELETE FROM operator_threads t
USING operator_slack_threads s
WHERE t.workspace_id = @workspace_id AND s.workspace_id = t.workspace_id AND s.operator_thread_id = t.id
  AND s.channel_connection_id = @channel_connection_id;

-- name: ScrubConnectionBroadcastRecipients :execrows
-- ScrubConnectionBroadcastRecipients forgets where in Slack broadcasts
-- were posted to the connection's rooms. The rows stay so a broadcast's
-- counts do not change.
UPDATE broadcast_recipients b
SET address = '', external_ref = NULL, updated_at = @now
FROM channel_rooms r
WHERE b.workspace_id = @workspace_id AND r.workspace_id = b.workspace_id AND r.id = b.room_id
  AND r.channel_connection_id = @channel_connection_id AND (b.address <> '' OR b.external_ref IS NOT NULL);

-- name: DeleteConnectionRooms :execrows
DELETE FROM channel_rooms WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id;

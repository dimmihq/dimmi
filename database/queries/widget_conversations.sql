-- name: ListWidgetConversations :many
-- ListWidgetConversations returns a page of a visitor's conversations on a
-- widget, latest activity first: those started on the widget, those of
-- another channel in the widget's inbox, and for a signed-in visitor their
-- imported history. A page after the first passes the activity time and id
-- of the last conversation it saw.
SELECT sqlc.embed(c), cc.type AS channel_type,
    (SELECT count(*) FROM messages m
     WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
       AND m.author_type IN ('member', 'agent') AND m.read_at IS NULL AND m.deleted_at IS NULL)::bigint AS unread_count
FROM conversations c
JOIN channel_connections cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id AND c.deleted_at IS NULL
  AND (
      c.source->>'widget_id' = @widget_id::text
      OR (cc.type IN ('email', 'api') AND c.inbox_id = @inbox_id::uuid)
      OR (@include_imported::boolean AND cc.type = 'import')
  )
  AND (cardinality(@statuses::text[]) = 0 OR c.status = ANY(@statuses::text[]))
  AND (
      sqlc.narg(before_id)::uuid IS NULL
      OR (coalesce(c.last_message_at, c.created_at), c.id)
         < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid)
  )
ORDER BY coalesce(c.last_message_at, c.created_at) DESC, c.id DESC
LIMIT @row_limit::bigint;

-- name: FindWidgetConversation :one
-- FindWidgetConversation returns one conversation of the visitor's on the
-- widget, by the same rule as the list.
SELECT sqlc.embed(c), cc.type AS channel_type,
    (SELECT count(*) FROM messages m
     WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
       AND m.author_type IN ('member', 'agent') AND m.read_at IS NULL AND m.deleted_at IS NULL)::bigint AS unread_count
FROM conversations c
JOIN channel_connections cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.id = @id AND c.contact_id = @contact_id AND c.deleted_at IS NULL
  AND (
      c.source->>'widget_id' = @widget_id::text
      OR (cc.type IN ('email', 'api') AND c.inbox_id = @inbox_id::uuid)
      OR (@include_imported::boolean AND cc.type = 'import')
  );

-- name: SummarizeWidgetConversations :one
-- SummarizeWidgetConversations counts the team's unread messages to the
-- visitor on the widget and lists their open conversations.
SELECT
    coalesce(sum((SELECT count(*) FROM messages m
        WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
          AND m.author_type IN ('member', 'agent') AND m.read_at IS NULL AND m.deleted_at IS NULL)), 0)::bigint
        AS unread_count,
    coalesce(array_agg(c.id ORDER BY c.created_at) FILTER (WHERE c.status <> 'closed'), '{}')::uuid[]
        AS open_ids
FROM conversations c
JOIN channel_connections cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id AND c.deleted_at IS NULL
  AND (
      c.source->>'widget_id' = @widget_id::text
      OR (cc.type IN ('email', 'api') AND c.inbox_id = @inbox_id::uuid)
  );

-- name: LatestWidgetMessages :many
-- LatestWidgetMessages returns the latest message a visitor may see in each
-- of the conversations.
SELECT DISTINCT ON (m.conversation_id) m.*
FROM messages m
WHERE m.workspace_id = @workspace_id AND m.conversation_id = ANY(@conversation_ids::uuid[]) AND m.type = 'message'
ORDER BY m.conversation_id, m.created_at DESC, m.id DESC;

-- name: ListWidgetMessages :many
-- ListWidgetMessages returns a page of the messages a visitor may see in a
-- conversation, newest first: deleted ones come back too, to keep the order,
-- and notes and events never. A page passes the time and id of the oldest
-- message it saw to scroll back, or of the newest to catch up.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message'
  AND (
      sqlc.narg(before_id)::uuid IS NULL
      OR (created_at, id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid)
  )
  AND (
      sqlc.narg(after_id)::uuid IS NULL
      OR (created_at, id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY
    CASE WHEN sqlc.narg(after_id)::uuid IS NULL THEN created_at END DESC,
    CASE WHEN sqlc.narg(after_id)::uuid IS NULL THEN id END DESC,
    created_at, id
LIMIT @row_limit::bigint;

-- name: FindWidgetMessage :one
-- FindWidgetMessage returns a message a visitor may see in the conversation.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id = @id AND type = 'message';

-- name: MarkWidgetMessagesRead :many
-- MarkWidgetMessagesRead marks the team's messages up to and including the
-- one given as read by the visitor, and returns those it marked.
UPDATE messages SET read_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message'
  AND author_type IN ('member', 'agent') AND read_at IS NULL AND deleted_at IS NULL
  AND (created_at, id) <= (@up_to_at::timestamptz, @up_to_id::uuid)
RETURNING id;

-- name: ListWidgetMemberAuthors :many
-- ListWidgetMemberAuthors returns the names and avatars of the members, as a
-- visitor sees them.
SELECT m.id, u.name, u.avatar_path
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.id = ANY(@member_ids::uuid[]);

-- name: StampWidgetMessage :exec
-- StampWidgetMessage records which browser message a stored message was, so
-- a retry of it is recognised and the team sees the page it was sent from.
UPDATE messages SET metadata = metadata || jsonb_build_object('client', @client::jsonb), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkWidgetConversationStart :exec
-- MarkWidgetConversationStart records that a conversation came from a test
-- session, and which earlier conversation it continues.
UPDATE conversations SET is_test = @is_test, previous_conversation_id = sqlc.narg(previous_conversation_id),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListWidgetUploads :many
-- ListWidgetUploads returns the session's uploads with the ids that no
-- message took yet and whose bytes arrived.
SELECT * FROM attachments
WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]) AND uploaded_by_type = 'widget_session'
  AND uploaded_by_id = @session_id AND message_id IS NULL AND uploaded_at IS NOT NULL;

-- name: ListWidgetMessageChanges :many
-- ListWidgetMessageChanges returns the visitor's messages on the widget that
-- changed after the cursor, oldest change first: new, edited or deleted.
SELECT m.* FROM messages m
JOIN conversations c ON c.id = m.conversation_id AND c.workspace_id = m.workspace_id
JOIN channel_connections cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
WHERE m.workspace_id = @workspace_id AND c.contact_id = @contact_id AND c.deleted_at IS NULL AND m.type = 'message'
  AND (
      c.source->>'widget_id' = @widget_id::text
      OR (cc.type IN ('email', 'api') AND c.inbox_id = @inbox_id::uuid)
  )
  AND (m.updated_at, m.id) > (@since_at::timestamptz, @since_id::uuid)
ORDER BY m.updated_at, m.id
LIMIT @row_limit::bigint;

-- name: ListWidgetConversationChanges :many
-- ListWidgetConversationChanges returns the visitor's conversations on the
-- widget that changed after the time, oldest change first.
SELECT c.* FROM conversations c
JOIN channel_connections cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id AND c.deleted_at IS NULL
  AND (
      c.source->>'widget_id' = @widget_id::text
      OR (cc.type IN ('email', 'api') AND c.inbox_id = @inbox_id::uuid)
  )
  AND c.updated_at > @since_at::timestamptz AND c.updated_at <= @until_at::timestamptz
ORDER BY c.updated_at, c.id
LIMIT @row_limit::bigint;

-- name: ConversationsByIDs :many
-- ConversationsByIDs returns the conversations of a list page; the list
-- itself decides their order.
SELECT * FROM conversations WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: UnreadConversationIDs :many
-- UnreadConversationIDs returns those of the conversations that hold a
-- message or note someone other than the member wrote after the member's
-- read marker.
SELECT c.id FROM conversations c
WHERE c.workspace_id = @workspace_id
  AND c.id = ANY (@ids::uuid[])
  AND EXISTS (
      SELECT 1 FROM messages m
      WHERE m.workspace_id = c.workspace_id
        AND m.conversation_id = c.id
        AND m.type <> 'event'
        AND m.deleted_at IS NULL
        AND NOT (m.author_type = 'member' AND m.author_id IS NOT DISTINCT FROM @member_id::uuid)
        AND m.id > coalesce((
            SELECT r.last_read_message_id FROM conversation_reads r
            WHERE r.workspace_id = c.workspace_id AND r.conversation_id = c.id
              AND r.workspace_member_id = @member_id::uuid
        ), '00000000-0000-0000-0000-000000000000'::uuid)
  );

-- name: LatestMessagesOf :many
-- LatestMessagesOf returns the newest live message, notes and events
-- aside, of each of the conversations, the preview a list row shows.
SELECT DISTINCT ON (conversation_id) * FROM messages
WHERE workspace_id = @workspace_id
  AND conversation_id = ANY (@ids::uuid[])
  AND type = 'message'
  AND deleted_at IS NULL
ORDER BY conversation_id, created_at DESC, id DESC;

-- name: CountConversations :one
-- CountConversations counts the live conversations behind each view of
-- the sidebar: the member's open and snoozed ones, the open ones nobody
-- holds, every open one, and spam.
SELECT
    count(*) FILTER (WHERE NOT spam AND status = 'open' AND assignee_member_id = @member_id) AS mine_open,
    count(*) FILTER (WHERE NOT spam AND status = 'snoozed' AND assignee_member_id = @member_id) AS mine_snoozed,
    count(*) FILTER (WHERE NOT spam AND status = 'open' AND assignee_member_id IS NULL) AS unassigned_open,
    count(*) FILTER (WHERE NOT spam AND status = 'open') AS all_open,
    count(*) FILTER (WHERE spam) AS spam_total
FROM conversations
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: CountUnreadConversations :one
-- CountUnreadConversations counts the open conversations unread by the
-- member among those assigned to them and those nobody holds.
SELECT
    count(*) FILTER (WHERE c.assignee_member_id = @member_id::uuid) AS mine,
    count(*) FILTER (WHERE c.assignee_member_id IS NULL) AS unassigned
FROM conversations c
WHERE c.workspace_id = @workspace_id
  AND c.deleted_at IS NULL
  AND NOT c.spam
  AND c.status = 'open'
  AND (c.assignee_member_id IS NULL OR c.assignee_member_id = @member_id::uuid)
  AND EXISTS (
      SELECT 1 FROM messages m
      WHERE m.workspace_id = c.workspace_id
        AND m.conversation_id = c.id
        AND m.type <> 'event'
        AND m.deleted_at IS NULL
        AND NOT (m.author_type = 'member' AND m.author_id IS NOT DISTINCT FROM @member_id::uuid)
        AND m.id > coalesce((
            SELECT r.last_read_message_id FROM conversation_reads r
            WHERE r.workspace_id = c.workspace_id AND r.conversation_id = c.id
              AND r.workspace_member_id = @member_id::uuid
        ), '00000000-0000-0000-0000-000000000000'::uuid)
  );

-- name: CountOpenConversationsByInbox :many
-- CountOpenConversationsByInbox counts the open conversations of each
-- inbox that has any.
SELECT inbox_id, count(*) AS open
FROM conversations
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND NOT spam AND status = 'open'
GROUP BY inbox_id;

-- name: SearchHighlights :many
-- SearchHighlights returns, for each of the conversations, the newest
-- live message or note matching the tsquery, with an excerpt around the
-- match. The body is escaped before the excerpt is cut, so the only
-- markup the excerpt holds is the mark tags around the matched words.
SELECT DISTINCT ON (m.conversation_id)
    m.conversation_id,
    m.id,
    ts_headline(
        'dimmi_search',
        replace(replace(replace(coalesce(m.body, ''), '&', '&amp;'), '<', '&lt;'), '>', '&gt;'),
        to_tsquery('dimmi_search', @query::text),
        'MaxFragments=2, MaxWords=18, MinWords=6, StartSel=<mark>, StopSel=</mark>'
    )::text AS highlight
FROM messages m
WHERE m.workspace_id = @workspace_id
  AND m.conversation_id = ANY (@ids::uuid[])
  AND m.deleted_at IS NULL
  AND m.type <> 'event'
  AND m.search @@ to_tsquery('dimmi_search', @query::text)
ORDER BY m.conversation_id, m.created_at DESC, m.id DESC;

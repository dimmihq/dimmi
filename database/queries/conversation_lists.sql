-- name: FilterConversations :many
-- FilterConversations returns a page of the workspace's live
-- conversations narrowed by the filters a list view sends, ordered by the
-- sort key, last message time or creation time, then id. A page after
-- the first passes the sort key and id of the last row it saw, and gets
-- the rows after it in the list's direction. Unread is judged for the
-- member: a message or note someone else wrote after their read marker.
-- The query, when given, is a tsquery the conversation's subject, one of
-- its messages or notes, or its customer's name, email, phone or external
-- id must match; a query of only stop words matches everything.
SELECT
    sqlc.embed(c),
    (CASE WHEN @sort_created::boolean THEN c.created_at ELSE coalesce(c.last_message_at, c.created_at) END)::timestamptz
        AS sort_at
FROM conversations c
WHERE c.workspace_id = @workspace_id
  AND c.deleted_at IS NULL
  AND c.spam = @spam
  AND (coalesce(cardinality(@statuses::text[]), 0) = 0 OR c.status = ANY (@statuses::text[]))
  AND (NOT @unassigned::boolean OR c.assignee_member_id IS NULL)
  AND (sqlc.narg(assignee_id)::uuid IS NULL OR c.assignee_member_id = sqlc.narg(assignee_id)::uuid)
  AND (sqlc.narg(team_id)::uuid IS NULL OR c.assignee_team_id = sqlc.narg(team_id)::uuid)
  AND (sqlc.narg(inbox_id)::uuid IS NULL OR c.inbox_id = sqlc.narg(inbox_id)::uuid)
  AND (sqlc.narg(contact_id)::uuid IS NULL OR c.contact_id = sqlc.narg(contact_id)::uuid)
  AND (
      sqlc.narg(company_id)::uuid IS NULL
      OR c.contact_id IN (
          SELECT co.id FROM contacts co
          WHERE co.workspace_id = @workspace_id AND co.company_id = sqlc.narg(company_id)::uuid
      )
  )
  AND (sqlc.narg(priority)::boolean IS NULL OR c.priority = sqlc.narg(priority)::boolean)
  AND (sqlc.narg(number)::bigint IS NULL OR c.number = sqlc.narg(number)::bigint)
  AND (
      coalesce(cardinality(@tag_ids::uuid[]), 0) = 0
      OR (
          SELECT count(*) FROM conversation_tag ct
          WHERE ct.workspace_id = c.workspace_id AND ct.conversation_id = c.id AND ct.tag_id = ANY (@tag_ids::uuid[])
      ) = cardinality(@tag_ids::uuid[])
  )
  AND (
      coalesce(cardinality(@any_tag_ids::uuid[]), 0) = 0
      OR EXISTS (
          SELECT 1 FROM conversation_tag ct
          WHERE ct.workspace_id = c.workspace_id AND ct.conversation_id = c.id
            AND ct.tag_id = ANY (@any_tag_ids::uuid[])
      )
  )
  AND (
      NOT @only_unread::boolean
      OR EXISTS (
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
      )
  )
  AND (
      sqlc.narg(query)::text IS NULL
      OR numnode(to_tsquery('english', sqlc.narg(query)::text)) = 0
      OR to_tsvector('english', coalesce(c.subject, '')) @@ to_tsquery('english', sqlc.narg(query)::text)
      OR c.id IN (
          SELECT m.conversation_id FROM messages m
          WHERE m.workspace_id = @workspace_id
            AND m.deleted_at IS NULL
            AND m.type <> 'event'
            AND m.search @@ to_tsquery('english', sqlc.narg(query)::text)
      )
      OR c.contact_id IN (
          SELECT ct.id FROM contacts ct
          WHERE ct.workspace_id = @workspace_id
            AND ct.deleted_at IS NULL
            AND ct.search @@ to_tsquery('simple', sqlc.narg(query)::text)
      )
  )
  AND (
      sqlc.narg(after_id)::uuid IS NULL
      OR (
          @descending::boolean
          AND ((CASE WHEN @sort_created::boolean THEN c.created_at ELSE coalesce(c.last_message_at, c.created_at) END),
              c.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
      )
      OR (
          NOT @descending::boolean
          AND ((CASE WHEN @sort_created::boolean THEN c.created_at ELSE coalesce(c.last_message_at, c.created_at) END),
              c.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
      )
  )
ORDER BY
    CASE WHEN @descending::boolean THEN
        (CASE WHEN @sort_created::boolean THEN c.created_at ELSE coalesce(c.last_message_at, c.created_at) END)
    END DESC,
    CASE WHEN @descending::boolean THEN c.id END DESC,
    CASE WHEN NOT @descending::boolean THEN
        (CASE WHEN @sort_created::boolean THEN c.created_at ELSE coalesce(c.last_message_at, c.created_at) END)
    END,
    CASE WHEN NOT @descending::boolean THEN c.id END
LIMIT @row_limit::bigint;

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
        'english',
        replace(replace(replace(coalesce(m.body, ''), '&', '&amp;'), '<', '&lt;'), '>', '&gt;'),
        to_tsquery('english', @query::text),
        'MaxFragments=2, MaxWords=18, MinWords=6, StartSel=<mark>, StopSel=</mark>'
    )::text AS highlight
FROM messages m
WHERE m.workspace_id = @workspace_id
  AND m.conversation_id = ANY (@ids::uuid[])
  AND m.deleted_at IS NULL
  AND m.type <> 'event'
  AND m.search @@ to_tsquery('english', @query::text)
ORDER BY m.conversation_id, m.created_at DESC, m.id DESC;

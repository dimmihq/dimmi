-- migrate: phase=expand
-- What an inbox row needs without touching the timeline: since when the
-- customer waits for an answer, when anything last happened, which message
-- the row previews, the closed conversation a new one continues, and
-- whether it is a test. The subject gets its own search document.
-- A customer waits when their latest message is newer than the latest
-- reply and the conversation is not closed.

-- +goose Up
ALTER TABLE conversations
    ADD COLUMN waiting_since timestamptz,
    ADD COLUMN last_activity_at timestamptz,
    ADD COLUMN last_message_id uuid,
    ADD COLUMN IF NOT EXISTS previous_conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS is_test boolean NOT NULL DEFAULT false,
    ADD COLUMN search tsvector GENERATED ALWAYS AS (to_tsvector('dimmi_search', coalesce(subject, ''))) STORED;

UPDATE conversations
SET waiting_since = last_customer_message_at
WHERE status IN ('open', 'snoozed')
  AND last_customer_message_at IS NOT NULL
  AND last_customer_message_at > coalesce(last_reply_at, 'epoch'::timestamptz);

UPDATE conversations c
SET last_activity_at = latest.at, last_message_id = latest.message_id
FROM (
    SELECT conversation_id,
        max(created_at) FILTER (WHERE type IN ('message', 'note') AND deleted_at IS NULL) AS at,
        (array_agg(id ORDER BY created_at DESC, id DESC)
            FILTER (WHERE type = 'message' AND deleted_at IS NULL))[1] AS message_id
    FROM messages
    GROUP BY conversation_id
) latest
WHERE latest.conversation_id = c.id;

-- +goose Down
ALTER TABLE conversations
    DROP COLUMN search,
    DROP COLUMN last_message_id,
    DROP COLUMN last_activity_at,
    DROP COLUMN waiting_since;

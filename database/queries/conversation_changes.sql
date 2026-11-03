-- name: LockConversation :one
-- LockConversation returns the live conversation and holds its row until
-- the transaction ends, so changes to one conversation happen one after
-- another.
SELECT * FROM conversations WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: FindConversationByNumber :one
-- FindConversationByNumber returns the conversation the workspace knows
-- by the number, deleted or not.
SELECT * FROM conversations WHERE workspace_id = @workspace_id AND number = @number;

-- name: SetConversationAssignment :one
-- SetConversationAssignment hands the conversation to the member and the
-- team, either or both of which may be nobody.
UPDATE conversations
SET assignee_member_id = sqlc.narg(assignee_member_id), assignee_team_id = sqlc.narg(assignee_team_id),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetConversationStatus :one
-- SetConversationStatus moves the conversation to the status. Closing
-- records when; any other status forgets it. Only a snoozed conversation
-- keeps a wake-up time.
UPDATE conversations
SET status = @status::text,
    snoozed_until = CASE WHEN @status::text = 'snoozed' THEN sqlc.narg(snoozed_until)::timestamptz END,
    closed_at = CASE WHEN @status::text = 'closed' THEN @now::timestamptz END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetConversationAttributes :one
-- SetConversationAttributes writes the conversation's subject, priority
-- and inbox as given.
UPDATE conversations
SET subject = sqlc.narg(subject), priority = @priority, inbox_id = @inbox_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetConversationSpam :one
-- SetConversationSpam flags the conversation as spam or clears the flag,
-- leaving its status as it was.
UPDATE conversations SET spam = @spam, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RecordConversationMessage :one
-- RecordConversationMessage counts a new message into the conversation:
-- its message count and last message time, and, by who wrote it, the
-- first and last customer message or the first and last reply.
UPDATE conversations
SET message_count = message_count + 1,
    last_message_at = @at::timestamptz,
    first_customer_message_at = CASE
        WHEN @from_customer::boolean THEN coalesce(first_customer_message_at, @at::timestamptz)
        ELSE first_customer_message_at END,
    last_customer_message_at = CASE
        WHEN @from_customer::boolean THEN @at::timestamptz ELSE last_customer_message_at END,
    first_reply_at = CASE WHEN @from_customer::boolean THEN first_reply_at
        ELSE coalesce(first_reply_at, @at::timestamptz) END,
    last_reply_at = CASE WHEN @from_customer::boolean THEN last_reply_at ELSE @at::timestamptz END,
    updated_at = @at::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchConversation :exec
-- TouchConversation records that the conversation changed, such as by a
-- new note, without changing anything else.
UPDATE conversations SET updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

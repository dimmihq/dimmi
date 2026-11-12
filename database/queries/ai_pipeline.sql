-- name: ListLatestPublicMessages :many
-- ListLatestPublicMessages returns a conversation's newest public
-- messages written by the customer, a teammate or the agent, newest
-- first. Notes, events, undone, deleted and still scheduled messages are
-- left out.
SELECT * FROM messages
WHERE workspace_id = @workspace_id
  AND conversation_id = @conversation_id
  AND type = 'message'
  AND author_type IN ('contact', 'member', 'agent')
  AND undone_at IS NULL
  AND deleted_at IS NULL
  AND (scheduled_send_at IS NULL OR scheduled_send_at <= @now)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: FindConversationMessage :one
-- FindConversationMessage returns one message of the conversation.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id = @id;

-- name: ListPublicMessagesAfter :many
-- ListPublicMessagesAfter returns who wrote each public message that
-- came after the given one, oldest first.
SELECT m.id, m.author_type FROM messages m
JOIN messages pivot ON pivot.workspace_id = m.workspace_id AND pivot.id = @after_id
WHERE m.workspace_id = @workspace_id
  AND m.conversation_id = @conversation_id
  AND m.type = 'message'
  AND m.author_type IN ('contact', 'member', 'agent')
  AND m.undone_at IS NULL
  AND m.deleted_at IS NULL
  AND (m.created_at, m.id) > (pivot.created_at, pivot.id)
ORDER BY m.created_at, m.id;

-- name: LockConversationForAi :one
-- LockConversationForAi returns the conversation and holds its row
-- until the transaction ends, so an agent reply and a teammate's
-- never cross.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND id = @id
FOR UPDATE;

-- name: ActivateConversationAi :exec
-- ActivateConversationAi hands a conversation the agent has not spoken
-- in yet to the agent.
UPDATE conversations SET ai_state = 'active', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND ai_state = 'off';

-- name: RunFinishedForMessage :one
-- RunFinishedForMessage reports whether a run already decided the
-- customer's newest message, so a retried job never answers twice.
SELECT EXISTS (
    SELECT 1 FROM ai_runs
    WHERE workspace_id = @workspace_id
      AND conversation_id = @conversation_id
      AND @message_id::uuid = ANY(included_message_ids)
      AND status IN ('completed', 'failed')
      AND outcome NOT IN ('superseded', 'skipped')
)::boolean;

-- name: RunInFlightForMessage :one
-- RunInFlightForMessage reports whether a run started since the cutoff
-- is still answering the customer's newest message, so two jobs for
-- one message never both ask the model.
SELECT EXISTS (
    SELECT 1 FROM ai_runs
    WHERE workspace_id = @workspace_id
      AND conversation_id = @conversation_id
      AND @message_id::uuid = ANY(included_message_ids)
      AND status = 'running'
      AND started_at > @since::timestamptz
)::boolean;

-- name: AssigneeActive :one
-- AssigneeActive reports whether the assigned teammate is around: online
-- now or wrote a public reply in the conversation since the cutoff.
SELECT (
    EXISTS (
        SELECT 1 FROM workspace_members wm
        WHERE wm.workspace_id = @workspace_id AND wm.id = @member_id AND wm.availability = 'online'
    ) OR EXISTS (
        SELECT 1 FROM messages m
        WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id
          AND m.type = 'message' AND m.author_type = 'member' AND m.author_id = @member_id
          AND m.created_at >= @since
    )
)::boolean;

-- name: CountOnlineMembers :one
-- CountOnlineMembers counts the workspace's active teammates who are
-- online now.
SELECT count(*) FROM workspace_members
WHERE workspace_id = @workspace_id AND status = 'active' AND availability = 'online';

-- name: LastChangeAuthor :one
-- LastChangeAuthor returns who made the newest timeline change of the
-- name, narrowed to changes of one field when given.
SELECT m.author_type FROM messages m
WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id
  AND m.type = 'event' AND m.event->>'name' = @name::text
  AND (@field::text = '' OR m.event->'changes' ? @field::text)
ORDER BY m.created_at DESC, m.id DESC
LIMIT 1;

-- name: ListConversationTagAuthors :many
-- ListConversationTagAuthors returns the conversation's tags and who put
-- each on.
SELECT ct.tag_id, ct.created_by_type FROM conversation_tag ct
WHERE ct.workspace_id = @workspace_id AND ct.conversation_id = @conversation_id;

-- name: ListAgentTags :many
-- ListAgentTags returns the workspace's tags by name for the agent's
-- tool descriptions.
SELECT * FROM tags WHERE workspace_id = @workspace_id ORDER BY name, id LIMIT @row_limit;

-- name: ListAgentTeams :many
-- ListAgentTeams returns the workspace's live teams by name for the
-- agent's tool descriptions.
SELECT * FROM teams WHERE workspace_id = @workspace_id AND deleted_at IS NULL
ORDER BY lower(name), id LIMIT @row_limit;

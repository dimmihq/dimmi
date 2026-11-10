-- name: SaveTriagePost :one
-- SaveTriagePost records where a conversation's triage post is, moving
-- it when the post was posted again.
INSERT INTO channel_conversation_posts (
    workspace_id, channel_connection_id, conversation_id, kind, external_channel, external_ts, created_at, updated_at
) VALUES (
    @workspace_id, @channel_connection_id, @conversation_id, 'triage', @external_channel, @external_ts,
    @now::timestamptz, @now::timestamptz
)
ON CONFLICT (conversation_id) WHERE kind = 'triage' DO UPDATE
SET external_channel = excluded.external_channel, external_ts = excluded.external_ts,
    updated_at = excluded.updated_at
RETURNING *;

-- name: CreateChannelConversationPost :one
-- CreateChannelConversationPost records a discussion thread or a draft
-- posted for a conversation.
INSERT INTO channel_conversation_posts (
    workspace_id, channel_connection_id, conversation_id, kind, external_channel, external_ts, run_id,
    created_at, updated_at
) VALUES (
    @workspace_id, @channel_connection_id, @conversation_id, @kind, @external_channel, @external_ts,
    sqlc.narg(run_id), @now::timestamptz, @now::timestamptz
)
ON CONFLICT (channel_connection_id, external_channel, external_ts) DO UPDATE SET updated_at = excluded.updated_at
RETURNING *;

-- name: FindTriagePost :one
-- FindTriagePost returns the conversation's triage post.
SELECT * FROM channel_conversation_posts
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND kind = 'triage';

-- name: FindChannelConversationPost :one
-- FindChannelConversationPost returns the post at the provider's place,
-- the root of a thread the team works a conversation in.
SELECT * FROM channel_conversation_posts
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_channel = @external_channel AND external_ts = @external_ts;

-- name: FindDraftPost :one
-- FindDraftPost returns where the run's draft was posted.
SELECT * FROM channel_conversation_posts
WHERE workspace_id = @workspace_id AND run_id = @run_id AND kind = 'draft'
ORDER BY created_at DESC LIMIT 1;

-- name: ListConversationThreadPosts :many
-- ListConversationThreadPosts returns the threads the team works the
-- conversation in: its triage post and its discussions.
SELECT * FROM channel_conversation_posts
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND kind IN ('triage', 'discussion')
ORDER BY created_at, id;

-- name: SetChannelConversationPostStatus :execrows
-- SetChannelConversationPostStatus settles a draft, once: sent or
-- discarded.
UPDATE channel_conversation_posts SET status = @status, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status = 'active';

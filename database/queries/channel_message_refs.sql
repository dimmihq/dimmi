-- name: CreateChannelMessageRef :exec
-- CreateChannelMessageRef files one provider id under its message. Filing
-- it again changes nothing.
INSERT INTO channel_message_refs (
    workspace_id, message_id, channel_connection_id, external_message_id, part_index, kind, created_at, updated_at
)
VALUES (
    @workspace_id, @message_id, @channel_connection_id, @external_message_id, least(@part_index::bigint, 32767),
    @kind,
    @now::timestamptz, @now::timestamptz
)
ON CONFLICT (channel_connection_id, external_message_id) DO NOTHING;

-- name: FindChannelMessageRef :one
-- FindChannelMessageRef returns the ref of a provider id on the
-- connection.
SELECT * FROM channel_message_refs
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_message_id = @external_message_id;

-- name: ListChannelMessageRefs :many
-- ListChannelMessageRefs returns every provider id of the message, in
-- the order its parts were sent.
SELECT * FROM channel_message_refs
WHERE workspace_id = @workspace_id AND message_id = @message_id
ORDER BY part_index, created_at;

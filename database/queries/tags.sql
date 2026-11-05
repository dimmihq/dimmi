-- name: CreateTag :one
-- CreateTag stores a tag in the workspace, with the icon given, in the
-- color given or else the palette color the workspace uses least. A name another tag holds, in
-- any case, fails on tags_workspace_id_name_key.
INSERT INTO tags (workspace_id, name, color, icon, created_at, updated_at)
VALUES (
    @workspace_id, @name::text, coalesce(sqlc.narg(color), dimmi_next_tag_color(@workspace_id, @name::text)), sqlc.narg(icon),
    @now, @now
)
RETURNING *;

-- name: EnsureTags :many
-- EnsureTags returns the workspace's tags with the names, creating those
-- that do not exist yet, by name, each in a palette color. Names are matched without regard to
-- case, so Billing finds billing and a name given twice is one tag. A
-- tag another transaction creates at the same moment is left out, so a
-- caller that needs every name compares the count and asks again.
WITH wanted AS (
    SELECT DISTINCT unnest(@names::text[])::citext AS name
),
created AS (
    INSERT INTO tags (workspace_id, name, color, created_at, updated_at)
    SELECT @workspace_id, name, dimmi_next_tag_color(@workspace_id, name::text), @now, @now FROM wanted
    ON CONFLICT (workspace_id, name) DO NOTHING
    RETURNING *
)
SELECT * FROM created
UNION ALL
SELECT t.* FROM tags AS t
JOIN wanted AS w ON w.name = t.name
WHERE t.workspace_id = @workspace_id
ORDER BY name;

-- name: FindTag :one
-- FindTag returns the tag with the id in the workspace.
SELECT * FROM tags WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListTags :many
-- ListTags returns the workspace's tags by name, narrowed to names that
-- start with the prefix when one is given.
SELECT * FROM tags
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(prefix)::text IS NULL OR starts_with(lower(name::text), lower(sqlc.narg(prefix)::text)))
ORDER BY name, id;

-- name: CountTaggedConversations :many
-- CountTaggedConversations returns, for each tag of the workspace on at
-- least one conversation that is not deleted, how many it is on.
SELECT ct.tag_id, count(*) AS conversations FROM conversation_tag AS ct
JOIN conversations AS c ON c.id = ct.conversation_id AND c.workspace_id = ct.workspace_id
WHERE ct.workspace_id = @workspace_id AND c.deleted_at IS NULL
GROUP BY ct.tag_id
ORDER BY ct.tag_id;

-- name: UpdateTag :one
-- UpdateTag renames a tag or changes its color or icon. A name another
-- tag holds fails on tags_workspace_id_name_key.
UPDATE tags SET name = @name, color = @color, icon = @icon, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteTag :execrows
-- DeleteTag removes a tag, and with it the tag from every conversation,
-- and reports whether it existed.
DELETE FROM tags WHERE workspace_id = @workspace_id AND id = @id;

-- name: TagConversation :many
-- TagConversation puts the workspace's tags with the ids on the
-- conversation and returns the ids of those it added. A tag already on
-- it is skipped, and an id of another workspace's tag is ignored, so the
-- result is exactly what changed. The message, when given, is the one
-- that prompted the tag.
INSERT INTO conversation_tag (
    workspace_id, conversation_id, tag_id, created_by_type, created_by_id, message_id, created_at
)
SELECT @workspace_id, @conversation_id, t.id, @created_by_type, @created_by_id, sqlc.narg(message_id), @now FROM tags AS t
WHERE t.workspace_id = @workspace_id AND t.id = ANY (@tag_ids::uuid[])
ON CONFLICT (conversation_id, tag_id) DO NOTHING
RETURNING tag_id;

-- name: UntagConversation :execrows
-- UntagConversation takes the tag off the conversation and reports
-- whether it was on.
DELETE FROM conversation_tag
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND tag_id = @tag_id;

-- name: CountConversationTags :one
-- CountConversationTags returns how many tags are on the conversation,
-- for the limit on tags per conversation.
SELECT count(*) FROM conversation_tag WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id;

-- name: ListConversationTags :many
-- ListConversationTags returns the tags on each of the conversations, by
-- conversation and then by name, so a page of conversations loads its
-- tags in one query.
SELECT ct.conversation_id, sqlc.embed(t) FROM conversation_tag AS ct
JOIN tags AS t ON t.id = ct.tag_id AND t.workspace_id = ct.workspace_id
WHERE ct.workspace_id = @workspace_id AND ct.conversation_id = ANY (@conversation_ids::uuid[])
ORDER BY ct.conversation_id, t.name, t.id;

-- name: TagsByNames :many
-- TagsByNames returns the workspace's tags with the names, without regard
-- to case.
SELECT * FROM tags
WHERE workspace_id = @workspace_id AND name = ANY (@names::citext[])
ORDER BY name;

-- name: TagsByIDs :many
-- TagsByIDs returns the workspace's tags with the ids, by name.
SELECT * FROM tags
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[])
ORDER BY name, id;

-- name: CountWorkspaceTags :one
-- CountWorkspaceTags returns how many tags the workspace has, for the
-- limit on its vocabulary.
SELECT count(*) FROM tags WHERE workspace_id = @workspace_id;

-- name: UntagConversationMany :many
-- UntagConversationMany takes the tags off the conversation and returns
-- the ids of those it took off.
DELETE FROM conversation_tag
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND tag_id = ANY (@tag_ids::uuid[])
RETURNING tag_id;

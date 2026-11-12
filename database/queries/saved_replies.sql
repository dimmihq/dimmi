-- name: CreateSavedReply :one
-- CreateSavedReply stores a reply, shared with the workspace when it has
-- no owner and the owner's own otherwise. A shortcut the owner, or the
-- workspace for a shared reply, already uses fails on
-- saved_replies_workspace_id_owner_member_id_shortcut_key. Without
-- actions the reply is a plain one.
INSERT INTO saved_replies (workspace_id, owner_member_id, title, body, shortcut, actions, created_at, updated_at)
VALUES (
    @workspace_id, @owner_member_id, @title, @body, @shortcut, coalesce(sqlc.narg(actions)::jsonb, '{}'::jsonb),
    @now, @now
)
RETURNING *;

-- name: FindSavedReply :one
-- FindSavedReply returns the reply with the id in the workspace, shared
-- or anyone's own; the caller decides who may see it.
SELECT * FROM saved_replies WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListSavedReplies :many
-- ListSavedReplies returns a page of the replies the member may use, the
-- shared ones and their own, by title, narrowed to those whose title or
-- shortcut contains the search when one is given. A page after the first
-- passes the title and id of the last reply it saw.
SELECT * FROM saved_replies
WHERE workspace_id = @workspace_id
  AND (owner_member_id IS NULL OR owner_member_id = @member_id::uuid)
  AND (
      sqlc.narg(search)::text IS NULL
      OR strpos(lower(title), lower(sqlc.narg(search)::text)) > 0
      OR strpos(lower(coalesce(shortcut::text, '')), lower(sqlc.narg(search)::text)) > 0
  )
  AND (
      sqlc.narg(after_id)::uuid IS NULL
      OR (title, id) > (sqlc.narg(after_title)::text, sqlc.narg(after_id)::uuid)
  )
ORDER BY title, id
LIMIT @row_limit;

-- name: FindSavedReplyByShortcut :one
-- FindSavedReplyByShortcut returns the reply the member reaches with the
-- shortcut: their own when they have one, the shared one otherwise.
SELECT * FROM saved_replies
WHERE workspace_id = @workspace_id
  AND shortcut = @shortcut::citext
  AND (owner_member_id IS NULL OR owner_member_id = @member_id::uuid)
ORDER BY owner_member_id NULLS LAST
LIMIT 1;

-- name: UpdateSavedReply :one
-- UpdateSavedReply replaces a reply's title, body and shortcut, its
-- actions when given, and its owner when set_owner is true, which moves
-- it between shared and personal. A shortcut its owner already uses on
-- another reply fails on
-- saved_replies_workspace_id_owner_member_id_shortcut_key.
UPDATE saved_replies
SET title = @title, body = @body, shortcut = @shortcut,
    actions = coalesce(sqlc.narg(actions)::jsonb, actions),
    owner_member_id = CASE WHEN @set_owner::boolean THEN sqlc.narg(new_owner_member_id)::uuid ELSE owner_member_id END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteSavedReply :execrows
-- DeleteSavedReply removes a reply and reports whether it existed.
DELETE FROM saved_replies WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountSavedReplies :one
-- CountSavedReplies returns how many replies the workspace keeps, shared
-- and personal.
SELECT count(*) FROM saved_replies WHERE workspace_id = @workspace_id;

-- name: TagContact :many
-- TagContact puts the workspace's tags with the ids on the contact and returns
-- the ids of those it added. A tag already on it is skipped, and an id of
-- another workspace's tag is ignored, so the result is exactly what
-- changed.
INSERT INTO contact_tag (workspace_id, contact_id, tag_id, created_by_type, created_by_id, created_at)
SELECT @workspace_id, @contact_id, t.id, @created_by_type, @created_by_id, @now FROM tags AS t
WHERE t.workspace_id = @workspace_id AND t.id = ANY (@tag_ids::uuid[])
ON CONFLICT (contact_id, tag_id) DO NOTHING
RETURNING tag_id;

-- name: UntagContact :many
-- UntagContact takes the tags off the contact and returns the ids of those it
-- took off.
DELETE FROM contact_tag
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND tag_id = ANY (@tag_ids::uuid[])
RETURNING tag_id;

-- name: CountContactTags :one
-- CountContactTags returns how many tags are on the contact, for the limit on
-- tags per contact.
SELECT count(*) FROM contact_tag WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: ListContactTags :many
-- ListContactTags returns the tags on each of the contacts, by contact and then
-- by name, so a page loads its tags in one query.
SELECT ct.contact_id, sqlc.embed(t) FROM contact_tag AS ct
JOIN tags AS t ON t.id = ct.tag_id AND t.workspace_id = ct.workspace_id
WHERE ct.workspace_id = @workspace_id AND ct.contact_id = ANY (@contact_ids::uuid[])
ORDER BY ct.contact_id, t.name, t.id;

-- name: CountTaggedContacts :many
-- CountTaggedContacts returns, for each tag on a live contact of the
-- workspace, how many it is on.
SELECT ct.tag_id, count(*) AS tagged FROM contact_tag AS ct
JOIN contacts AS c ON c.id = ct.contact_id AND c.workspace_id = ct.workspace_id
WHERE ct.workspace_id = @workspace_id AND c.deleted_at IS NULL
GROUP BY ct.tag_id
ORDER BY ct.tag_id;

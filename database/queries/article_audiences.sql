-- name: SetArticleAudience :one
-- SetArticleAudience keeps an article to a saved audience, or opens it to
-- everyone with null.
UPDATE articles SET audience_id = @audience_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetCollectionAudience :one
-- SetCollectionAudience keeps a collection, and everything in it, to a
-- saved audience, or opens it to everyone with null.
UPDATE collections SET audience_id = @audience_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ArticleAudiences :many
-- ArticleAudiences returns every live article's audience and collection,
-- so what a reader may see can be worked out.
SELECT id, help_center_id, collection_id, audience_id FROM articles
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: CollectionAudiences :many
-- CollectionAudiences returns every live collection's parent and audience.
SELECT id, parent_id, audience_id FROM collections
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

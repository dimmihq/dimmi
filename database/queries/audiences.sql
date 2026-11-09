-- name: ListAudiences :many
-- ListAudiences returns the workspace's saved audiences by name.
SELECT * FROM audiences WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY lower(name), id;

-- name: ListAudiencePage :many
-- ListAudiencePage returns a page of the workspace's saved audiences by
-- name, narrowed to those whose name contains the search when one is
-- given.
SELECT * FROM audiences
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
  AND (sqlc.narg(search)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(search)::text)) > 0)
ORDER BY lower(name), id
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: CountAudiences :one
-- CountAudiences counts the audiences ListAudiencePage would list, up to
-- the cap.
SELECT count(*) FROM (
SELECT 1 FROM audiences
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
  AND (sqlc.narg(search)::text IS NULL OR strpos(lower(name), lower(sqlc.narg(search)::text)) > 0)
LIMIT @row_limit::bigint
) AS counted;

-- name: FindAudience :one
-- FindAudience returns one saved audience.
SELECT * FROM audiences WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: AudienceNameTaken :one
-- AudienceNameTaken reports whether another live audience has the name.
SELECT EXISTS (
    SELECT 1 FROM audiences
    WHERE workspace_id = @workspace_id AND lower(name) = lower(@name::text) AND id <> @id AND deleted_at IS NULL
);

-- name: CreateAudience :one
-- CreateAudience stores a new saved audience.
INSERT INTO audiences (id, workspace_id, name, description, rules, created_by_member_id, updated_by_member_id,
    created_at, updated_at)
VALUES (@id, @workspace_id, @name, @description, @rules, @member_id, @member_id, @now, @now)
RETURNING *;

-- name: SaveAudience :one
-- SaveAudience saves an audience when the editor started from the stored
-- lock version, and moves the version on.
UPDATE audiences SET name = @name, description = @description, rules = @rules, lock_version = lock_version + 1,
    updated_by_member_id = @member_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND lock_version = @lock_version
RETURNING *;

-- name: SetAudienceEstimate :exec
-- SetAudienceEstimate keeps the last preview count of a saved audience.
UPDATE audiences SET last_estimate = @estimate WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteAudience :one
-- DeleteAudience removes a saved audience.
UPDATE audiences SET deleted_at = @now, updated_by_member_id = @member_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING id;

-- name: AudienceUsers :many
-- AudienceUsers returns what names a saved audience: other saved audiences,
-- workflows whose working copy names it, workflows whose live version does,
-- and the articles and collections kept to it. live is true for live
-- workflows, articles and collections, which rely on it now.
SELECT 'audience'::text AS kind, a.id, a.name, false AS live FROM audiences a
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL AND a.id <> @audience_id
  AND strpos(a.rules::text, @public_id::text) > 0
UNION ALL
SELECT 'workflow', w.id, w.name, (v.id IS NOT NULL) AS live FROM workflows w
LEFT JOIN workflow_versions v ON v.id = w.live_version_id AND w.status = 'live'
  AND v.compiled -> 'references' -> 'audiences' ? @public_id::text
WHERE w.workspace_id = @workspace_id AND w.deleted_at IS NULL
  AND (strpos(w.draft::text, @public_id::text) > 0 OR v.id IS NOT NULL)
UNION ALL
SELECT 'article', ar.id, ar.title, true FROM articles ar
WHERE ar.workspace_id = @workspace_id AND ar.deleted_at IS NULL AND ar.audience_id = @audience_id
UNION ALL
SELECT 'collection', co.id, co.name, true FROM collections co
WHERE co.workspace_id = @workspace_id AND co.deleted_at IS NULL AND co.audience_id = @audience_id
ORDER BY 1, 3;

-- name: CountAudienceContacts :one
-- CountAudienceContacts counts the people an audience can be checked
-- against: live contacts that were not merged away.
SELECT count(*) FROM contacts
WHERE workspace_id = @workspace_id AND merged_into_id IS NULL AND deleted_at IS NULL;

-- name: AudienceContactPage :many
-- AudienceContactPage returns the next page of live contacts after the
-- given id, in id order. A fraction under one keeps each contact with
-- that chance, so a large workspace is previewed from a random sample.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND merged_into_id IS NULL AND deleted_at IS NULL AND id > @after
  AND (@fraction::float8 >= 1 OR random() < @fraction::float8)
ORDER BY id
LIMIT @page_size;

-- name: CustomAttributeKeys :many
-- CustomAttributeKeys returns the custom attribute keys in use on a
-- recent slice of contacts and companies, with one sample value's JSON
-- type, so the rule builder can offer them.
WITH recent AS (
    SELECT 'contact'::text AS object, x.custom_attributes AS attrs FROM (
        SELECT custom_attributes FROM contacts
        WHERE contacts.workspace_id = @workspace_id AND contacts.deleted_at IS NULL
        ORDER BY contacts.updated_at DESC LIMIT 2000) x
    UNION ALL
    SELECT 'company'::text, y.custom_attributes FROM (
        SELECT custom_attributes FROM companies
        WHERE companies.workspace_id = @workspace_id AND companies.deleted_at IS NULL
        ORDER BY companies.updated_at DESC LIMIT 2000) y
)
SELECT DISTINCT ON (r.object, k.key) r.object::text AS object, k.key::text AS key,
    jsonb_typeof(r.attrs -> k.key)::text AS kind
FROM recent r CROSS JOIN LATERAL jsonb_object_keys(r.attrs) AS k(key)
ORDER BY r.object, k.key, jsonb_typeof(r.attrs -> k.key);

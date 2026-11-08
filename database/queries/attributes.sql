-- name: ListAttributeDefinitions :many
-- ListAttributeDefinitions returns the workspace's definitions of one or
-- both objects, by object and key.
SELECT * FROM attribute_definitions
WHERE workspace_id = @workspace_id AND (sqlc.narg(object)::text IS NULL OR object = sqlc.narg(object)::text)
ORDER BY object, key;

-- name: FindAttributeDefinition :one
-- FindAttributeDefinition returns one definition.
SELECT * FROM attribute_definitions WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountActiveAttributeDefinitions :one
-- CountActiveAttributeDefinitions counts an object's active definitions,
-- which a workspace may have 500 of.
SELECT count(*) FROM attribute_definitions
WHERE workspace_id = @workspace_id AND object = @object AND status = 'active';

-- name: CreateAttributeDefinition :one
-- CreateAttributeDefinition stores a definition, leaving one that already
-- exists for the key as it is.
INSERT INTO attribute_definitions (id, workspace_id, object, key, label, description, data_type, options, origin,
    created_at, updated_at)
VALUES (@id, @workspace_id, @object, @key, @label, @description, @data_type, @options, @origin, @now, @now)
ON CONFLICT (workspace_id, object, key) DO NOTHING
RETURNING *;

-- name: UpdateAttributeDefinition :one
-- UpdateAttributeDefinition changes what may change: the label, the
-- description, the options and the status.
UPDATE attribute_definitions SET label = @label, description = @description, options = @options, status = @status,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteAttributeDefinition :exec
-- DeleteAttributeDefinition removes a definition.
DELETE FROM attribute_definitions WHERE workspace_id = @workspace_id AND id = @id;

-- name: TouchAttributeDefinitions :exec
-- TouchAttributeDefinitions records when keys last took a value, at most
-- once an hour each: a key last touched after stale_before is left alone.
UPDATE attribute_definitions SET last_value_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND object = @object AND key = ANY (@keys::text[])
  AND (last_value_at IS NULL OR last_value_at < @stale_before::timestamptz);

-- name: RecordAttributeProblem :exec
-- RecordAttributeProblem counts one dropped write of a key for a reason.
INSERT INTO attribute_problems (workspace_id, object, key, code, count, sample, last_at)
VALUES (@workspace_id, @object, @key, @code, 1, @sample, @now)
ON CONFLICT (workspace_id, object, key, code)
DO UPDATE SET count = attribute_problems.count + 1, sample = EXCLUDED.sample, last_at = EXCLUDED.last_at;

-- name: ListAttributeProblems :many
-- ListAttributeProblems returns the problems of the last 30 days, newest
-- first.
SELECT * FROM attribute_problems
WHERE workspace_id = @workspace_id AND last_at > @since
ORDER BY last_at DESC, object, key, code
LIMIT 500;

-- name: CountContactsWithAttribute :one
-- CountContactsWithAttribute counts live contacts holding a key.
SELECT count(*) FROM contacts
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND merged_into_id IS NULL
  AND custom_attributes ? @key::text;

-- name: CountCompaniesWithAttribute :one
-- CountCompaniesWithAttribute counts live companies holding a key.
SELECT count(*) FROM companies
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND custom_attributes ? @key::text;

-- name: RemoveContactAttribute :exec
-- RemoveContactAttribute removes a key from every contact, and from the
-- keys they trust.
UPDATE contacts SET custom_attributes = custom_attributes - @key::text,
    trusted_attribute_keys = array_remove(trusted_attribute_keys, @key::text)
WHERE workspace_id = @workspace_id AND custom_attributes ? @key::text;

-- name: RemoveCompanyAttribute :exec
-- RemoveCompanyAttribute removes a key from every company.
UPDATE companies SET custom_attributes = custom_attributes - @key::text
WHERE workspace_id = @workspace_id AND custom_attributes ? @key::text;

-- name: AttributeUsers :many
-- AttributeUsers returns the saved audiences and workflows whose rules
-- read a field, which must change before the attribute is archived.
SELECT 'audience'::text AS kind, a.id, a.name FROM audiences a
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL AND strpos(a.rules::text, @field::text) > 0
UNION ALL
SELECT 'workflow', w.id, w.name FROM workflows w
LEFT JOIN workflow_versions v ON v.id = w.live_version_id
WHERE w.workspace_id = @workspace_id AND w.deleted_at IS NULL
  AND (strpos(w.draft::text, @field::text) > 0 OR strpos(coalesce(v.spec::text, ''), @field::text) > 0)
ORDER BY 1, 3;

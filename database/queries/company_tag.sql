-- name: TagCompany :many
-- TagCompany puts the workspace's tags with the ids on the company and returns
-- the ids of those it added. A tag already on it is skipped, and an id of
-- another workspace's tag is ignored, so the result is exactly what
-- changed.
INSERT INTO company_tag (workspace_id, company_id, tag_id, created_by_type, created_by_id, created_at)
SELECT @workspace_id, @company_id, t.id, @created_by_type, @created_by_id, @now FROM tags AS t
WHERE t.workspace_id = @workspace_id AND t.id = ANY (@tag_ids::uuid[])
ON CONFLICT (company_id, tag_id) DO NOTHING
RETURNING tag_id;

-- name: UntagCompany :many
-- UntagCompany takes the tags off the company and returns the ids of those it
-- took off.
DELETE FROM company_tag
WHERE workspace_id = @workspace_id AND company_id = @company_id AND tag_id = ANY (@tag_ids::uuid[])
RETURNING tag_id;

-- name: CountCompanyTags :one
-- CountCompanyTags returns how many tags are on the company, for the limit on
-- tags per company.
SELECT count(*) FROM company_tag WHERE workspace_id = @workspace_id AND company_id = @company_id;

-- name: ListCompanyTags :many
-- ListCompanyTags returns the tags on each of the companies, by company and then
-- by name, so a page loads its tags in one query.
SELECT ct.company_id, sqlc.embed(t) FROM company_tag AS ct
JOIN tags AS t ON t.id = ct.tag_id AND t.workspace_id = ct.workspace_id
WHERE ct.workspace_id = @workspace_id AND ct.company_id = ANY (@company_ids::uuid[])
ORDER BY ct.company_id, t.name, t.id;

-- name: CountTaggedCompanies :many
-- CountTaggedCompanies returns, for each tag on a live company of the
-- workspace, how many it is on.
SELECT ct.tag_id, count(*) AS tagged FROM company_tag AS ct
JOIN companies AS c ON c.id = ct.company_id AND c.workspace_id = ct.workspace_id
WHERE ct.workspace_id = @workspace_id AND c.deleted_at IS NULL
GROUP BY ct.tag_id
ORDER BY ct.tag_id;

-- name: CreateOrganizationDomain :one
-- CreateOrganizationDomain stores a domain the organization means to
-- prove it owns.
INSERT INTO organization_domains (organization_id, domain, verification_token, created_at, updated_at)
VALUES (@organization_id, @domain, @verification_token, @now, @now)
RETURNING *;

-- name: CountOrganizationDomains :one
-- CountOrganizationDomains counts the organization's domains, to keep
-- them under the limit.
SELECT count(*) FROM organization_domains WHERE organization_id = @organization_id;

-- name: ListOrganizationDomains :many
-- ListOrganizationDomains returns the organization's domains by name.
SELECT * FROM organization_domains WHERE organization_id = @organization_id ORDER BY domain, id;

-- name: FindOrganizationDomain :one
-- FindOrganizationDomain returns the organization's domain with the id.
SELECT * FROM organization_domains WHERE organization_id = @organization_id AND id = @id;

-- name: LockOrganizationDomain :one
-- LockOrganizationDomain returns the organization's domain and holds its
-- row until the transaction ends.
SELECT * FROM organization_domains WHERE organization_id = @organization_id AND id = @id FOR UPDATE;

-- name: FindOrganizationDomainByName :one
-- FindOrganizationDomainByName returns the organization's row for the
-- domain.
SELECT * FROM organization_domains WHERE organization_id = @organization_id AND domain = @domain;

-- name: MarkOrganizationDomainChecked :one
-- MarkOrganizationDomainChecked records a check of the domain: verified,
-- still pending, or failing since a time.
UPDATE organization_domains
SET status = @status, verified_at = sqlc.narg('verified_at'), verified_via = sqlc.narg('verified_via'),
    last_checked_at = @now::timestamptz, failed_since = sqlc.narg('failed_since'), updated_at = @now::timestamptz
WHERE organization_id = @organization_id AND id = @id
RETURNING *;

-- name: RouteOrganizationDomain :one
-- RouteOrganizationDomain sends sign-ins under the domain to the
-- connection, or to none.
UPDATE organization_domains SET sso_connection_id = sqlc.narg('sso_connection_id'), updated_at = @now
WHERE organization_id = @organization_id AND id = @id
RETURNING *;

-- name: DeleteOrganizationDomain :execrows
-- DeleteOrganizationDomain removes the domain.
DELETE FROM organization_domains WHERE organization_id = @organization_id AND id = @id;

-- name: CountVerifiedDomainsForConnection :one
-- CountVerifiedDomainsForConnection counts the verified domains routed
-- to the connection, which it needs before it may go active.
SELECT count(*) FROM organization_domains
WHERE organization_id = @organization_id AND sso_connection_id = @sso_connection_id AND status = 'verified';

-- name: ListDomainsToRecheck :many
-- ListDomainsToRecheck returns verified and failing domains proven by DNS,
-- checked again every day.
SELECT * FROM organization_domains WHERE verified_via = 'dns' AND status IN ('verified', 'failed') ORDER BY id;

-- name: FindPendingDomainByName :many
-- FindPendingDomainByName returns every organization's unverified row for
-- the domain, for the operator's command.
SELECT d.*, w.slug AS workspace_slug, w.id AS workspace_id
FROM organization_domains d
JOIN workspaces w ON w.organization_id = d.organization_id AND w.deleted_at IS NULL
WHERE d.domain = @domain AND d.status <> 'verified'
ORDER BY d.created_at;

-- name: UnenforceSSOConnection :exec
-- UnenforceSSOConnection stops enforcing the connection.
UPDATE sso_connections SET enforced = false, updated_at = @now WHERE organization_id = @organization_id AND id = @id;

-- name: ListOrganizationOwners :many
-- ListOrganizationOwners returns every active owner of the organization's
-- live workspaces, with the workspace they own, across workspaces.
SELECT
    o.id::uuid AS id,
    o.name::text AS name,
    o.email::citext AS email,
    coalesce(o.locale, '')::text AS locale,
    o.workspace_id::uuid AS workspace_id
FROM auth_organization_owners(@organization_id::uuid) AS o;

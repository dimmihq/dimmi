-- name: FindHelpCenterDomain :one
-- FindHelpCenterDomain returns the domain a help center answers on, if any.
SELECT * FROM help_center_domains WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id;

-- name: FindHelpCenterDomainForUpdate :one
-- FindHelpCenterDomainForUpdate locks a domain for its check.
SELECT * FROM help_center_domains WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: CreateHelpCenterDomain :one
-- CreateHelpCenterDomain stores a domain to verify, due a check at once.
INSERT INTO help_center_domains (
    id, workspace_id, help_center_id, hostname, verification_token, next_check_at, created_at, updated_at
) VALUES (
    @id, @workspace_id, @help_center_id, @hostname, @verification_token, @now::timestamptz, @now::timestamptz,
    @now::timestamptz
)
RETURNING *;

-- name: SaveHelpCenterDomainCheck :one
-- SaveHelpCenterDomainCheck stores what a check found and when to look
-- again.
UPDATE help_center_domains SET
    status = @status, failure_reason = sqlc.narg(failure_reason), check_count = @check_count,
    first_checked_at = sqlc.narg(first_checked_at), last_checked_at = sqlc.narg(last_checked_at),
    next_check_at = sqlc.narg(next_check_at), verified_at = sqlc.narg(verified_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RestartHelpCenterDomainChecks :one
-- RestartHelpCenterDomainChecks makes a domain due a check now, starting
-- its schedule over.
UPDATE help_center_domains SET
    status = CASE WHEN status = 'failed' THEN 'pending' ELSE status END, check_count = 0,
    first_checked_at = NULL, next_check_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteHelpCenterDomain :exec
-- DeleteHelpCenterDomain forgets a help center's domain.
DELETE FROM help_center_domains WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id;

-- name: SetHelpCenterCustomDomain :exec
-- SetHelpCenterCustomDomain names the verified domain a help center
-- answers on, or none, and moves its cache version on.
UPDATE help_centers SET custom_domain = sqlc.narg(custom_domain), cache_version = cache_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ResolveHelpCenterDomain :one
-- ResolveHelpCenterDomain finds the help center answering on a verified
-- domain, before any workspace is known.
SELECT r.workspace_id::uuid AS workspace_id, r.id::uuid AS id, r.subdomain::text AS subdomain
FROM help_center_resolve_domain(@hostname::citext) AS r;

-- name: HelpCenterDomainTaken :one
-- HelpCenterDomainTaken reports whether any workspace claimed the domain.
SELECT help_center_domain_taken(@hostname::citext)::boolean AS taken;

-- name: ListHelpCenterDomainsDue :many
-- ListHelpCenterDomainsDue returns the domains of any workspace due a
-- check.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM help_center_domains_due(@due_at::timestamptz, @row_limit::integer) AS d;

-- name: SoftDeleteHelpCenter :exec
-- SoftDeleteHelpCenter moves a help center that is not the default to
-- the trash, freeing its subdomain and domain.
UPDATE help_centers SET deleted_at = @now::timestamptz, custom_domain = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND NOT is_default AND deleted_at IS NULL;

-- name: RepointWidgetsHelpCenter :exec
-- RepointWidgetsHelpCenter moves the messengers showing one help center
-- to another.
UPDATE widget_settings SET help_center_id = @to_help_center_id, updated_at = @now
WHERE workspace_id = @workspace_id AND help_center_id = @from_help_center_id;

-- name: CountHelpCenters :one
-- CountHelpCenters counts the workspace's live help centers.
SELECT count(*) FROM help_centers WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: TrashHelpCenterArticles :many
-- TrashHelpCenterArticles moves every article of a help center to the
-- trash and returns their ids.
UPDATE articles SET deleted_at = @now::timestamptz, status = 'archived', published_version_id = NULL,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
RETURNING id;

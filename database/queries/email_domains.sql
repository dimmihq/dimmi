-- name: CreateEmailDomain :one
-- CreateEmailDomain adds a domain as pending.
INSERT INTO email_domains (
    workspace_id, domain, driver, ownership_token, dkim_selector, dkim_private_key_encrypted, dkim_tokens,
    dkim_signing_hosted_zone, mail_from_domain, provider_domain_id, provider_trusted_at, records,
    created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @domain, @driver, @ownership_token, sqlc.narg(dkim_selector), sqlc.narg(dkim_private_key_encrypted),
    @dkim_tokens, sqlc.narg(dkim_signing_hosted_zone), sqlc.narg(mail_from_domain), sqlc.narg(provider_domain_id),
    sqlc.narg(provider_trusted_at), @records, sqlc.narg(created_by_member_id), @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindEmailDomain :one
-- FindEmailDomain returns a live domain of the workspace.
SELECT * FROM email_domains WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindEmailDomainForUpdate :one
-- FindEmailDomainForUpdate returns a live domain and holds it, so two
-- checks of one domain run one after the other.
SELECT * FROM email_domains WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: FindEmailDomainByName :one
-- FindEmailDomainByName returns the workspace's live domain of that name.
SELECT * FROM email_domains WHERE workspace_id = @workspace_id AND domain = @domain AND deleted_at IS NULL;

-- name: ListEmailDomains :many
-- ListEmailDomains returns the workspace's live domains, by name.
SELECT * FROM email_domains WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY domain;

-- name: ListEmailDomainClaims :many
-- ListEmailDomainClaims returns every workspace's claim on the name.
SELECT * FROM email_domain_claims(@name::citext);

-- name: ListEmailDomainsDue :many
-- ListEmailDomainsDue returns the domains of any workspace due a check.
SELECT * FROM email_domains_due(@due_at::timestamptz, @row_limit::integer);

-- name: SaveEmailDomainCheck :one
-- SaveEmailDomainCheck stores the outcome of a check of the domain.
UPDATE email_domains
SET status = @status, records = @records, warnings = @warnings, check_attempts = @check_attempts,
    ses_identity_status = sqlc.narg(ses_identity_status), ownership_verified_at = sqlc.narg(ownership_verified_at),
    provider_trusted_at = sqlc.narg(provider_trusted_at), last_checked_at = sqlc.narg(last_checked_at),
    next_check_at = sqlc.narg(next_check_at), verified_at = sqlc.narg(verified_at),
    degraded_at = sqlc.narg(degraded_at), failed_at = sqlc.narg(failed_at), failure_reason = sqlc.narg(failure_reason),
    dkim_tokens = @dkim_tokens, dkim_signing_hosted_zone = sqlc.narg(dkim_signing_hosted_zone),
    provider_domain_id = sqlc.narg(provider_domain_id), driver = @driver,
    verifying_since = sqlc.narg(verifying_since), ses_dkim_restarted_at = sqlc.narg(ses_dkim_restarted_at),
    dkim_selector = sqlc.narg(dkim_selector), dkim_private_key_encrypted = sqlc.narg(dkim_private_key_encrypted),
    dkim_next_selector = sqlc.narg(dkim_next_selector),
    dkim_next_private_key_encrypted = sqlc.narg(dkim_next_private_key_encrypted),
    mail_from_domain = sqlc.narg(mail_from_domain), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UpdateEmailDomainSettings :one
-- UpdateEmailDomainSettings changes what an admin may change on a domain.
UPDATE email_domains
SET mail_from_domain = sqlc.narg(mail_from_domain), dns_host_override = sqlc.narg(dns_host_override),
    records = @records, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: StartEmailDomainRotation :one
-- StartEmailDomainRotation holds a new signing key until its record shows.
UPDATE email_domains
SET dkim_next_selector = @dkim_next_selector, dkim_next_private_key_encrypted = @dkim_next_private_key_encrypted,
    records = @records, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: MarkEmailDomainConnectApplied :exec
-- MarkEmailDomainConnectApplied notes the host's automatic setup started.
UPDATE email_domains SET domain_connect_applied_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteEmailDomain :exec
-- DeleteEmailDomain removes the domain softly: mail falls back to the
-- default sender at once and the row stays for the record.
UPDATE email_domains
SET deleted_at = @now::timestamptz, next_check_at = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: LinkEmailAddressesToDomain :exec
-- LinkEmailAddressesToDomain ties every live address on exactly this
-- domain to it, so sending from them is proven by the domain's row.
UPDATE email_addresses
SET email_domain_id = @email_domain_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND split_part(address::text, '@', 2) = @domain::text;

-- name: UnlinkEmailAddressesFromDomain :exec
-- UnlinkEmailAddressesFromDomain makes the domain's addresses fall back
-- to the default sender.
UPDATE email_addresses SET email_domain_id = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND email_domain_id = @email_domain_id;

-- name: ListEmailDomainAddresses :many
-- ListEmailDomainAddresses returns the live addresses sending from it.
SELECT address::text FROM email_addresses
WHERE workspace_id = @workspace_id AND email_domain_id = @email_domain_id AND deleted_at IS NULL
ORDER BY address;

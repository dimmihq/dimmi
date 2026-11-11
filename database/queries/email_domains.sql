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
SELECT c.workspace_id::uuid AS workspace_id, c.id::uuid AS id, c.status::text AS status,
    c.ownership_token::text AS ownership_token
FROM email_domain_claims(@name::citext) AS c;

-- name: ListEmailDomainsDue :many
-- ListEmailDomainsDue returns the domains of any workspace due a check.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM email_domains_due(@due_at::timestamptz, @row_limit::integer) AS d;

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

-- name: ListEmailDomainSenders :many
-- ListEmailDomainSenders returns the broadcasts still due to send and the
-- series still running from the domain's addresses, by kind and name.
SELECT 'broadcast'::text AS kind, b.name::text AS name FROM broadcasts b
JOIN email_addresses a ON a.id = b.email_address_id
WHERE b.workspace_id = @workspace_id AND a.email_domain_id = @email_domain_id AND b.deleted_at IS NULL
  AND b.kind = 'one_off' AND b.status IN ('scheduled', 'sending', 'paused')
UNION ALL
SELECT 'series'::text AS kind, s.name::text AS name FROM series s
JOIN email_addresses a ON a.id = s.email_address_id
WHERE s.workspace_id = @workspace_id AND a.email_domain_id = @email_domain_id AND s.status IN ('live', 'paused')
ORDER BY kind, name;

-- name: RequestEmailDomainCheck :one
-- RequestEmailDomainCheck starts or keeps verifying the domain after
-- someone asked. A pending or failed domain starts a fresh window.
UPDATE email_domains
SET status = CASE WHEN status IN ('pending', 'failed') THEN 'verifying' ELSE status END,
    verifying_since = CASE WHEN status IN ('pending', 'failed') THEN @now::timestamptz ELSE verifying_since END,
    check_attempts = CASE WHEN status IN ('pending', 'failed') THEN 0 ELSE check_attempts END,
    failed_at = CASE WHEN status = 'failed' THEN NULL ELSE failed_at END,
    failure_reason = CASE WHEN status = 'failed' THEN NULL ELSE failure_reason END,
    check_requested_at = @now::timestamptz, next_check_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: FinishEmailDomainRotation :one
-- FinishEmailDomainRotation makes the next key the signing key once its
-- record shows, keeping the retired one for a week.
UPDATE email_domains
SET dkim_selector = dkim_next_selector, dkim_private_key_encrypted = dkim_next_private_key_encrypted,
    dkim_next_selector = NULL, dkim_next_private_key_encrypted = NULL, dkim_rotated_at = @now::timestamptz,
    records = @records, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND dkim_next_selector IS NOT NULL
RETURNING *;

-- name: FailEmailDomainClaims :many
-- FailEmailDomainClaims fails every other workspace's hold on the name.
SELECT f.workspace_id::uuid AS workspace_id, f.id::uuid AS id, f.status::text AS status
FROM fail_email_domain_claims(@name::citext, @keep_workspace::uuid, @reason::text, @at::timestamptz) AS f;

-- name: FindVerifiedEmailDomain :one
-- FindVerifiedEmailDomain returns the workspace's verified domain of that
-- exact name, the proof that mail may leave from an address on it.
SELECT * FROM email_domains
WHERE workspace_id = @workspace_id AND domain = @domain AND status = 'verified' AND deleted_at IS NULL;

-- name: ListEmailDomainStates :many
-- ListEmailDomainStates returns the name and state of each live domain,
-- for telling every address of the workspace how it sends.
SELECT domain::text AS domain, status FROM email_domains
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

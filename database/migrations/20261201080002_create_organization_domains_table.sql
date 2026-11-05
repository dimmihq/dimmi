-- migrate: phase=expand
-- lint:org-scoped
-- A domain a company proves it owns, by a TXT record or, on its own
-- install, by the operator's word. Only a verified domain routes sign-ins
-- to a connection, and a domain is verified for one organization at a
-- time: the first to prove it. Its row stays per organization, so a
-- second company may try and fail without disturbing the first.

-- +goose Up
CREATE TABLE organization_domains (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id uuid NOT NULL REFERENCES organizations ON DELETE CASCADE,
    domain citext NOT NULL,
    verification_token text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'failed')),
    verified_at timestamptz,
    last_checked_at timestamptz,
    verified_via text CHECK (verified_via IN ('dns', 'cli')),
    sso_connection_id uuid REFERENCES sso_connections ON DELETE SET NULL,
    failed_since timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX organization_domains_organization_id_domain_key ON organization_domains (organization_id, domain);
CREATE UNIQUE INDEX organization_domains_verified_domain_key ON organization_domains (domain) WHERE status = 'verified';
CREATE INDEX organization_domains_sso_connection_id_index ON organization_domains (sso_connection_id);

-- +goose Down
DROP TABLE organization_domains;

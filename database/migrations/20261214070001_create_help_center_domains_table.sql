-- migrate: phase=expand
-- A help center may answer on a domain the workspace owns, such as
-- help.acme.com. The team adds the domain, points it at the install with
-- a CNAME and proves it owns it with a TXT record holding a token only
-- it was given. A background check looks again and again, less often as
-- time passes, until both hold or it gives up; the reason it last failed
-- is kept for the team to fix. Once verified, the help center's
-- custom_domain names it, which is what routing, certificates and
-- canonical links read. Requests arrive by host before any workspace is
-- known, so those lookups run as security definer functions.

-- +goose Up
CREATE TABLE help_center_domains (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    hostname citext NOT NULL CHECK (char_length(hostname) BETWEEN 4 AND 253),
    verification_token text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'failed')),
    failure_reason text,
    check_count integer NOT NULL DEFAULT 0,
    first_checked_at timestamptz,
    last_checked_at timestamptz,
    next_check_at timestamptz,
    verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX help_center_domains_hostname_key ON help_center_domains (hostname);
CREATE UNIQUE INDEX help_center_domains_help_center_id_key ON help_center_domains (help_center_id);
CREATE INDEX help_center_domains_next_check_at_index ON help_center_domains (next_check_at)
    WHERE next_check_at IS NOT NULL;

ALTER TABLE help_center_domains ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_center_domains_tenant ON help_center_domains
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE help_centers ADD COLUMN custom_domain citext;

CREATE FUNCTION help_center_resolve_domain(wanted citext)
    RETURNS TABLE (workspace_id uuid, id uuid, subdomain citext)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT h.workspace_id, h.id, h.subdomain
        FROM help_centers h
        JOIN workspaces w ON w.id = h.workspace_id
        WHERE h.custom_domain = wanted AND h.deleted_at IS NULL AND h.status = 'active' AND w.deleted_at IS NULL
    $$;

CREATE FUNCTION help_center_domain_taken(wanted citext) RETURNS boolean
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$ SELECT EXISTS (SELECT 1 FROM help_center_domains WHERE hostname = wanted) $$;

CREATE FUNCTION help_center_domains_due(due_at timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT d.workspace_id, d.id
        FROM help_center_domains d
        JOIN workspaces w ON w.id = d.workspace_id
        WHERE d.next_check_at <= due_at AND w.deleted_at IS NULL
        ORDER BY d.next_check_at
        LIMIT row_limit
    $$;

-- +goose Down
DROP FUNCTION help_center_domains_due(timestamptz, integer);
DROP FUNCTION help_center_domain_taken(citext);
DROP FUNCTION help_center_resolve_domain(citext);
ALTER TABLE help_centers DROP COLUMN custom_domain;
DROP TABLE help_center_domains;

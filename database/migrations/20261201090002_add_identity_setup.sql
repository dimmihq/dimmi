-- migrate: phase=expand
-- Signing in visitors grows three ways. A secret can point at an auth
-- provider's published key set (Clerk, Auth0, Supabase) instead of
-- holding a key, so the host never signs anything itself; the set is
-- cached on the row so every replica shares one copy; the checks that
-- let such rows in follow in their own migration. Each workspace picks how
-- strict sign-in is: off, optional or required. Daily counts of how
-- sign-ins went tell the owner whether switching to required would lock
-- real customers out; they hold no per-person data.

-- +goose Up
ALTER TABLE identity_secrets
    ADD COLUMN jwks_url text,
    ADD COLUMN jwks_issuer text,
    ADD COLUMN audience text,
    ADD COLUMN jwks_cache jsonb,
    ADD COLUMN jwks_fetched_at timestamptz,
    ADD COLUMN jwks_error text;

CREATE TABLE identity_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    mode text NOT NULL DEFAULT 'optional' CHECK (mode IN ('off', 'optional', 'required')),
    client_writable_attributes text[] NOT NULL DEFAULT '{}',
    updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE identity_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY identity_settings_tenant ON identity_settings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE identity_verification_stats (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    day date NOT NULL,
    outcome text NOT NULL,
    count bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, day, outcome)
);

ALTER TABLE identity_verification_stats ENABLE ROW LEVEL SECURITY;
CREATE POLICY identity_verification_stats_tenant ON identity_verification_stats
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- identity_secret_exists tells the token checker that a key id belongs
-- to some other workspace, without saying which, so a developer who
-- pasted another workspace's secret hears why it fails.
CREATE FUNCTION identity_secret_exists(secret_id uuid) RETURNS boolean
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public AS $$
    SELECT EXISTS (SELECT 1 FROM identity_secrets WHERE id = secret_id)
$$;

-- identity_jwks_due finds the provider key sets across every workspace
-- whose cached copy is older than the given time, for the refresh round.
CREATE FUNCTION identity_jwks_due(stale_before timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT s.workspace_id, s.id FROM identity_secrets s
        WHERE s.jwks_url IS NOT NULL AND s.status <> 'revoked'
          AND (s.jwks_fetched_at IS NULL OR s.jwks_fetched_at < stale_before)
        ORDER BY s.jwks_fetched_at NULLS FIRST, s.id
        LIMIT max_rows
    $$;

-- identity_stats_prune drops every workspace's sign-in counts older
-- than the given day.
CREATE FUNCTION identity_stats_prune(before date) RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH gone AS (DELETE FROM identity_verification_stats WHERE day < before RETURNING 1)
        SELECT count(*) FROM gone
    $$;

-- +goose Down
DROP FUNCTION identity_stats_prune(date);
DROP FUNCTION identity_jwks_due(timestamptz, integer);
DROP FUNCTION identity_secret_exists(uuid);
DROP TABLE identity_verification_stats;
DROP TABLE identity_settings;
ALTER TABLE identity_secrets
    DROP COLUMN jwks_error,
    DROP COLUMN jwks_fetched_at,
    DROP COLUMN jwks_cache,
    DROP COLUMN audience,
    DROP COLUMN jwks_issuer,
    DROP COLUMN jwks_url;

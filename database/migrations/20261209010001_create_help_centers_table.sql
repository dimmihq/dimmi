-- migrate: phase=expand
-- A help center is a workspace's public docs: a name, an address under
-- the install's help domain, a language and how it looks. One per
-- workspace is the default, which the agent and new messengers use. A
-- disabled help center keeps its content but answers nothing publicly.
-- cache_version moves on every change a reader could see, so every
-- replica's page cache knows when to let go. Readers reach a help center
-- by its subdomain before any workspace is known, so that one lookup
-- runs as a security definer function and returns only ids.

-- +goose Up
CREATE TABLE help_centers (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    subdomain citext NOT NULL CHECK (subdomain ~ '^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])$'),
    locale text NOT NULL DEFAULT 'en',
    is_default boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    settings jsonb NOT NULL DEFAULT '{}',
    indexing_enabled boolean NOT NULL DEFAULT true,
    ask_ai_enabled boolean NOT NULL DEFAULT true,
    cache_version bigint NOT NULL DEFAULT 1,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX help_centers_workspace_id_created_at_index ON help_centers (workspace_id, created_at);
CREATE UNIQUE INDEX help_centers_subdomain_key ON help_centers (subdomain) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX help_centers_default_key ON help_centers (workspace_id) WHERE is_default AND deleted_at IS NULL;

ALTER TABLE help_centers ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_centers_tenant ON help_centers USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE FUNCTION help_center_resolve(wanted citext)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT h.workspace_id, h.id
        FROM help_centers h
        JOIN workspaces w ON w.id = h.workspace_id
        WHERE h.subdomain = wanted AND h.deleted_at IS NULL AND h.status = 'active' AND w.deleted_at IS NULL
    $$;

CREATE FUNCTION help_center_subdomain_taken(wanted citext) RETURNS boolean
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$ SELECT EXISTS (SELECT 1 FROM help_centers WHERE subdomain = wanted AND deleted_at IS NULL) $$;

-- +goose Down
DROP FUNCTION help_center_subdomain_taken(citext);
DROP FUNCTION help_center_resolve(citext);
DROP TABLE help_centers;

-- migrate: phase=expand
-- lint:global
-- A workspace is the tenant: every tenant row carries its id and every
-- row level security policy compares against it. The organization is
-- not cascaded, because an organization outlives its workspaces for as
-- long as billing records must be kept.
--
-- dimmi_visible_workspaces is the one place that says which workspaces
-- the current transaction may see. Every tenant policy calls it, so
-- letting a transaction read several workspaces later is a change to
-- this function, not to every policy. An unset tenant sees nothing. It
-- reads the setting the database layer sets at the start of every
-- transaction, app.tenant_id, so it works without configuration.

-- +goose Up
CREATE TABLE workspaces (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id uuid NOT NULL REFERENCES organizations,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    slug citext NOT NULL,
    timezone text NOT NULL DEFAULT 'UTC',
    locale text NOT NULL DEFAULT 'en',
    settings jsonb NOT NULL DEFAULT '{}',
    test_mode boolean NOT NULL DEFAULT false,
    next_conversation_number bigint NOT NULL DEFAULT 1,
    suspended_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspaces_slug_key UNIQUE (slug)
);

CREATE INDEX workspaces_organization_id_index ON workspaces (organization_id);

CREATE FUNCTION dimmi_visible_workspaces() RETURNS uuid[]
    LANGUAGE sql STABLE
    AS $$ SELECT ARRAY[nullif(current_setting('app.tenant_id', true), '')::uuid] $$;

-- +goose Down
DROP FUNCTION dimmi_visible_workspaces();
DROP TABLE workspaces;

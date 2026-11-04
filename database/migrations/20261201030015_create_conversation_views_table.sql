-- migrate: phase=expand
-- A saved view is a named list of conversations described by conditions,
-- shared with the workspace or kept by one member for themselves. The
-- list is computed each time it opens, never stored. Shared views keep
-- the order the team gave them; names are unique within a scope and an
-- owner, deleted views aside.

-- +goose Up
CREATE TABLE conversation_views (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    icon text,
    scope text NOT NULL CHECK (scope IN ('shared', 'personal')),
    owner_member_id uuid REFERENCES workspace_members ON DELETE CASCADE,
    definition jsonb NOT NULL,
    sort text,
    status_filter text CHECK (status_filter IN ('open', 'snoozed', 'closed', 'any')),
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT conversation_views_owner_check
        CHECK ((scope = 'personal') = (owner_member_id IS NOT NULL))
);

CREATE UNIQUE INDEX conversation_views_workspace_id_scope_owner_name_key
    ON conversation_views (workspace_id, scope, owner_member_id, name) NULLS NOT DISTINCT
    WHERE deleted_at IS NULL;
CREATE INDEX conversation_views_workspace_id_scope_owner_index
    ON conversation_views (workspace_id, scope, owner_member_id);

ALTER TABLE conversation_views ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_views_tenant ON conversation_views USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_views;

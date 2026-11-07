-- migrate: phase=expand
-- Collections group a help center's articles, two levels deep at most: a
-- collection and its sub-collections. Positions leave gaps so a move
-- touches one row. A deleted collection stays in the trash; its articles
-- simply become uncollected.

-- +goose Up
CREATE TABLE collections (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    parent_id uuid REFERENCES collections ON DELETE RESTRICT,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,80}$'),
    description text CHECK (char_length(description) <= 300),
    icon text,
    position integer NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX collections_workspace_id_help_center_id_parent_id_position_index
    ON collections (workspace_id, help_center_id, parent_id, position);
CREATE UNIQUE INDEX collections_help_center_id_slug_key ON collections (help_center_id, slug) WHERE deleted_at IS NULL;

ALTER TABLE collections ENABLE ROW LEVEL SECURITY;
CREATE POLICY collections_tenant ON collections USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE collections;

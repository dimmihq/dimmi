-- migrate: phase=expand
-- A help center may wear its team's own theme. Each upload of a theme is
-- a version of its own, kept whole and never changed, so going back to
-- an earlier one is pointing at it again. Only a theme that loaded with
-- no problems is stored. Its files live beside it, the templates as text
-- and the assets as they came. settings holds the team's values for the
-- settings theme.json declares. help_centers.theme_id is the version its
-- pages wear; empty, they wear the default theme.

-- +goose Up
CREATE TABLE help_center_themes (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    manifest jsonb NOT NULL,
    content_hash text NOT NULL,
    file_count integer NOT NULL CHECK (file_count BETWEEN 1 AND 1000),
    size_bytes integer NOT NULL CHECK (size_bytes BETWEEN 1 AND 26214400),
    settings jsonb NOT NULL DEFAULT '{}',
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (help_center_id, version)
);

CREATE TABLE help_center_theme_files (
    theme_id uuid NOT NULL REFERENCES help_center_themes ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    path text NOT NULL CHECK (char_length(path) BETWEEN 1 AND 512),
    body bytea NOT NULL CHECK (octet_length(body) <= 5242880),
    PRIMARY KEY (theme_id, path)
);

ALTER TABLE help_center_themes ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_center_themes_tenant ON help_center_themes
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE help_center_theme_files ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_center_theme_files_tenant ON help_center_theme_files
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE help_centers ADD COLUMN theme_id uuid REFERENCES help_center_themes ON DELETE SET NULL;

-- +goose Down
ALTER TABLE help_centers DROP COLUMN theme_id;
DROP TABLE help_center_theme_files;
DROP TABLE help_center_themes;

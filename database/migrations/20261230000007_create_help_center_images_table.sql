-- migrate: phase=expand
-- A picture placed in an article is a file on the default disk, served
-- by its name alone. This table says which workspace kept each one, so a
-- purged workspace takes its pictures with it. Pictures kept before the
-- table existed are found by the articles that show them.

-- +goose Up
CREATE TABLE help_center_images (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    path text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT help_center_images_path_key UNIQUE (path)
);

CREATE INDEX help_center_images_workspace_id_index ON help_center_images (workspace_id);

ALTER TABLE help_center_images ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_center_images_tenant ON help_center_images
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

INSERT INTO help_center_images (workspace_id, path)
SELECT DISTINCT ON (found.path) found.workspace_id, found.path
FROM (
    SELECT workspace_id, 'help-images/' || (regexp_matches(body_markdown, '/help-images/([A-Za-z0-9._-]+)', 'g'))[1] AS path
    FROM articles
    UNION ALL
    SELECT workspace_id, 'help-images/' || (regexp_matches(body_markdown, '/help-images/([A-Za-z0-9._-]+)', 'g'))[1]
    FROM article_versions
) AS found
ORDER BY found.path, found.workspace_id
ON CONFLICT (path) DO NOTHING;

-- +goose Down
DROP TABLE help_center_images;

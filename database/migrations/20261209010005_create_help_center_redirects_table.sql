-- migrate: phase=expand
-- Old addresses keep working: when an article or collection changes its
-- slug, its old path answers with a permanent redirect to the new one.
-- Each redirect points at exactly one target.

-- +goose Up
CREATE TABLE help_center_redirects (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    from_path text NOT NULL CHECK (char_length(from_path) BETWEEN 1 AND 512),
    target_type text NOT NULL CHECK (target_type IN ('article', 'collection', 'url')),
    target_article_id uuid REFERENCES articles ON DELETE CASCADE,
    target_collection_id uuid REFERENCES collections ON DELETE CASCADE,
    target_url text CHECK (target_url LIKE 'https://%'),
    source text NOT NULL CHECK (source IN ('slug_change', 'manual', 'import')),
    hits bigint NOT NULL DEFAULT 0,
    last_hit_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (help_center_id, from_path),
    CHECK (num_nonnulls(target_article_id, target_collection_id, target_url) = 1)
);

ALTER TABLE help_center_redirects ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_center_redirects_tenant ON help_center_redirects
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE help_center_redirects;

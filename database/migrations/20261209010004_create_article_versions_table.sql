-- migrate: phase=expand
-- A version is a frozen copy of an article as it was published, with
-- its HTML and table of contents rendered once, so a page is served
-- without rendering anything. Versions are never edited, only rendered
-- again when the renderer changes. The search document is computed by
-- the database from the frozen text, so a publish is searchable in the
-- same transaction.

-- +goose Up
CREATE TABLE article_versions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    article_id uuid NOT NULL REFERENCES articles ON DELETE CASCADE,
    number integer NOT NULL,
    kind text NOT NULL CHECK (kind IN ('published', 'snapshot', 'restored', 'suggestion')),
    title text NOT NULL,
    slug citext NOT NULL,
    description text,
    body_markdown text NOT NULL,
    body_html text NOT NULL,
    toc jsonb NOT NULL DEFAULT '[]',
    visibility text NOT NULL,
    collection_id uuid,
    content_hash text NOT NULL,
    created_by_type text NOT NULL,
    created_by_id uuid,
    renderer_version smallint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    search_document tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('dimmi_search', title), 'A')
        || setweight(to_tsvector('dimmi_search', coalesce(description, '')), 'B')
        || setweight(to_tsvector('dimmi_search', body_markdown), 'C')
    ) STORED,
    UNIQUE (article_id, number)
);

CREATE INDEX article_versions_workspace_id_article_id_number_index
    ON article_versions (workspace_id, article_id, number DESC);
CREATE INDEX article_versions_search_document_index ON article_versions USING gin (workspace_id, search_document);

ALTER TABLE article_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY article_versions_tenant ON article_versions USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE articles ADD CONSTRAINT articles_published_version_id_fkey
    FOREIGN KEY (published_version_id) REFERENCES article_versions ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
ALTER TABLE articles DROP CONSTRAINT articles_published_version_id_fkey;
DROP TABLE article_versions;

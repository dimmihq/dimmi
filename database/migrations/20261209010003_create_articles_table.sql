-- migrate: phase=expand
-- An article is a working copy anyone allowed may edit, and immutable
-- versions readers see: publishing freezes the working copy into a
-- version and points the article at it, so editing never changes what is
-- live until the next publish. content_hash tells whether the working
-- copy differs from what is published. lock_version stops two editors
-- from overwriting each other. agent_only articles are never shown
-- publicly, but the agent may quote them to customers.

-- +goose Up
CREATE TABLE articles (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    collection_id uuid REFERENCES collections ON DELETE SET NULL,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 150),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,120}$'),
    description text CHECK (char_length(description) <= 300),
    body_markdown text NOT NULL DEFAULT '' CHECK (octet_length(body_markdown) <= 262144),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'agent_only')),
    published_version_id uuid,
    has_unpublished_changes boolean NOT NULL DEFAULT false,
    content_hash text NOT NULL,
    position integer NOT NULL,
    seo jsonb NOT NULL DEFAULT '{}',
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    last_edited_by_type text,
    last_edited_by_id uuid,
    lock_version integer NOT NULL DEFAULT 1,
    word_count integer NOT NULL DEFAULT 0,
    published_at timestamptz,
    first_published_at timestamptz,
    archived_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title_trgm text GENERATED ALWAYS AS (lower(dimmi_unaccent(title))) STORED
);

CREATE INDEX articles_workspace_id_help_center_id_collection_id_position_index
    ON articles (workspace_id, help_center_id, collection_id, position);
CREATE INDEX articles_workspace_id_status_updated_at_index ON articles (workspace_id, status, updated_at DESC);
CREATE INDEX articles_title_trgm_index ON articles USING gin (workspace_id, title_trgm gin_trgm_ops);
CREATE UNIQUE INDEX articles_help_center_id_slug_key ON articles (help_center_id, slug) WHERE deleted_at IS NULL;

ALTER TABLE articles ENABLE ROW LEVEL SECURITY;
CREATE POLICY articles_tenant ON articles USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE articles;

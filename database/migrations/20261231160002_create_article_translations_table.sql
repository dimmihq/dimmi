-- migrate: phase=expand
-- An article is written in its help center's default language, on the
-- articles row, which stays the source. Each other language gets a
-- translation: its own title, slug, description and body, a working
-- copy saved apart from what is live, its own draft or published state,
-- author and versions. Translation versions live beside the source's in
-- article_versions, marked with their language, so search and the
-- agent's index read them the same way. source_hash is the source text
-- the translation last caught up with: when the source is published
-- with other words, the translation is outdated until it is published
-- again or marked up to date.
--
-- Collections get their translated name, slug and description the same
-- way. A redirect may lead to an article or collection in one language.

-- +goose Up
CREATE FUNCTION help_text_hash(title text, description text, body text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $$ SELECT md5(coalesce(title, '') || E'\x1f' || coalesce(description, '') || E'\x1f' || coalesce(body, '')) $$;

CREATE TABLE article_translations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    article_id uuid NOT NULL REFERENCES articles ON DELETE CASCADE,
    locale text NOT NULL CHECK (locale ~ '^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$'),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 150),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,120}$'),
    description text CHECK (char_length(description) <= 300),
    body_markdown text NOT NULL DEFAULT '' CHECK (octet_length(body_markdown) <= 262144),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_version_id uuid REFERENCES article_versions ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED,
    has_unpublished_changes boolean NOT NULL DEFAULT true,
    content_hash text NOT NULL,
    source_hash text,
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    last_edited_by_type text,
    last_edited_by_id uuid,
    lock_version integer NOT NULL DEFAULT 1,
    word_count integer NOT NULL DEFAULT 0,
    published_at timestamptz,
    first_published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX article_translations_article_id_locale_key ON article_translations (article_id, locale);
CREATE UNIQUE INDEX article_translations_help_center_id_locale_slug_key
    ON article_translations (help_center_id, locale, slug);
CREATE INDEX article_translations_workspace_id_help_center_id_locale_index
    ON article_translations (workspace_id, help_center_id, locale);

ALTER TABLE article_translations ENABLE ROW LEVEL SECURITY;
CREATE POLICY article_translations_tenant ON article_translations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE collection_translations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    collection_id uuid NOT NULL REFERENCES collections ON DELETE CASCADE,
    locale text NOT NULL CHECK (locale ~ '^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,80}$'),
    description text CHECK (char_length(description) <= 300),
    source_hash text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX collection_translations_collection_id_locale_key
    ON collection_translations (collection_id, locale);
CREATE UNIQUE INDEX collection_translations_help_center_id_locale_slug_key
    ON collection_translations (help_center_id, locale, slug);
CREATE INDEX collection_translations_workspace_id_help_center_id_locale_index
    ON collection_translations (workspace_id, help_center_id, locale);

ALTER TABLE collection_translations ENABLE ROW LEVEL SECURITY;
CREATE POLICY collection_translations_tenant ON collection_translations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE article_versions ADD COLUMN locale text;
ALTER TABLE knowledge_chunks ADD COLUMN locale text;
ALTER TABLE help_center_redirects ADD COLUMN target_locale text;

-- +goose Down
ALTER TABLE help_center_redirects DROP COLUMN target_locale;
ALTER TABLE knowledge_chunks DROP COLUMN locale;
ALTER TABLE article_versions DROP COLUMN locale;
DROP TABLE collection_translations;
DROP TABLE article_translations;
DROP FUNCTION help_text_hash(text, text, text);

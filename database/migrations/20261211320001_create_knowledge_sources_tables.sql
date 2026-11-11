-- migrate: phase=expand
-- Knowledge the workspace keeps outside its help center: a website, a
-- repository, an uploaded archive, or another help center tool. Each
-- source is read by one driver and either synced, a read-only copy the
-- agent and the Operator answer from and that is refreshed on a
-- schedule, or imported once into editable help center articles. A
-- sync run discovers the source's documents, fetches only what changed
-- since the last run, and removes what a complete run no longer saw;
-- its counters are kept so the last fifty runs explain themselves.
-- Documents are cut into chunks of their own, embedded through the same
-- vector store as articles, so unchanged text is never embedded twice.
-- Secrets a driver needs are sealed with the workspace's keys.

-- +goose Up
CREATE TABLE knowledge_sources (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    type text NOT NULL CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    mode text NOT NULL CHECK (mode IN ('sync', 'import')),
    targets text[] NOT NULL DEFAULT '{agent}'
        CHECK (targets <@ ARRAY['agent', 'operator', 'help_center']::text[]),
    help_center_id uuid REFERENCES help_centers ON DELETE SET NULL,
    config jsonb NOT NULL DEFAULT '{}',
    include text[] NOT NULL DEFAULT '{}',
    exclude text[] NOT NULL DEFAULT '{}',
    credentials_encrypted bytea,
    status text NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'queued', 'syncing', 'error', 'paused')),
    schedule text NOT NULL DEFAULT 'weekly' CHECK (schedule IN ('manual', 'hourly', 'daily', 'weekly')),
    next_sync_at timestamptz,
    last_synced_at timestamptz,
    last_success_at timestamptz,
    last_error_code text,
    last_error text,
    document_count integer NOT NULL DEFAULT 0,
    created_by_member_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX knowledge_sources_workspace_id_created_at_index ON knowledge_sources (workspace_id, created_at);
CREATE INDEX knowledge_sources_next_sync_at_index ON knowledge_sources (next_sync_at)
    WHERE status IN ('idle', 'error') AND schedule <> 'manual';

ALTER TABLE knowledge_sources ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_sources_tenant ON knowledge_sources
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_sync_runs (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    source_id uuid NOT NULL REFERENCES knowledge_sources ON DELETE CASCADE,
    trigger text NOT NULL CHECK (trigger IN ('create', 'manual', 'schedule', 'webhook', 'retry', 'document')),
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'partial', 'failed', 'canceled')),
    full_run boolean NOT NULL DEFAULT true,
    discovered boolean NOT NULL DEFAULT false,
    pending integer NOT NULL DEFAULT 0,
    stats jsonb NOT NULL DEFAULT '{}',
    error_code text,
    error text,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX knowledge_sync_runs_source_id_created_at_index ON knowledge_sync_runs (source_id, created_at DESC);

ALTER TABLE knowledge_sync_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_sync_runs_tenant ON knowledge_sync_runs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_documents (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    source_id uuid NOT NULL REFERENCES knowledge_sources ON DELETE CASCADE,
    external_id text NOT NULL CHECK (char_length(external_id) BETWEEN 1 AND 2048),
    url text,
    path text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    locale text NOT NULL DEFAULT '',
    translation_of text,
    collection jsonb NOT NULL DEFAULT '[]',
    version text NOT NULL DEFAULT '',
    content_markdown text NOT NULL DEFAULT '',
    content_hash text NOT NULL DEFAULT '',
    visibility text NOT NULL DEFAULT 'agent_only' CHECK (visibility IN ('public', 'agent_only')),
    draft boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'excluded', 'error', 'removed')),
    excluded_by text CHECK (excluded_by IN ('member', 'robots', 'pattern', 'js_rendered', 'duplicate', 'too_large',
        'unsupported_type', 'empty', 'draft')),
    error_code text,
    error text,
    assets jsonb NOT NULL DEFAULT '[]',
    search tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('dimmi_search', title), 'A') || to_tsvector('dimmi_search', content_markdown)
    ) STORED,
    linked_article_id uuid REFERENCES articles ON DELETE SET NULL,
    imported_hash text,
    last_seen_run_id uuid,
    fetched_at timestamptz,
    indexed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id, external_id)
);

CREATE INDEX knowledge_documents_workspace_id_source_id_status_index
    ON knowledge_documents (workspace_id, source_id, status);
CREATE INDEX knowledge_documents_search_index ON knowledge_documents USING gin (workspace_id, search);

ALTER TABLE knowledge_documents ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_documents_tenant ON knowledge_documents
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_document_chunks (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    document_id uuid NOT NULL REFERENCES knowledge_documents ON DELETE CASCADE,
    source_id uuid NOT NULL REFERENCES knowledge_sources ON DELETE CASCADE,
    ordinal integer NOT NULL,
    title text NOT NULL,
    heading_path text[] NOT NULL DEFAULT '{}',
    body text NOT NULL,
    visibility text NOT NULL CHECK (visibility IN ('public', 'agent_only')),
    embed_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, ordinal)
);

CREATE INDEX knowledge_document_chunks_workspace_id_embed_hash_index
    ON knowledge_document_chunks (workspace_id, embed_hash);

ALTER TABLE knowledge_document_chunks ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_document_chunks_tenant ON knowledge_document_chunks
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE knowledge_document_chunks;
DROP TABLE knowledge_documents;
DROP TABLE knowledge_sync_runs;
DROP TABLE knowledge_sources;

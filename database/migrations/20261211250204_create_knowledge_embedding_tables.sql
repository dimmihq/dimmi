-- migrate: phase=expand
-- Semantic search over a workspace's knowledge. Each live published
-- article version is cut into chunks, and every chunk's embed input is
-- hashed; a vector is stored once per workspace, model and hash, so
-- content that did not change is never embedded again and switching
-- back to a recent model costs nothing. Vectors are half precision; the
-- partial HNSW index serves the 1024-dimension models, others are
-- scanned exactly. A workspace binds the one model its queries are
-- embedded with in its knowledge settings; a model being built for a
-- switch is the pending one. Inputs a provider refused are remembered so
-- the nightly sweep retries them. A closed conversation is kept as one
-- searchable text whose vector shares the same store, so similar
-- conversations come from the same hybrid search.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE knowledge_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    active_model_key text,
    embedding_key_id uuid,
    pending_model_key text,
    pending_key_id uuid,
    previous_model_key text,
    embeddings_status text NOT NULL DEFAULT 'none'
        CHECK (embeddings_status IN ('none', 'ready', 'building', 'degraded')),
    embeddings_error text,
    reembed_total integer NOT NULL DEFAULT 0,
    reembed_done integer NOT NULL DEFAULT 0,
    reembed_failed integer NOT NULL DEFAULT 0,
    reembed_started_at timestamptz,
    switched_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE knowledge_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_settings_tenant ON knowledge_settings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_chunks (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    article_id uuid NOT NULL REFERENCES articles ON DELETE CASCADE,
    article_version_id uuid NOT NULL REFERENCES article_versions ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    ordinal integer NOT NULL,
    title text NOT NULL,
    heading_path text[] NOT NULL DEFAULT '{}',
    body text NOT NULL,
    visibility text NOT NULL CHECK (visibility IN ('public', 'agent_only')),
    embed_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX knowledge_chunks_article_version_id_ordinal_key ON knowledge_chunks (article_version_id, ordinal);
CREATE INDEX knowledge_chunks_workspace_id_article_id_index ON knowledge_chunks (workspace_id, article_id);
CREATE INDEX knowledge_chunks_workspace_id_embed_hash_index ON knowledge_chunks (workspace_id, embed_hash);

ALTER TABLE knowledge_chunks ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_chunks_tenant ON knowledge_chunks
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_embeddings (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    model_key text NOT NULL,
    embed_hash text NOT NULL,
    dims integer NOT NULL,
    embedding halfvec NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, model_key, embed_hash)
);

CREATE INDEX knowledge_embeddings_embedding_1024_index ON knowledge_embeddings
    USING hnsw ((embedding::halfvec(1024)) halfvec_cosine_ops) WHERE dims = 1024;

ALTER TABLE knowledge_embeddings ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_embeddings_tenant ON knowledge_embeddings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_embedding_failures (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    model_key text NOT NULL,
    embed_hash text NOT NULL,
    attempts integer NOT NULL DEFAULT 1,
    last_error text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, model_key, embed_hash)
);

ALTER TABLE knowledge_embedding_failures ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_embedding_failures_tenant ON knowledge_embedding_failures
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE knowledge_conversations (
    conversation_id uuid PRIMARY KEY REFERENCES conversations ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    body text NOT NULL,
    embed_hash text NOT NULL,
    search tsvector GENERATED ALWAYS AS (to_tsvector('dimmi_search', body)) STORED,
    closed_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX knowledge_conversations_workspace_id_closed_at_index
    ON knowledge_conversations (workspace_id, closed_at DESC);
CREATE INDEX knowledge_conversations_search_index ON knowledge_conversations USING gin (workspace_id, search);

ALTER TABLE knowledge_conversations ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_conversations_tenant ON knowledge_conversations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE knowledge_conversations;
DROP TABLE knowledge_embedding_failures;
DROP TABLE knowledge_embeddings;
DROP TABLE knowledge_chunks;
DROP TABLE knowledge_settings;

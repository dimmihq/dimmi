-- migrate: phase=expand
-- A workspace runs many agents, each edited as a draft and published as
-- a version, the way procedures are. The columns an agent already has
-- stay what customers get: the published config. A draft holds the
-- working copy until it is published, and is empty when nothing changed.
-- Each agent says what it may search, follow and call, and which other
-- agents it may hand a conversation to. One agent per workspace is the
-- default: it answers every conversation no other agent takes, and
-- cannot be deleted. Every agent a workspace has today becomes its
-- default, live if it was answering.

-- +goose Up
ALTER TABLE ai_agents
    ADD COLUMN is_default boolean NOT NULL DEFAULT false,
    ADD COLUMN role text NOT NULL DEFAULT 'support'
        CHECK (role IN ('support', 'sales', 'onboarding', 'ecommerce', 'billing', 'internal', 'developer', 'custom')),
    ADD COLUMN template_key text CHECK (char_length(template_key) <= 40),
    ADD COLUMN status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'live', 'paused')),
    ADD COLUMN lock_version integer NOT NULL DEFAULT 1,
    ADD COLUMN draft jsonb CHECK (octet_length(draft::text) <= 131072),
    ADD COLUMN knowledge_scope jsonb NOT NULL DEFAULT '{"mode": "all"}',
    ADD COLUMN procedure_scope jsonb NOT NULL DEFAULT '{"mode": "all"}',
    ADD COLUMN action_scope jsonb NOT NULL DEFAULT '{"mode": "all"}',
    ADD COLUMN transfer_agent_ids uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN published_at timestamptz,
    ADD COLUMN created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    ADD COLUMN updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL;

UPDATE ai_agents SET
    is_default = true,
    status = CASE WHEN enabled THEN 'live' ELSE 'draft' END,
    published_at = CASE WHEN enabled THEN updated_at END,
    monthly_spend_cap_micros = NULL
WHERE deleted_at IS NULL;

CREATE TABLE ai_agent_versions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES ai_agents ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    definition jsonb NOT NULL CHECK (octet_length(definition::text) <= 131072),
    change_note text CHECK (char_length(change_note) <= 500),
    published_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    published_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (agent_id, version)
);

CREATE INDEX ai_agent_versions_workspace_id_agent_id_index
    ON ai_agent_versions (workspace_id, agent_id, version DESC);

ALTER TABLE ai_agent_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_agent_versions_tenant ON ai_agent_versions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_agents ADD COLUMN live_version_id uuid REFERENCES ai_agent_versions ON DELETE SET NULL;

-- +goose Down
ALTER TABLE ai_agents DROP COLUMN live_version_id;
DROP TABLE ai_agent_versions;
ALTER TABLE ai_agents
    DROP COLUMN is_default,
    DROP COLUMN role,
    DROP COLUMN template_key,
    DROP COLUMN status,
    DROP COLUMN lock_version,
    DROP COLUMN draft,
    DROP COLUMN knowledge_scope,
    DROP COLUMN procedure_scope,
    DROP COLUMN action_scope,
    DROP COLUMN transfer_agent_ids,
    DROP COLUMN published_at,
    DROP COLUMN created_by_member_id,
    DROP COLUMN updated_by_member_id;

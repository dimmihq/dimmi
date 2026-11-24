-- migrate: phase=expand
-- An agent's simulations: whole chats it must keep getting right, each
-- with the customer it plays as, the messages the customer sends, what
-- actions answer instead of the team's systems, and the checks that
-- judge it. A simulation runs against the agent's draft or its live
-- version, and keeps how each last went, so the two can be compared
-- before publishing.

-- +goose Up
CREATE TABLE ai_agent_test_cases (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES ai_agents ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    source_conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    persona jsonb NOT NULL DEFAULT '{}',
    messages jsonb NOT NULL DEFAULT '[]',
    action_mocks jsonb NOT NULL DEFAULT '{}',
    assertions jsonb NOT NULL DEFAULT '[]',
    simulated_at timestamptz,
    draft_result jsonb,
    live_result jsonb,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ai_agent_test_cases_size_check CHECK (
        octet_length(persona::text) + octet_length(messages::text) + octet_length(action_mocks::text)
            + octet_length(assertions::text) <= 131072
    )
);

CREATE INDEX ai_agent_test_cases_workspace_id_agent_id_index
    ON ai_agent_test_cases (workspace_id, agent_id, created_at);

ALTER TABLE ai_agent_test_cases ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_agent_test_cases_tenant ON ai_agent_test_cases
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ai_agent_test_cases;

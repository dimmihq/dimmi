-- migrate: phase=expand
-- Saved tests of a procedure: a made-up customer, the messages they send,
-- what each action returns instead of calling the team's systems, and
-- what must hold after, such as an action being called or the branch
-- taken. Running them simulates the conversation against the working
-- copy, so a change is checked before it is published. A test made from
-- a real conversation or by Operator names it.

-- +goose Up
CREATE TABLE procedure_test_cases (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    procedure_id uuid NOT NULL REFERENCES procedures ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    source_conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    persona jsonb NOT NULL DEFAULT '{}',
    messages jsonb NOT NULL DEFAULT '[]',
    action_mocks jsonb NOT NULL DEFAULT '{}',
    assertions jsonb NOT NULL DEFAULT '[]',
    last_result jsonb,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT procedure_test_cases_size_check CHECK (
        octet_length(persona::text) + octet_length(messages::text) + octet_length(action_mocks::text)
            + octet_length(assertions::text) <= 131072
    )
);

CREATE INDEX procedure_test_cases_workspace_id_procedure_id_index
    ON procedure_test_cases (workspace_id, procedure_id, created_at);

ALTER TABLE procedure_test_cases ENABLE ROW LEVEL SECURITY;
CREATE POLICY procedure_test_cases_tenant ON procedure_test_cases
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE procedure_test_cases;

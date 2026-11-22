-- migrate: phase=expand
-- The procedure a conversation is following, kept across turns so dimmi
-- picks it up again on the customer's next message instead of starting
-- over: which steps are done, which actions ran, and what they returned
-- (masked), so later steps and rules can read it. A conversation follows
-- at most one procedure at a time; the row ends when the procedure is
-- done, the customer moves on, or a person takes over.

-- +goose Up
CREATE TABLE conversation_procedure_states (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    procedure_id uuid NOT NULL REFERENCES procedures ON DELETE CASCADE,
    procedure_version_id uuid,
    status text NOT NULL CHECK (status IN ('active', 'completed', 'handed_off', 'exited')),
    completed_step_ids text[] NOT NULL DEFAULT '{}',
    called_action_ids text[] NOT NULL DEFAULT '{}',
    step_outputs jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(step_outputs::text) <= 65536),
    started_run_id uuid REFERENCES ai_runs ON DELETE SET NULL,
    last_run_id uuid REFERENCES ai_runs ON DELETE SET NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz
);

CREATE UNIQUE INDEX conversation_procedure_states_one_active
    ON conversation_procedure_states (workspace_id, conversation_id)
    WHERE status = 'active';
CREATE INDEX conversation_procedure_states_workspace_id_procedure_id_index
    ON conversation_procedure_states (workspace_id, procedure_id, started_at DESC);

ALTER TABLE conversation_procedure_states ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_procedure_states_tenant ON conversation_procedure_states
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_procedure_states;

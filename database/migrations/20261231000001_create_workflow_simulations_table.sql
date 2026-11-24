-- migrate: phase=expand
-- A workflow's simulations: whole chats it must keep handling the way
-- the team means, each with the customer it plays as, the messages the
-- customer sends, what HTTP request steps answer instead of the team's
-- systems, and the checks that judge it. A simulation runs against the
-- workflow's draft or its live version and keeps how each last went,
-- so the two can be compared before publishing. Every run is kept with
-- a fingerprint of what it tested, as an agent's are.

-- +goose Up
CREATE TABLE workflow_simulations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workflow_id uuid NOT NULL REFERENCES workflows ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    source_conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    persona jsonb NOT NULL DEFAULT '{}',
    messages jsonb NOT NULL DEFAULT '[]',
    action_mocks jsonb NOT NULL DEFAULT '{}',
    assertions jsonb NOT NULL DEFAULT '[]',
    scenario text NOT NULL DEFAULT '' CHECK (char_length(scenario) <= 1000),
    simulated_at timestamptz,
    draft_result jsonb,
    live_result jsonb,
    draft_history jsonb,
    live_history jsonb,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workflow_simulations_size_check CHECK (
        octet_length(persona::text) + octet_length(messages::text) + octet_length(action_mocks::text)
            + octet_length(assertions::text) <= 131072
    )
);

CREATE INDEX workflow_simulations_workspace_id_workflow_id_index
    ON workflow_simulations (workspace_id, workflow_id, created_at);

ALTER TABLE workflow_simulations ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflow_simulations_tenant ON workflow_simulations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE workflow_simulation_runs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    simulation_id uuid NOT NULL REFERENCES workflow_simulations(id) ON DELETE CASCADE,
    version text NOT NULL CHECK (version IN ('draft', 'live')),
    fingerprint text NOT NULL,
    passed boolean NOT NULL,
    errored boolean NOT NULL DEFAULT false,
    result jsonb NOT NULL,
    ran_at timestamptz NOT NULL
);

CREATE INDEX workflow_simulation_runs_simulation_id_version_ran_at_index
    ON workflow_simulation_runs (simulation_id, version, ran_at DESC, id DESC);
CREATE INDEX workflow_simulation_runs_workspace_id_index ON workflow_simulation_runs (workspace_id);

ALTER TABLE workflow_simulation_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflow_simulation_runs_tenant ON workflow_simulation_runs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE workflow_simulation_runs;
DROP TABLE workflow_simulations;

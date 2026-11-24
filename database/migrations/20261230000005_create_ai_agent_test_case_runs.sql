-- migrate: phase=expand
-- A simulation is judged by how it has gone lately, not by its last run
-- alone: an agent's answers vary from run to run, so one pass proves
-- little. Each run is kept with what it tested, a fingerprint of the
-- agent's copy and of the simulation itself, so the runs that count are
-- the ones made on what is in front of the team now. The simulation
-- carries the short history of each copy beside its last result.

-- +goose Up
CREATE TABLE ai_agent_test_case_runs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    test_case_id uuid NOT NULL REFERENCES ai_agent_test_cases(id) ON DELETE CASCADE,
    version text NOT NULL CHECK (version IN ('draft', 'live')),
    fingerprint text NOT NULL,
    passed boolean NOT NULL,
    errored boolean NOT NULL DEFAULT false,
    result jsonb NOT NULL,
    ran_at timestamptz NOT NULL
);

CREATE INDEX ai_agent_test_case_runs_test_case_id_version_ran_at_index
    ON ai_agent_test_case_runs (test_case_id, version, ran_at DESC, id DESC);
CREATE INDEX ai_agent_test_case_runs_workspace_id_index ON ai_agent_test_case_runs (workspace_id);

ALTER TABLE ai_agent_test_case_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_agent_test_case_runs_tenant ON ai_agent_test_case_runs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_agent_test_cases
    ADD COLUMN draft_history jsonb,
    ADD COLUMN live_history jsonb;

-- +goose Down
ALTER TABLE ai_agent_test_cases
    DROP COLUMN draft_history,
    DROP COLUMN live_history;
DROP TABLE ai_agent_test_case_runs;

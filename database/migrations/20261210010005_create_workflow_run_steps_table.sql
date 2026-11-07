-- migrate: phase=expand
-- The trace of a run: one row per step it executed, with the path it
-- took, the outcome and a few small facts such as the member it picked.
-- Message bodies, request bodies and other customer text are never kept
-- here, so the trace can be shown to every member who sees the
-- conversation.

-- +goose Up
CREATE TABLE workflow_run_steps (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    run_id uuid NOT NULL REFERENCES workflow_runs ON DELETE CASCADE,
    workflow_id uuid NOT NULL,
    version_id uuid,
    step_id text NOT NULL,
    kind text NOT NULL,
    status text NOT NULL CHECK (status IN ('completed', 'waiting', 'skipped', 'failed', 'interrupted')),
    path_id text,
    outcome text,
    detail jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(detail::text) <= 4096),
    attempts smallint NOT NULL DEFAULT 1,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workflow_run_steps_workspace_id_run_id_started_at_index
    ON workflow_run_steps (workspace_id, run_id, started_at);
CREATE INDEX workflow_run_steps_workspace_id_workflow_id_step_index
    ON workflow_run_steps (workspace_id, workflow_id, version_id, step_id, started_at);

ALTER TABLE workflow_run_steps ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflow_run_steps_tenant ON workflow_run_steps
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE workflow_run_steps;

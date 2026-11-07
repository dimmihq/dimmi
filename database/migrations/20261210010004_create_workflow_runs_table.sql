-- migrate: phase=expand
-- A run is one workflow version working one conversation: where it is
-- (cursor_step_id), what it waits for (wait, wake_at), and what it has
-- learned (vars). seq goes up on every step, so a job that carries an
-- older seq knows it is stale and does nothing. One run per workflow and
-- conversation can be active at a time; a triggering event starts at
-- most one run per workflow. chain lists the workflows whose changes led
-- to this run, so a workflow never triggers itself.

-- +goose Up
CREATE TABLE workflow_runs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workflow_id uuid NOT NULL REFERENCES workflows ON DELETE CASCADE,
    version_id uuid REFERENCES workflow_versions ON DELETE RESTRICT,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    kind text NOT NULL CHECK (kind IN ('customer_facing', 'background')),
    mode text NOT NULL DEFAULT 'live' CHECK (mode IN ('live', 'test')),
    status text NOT NULL CHECK (
        status IN ('running', 'waiting', 'completed', 'passed', 'exited', 'canceled', 'failed', 'expired')
    ),
    end_reason text,
    trigger text NOT NULL,
    trigger_event_id text,
    trigger_data jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(trigger_data::text) <= 16384),
    spec jsonb,
    parent_run_id uuid REFERENCES workflow_runs ON DELETE SET NULL,
    parent_step_id text,
    chain uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(chain) <= 3),
    cursor_step_id text,
    seq bigint NOT NULL DEFAULT 0,
    wait jsonb,
    wake_at timestamptz,
    vars jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(vars::text) <= 16384),
    steps_executed integer NOT NULL DEFAULT 0,
    acted boolean NOT NULL DEFAULT false,
    error jsonb,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (version_id IS NOT NULL OR mode = 'test')
);

CREATE UNIQUE INDEX workflow_runs_workflow_id_conversation_id_active_key ON workflow_runs (workflow_id, conversation_id)
    WHERE status IN ('running', 'waiting');
CREATE UNIQUE INDEX workflow_runs_workflow_id_trigger_event_id_key ON workflow_runs (workflow_id, trigger_event_id)
    WHERE trigger_event_id IS NOT NULL;
CREATE INDEX workflow_runs_workspace_id_conversation_id_started_at_index
    ON workflow_runs (workspace_id, conversation_id, started_at DESC);
CREATE INDEX workflow_runs_workspace_id_workflow_id_started_at_index
    ON workflow_runs (workspace_id, workflow_id, started_at DESC);
CREATE INDEX workflow_runs_wake_at_index ON workflow_runs (wake_at)
    WHERE status = 'waiting' AND wake_at IS NOT NULL;
CREATE INDEX workflow_runs_workspace_id_status_active_index ON workflow_runs (workspace_id, status)
    WHERE status IN ('running', 'waiting');

ALTER TABLE workflow_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflow_runs_tenant ON workflow_runs USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE workflow_runs;

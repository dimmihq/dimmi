-- migrate: phase=expand
-- Routines put Operator on a clock: a prompt that runs daily, weekly,
-- monthly, on a cron or once, as a narrowed copy of its owner, and
-- leaves its findings as changesets the team reviews. Each run opens a
-- shared thread and is billed like any other Operator run. A routine is
-- soft deleted so the threads and suggestions it left stay readable.

-- +goose Up
CREATE TABLE operator_routines (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    template_key text CHECK (char_length(template_key) BETWEEN 1 AND 64),
    prompt text NOT NULL CHECK (char_length(prompt) BETWEEN 1 AND 8000),

    cadence text NOT NULL CHECK (cadence IN ('daily', 'weekly', 'monthly', 'cron', 'once')),
    cron_expr text CHECK (char_length(cron_expr) BETWEEN 1 AND 100),
    run_at timestamptz,
    timezone text NOT NULL DEFAULT 'UTC' CHECK (char_length(timezone) BETWEEN 1 AND 64),
    next_run_at timestamptz,

    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    owner_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    permissions text[] NOT NULL DEFAULT '{}' CHECK (cardinality(permissions) <= 64),
    tool_groups text[] NOT NULL DEFAULT '{}' CHECK (cardinality(tool_groups) <= 16),
    inbox_ids uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(inbox_ids) <= 100),
    write_policies jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(write_policies) = 'object'),
    scope jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(scope) = 'object'),

    model text CHECK (char_length(model) BETWEEN 1 AND 200),
    effort text CHECK (effort IN ('low', 'medium', 'high')),
    max_model_calls integer NOT NULL DEFAULT 16 CHECK (max_model_calls BETWEEN 1 AND 30),
    max_tool_calls integer NOT NULL DEFAULT 50 CHECK (max_tool_calls BETWEEN 1 AND 100),
    monthly_budget_micros bigint CHECK (monthly_budget_micros > 0),

    notify_policy text NOT NULL DEFAULT 'on_findings'
        CHECK (notify_policy IN ('on_findings', 'always', 'on_error_only')),
    delivery jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(delivery) = 'object'),

    state jsonb NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof(state) = 'object' AND octet_length(state::text) <= 65536),

    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'completed')),
    pause_reason text CHECK (char_length(pause_reason) <= 200),
    consecutive_failures integer NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    max_consecutive_failures integer NOT NULL DEFAULT 3 CHECK (max_consecutive_failures BETWEEN 1 AND 10),
    last_run_at timestamptz,
    last_success_at timestamptz,
    last_outcome text CHECK (last_outcome IN ('findings', 'noop', 'failed', 'skipped')),
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CHECK ((cadence = 'once') = (run_at IS NOT NULL)),
    CHECK (cadence = 'once' OR cron_expr IS NOT NULL)
);

CREATE INDEX operator_routines_workspace_id_created_at_index
    ON operator_routines (workspace_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX operator_routines_next_run_at_index
    ON operator_routines (next_run_at, id)
    WHERE status = 'active' AND next_run_at IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX operator_routines_owner_member_id_index ON operator_routines (owner_member_id);
CREATE INDEX operator_routines_created_by_member_id_index ON operator_routines (created_by_member_id);

ALTER TABLE operator_routines ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_routines_tenant ON operator_routines
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- The routines due to run, across workspaces.
-- +goose StatementBegin
CREATE FUNCTION dimmi_operator_routines_due(at timestamptz, batch integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT r.workspace_id, r.id
        FROM operator_routines r
        JOIN workspaces w ON w.id = r.workspace_id
        WHERE r.status = 'active' AND r.deleted_at IS NULL
          AND r.next_run_at IS NOT NULL AND r.next_run_at <= at
          AND w.deleted_at IS NULL
        ORDER BY r.next_run_at, r.id
        LIMIT batch;
    $$;
-- +goose StatementEnd

CREATE TABLE operator_routine_runs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    routine_id uuid NOT NULL REFERENCES operator_routines ON DELETE CASCADE,
    trigger text NOT NULL CHECK (trigger IN ('scheduled', 'manual', 'dry_run')),
    triggered_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    scheduled_for timestamptz,
    thread_id uuid,
    ai_run_id uuid REFERENCES ai_runs ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'skipped')),
    outcome text CHECK (outcome IN ('findings', 'noop', 'failed', 'skipped')),
    noop_reason text CHECK (char_length(noop_reason) <= 500),
    skip_reason text CHECK (char_length(skip_reason) <= 200),
    error_code text CHECK (char_length(error_code) <= 100),
    error_message text CHECK (char_length(error_message) <= 1000),
    changesets_count integer NOT NULL DEFAULT 0 CHECK (changesets_count >= 0),
    cost_micros bigint NOT NULL DEFAULT 0 CHECK (cost_micros >= 0),
    notified boolean NOT NULL DEFAULT false,
    delivery_results jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(delivery_results) = 'array'),
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- One live run per routine at a time; dry runs never hold the lock.
CREATE UNIQUE INDEX operator_routine_runs_active_key
    ON operator_routine_runs (routine_id)
    WHERE status IN ('queued', 'running') AND trigger <> 'dry_run';
-- One run per scheduled slot.
CREATE UNIQUE INDEX operator_routine_runs_slot_key
    ON operator_routine_runs (routine_id, scheduled_for)
    WHERE trigger = 'scheduled' AND scheduled_for IS NOT NULL;
CREATE INDEX operator_routine_runs_routine_id_created_at_index
    ON operator_routine_runs (workspace_id, routine_id, created_at DESC);
CREATE INDEX operator_routine_runs_triggered_by_member_id_index ON operator_routine_runs (triggered_by_member_id);
CREATE INDEX operator_routine_runs_ai_run_id_index ON operator_routine_runs (ai_run_id);
CREATE INDEX operator_routine_runs_thread_id_index ON operator_routine_runs (thread_id);

ALTER TABLE operator_routine_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_routine_runs_tenant ON operator_routine_runs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE operator_routine_runs
    ADD CONSTRAINT operator_routine_runs_thread_id_fkey
        FOREIGN KEY (thread_id) REFERENCES operator_threads ON DELETE SET NULL;

-- +goose Down
DROP TABLE operator_routine_runs;
DROP FUNCTION dimmi_operator_routines_due(timestamptz, integer);
DROP TABLE operator_routines;

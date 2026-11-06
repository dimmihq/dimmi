-- migrate: phase=expand
-- The agent's tables are tenant data like the rest of the inbox: a
-- connection sees the keys, settings, runs, steps and spend of only the
-- workspaces it may see.

-- +goose Up
ALTER TABLE ai_provider_keys ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_provider_keys_tenant ON ai_provider_keys
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_agents ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_agents_tenant ON ai_agents USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_runs_tenant ON ai_runs USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_run_steps ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_run_steps_tenant ON ai_run_steps USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_spend_periods ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_spend_periods_tenant ON ai_spend_periods
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP POLICY ai_spend_periods_tenant ON ai_spend_periods;
ALTER TABLE ai_spend_periods DISABLE ROW LEVEL SECURITY;

DROP POLICY ai_run_steps_tenant ON ai_run_steps;
ALTER TABLE ai_run_steps DISABLE ROW LEVEL SECURITY;

DROP POLICY ai_runs_tenant ON ai_runs;
ALTER TABLE ai_runs DISABLE ROW LEVEL SECURITY;

DROP POLICY ai_agents_tenant ON ai_agents;
ALTER TABLE ai_agents DISABLE ROW LEVEL SECURITY;

DROP POLICY ai_provider_keys_tenant ON ai_provider_keys;
ALTER TABLE ai_provider_keys DISABLE ROW LEVEL SECURITY;

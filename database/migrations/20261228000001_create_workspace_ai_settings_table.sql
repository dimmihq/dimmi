-- migrate: phase=expand
-- A workspace runs many agents, but some AI settings belong to the
-- workspace, not to one agent: the default answer model an agent uses
-- until it picks its own, the classifier that judges conversations, the
-- fallback when a provider fails, and the monthly spend cap every agent
-- and Operator share. Each workspace gets one row, filled from the agent
-- it already has.

-- +goose Up
CREATE TABLE workspace_ai_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    answer_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    answer_model text,
    classifier_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    classifier_model text,
    fallback_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    fallback_model text,
    monthly_spend_cap_micros bigint DEFAULT 25000000 CHECK (monthly_spend_cap_micros >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE workspace_ai_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY workspace_ai_settings_tenant ON workspace_ai_settings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

INSERT INTO workspace_ai_settings (
    workspace_id, answer_key_id, answer_model, classifier_key_id, classifier_model,
    fallback_key_id, fallback_model, monthly_spend_cap_micros
)
SELECT workspace_id, answer_key_id, answer_model, classifier_key_id, classifier_model,
       fallback_key_id, fallback_model, monthly_spend_cap_micros
FROM ai_agents
WHERE deleted_at IS NULL
ON CONFLICT (workspace_id) DO NOTHING;

-- +goose Down
DROP TABLE workspace_ai_settings;

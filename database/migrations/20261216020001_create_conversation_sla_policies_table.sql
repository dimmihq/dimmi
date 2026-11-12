-- migrate: phase=expand
-- A plain conversation gets a service level policy from a workflow's
-- apply SLA step. The policy it follows is kept here, one per
-- conversation, so the clocks its messages start and stop know their
-- targets. A customer ticket keeps its own policy on the ticket.

-- +goose Up
CREATE TABLE conversation_sla_policies (
    conversation_id uuid PRIMARY KEY REFERENCES conversations ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    policy_id uuid NOT NULL REFERENCES sla_policies ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX conversation_sla_policies_workspace_id_index ON conversation_sla_policies (workspace_id);
CREATE INDEX conversation_sla_policies_policy_id_index ON conversation_sla_policies (policy_id);

ALTER TABLE conversation_sla_policies ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_sla_policies_tenant ON conversation_sla_policies
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_sla_policies;

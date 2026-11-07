-- migrate: phase=expand
-- One request the agent made to run an action, from the moment it was
-- proposed to its result: who had to agree, who did, what was sent and
-- what came back. The input is frozen when the call is proposed, so
-- what a customer confirmed or a teammate approved is exactly what
-- runs. sent_at is written before the first byte leaves, so a crashed
-- write is never sent twice. bindings holds the customer values the
-- call was bound to and never leaves the server.

-- +goose Up
CREATE TABLE ai_action_calls (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    action_id uuid NOT NULL REFERENCES ai_actions,
    action_version_id uuid REFERENCES ai_action_versions ON DELETE SET NULL,
    connection_id uuid NOT NULL REFERENCES ai_action_connections,
    mode text NOT NULL CHECK (mode IN ('live', 'test')),
    side_effect text NOT NULL CHECK (side_effect IN ('read', 'write')),
    confirmation text NOT NULL DEFAULT 'none' CHECK (confirmation IN ('none', 'customer')),
    approval text NOT NULL DEFAULT 'none' CHECK (approval IN ('none', 'teammate', 'admin')),
    state text NOT NULL CHECK (state IN (
        'awaiting_confirmation', 'pending_approval', 'approved', 'declined', 'expired', 'cancelled',
        'running', 'succeeded', 'failed'
    )),
    error_code text,
    http_status integer,
    run_id uuid REFERENCES ai_runs ON DELETE SET NULL,
    report_run_id uuid REFERENCES ai_runs ON DELETE SET NULL,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    message_id uuid REFERENCES messages ON DELETE SET NULL,
    tool_call_id text,
    title text NOT NULL DEFAULT '',
    summary text NOT NULL DEFAULT '',
    input jsonb NOT NULL DEFAULT '{}',
    bindings jsonb NOT NULL DEFAULT '{}',
    request jsonb,
    response jsonb,
    result jsonb,
    decided_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    decision_reason text,
    customer_message text,
    cancel_reason text,
    attempts integer NOT NULL DEFAULT 0,
    latency_ms integer,
    requested_at timestamptz NOT NULL,
    confirmed_at timestamptz,
    decided_at timestamptz,
    sent_at timestamptz,
    completed_at timestamptz,
    expires_at timestamptz,
    reported_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ai_action_calls_write_confirmed CHECK (
        mode <> 'live' OR confirmation = 'none' OR state NOT IN ('running', 'succeeded', 'failed')
        OR confirmed_at IS NOT NULL
    ),
    CONSTRAINT ai_action_calls_write_approved CHECK (
        mode <> 'live' OR approval = 'none' OR state NOT IN ('running', 'succeeded', 'failed')
        OR decided_at IS NOT NULL
    )
);

CREATE INDEX ai_action_calls_workspace_id_conversation_id_created_at_index
    ON ai_action_calls (workspace_id, conversation_id, created_at);
CREATE INDEX ai_action_calls_workspace_id_action_id_created_at_index
    ON ai_action_calls (workspace_id, action_id, created_at DESC);
CREATE INDEX ai_action_calls_workspace_id_created_at_index
    ON ai_action_calls (workspace_id, created_at DESC);
CREATE INDEX ai_action_calls_workspace_id_state_expires_at_index
    ON ai_action_calls (workspace_id, state, expires_at)
    WHERE state IN ('awaiting_confirmation', 'pending_approval');
CREATE INDEX ai_action_calls_pending_expiry_index
    ON ai_action_calls (expires_at) WHERE state IN ('awaiting_confirmation', 'pending_approval');

ALTER TABLE ai_action_calls ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_action_calls_tenant ON ai_action_calls
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_runs ADD COLUMN trigger_action_call_id uuid REFERENCES ai_action_calls ON DELETE SET NULL;

-- +goose Down
ALTER TABLE ai_runs DROP COLUMN trigger_action_call_id;
DROP TABLE ai_action_calls;

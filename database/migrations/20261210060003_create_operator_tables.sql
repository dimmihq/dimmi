-- migrate: phase=expand
-- Operator, the team's own agent. Each workspace keeps its settings,
-- each member their private threads with Operator and what they asked it
-- to remember, and every change Operator wants to make waits as a
-- changeset: the operations with their values before and after, applied
-- and reverted as one.

-- +goose Up
CREATE TABLE operator_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT true,
    brief_on_open boolean NOT NULL DEFAULT false,
    triage_on_arrival boolean NOT NULL DEFAULT false,
    instructions text NOT NULL DEFAULT '' CHECK (char_length(instructions) <= 8000),
    policies jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(policies) = 'object'),
    model text CHECK (char_length(model) BETWEEN 1 AND 200),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE operator_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_settings_tenant ON operator_settings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE operator_threads (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    title text NOT NULL DEFAULT '' CHECK (char_length(title) <= 200),
    last_message_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX operator_threads_member_id_conversation_id_key
    ON operator_threads (member_id, conversation_id) WHERE conversation_id IS NOT NULL;
CREATE INDEX operator_threads_workspace_id_member_id_updated_at_index
    ON operator_threads (workspace_id, member_id, updated_at DESC);
CREATE INDEX operator_threads_conversation_id_index ON operator_threads (conversation_id);

ALTER TABLE operator_threads ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_threads_tenant ON operator_threads
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE operator_messages (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    thread_id uuid NOT NULL REFERENCES operator_threads ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'assistant')),
    body text NOT NULL DEFAULT '' CHECK (octet_length(body) <= 65536),
    steps jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(steps) = 'array'),
    status text NOT NULL DEFAULT 'done' CHECK (status IN ('done', 'stopped', 'failed')),
    run_id uuid REFERENCES ai_runs ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX operator_messages_thread_id_created_at_index ON operator_messages (thread_id, created_at);
CREATE INDEX operator_messages_workspace_id_index ON operator_messages (workspace_id);
CREATE INDEX operator_messages_run_id_index ON operator_messages (run_id);

ALTER TABLE operator_messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_messages_tenant ON operator_messages
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE operator_changesets (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    thread_id uuid REFERENCES operator_threads ON DELETE SET NULL,
    message_id uuid REFERENCES operator_messages ON DELETE SET NULL,
    member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    reason text NOT NULL DEFAULT '' CHECK (char_length(reason) <= 2000),
    operations jsonb NOT NULL CHECK (jsonb_typeof(operations) = 'array' AND jsonb_array_length(operations) BETWEEN 1 AND 50),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'applied', 'declined', 'reverted', 'failed')),
    auto boolean NOT NULL DEFAULT false,
    error text CHECK (char_length(error) <= 2000),
    decided_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    applied_at timestamptz,
    reverted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX operator_changesets_workspace_id_status_created_at_index
    ON operator_changesets (workspace_id, status, created_at DESC);
CREATE INDEX operator_changesets_conversation_id_created_at_index
    ON operator_changesets (conversation_id, created_at DESC);
CREATE INDEX operator_changesets_thread_id_index ON operator_changesets (thread_id);
CREATE INDEX operator_changesets_message_id_index ON operator_changesets (message_id);
CREATE INDEX operator_changesets_member_id_index ON operator_changesets (member_id);
CREATE INDEX operator_changesets_decided_by_member_id_index ON operator_changesets (decided_by_member_id);

ALTER TABLE operator_changesets ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_changesets_tenant ON operator_changesets
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE operator_memories (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX operator_memories_workspace_id_member_id_created_at_index
    ON operator_memories (workspace_id, member_id, created_at DESC);
CREATE INDEX operator_memories_member_id_index ON operator_memories (member_id);

ALTER TABLE operator_memories ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_memories_tenant ON operator_memories
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE operator_memories;
DROP TABLE operator_changesets;
DROP TABLE operator_messages;
DROP TABLE operator_threads;
DROP TABLE operator_settings;

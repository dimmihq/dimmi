-- migrate: phase=expand
-- AI conversation attributes: fields a workspace defines once (a name,
-- a type, what it means and, for a choice, the values it may take) that
-- the decision model fills in on every conversation and checks again as
-- the customer writes. A definition is archived, never deleted, so the
-- workflows and views that read it keep reading what it last held.
--
-- conversation_ai_values keeps one row per conversation: the values
-- detected so far by key, and the message and moment they were last
-- checked at, which throttles how often a conversation is read again.

-- +goose Up
CREATE TABLE ai_conversation_attributes (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    key text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{0,63}$'),
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    data_type text NOT NULL CHECK (data_type IN ('text', 'number', 'date', 'boolean', 'choice')),
    options text[] NOT NULL DEFAULT '{}' CHECK (cardinality(options) <= 30),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((data_type = 'choice') = (cardinality(options) > 0))
);

CREATE UNIQUE INDEX ai_conversation_attributes_workspace_id_key_index
    ON ai_conversation_attributes (workspace_id, key);

ALTER TABLE ai_conversation_attributes ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_conversation_attributes_tenant ON ai_conversation_attributes
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE conversation_ai_values (
    conversation_id uuid PRIMARY KEY REFERENCES conversations ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    "values" jsonb NOT NULL DEFAULT '{}'
        CHECK (jsonb_typeof("values") = 'object' AND octet_length("values"::text) <= 65536),
    checked_message_id uuid,
    checked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX conversation_ai_values_workspace_id_index ON conversation_ai_values (workspace_id);
CREATE INDEX conversation_ai_values_values_index ON conversation_ai_values USING gin ("values" jsonb_path_ops);

ALTER TABLE conversation_ai_values ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_ai_values_tenant ON conversation_ai_values
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_ai_values;
DROP TABLE ai_conversation_attributes;

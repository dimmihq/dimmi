-- migrate: phase=expand
-- A workspace's own patterns for text that must never be stored, applied
-- in their position order after the built-in rules. A workspace keeps at
-- most ten; the application checks that and that each pattern compiles.

-- +goose Up
CREATE TABLE redaction_rules (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    pattern text NOT NULL CHECK (char_length(pattern) BETWEEN 1 AND 256),
    replacement text NOT NULL DEFAULT '[redacted]' CHECK (char_length(replacement) <= 80),
    enabled boolean NOT NULL DEFAULT true,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX redaction_rules_workspace_id_position_index ON redaction_rules (workspace_id, position);

ALTER TABLE redaction_rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY redaction_rules_tenant ON redaction_rules USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE redaction_rules;

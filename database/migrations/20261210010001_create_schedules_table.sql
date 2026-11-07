-- migrate: phase=expand
-- A schedule is a workspace's office hours: up to four open ranges per
-- weekday in one timezone, plus holidays closed all day. Workflows ask
-- whether the team is around, wait until it is, and count business time
-- with it. One schedule per workspace is the default, used wherever no
-- other is named.

-- +goose Up
CREATE TABLE schedules (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    timezone text NOT NULL,
    weekly jsonb NOT NULL,
    holidays jsonb NOT NULL DEFAULT '[]',
    is_default boolean NOT NULL DEFAULT false,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX schedules_workspace_id_name_key ON schedules (workspace_id, lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX schedules_workspace_id_default_key ON schedules (workspace_id)
    WHERE is_default AND deleted_at IS NULL;

ALTER TABLE schedules ENABLE ROW LEVEL SECURITY;
CREATE POLICY schedules_tenant ON schedules USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE schedules;

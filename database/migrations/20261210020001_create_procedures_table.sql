-- migrate: phase=expand
-- A procedure is how the team handles one kind of request, written in
-- plain words for dimmi to follow: when it applies, the steps in order
-- (instructions, conditions, and actions dimmi takes in the team's own
-- systems) and when to hand the conversation to a person. The working
-- copy is edited freely; dimmi only ever reads the live copy, made by
-- publishing. lock_version stops two editors from overwriting each other.

-- +goose Up
CREATE TABLE procedures (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'live')),
    draft jsonb NOT NULL CHECK (octet_length(draft::text) <= 65536),
    live jsonb CHECK (octet_length(live::text) <= 65536),
    lock_version integer NOT NULL DEFAULT 1,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    published_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX procedures_workspace_id_name_key ON procedures (workspace_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX procedures_workspace_id_live_index ON procedures (workspace_id) WHERE status = 'live' AND deleted_at IS NULL;

ALTER TABLE procedures ENABLE ROW LEVEL SECURITY;
CREATE POLICY procedures_tenant ON procedures USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE procedures;

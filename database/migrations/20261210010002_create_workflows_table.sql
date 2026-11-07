-- migrate: phase=expand
-- A workflow decides what happens to a conversation without a person:
-- one trigger it never changes, a working copy anyone allowed edits, and
-- a live version conversations run on. Editing never changes what runs
-- until the next publish; draft_hash against live_hash tells whether
-- the working copy differs from the live version. lock_version stops two editors from
-- overwriting each other. position is the list order, which is also the
-- priority when several workflows want the same conversation.

-- +goose Up
CREATE TABLE workflows (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    description text CHECK (char_length(description) <= 500),
    trigger text NOT NULL CHECK (trigger ~ '^[a-z][a-z0-9_.]{1,39}$'),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'live', 'paused')),
    position integer NOT NULL,
    draft jsonb NOT NULL CHECK (octet_length(draft::text) <= 262144),
    draft_hash text NOT NULL,
    live_version_id uuid,
    live_hash text,
    lock_version integer NOT NULL DEFAULT 1,
    kind text NOT NULL DEFAULT 'background' CHECK (kind IN ('customer_facing', 'background')),
    test_on_test_conversations boolean NOT NULL DEFAULT false,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    paused_at timestamptz,
    published_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workflows_workspace_id_deleted_at_position_index ON workflows (workspace_id, deleted_at, position);
CREATE INDEX workflows_workspace_id_trigger_live_index ON workflows (workspace_id, trigger)
    WHERE status = 'live' AND deleted_at IS NULL;
CREATE UNIQUE INDEX workflows_workspace_id_name_key ON workflows (workspace_id, lower(name)) WHERE deleted_at IS NULL;

ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflows_tenant ON workflows USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE workflows;

-- migrate: phase=expand
-- Where automatic assignment last stopped in each pool, so turns go round
-- fairly. A pool is a team, or an inbox when no team holds the
-- conversation. Locking the row serializes picks in one pool, so two
-- arrivals at the same moment never take the same slot. The pool id has
-- no foreign key: deleting a team or an inbox removes its row.

-- +goose Up
CREATE TABLE team_assignment_cursors (
    pool_kind text NOT NULL CHECK (pool_kind IN ('team', 'inbox')),
    pool_id uuid NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    last_member_id uuid,
    last_assigned_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (pool_kind, pool_id)
);

CREATE INDEX team_assignment_cursors_workspace_id_index ON team_assignment_cursors (workspace_id);

ALTER TABLE team_assignment_cursors ENABLE ROW LEVEL SECURITY;
CREATE POLICY team_assignment_cursors_tenant ON team_assignment_cursors
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE team_assignment_cursors;

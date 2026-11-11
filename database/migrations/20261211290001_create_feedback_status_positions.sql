-- migrate: phase=expand
-- The order a workspace shows its feature request statuses in: the
-- roadmap columns and filters of its portal, and every status list and
-- picker in the app. A workspace without rows keeps the built-in order;
-- a status without a row follows the ones that have one. The team saves
-- the whole order at once, so positions are unique per workspace.

-- +goose Up
CREATE TABLE feedback_status_positions (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('under_review', 'planned', 'in_progress', 'shipped', 'closed')),
    position smallint NOT NULL CHECK (position >= 0),
    PRIMARY KEY (workspace_id, status)
);

ALTER TABLE feedback_status_positions ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_status_positions_tenant ON feedback_status_positions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE IF EXISTS feedback_status_positions;

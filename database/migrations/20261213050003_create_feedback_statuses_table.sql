-- migrate: phase=expand
-- The statuses a workspace's feature requests move through are its own:
-- the team names, colors, adds and removes them. Each falls in a
-- category, open, planned, in progress, done or closed, which says what
-- it means: new requests start in the first open status, a done status
-- is what shipping means, and closed ones never show on the roadmap.
-- A request names its status by key, so the API keeps reading
-- under_review, planned and the rest. Every workspace starts with the
-- five built-in statuses, in the order it saved, if it saved one.

-- +goose Up
CREATE TABLE feedback_statuses (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    key text NOT NULL CHECK (key ~ '^[a-z0-9_]{1,40}$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    color text NOT NULL CHECK (color ~ '^#[0-9a-fA-F]{6}$'),
    category text NOT NULL CHECK (category IN ('open', 'planned', 'in_progress', 'done', 'closed')),
    roadmap boolean NOT NULL DEFAULT true,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX feedback_statuses_workspace_id_key_key ON feedback_statuses (workspace_id, key);

ALTER TABLE feedback_statuses ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_statuses_tenant ON feedback_statuses USING (workspace_id = ANY (dimmi_visible_workspaces()));

INSERT INTO feedback_statuses (workspace_id, key, name, color, category, roadmap, position)
SELECT w.id, b.key, b.name, b.color, b.category, b.roadmap,
    coalesce(p.position, 100 + b.ord)
FROM workspaces w
CROSS JOIN (VALUES
    ('in_progress', 'In progress', '#f59e0b', 'in_progress', true, 0),
    ('planned', 'Planned', '#3b82f6', 'planned', true, 1),
    ('under_review', 'Under consideration', '#a1a1aa', 'open', true, 2),
    ('shipped', 'Shipped', '#16a34a', 'done', true, 3),
    ('closed', 'Closed', '#71717a', 'closed', false, 4)
) AS b (key, name, color, category, roadmap, ord)
LEFT JOIN feedback_status_positions p ON p.workspace_id = w.id AND p.status = b.key
ON CONFLICT (workspace_id, key) DO NOTHING;

-- +goose Down
DROP TABLE feedback_statuses;

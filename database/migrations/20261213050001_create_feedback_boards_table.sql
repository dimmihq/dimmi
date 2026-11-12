-- migrate: phase=expand
-- A help center's feature requests are split across boards, such as
-- feature requests and bugs. A public board is open to everyone, a
-- private one only to signed-in customers, and an internal one only to
-- the team. A board can hold what customers post until the team
-- approves it. Boards show in the team's order. Every help center gets
-- a default board that its existing requests move onto, taking over the
-- help center's approval setting; board_id stays nullable so the
-- release before this one keeps posting.

-- +goose Up
CREATE TABLE feedback_boards (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,60}$'),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private', 'internal')),
    approval_required boolean NOT NULL DEFAULT false,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX feedback_boards_help_center_id_slug_key ON feedback_boards (help_center_id, slug);
CREATE INDEX feedback_boards_workspace_id_help_center_id_position_index
    ON feedback_boards (workspace_id, help_center_id, position);

ALTER TABLE feedback_boards ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_boards_tenant ON feedback_boards USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE feedback_posts ADD COLUMN board_id uuid REFERENCES feedback_boards ON DELETE RESTRICT;

INSERT INTO feedback_boards (workspace_id, help_center_id, name, slug, approval_required, position)
SELECT h.workspace_id, h.id, 'Feature requests', 'feature-requests',
    coalesce((h.settings ->> 'feedback_approval')::boolean, false), 0
FROM help_centers h;

UPDATE feedback_posts p SET board_id = b.id
FROM feedback_boards b
WHERE b.help_center_id = p.help_center_id AND b.slug = 'feature-requests' AND p.board_id IS NULL;

-- +goose Down
ALTER TABLE feedback_posts DROP COLUMN board_id;
DROP TABLE feedback_boards;

-- migrate: phase=expand
-- Feature requests customers post and vote on, on their help center's
-- portal, and the team's replies. A post is numbered per workspace so it
-- can be named as #12, moves through statuses as the team works on it,
-- and waits for approval when the team asks to see posts first. Each
-- customer votes once per post; a teammate can vote on a customer's
-- behalf, with how much it matters to them. vote_count and comment_count
-- are kept in step with their rows; trending_log is the log2 of the sum
-- of each vote's weight, doubling every week, so newer votes count more
-- without anything recomputed later. A post merged into another points
-- at it and keeps its rows only as history. Comments are public replies
-- or internal notes only the team sees, one level of replies deep.

-- +goose Up
CREATE TABLE feedback_posts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    number integer NOT NULL,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 150),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,120}$'),
    body_markdown text NOT NULL DEFAULT '' CHECK (octet_length(body_markdown) <= 65536),
    status text NOT NULL DEFAULT 'under_review'
        CHECK (status IN ('under_review', 'planned', 'in_progress', 'shipped', 'closed')),
    moderation text NOT NULL DEFAULT 'approved' CHECK (moderation IN ('approved', 'pending', 'spam')),
    author_contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    vote_count integer NOT NULL DEFAULT 0,
    comment_count integer NOT NULL DEFAULT 0,
    trending_log double precision,
    merged_into_id uuid REFERENCES feedback_posts ON DELETE SET NULL,
    changelog_entry_id uuid REFERENCES changelog_entries ON DELETE SET NULL,
    status_changed_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    title_trgm text GENERATED ALWAYS AS (lower(dimmi_unaccent(title))) STORED
);

CREATE UNIQUE INDEX feedback_posts_workspace_id_number_key ON feedback_posts (workspace_id, number);
CREATE INDEX feedback_posts_workspace_id_help_center_id_status_index
    ON feedback_posts (workspace_id, help_center_id, status, vote_count DESC) WHERE deleted_at IS NULL;
CREATE INDEX feedback_posts_title_trgm_index ON feedback_posts USING gin (workspace_id, title_trgm gin_trgm_ops);

ALTER TABLE feedback_posts ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_posts_tenant ON feedback_posts USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE feedback_votes (
    post_id uuid NOT NULL REFERENCES feedback_posts ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    priority text NOT NULL DEFAULT 'nice' CHECK (priority IN ('nice', 'important', 'must')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, contact_id)
);

CREATE INDEX feedback_votes_contact_id_index ON feedback_votes (contact_id);

ALTER TABLE feedback_votes ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_votes_tenant ON feedback_votes USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE feedback_comments (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    post_id uuid NOT NULL REFERENCES feedback_posts ON DELETE CASCADE,
    parent_id uuid REFERENCES feedback_comments ON DELETE CASCADE,
    author_contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    internal boolean NOT NULL DEFAULT false,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX feedback_comments_post_id_created_at_index ON feedback_comments (post_id, created_at);

ALTER TABLE feedback_comments ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_comments_tenant ON feedback_comments USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE feedback_comments;
DROP TABLE feedback_votes;
DROP TABLE feedback_posts;

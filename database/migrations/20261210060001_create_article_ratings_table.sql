-- migrate: phase=expand
-- Readers' answers to "Was this article helpful?". Readers need no
-- account, so each answer belongs to a keyed hash of who gave it, a
-- signed-in reader or the address they came from, and answering again
-- changes the one answer rather than adding another. A reader who says
-- no may add a comment, which the team reads beside the article.

-- +goose Up
CREATE TABLE article_ratings (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    article_id uuid NOT NULL REFERENCES articles ON DELETE CASCADE,
    rater_hash text NOT NULL,
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    helpful boolean NOT NULL,
    comment text CHECK (char_length(comment) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX article_ratings_article_id_rater_hash_key ON article_ratings (article_id, rater_hash);
CREATE INDEX article_ratings_workspace_id_article_id_updated_at_index
    ON article_ratings (workspace_id, article_id, updated_at DESC);

ALTER TABLE article_ratings ENABLE ROW LEVEL SECURITY;
CREATE POLICY article_ratings_tenant ON article_ratings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE article_ratings;

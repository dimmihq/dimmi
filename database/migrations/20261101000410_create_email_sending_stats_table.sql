-- migrate: phase=expand
-- Daily counts of what a workspace sent and how it fared, per stream.
-- They are the evidence for sending caps and for pausing a workspace
-- that bounces or draws complaints, so they live in the database and
-- survive any restart.

-- +goose Up
CREATE TABLE email_sending_stats (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    day date NOT NULL,
    stream text NOT NULL DEFAULT 'customer' CHECK (stream IN ('customer', 'changelog', 'feedback')),
    sent integer NOT NULL DEFAULT 0,
    delivered integer NOT NULL DEFAULT 0,
    hard_bounced integer NOT NULL DEFAULT 0,
    soft_bounced integer NOT NULL DEFAULT 0,
    complained integer NOT NULL DEFAULT 0,
    blocked integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, day, stream)
);

-- +goose Down
DROP TABLE email_sending_stats;

-- migrate: phase=expand
-- Customers rate how a conversation went once it closes. Each close
-- that asks gets one row: when it was closed, when the ask is due, on
-- which channel it went out, and the teammate the conversation was
-- assigned to, who is the one rated. The customer picks a score from
-- one to five and may add a comment, and can change either for a week.
-- A conversation closed again later asks again under a new row.
--
-- Each workspace chooses whether to ask at all, how long after the
-- close, whether conversations the agent closed ask too, and on which
-- channels.

-- +goose Up
CREATE TABLE conversation_rating_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT false,
    delay_minutes integer NOT NULL DEFAULT 0 CHECK (delay_minutes IN (0, 10, 60)),
    include_agent_closed boolean NOT NULL DEFAULT false,
    messenger boolean NOT NULL DEFAULT true,
    email boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE conversation_rating_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_rating_settings_tenant ON conversation_rating_settings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE conversation_ratings (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    channel text NOT NULL CHECK (channel IN ('widget', 'email')),
    closed_at timestamptz NOT NULL,
    ask_at timestamptz NOT NULL,
    asked_at timestamptz,
    score smallint CHECK (score BETWEEN 1 AND 5),
    comment text CHECK (char_length(comment) <= 2000),
    rated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX conversation_ratings_conversation_id_closed_at_key
    ON conversation_ratings (conversation_id, closed_at);
CREATE INDEX conversation_ratings_workspace_id_conversation_id_created_at_index
    ON conversation_ratings (workspace_id, conversation_id, created_at DESC);
CREATE INDEX conversation_ratings_workspace_id_contact_id_rated_at_index
    ON conversation_ratings (workspace_id, contact_id, rated_at DESC) WHERE score IS NOT NULL;

ALTER TABLE conversation_ratings ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_ratings_tenant ON conversation_ratings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_ratings;
DROP TABLE conversation_rating_settings;

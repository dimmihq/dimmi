-- migrate: phase=expand
-- Conversation attributes are filled by teammates as well as the AI.
-- Each attribute says who fills it, whether a teammate must fill it
-- before closing a conversation, which teams see it, and where it sits
-- in the list. Each conversation keeps, beside its values, who set each
-- one and when, so a value a teammate set is never replaced by the AI.

-- +goose Up
ALTER TABLE ai_conversation_attributes
    ADD COLUMN filled_by text NOT NULL DEFAULT 'both' CHECK (filled_by IN ('ai', 'teammates', 'both')),
    ADD COLUMN required_to_close boolean NOT NULL DEFAULT false,
    ADD COLUMN team_ids uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(team_ids) <= 100),
    ADD COLUMN position integer NOT NULL DEFAULT 0;

ALTER TABLE conversation_ai_values
    ADD COLUMN sources jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(sources) = 'object');

-- +goose Down
ALTER TABLE conversation_ai_values DROP COLUMN sources;
ALTER TABLE ai_conversation_attributes
    DROP COLUMN position,
    DROP COLUMN team_ids,
    DROP COLUMN required_to_close,
    DROP COLUMN filled_by;

-- migrate: phase=expand
-- How available a member is for new conversations: away with or without
-- handing back what customers answer, whether they or the system set it,
-- whether replying takes a conversation, and how many open conversations
-- they hold at most, overall and per channel.

-- +goose Up
ALTER TABLE workspace_members
    ADD COLUMN away_reassign boolean NOT NULL DEFAULT false,
    ADD COLUMN away_since timestamptz,
    ADD COLUMN availability_source text NOT NULL DEFAULT 'manual'
        CHECK (availability_source IN ('manual', 'auto')),
    ADD COLUMN away_reason_id uuid,
    ADD COLUMN assign_on_reply text NOT NULL DEFAULT 'inherit'
        CHECK (assign_on_reply IN ('inherit', 'always', 'never')),
    ADD COLUMN conversation_capacity integer CHECK (conversation_capacity BETWEEN 1 AND 500),
    ADD COLUMN capacity_by_channel jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN last_assigned_at timestamptz,
    ADD CONSTRAINT workspace_members_away_reassign_check CHECK (NOT away_reassign OR availability = 'away');

-- +goose Down
ALTER TABLE workspace_members
    DROP CONSTRAINT workspace_members_away_reassign_check,
    DROP COLUMN last_assigned_at,
    DROP COLUMN capacity_by_channel,
    DROP COLUMN conversation_capacity,
    DROP COLUMN assign_on_reply,
    DROP COLUMN away_reason_id,
    DROP COLUMN availability_source,
    DROP COLUMN away_since,
    DROP COLUMN away_reassign;

-- migrate: phase=contract
-- A broadcast on the conversation channel answers each person inside a
-- conversation they already have, on that conversation's own channel:
-- email, the Messenger, Slack or any other. Started from a tracker, it
-- reaches only the conversations linked to that tracker, and can leave
-- out customers still waiting on an answer or already answered since
-- the tracker last moved. Widening the channel check replaces it, which
-- the lint counts as a contract change. The index on ticket_id is built
-- in the next migration, without locking broadcasts.

-- +goose Up
ALTER TABLE broadcasts DROP CONSTRAINT broadcasts_channel_check;
ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_channel_check
    CHECK (channel IN ('email', 'chat', 'post', 'slack', 'conversation'));

ALTER TABLE broadcasts ADD COLUMN ticket_id uuid REFERENCES tickets ON DELETE SET NULL;
ALTER TABLE broadcasts ADD COLUMN holds text[] NOT NULL DEFAULT '{}'
    CHECK (holds <@ ARRAY['customer_waiting', 'team_replied']::text[]);


-- +goose Down
ALTER TABLE broadcasts DROP COLUMN holds;
ALTER TABLE broadcasts DROP COLUMN ticket_id;
DELETE FROM broadcasts WHERE channel = 'conversation';
ALTER TABLE broadcasts DROP CONSTRAINT broadcasts_channel_check;
ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_channel_check CHECK (channel IN ('email', 'chat', 'post', 'slack'));

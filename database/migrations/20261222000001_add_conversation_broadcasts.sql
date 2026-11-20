-- migrate: phase=contract
-- A broadcast on the conversation channel answers each person inside a
-- conversation they already have, on that conversation's own channel:
-- email, the Messenger, Slack or any other. Started from a tracker, it
-- reaches only the conversations linked to that tracker, and can leave
-- out customers still waiting on an answer or already answered since
-- the tracker last moved. Widening the channel check replaces it, which
-- the lint counts as a contract change.

-- +goose Up
ALTER TABLE broadcasts DROP CONSTRAINT broadcasts_channel_check;
ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_channel_check
    CHECK (channel IN ('email', 'chat', 'post', 'slack', 'conversation'));

ALTER TABLE broadcasts ADD COLUMN ticket_id uuid REFERENCES tickets ON DELETE SET NULL;
ALTER TABLE broadcasts ADD COLUMN holds text[] NOT NULL DEFAULT '{}'
    CHECK (holds <@ ARRAY['customer_waiting', 'team_replied']::text[]);

CREATE INDEX broadcasts_ticket_id_index ON broadcasts (ticket_id) WHERE ticket_id IS NOT NULL;

-- +goose Down
DROP INDEX broadcasts_ticket_id_index;
ALTER TABLE broadcasts DROP COLUMN holds;
ALTER TABLE broadcasts DROP COLUMN ticket_id;
DELETE FROM broadcasts WHERE channel = 'conversation';
ALTER TABLE broadcasts DROP CONSTRAINT broadcasts_channel_check;
ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_channel_check CHECK (channel IN ('email', 'chat', 'post', 'slack'));

-- migrate: phase=expand
-- A tag put on a conversation from one of its messages remembers which
-- message prompted it. Filtering stays on the conversation, so there is
-- one vocabulary and reports never split between message and conversation
-- tags.

-- +goose Up
ALTER TABLE conversation_tag ADD COLUMN message_id uuid REFERENCES messages ON DELETE SET NULL;

-- +goose Down
ALTER TABLE conversation_tag DROP COLUMN message_id;

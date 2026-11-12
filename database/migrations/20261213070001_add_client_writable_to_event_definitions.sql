-- migrate: phase=expand
-- An event name the messenger may send from the visitor's browser. Off
-- by default: only the team's servers report what customers did until
-- the team opens a name to the browser, where anyone can forge it.

-- +goose Up
ALTER TABLE event_definitions ADD COLUMN client_writable boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE event_definitions DROP COLUMN client_writable;

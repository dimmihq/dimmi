-- migrate: phase=expand
-- Teammates keep free notes about a person or a company. Notes are for
-- the team alone: they never reach the widget or a webhook, and erasing a
-- customer wipes them.

-- +goose Up
ALTER TABLE contacts ADD COLUMN notes text;
ALTER TABLE companies ADD COLUMN notes text;

-- +goose Down
ALTER TABLE companies DROP COLUMN notes;
ALTER TABLE contacts DROP COLUMN notes;

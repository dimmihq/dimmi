-- migrate: phase=contract
-- Members may keep Slack on or off per trigger like any other channel.
-- The check only widens, so every row the old release writes still
-- passes it; it is added unvalidated and validated after, so writers are
-- never blocked while existing rows are read.

-- +goose Up
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_channel_check;
ALTER TABLE notification_preferences ADD CONSTRAINT notification_preferences_channel_check
    CHECK (channel IN ('in_app', 'email', 'push', 'slack')) NOT VALID;
ALTER TABLE notification_preferences VALIDATE CONSTRAINT notification_preferences_channel_check;

-- +goose Down
DELETE FROM notification_preferences WHERE channel = 'slack';
ALTER TABLE notification_preferences DROP CONSTRAINT notification_preferences_channel_check;
ALTER TABLE notification_preferences ADD CONSTRAINT notification_preferences_channel_check
    CHECK (channel IN ('in_app', 'email', 'push'));

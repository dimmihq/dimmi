-- migrate: phase=contract
-- Messenger and Instagram deliver through Meta's webhooks like WhatsApp
-- does, and their deliveries are kept the same way before anything reads
-- them, so the kept deliveries' provider may now name either. Widening
-- the provider check replaces it, which the lint counts as a contract
-- change; no code relies on the narrower check.

-- +goose Up
ALTER TABLE channel_inbound_events
    DROP CONSTRAINT channel_inbound_events_provider_check,
    ADD CONSTRAINT channel_inbound_events_provider_check
        CHECK (provider IN ('slack', 'discord', 'telegram', 'whatsapp', 'facebook', 'instagram'));

-- +goose Down
ALTER TABLE channel_inbound_events
    DROP CONSTRAINT channel_inbound_events_provider_check,
    ADD CONSTRAINT channel_inbound_events_provider_check
        CHECK (provider IN ('slack', 'discord', 'telegram', 'whatsapp'));

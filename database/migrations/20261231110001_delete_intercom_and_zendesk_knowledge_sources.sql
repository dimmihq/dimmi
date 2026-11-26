-- migrate: phase=contract
-- dimmi no longer connects to Intercom or Zendesk, so their knowledge
-- sources and the installations they read through are removed. A
-- source's runs, documents and chunks go with it, and an installation's
-- grants and notification destinations go with it.

-- +goose Up
DELETE FROM knowledge_sources WHERE type IN ('intercom_help_center', 'zendesk_help_center');
DELETE FROM integration_installations WHERE provider IN ('intercom', 'zendesk');

-- +goose Down
SELECT 1;

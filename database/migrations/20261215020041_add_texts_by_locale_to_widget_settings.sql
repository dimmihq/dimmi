-- migrate: phase=expand
-- A messenger's own words can differ by language: a greeting written for
-- French visitors, another for everyone else. Each language's words are
-- kept apart from the texts every language shares, by language tag.

-- +goose Up
ALTER TABLE widget_settings ADD COLUMN IF NOT EXISTS texts_by_locale jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE widget_settings DROP COLUMN IF EXISTS texts_by_locale;

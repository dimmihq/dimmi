-- migrate: phase=expand
-- A help center speaks several languages. locale stays its default
-- language, the one its articles and collections are written in, and
-- the one its addresses carry no prefix for. locales lists every
-- language readers may pick, the default among them. Help centers made
-- before this keep the one language they had.

-- +goose Up
ALTER TABLE help_centers ADD COLUMN locales text[] NOT NULL DEFAULT '{}';
UPDATE help_centers SET locales = ARRAY[locale] WHERE locales = '{}';

-- +goose Down
ALTER TABLE help_centers DROP COLUMN locales;

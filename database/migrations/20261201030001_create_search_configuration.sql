-- migrate: phase=expand
-- Search folds accents and stems words the same way in stored documents
-- and in queries, so "Rückerstattung" and "ruckerstattung" meet. The text
-- search configuration can be used in generated columns because
-- to_tsvector with a named configuration is immutable; unaccent itself is
-- only stable, so names and subjects go through an immutable wrapper when
-- they are indexed as trigrams. btree_gin lets a GIN index lead with the
-- workspace, as every other index does.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gin;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TEXT SEARCH CONFIGURATION dimmi_search (COPY = english);
ALTER TEXT SEARCH CONFIGURATION dimmi_search
    ALTER MAPPING FOR hword, hword_part, word WITH unaccent, english_stem;

-- +goose StatementBegin
CREATE FUNCTION dimmi_unaccent(value text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
    AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, value) $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION dimmi_unaccent(text);
DROP TEXT SEARCH CONFIGURATION dimmi_search;

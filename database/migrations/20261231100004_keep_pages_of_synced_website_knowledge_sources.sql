-- migrate: phase=expand
-- A website source with no pages chosen now reads the site's top twenty
-- pages, where it used to read up to five hundred. Sources that already
-- hold pages and never chose any keep the most a source may read, a
-- hundred, so their next sync does not shrink them to twenty.

-- +goose Up
UPDATE knowledge_sources s SET config = s.config || '{"max_pages": 100}'::jsonb
WHERE s.type = 'website' AND NOT s.config ? 'max_pages' AND s.include = '{}'
    AND EXISTS (SELECT 1 FROM knowledge_documents d WHERE d.source_id = s.id);

-- +goose Down
SELECT 1;

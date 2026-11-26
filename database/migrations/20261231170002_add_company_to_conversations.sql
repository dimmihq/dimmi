-- migrate: phase=expand
-- A conversation is about one of its contact's companies. A new
-- conversation takes the contact's default company unless it names one,
-- and a teammate can switch it to another of the contact's companies
-- later. Existing conversations take their contact's company now.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS company_id uuid REFERENCES companies ON DELETE SET NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION conversations_default_company() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.company_id IS NULL THEN
        SELECT ct.company_id INTO NEW.company_id FROM contacts ct WHERE ct.id = NEW.contact_id;
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE OR REPLACE TRIGGER conversations_default_company
    BEFORE INSERT ON conversations
    FOR EACH ROW EXECUTE FUNCTION conversations_default_company();

UPDATE conversations c SET company_id = ct.company_id
FROM contacts ct
WHERE ct.id = c.contact_id AND ct.company_id IS NOT NULL AND c.company_id IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS conversations_workspace_id_company_id_index
    ON conversations (workspace_id, company_id) WHERE company_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS conversations_workspace_id_company_id_index;
DROP TRIGGER IF EXISTS conversations_default_company ON conversations;
DROP FUNCTION IF EXISTS conversations_default_company();
ALTER TABLE conversations DROP COLUMN IF EXISTS company_id;

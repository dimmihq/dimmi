-- migrate: phase=expand
-- A contact can belong to several companies, one of them the default.
-- contacts.company_id stays the default company and the two are kept in
-- step by triggers, so whatever writes either one leaves both right:
-- setting contacts.company_id links that company and makes it the
-- default; clearing it unlinks the default company; linking the first
-- company makes it the default; unlinking the default makes the oldest
-- remaining link the default. The custom setting
-- dimmi.contact_companies_sync stops the link trigger from reacting to
-- the contact trigger's own writes. Contact list filters on company now
-- match any linked company. The current links are backfilled.

-- +goose Up
CREATE TABLE contact_companies (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    company_id uuid NOT NULL REFERENCES companies ON DELETE CASCADE,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contact_id, company_id)
);

CREATE UNIQUE INDEX contact_companies_contact_id_default_key ON contact_companies (contact_id) WHERE is_default;
CREATE INDEX contact_companies_workspace_id_company_id_index ON contact_companies (workspace_id, company_id);

ALTER TABLE contact_companies ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_companies_tenant ON contact_companies
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE FUNCTION contacts_sync_companies() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    next_company uuid;
BEGIN
    PERFORM set_config('dimmi.contact_companies_sync', 'on', true);

    IF NEW.company_id IS NOT NULL THEN
        UPDATE contact_companies SET is_default = false
        WHERE contact_id = NEW.id AND is_default AND company_id <> NEW.company_id;

        INSERT INTO contact_companies (workspace_id, contact_id, company_id, is_default)
        VALUES (NEW.workspace_id, NEW.id, NEW.company_id, true)
        ON CONFLICT (contact_id, company_id) DO UPDATE SET is_default = true
        WHERE NOT contact_companies.is_default;
    ELSE
        DELETE FROM contact_companies WHERE contact_id = NEW.id AND company_id = OLD.company_id;

        SELECT cc.company_id INTO next_company FROM contact_companies cc
        WHERE cc.contact_id = NEW.id
        ORDER BY cc.created_at, cc.company_id
        LIMIT 1;
    END IF;

    PERFORM set_config('dimmi.contact_companies_sync', '', true);

    IF next_company IS NOT NULL THEN
        UPDATE contacts SET company_id = next_company WHERE id = NEW.id;
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER contacts_sync_companies_insert
    AFTER INSERT ON contacts
    FOR EACH ROW WHEN (NEW.company_id IS NOT NULL)
    EXECUTE FUNCTION contacts_sync_companies();

CREATE TRIGGER contacts_sync_companies_update
    AFTER UPDATE OF company_id ON contacts
    FOR EACH ROW WHEN (OLD.company_id IS DISTINCT FROM NEW.company_id)
    EXECUTE FUNCTION contacts_sync_companies();

-- +goose StatementBegin
CREATE FUNCTION contact_companies_sync_contact() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    next_company uuid;
BEGIN
    IF current_setting('dimmi.contact_companies_sync', true) = 'on' THEN
        RETURN NULL;
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.is_default OR NOT EXISTS (
            SELECT 1 FROM contact_companies cc
            WHERE cc.contact_id = NEW.contact_id AND cc.is_default
        ) THEN
            UPDATE contacts SET company_id = NEW.company_id
            WHERE id = NEW.contact_id AND company_id IS DISTINCT FROM NEW.company_id;
        END IF;
    ELSIF OLD.is_default THEN
        SELECT cc.company_id INTO next_company FROM contact_companies cc
        WHERE cc.contact_id = OLD.contact_id
        ORDER BY cc.created_at, cc.company_id
        LIMIT 1;

        UPDATE contacts SET company_id = next_company
        WHERE id = OLD.contact_id AND company_id IS DISTINCT FROM next_company;
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER contact_companies_sync_contact
    AFTER INSERT OR DELETE ON contact_companies
    FOR EACH ROW EXECUTE FUNCTION contact_companies_sync_contact();

INSERT INTO contact_companies (workspace_id, contact_id, company_id, is_default, created_at)
SELECT c.workspace_id, c.id, c.company_id, true, c.created_at FROM contacts c
WHERE c.company_id IS NOT NULL
ON CONFLICT (contact_id, company_id) DO NOTHING;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION contact_matches(c contacts, conditions jsonb) RETURNS boolean
LANGUAGE plpgsql STABLE AS $$
DECLARE
    cond jsonb;
    field text;
    op text;
    want text;
    wants text[];
    actual text;
    raw jsonb;
    num numeric;
    stamp timestamptz;
    tagged bigint;
    matched bigint;
    linked bigint;
    ok boolean;
BEGIN
    FOR cond IN SELECT jsonb_array_elements(conditions) LOOP
        field := cond ->> 'field';
        op := cond ->> 'op';
        want := cond ->> 'value';
        wants := ARRAY(SELECT jsonb_array_elements_text(coalesce(cond -> 'values', '[]'::jsonb)));

        IF field = 'tags' THEN
            SELECT count(*) INTO tagged FROM contact_tag ct
            WHERE ct.workspace_id = c.workspace_id AND ct.contact_id = c.id AND ct.tag_id::text = ANY (wants);
            ok := CASE op
                WHEN 'has_any' THEN tagged > 0
                WHEN 'has_all' THEN tagged = cardinality(wants)
                WHEN 'has_none' THEN tagged = 0
                ELSE false
            END;
        ELSIF field IN ('last_seen', 'created') THEN
            stamp := CASE field WHEN 'last_seen' THEN c.last_seen_at ELSE c.created_at END;
            ok := CASE op
                WHEN 'before' THEN stamp < want::timestamptz
                WHEN 'after' THEN stamp >= want::timestamptz
                WHEN 'unknown' THEN stamp IS NULL
                WHEN 'known' THEN stamp IS NOT NULL
                ELSE false
            END;
        ELSIF field = 'kind' THEN
            ok := CASE op WHEN 'is' THEN c.kind = want WHEN 'is_not' THEN c.kind <> want ELSE false END;
        ELSIF field = 'company' THEN
            SELECT count(*) FILTER (WHERE cc.company_id::text = ANY (wants)), count(*) INTO matched, linked
            FROM contact_companies cc
            WHERE cc.workspace_id = c.workspace_id AND cc.contact_id = c.id;
            ok := CASE op
                WHEN 'is' THEN matched > 0
                WHEN 'is_not' THEN matched = 0
                WHEN 'unknown' THEN linked = 0
                WHEN 'known' THEN linked > 0
                ELSE false
            END;
        ELSIF field LIKE 'custom.%' AND cond ->> 'type' IN ('number', 'boolean') THEN
            raw := c.custom_attributes -> substr(field, 8);
            num := CASE WHEN jsonb_typeof(raw) = 'number' THEN (raw #>> '{}')::numeric END;
            ok := CASE op
                WHEN 'unknown' THEN raw IS NULL OR jsonb_typeof(raw) = 'null'
                WHEN 'known' THEN raw IS NOT NULL AND jsonb_typeof(raw) <> 'null'
                WHEN 'true' THEN raw = 'true'::jsonb
                WHEN 'false' THEN raw = 'false'::jsonb
                WHEN 'eq' THEN num = want::numeric
                WHEN 'neq' THEN num IS NULL OR num <> want::numeric
                WHEN 'gt' THEN num > want::numeric
                WHEN 'lt' THEN num < want::numeric
                ELSE false
            END;
        ELSE
            actual := lower(CASE field
                WHEN 'name' THEN c.name
                WHEN 'email' THEN c.email::text
                WHEN 'phone' THEN c.phone
                WHEN 'user_id' THEN c.external_id
                ELSE c.custom_attributes ->> substr(field, 8)
            END);
            want := lower(want);
            ok := CASE op
                WHEN 'is' THEN actual = want
                WHEN 'is_not' THEN actual IS NULL OR actual <> want
                WHEN 'contains' THEN strpos(actual, want) > 0
                WHEN 'not_contains' THEN actual IS NULL OR strpos(actual, want) = 0
                WHEN 'starts_with' THEN left(actual, length(want)) = want
                WHEN 'ends_with' THEN right(actual, length(want)) = want
                WHEN 'unknown' THEN actual IS NULL OR actual = ''
                WHEN 'known' THEN actual IS NOT NULL AND actual <> ''
                ELSE false
            END;
        END IF;

        IF ok IS NOT TRUE THEN
            RETURN false;
        END IF;
    END LOOP;

    RETURN true;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER contact_companies_sync_contact ON contact_companies;
DROP TRIGGER contacts_sync_companies_update ON contacts;
DROP TRIGGER contacts_sync_companies_insert ON contacts;
DROP FUNCTION contact_companies_sync_contact();
DROP FUNCTION contacts_sync_companies();
DROP TABLE contact_companies;

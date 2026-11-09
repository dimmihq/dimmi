-- migrate: phase=expand
-- contact_matches says whether a contact passes every condition of a
-- list's filters. A condition names a field (name, email, phone, user_id,
-- kind, company, tags, last_seen, created, or custom.<key> with the
-- attribute's type), a comparison and a value or values. Times arrive as
-- instants the application worked out from its own clock, so the database
-- never reads the time itself. Text compares without case. A missing
-- value passes only "unknown" and the negative comparisons. The checks
-- are made in the application before they reach here, so an unknown field
-- or comparison simply fails.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION contact_matches(c contacts, conditions jsonb) RETURNS boolean
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
            ok := CASE op
                WHEN 'is' THEN c.company_id::text = ANY (wants)
                WHEN 'is_not' THEN c.company_id IS NULL OR NOT (c.company_id::text = ANY (wants))
                WHEN 'unknown' THEN c.company_id IS NULL
                WHEN 'known' THEN c.company_id IS NOT NULL
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
DROP FUNCTION contact_matches(contacts, jsonb);

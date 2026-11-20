-- migrate: phase=expand
-- What the team knows about a customer or a company, as separate notes:
-- each is about one or the other, has the teammate who wrote it and
-- when, and one of a record's notes can be pinned so it shows first. The
-- single notes text a contact or company held before becomes that
-- record's first note, with no author; the old columns stay as they were
-- for the API.

-- +goose Up
CREATE TABLE customer_notes (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid REFERENCES contacts ON DELETE CASCADE,
    company_id uuid REFERENCES companies ON DELETE CASCADE,
    author_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 10000),
    pinned_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT customer_notes_one_subject_check CHECK ((contact_id IS NULL) <> (company_id IS NULL))
);

CREATE INDEX customer_notes_workspace_id_contact_id_created_at_index
    ON customer_notes (workspace_id, contact_id, created_at DESC) WHERE contact_id IS NOT NULL;
CREATE INDEX customer_notes_workspace_id_company_id_created_at_index
    ON customer_notes (workspace_id, company_id, created_at DESC) WHERE company_id IS NOT NULL;

ALTER TABLE customer_notes ENABLE ROW LEVEL SECURITY;
CREATE POLICY customer_notes_tenant ON customer_notes USING (workspace_id = ANY (dimmi_visible_workspaces()));

INSERT INTO customer_notes (workspace_id, contact_id, body, created_at, updated_at)
SELECT workspace_id, id, left(notes, 10000), updated_at, updated_at
FROM contacts
WHERE notes IS NOT NULL AND btrim(notes) <> '';

INSERT INTO customer_notes (workspace_id, company_id, body, created_at, updated_at)
SELECT workspace_id, id, left(notes, 10000), updated_at, updated_at
FROM companies
WHERE notes IS NOT NULL AND btrim(notes) <> '';

-- +goose Down
DROP TABLE customer_notes;

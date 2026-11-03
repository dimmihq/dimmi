-- migrate: phase=expand
-- An attachment is a file stored on a disk and shown in a message. An
-- upload exists before its message does, so the message is optional
-- until the message is sent; a job removes uploads never attached. Who
-- uploaded it is a registry value, since every kind of author can. A
-- file is served only after its scan says so.

-- +goose Up
CREATE TABLE attachments (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    message_id uuid REFERENCES messages ON DELETE CASCADE,
    uploaded_by_type text NOT NULL CHECK (uploaded_by_type ~ '^[a-z][a-z0-9_]{1,39}$'),
    uploaded_by_id uuid,
    disk text NOT NULL,
    path text NOT NULL,
    filename text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256 text,
    width integer,
    height integer,
    inline boolean NOT NULL DEFAULT false,
    content_id text,
    scan_status text NOT NULL DEFAULT 'pending' CHECK (scan_status IN ('pending', 'clean', 'infected', 'skipped')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE attachments ENABLE ROW LEVEL SECURITY;
CREATE POLICY attachments_tenant ON attachments USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE attachments;

-- migrate: phase=expand
-- Attribute definitions give the custom attributes of contacts and
-- companies a type, a label and a status. A value that does not fit its
-- type is dropped instead of stored, so rules compare like with like;
-- dates are kept as Unix seconds. A definition is made by the team, or
-- observed on the first verified write of a new key; browsers never make
-- one. Archived definitions keep their values but take no new writes.
--
-- Problems group the writes that were dropped, one row per key and
-- reason, so silent failures are visible. They hold no values, only a
-- shape, and can be rebuilt at any time.

-- +goose Up
CREATE TABLE attribute_definitions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    object text NOT NULL CHECK (object IN ('contact', 'company')),
    key text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{0,63}$'),
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    description text CHECK (char_length(description) <= 500),
    data_type text NOT NULL CHECK (data_type IN ('string', 'number', 'boolean', 'date', 'list')),
    options jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(options) = 'array' AND jsonb_array_length(options) <= 200),
    origin text NOT NULL DEFAULT 'created' CHECK (origin IN ('created', 'observed')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    last_value_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX attribute_definitions_workspace_id_object_key_key
    ON attribute_definitions (workspace_id, object, key);

ALTER TABLE attribute_definitions ENABLE ROW LEVEL SECURITY;
CREATE POLICY attribute_definitions_tenant ON attribute_definitions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE attribute_problems (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    object text NOT NULL CHECK (object IN ('contact', 'company')),
    key text NOT NULL,
    code text NOT NULL,
    count bigint NOT NULL DEFAULT 1,
    sample text,
    last_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, object, key, code)
);

ALTER TABLE attribute_problems ENABLE ROW LEVEL SECURITY;
CREATE POLICY attribute_problems_tenant ON attribute_problems
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE attribute_problems;
DROP TABLE attribute_definitions;

-- migrate: phase=expand
-- A series is a journey people or whole accounts travel through over
-- days or weeks: messages, waits, checks and actions joined by paths.
-- The working copy changes freely; publishing freezes it into a version
-- nobody edits again. Each traveler is one enrollment with a cursor
-- and a sequence number that moves on every step, so a job carrying an
-- old number finds nothing to do and a retry never repeats a step. At
-- most one enrollment per series and person, or company, is active at
-- a time. Every step a traveler takes leaves one row, which is both
-- the trace a teammate reads and what the report counts.

-- +goose Up
CREATE TABLE series (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    subject text NOT NULL CHECK (subject IN ('contact', 'company')),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'live', 'paused', 'archived')),
    working jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(working::text) <= 262144),
    working_hash text NOT NULL DEFAULT '',
    live_version_id uuid,
    lock_version integer NOT NULL DEFAULT 1,
    topic_id uuid REFERENCES broadcast_topics ON DELETE SET NULL,
    service boolean NOT NULL DEFAULT false,
    email_address_id uuid REFERENCES email_addresses ON DELETE SET NULL,
    sender_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    slack_as text NOT NULL DEFAULT 'bot' CHECK (slack_as IN ('bot', 'member')),
    reply_team_id uuid REFERENCES teams ON DELETE SET NULL,
    reply_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    ai_replies boolean NOT NULL DEFAULT false,
    counts jsonb NOT NULL DEFAULT '{}',
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    published_at timestamptz,
    paused_at timestamptz,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX series_workspace_id_name_key ON series (workspace_id, lower(name)) WHERE archived_at IS NULL;
CREATE INDEX series_workspace_id_status_index ON series (workspace_id, status);
CREATE INDEX series_live_index ON series (status) WHERE status = 'live';

CREATE TABLE series_versions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    series_id uuid NOT NULL REFERENCES series ON DELETE CASCADE,
    number integer NOT NULL,
    spec jsonb NOT NULL,
    spec_hash text NOT NULL,
    note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 280),
    moves jsonb NOT NULL DEFAULT '{}',
    published_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    restored_from_version_id uuid REFERENCES series_versions ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX series_versions_series_id_number_key ON series_versions (series_id, number);

ALTER TABLE series ADD CONSTRAINT series_live_version_id_fkey
    FOREIGN KEY (live_version_id) REFERENCES series_versions ON DELETE SET NULL;

CREATE TABLE series_enrollments (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    series_id uuid NOT NULL REFERENCES series ON DELETE CASCADE,
    version_id uuid NOT NULL REFERENCES series_versions ON DELETE RESTRICT,
    subject text NOT NULL CHECK (subject IN ('contact', 'company')),
    contact_id uuid REFERENCES contacts ON DELETE CASCADE,
    company_id uuid REFERENCES companies ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'waiting', 'finished', 'exited')),
    node_id text NOT NULL,
    seq bigint NOT NULL DEFAULT 0,
    wait jsonb,
    wake_at timestamptz,
    entered_at timestamptz NOT NULL,
    entered_by text NOT NULL CHECK (entered_by IN ('rules', 'event', 'manual', 'api', 'test')),
    trigger_ref text,
    goal_at timestamptz,
    left_at timestamptz,
    end_reason text CHECK (end_reason ~ '^[a-z][a-z0-9_]{1,39}$'),
    variants jsonb NOT NULL DEFAULT '{}',
    test boolean NOT NULL DEFAULT false,
    steps_executed integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((subject = 'contact' AND contact_id IS NOT NULL) OR (subject = 'company' AND company_id IS NOT NULL))
);

CREATE UNIQUE INDEX series_enrollments_active_contact_key ON series_enrollments (series_id, contact_id)
    WHERE status IN ('active', 'waiting') AND subject = 'contact';
CREATE UNIQUE INDEX series_enrollments_active_company_key ON series_enrollments (series_id, company_id)
    WHERE status IN ('active', 'waiting') AND subject = 'company';
CREATE UNIQUE INDEX series_enrollments_trigger_ref_key ON series_enrollments (series_id, trigger_ref)
    WHERE trigger_ref IS NOT NULL;
CREATE INDEX series_enrollments_wake_at_index ON series_enrollments (wake_at) WHERE status = 'waiting';
CREATE INDEX series_enrollments_contact_id_index ON series_enrollments (workspace_id, contact_id, entered_at DESC)
    WHERE contact_id IS NOT NULL;
CREATE INDEX series_enrollments_company_id_index ON series_enrollments (workspace_id, company_id, entered_at DESC)
    WHERE company_id IS NOT NULL;
CREATE INDEX series_enrollments_series_id_node_id_index ON series_enrollments (series_id, node_id)
    WHERE status IN ('active', 'waiting');
CREATE INDEX series_enrollments_series_id_entered_at_index ON series_enrollments (series_id, entered_at DESC);

CREATE TABLE series_steps (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    series_id uuid NOT NULL REFERENCES series ON DELETE CASCADE,
    enrollment_id uuid NOT NULL REFERENCES series_enrollments ON DELETE CASCADE,
    version_id uuid NOT NULL REFERENCES series_versions ON DELETE RESTRICT,
    node_id text NOT NULL,
    kind text NOT NULL,
    status text NOT NULL CHECK (status IN ('done', 'waiting', 'skipped', 'failed')),
    path_id text,
    outcome text,
    detail jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(detail::text) <= 4096),
    test boolean NOT NULL DEFAULT false,
    entered_at timestamptz NOT NULL,
    left_at timestamptz
);

CREATE INDEX series_steps_enrollment_id_index ON series_steps (workspace_id, enrollment_id, entered_at);
CREATE INDEX series_steps_series_id_node_id_index ON series_steps (series_id, node_id, entered_at);

ALTER TABLE series ENABLE ROW LEVEL SECURITY;
CREATE POLICY series_tenant ON series USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE series_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY series_versions_tenant ON series_versions USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE series_enrollments ENABLE ROW LEVEL SECURITY;
CREATE POLICY series_enrollments_tenant ON series_enrollments
    USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE series_steps ENABLE ROW LEVEL SECURITY;
CREATE POLICY series_steps_tenant ON series_steps USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE series_steps;
DROP TABLE series_enrollments;
ALTER TABLE series DROP CONSTRAINT series_live_version_id_fkey;
DROP TABLE series_versions;
DROP TABLE series;

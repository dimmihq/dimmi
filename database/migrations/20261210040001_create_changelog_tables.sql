-- migrate: phase=expand
-- A changelog lives on its help center. An entry is edited as a working
-- copy and read as its published copy: publishing copies the working
-- fields over, so saving never changes what readers see until the next
-- publish. has_unpublished_changes says the two differ. A scheduled
-- entry publishes itself at scheduled_for. notified_at marks the one
-- mailing an entry ever sends, so republishing never mails twice.
-- Labels file entries under kinds of change. People are the teammates
-- who edited an entry and the ones chosen as its authors; while nobody
-- has chosen, the editors are its authors. Reactions count one emoji per
-- reader, who is known only by a keyed hash.

-- +goose Up
CREATE TABLE changelog_labels (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,60}$'),
    color text NOT NULL CHECK (color ~ '^#[0-9a-fA-F]{6}$'),
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX changelog_labels_help_center_id_slug_key ON changelog_labels (help_center_id, slug);
CREATE INDEX changelog_labels_workspace_id_help_center_id_position_index
    ON changelog_labels (workspace_id, help_center_id, position);

ALTER TABLE changelog_labels ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_labels_tenant ON changelog_labels USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE changelog_entries (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 150),
    slug citext NOT NULL CHECK (slug ~ '^[a-z0-9-]{1,120}$'),
    summary text CHECK (char_length(summary) <= 300),
    body_markdown text NOT NULL DEFAULT '' CHECK (octet_length(body_markdown) <= 262144),
    cover_url text CHECK (char_length(cover_url) <= 500),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'scheduled', 'published')),
    scheduled_for timestamptz,
    published_at timestamptz,
    published_title text,
    published_summary text,
    published_body_markdown text,
    published_body_html text,
    published_cover_url text,
    has_unpublished_changes boolean NOT NULL DEFAULT false,
    notify_subscribers boolean NOT NULL DEFAULT false,
    notified_at timestamptz,
    authors_chosen boolean NOT NULL DEFAULT false,
    lock_version integer NOT NULL DEFAULT 1,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (status <> 'scheduled' OR scheduled_for IS NOT NULL),
    CHECK (status <> 'published' OR published_at IS NOT NULL)
);

CREATE UNIQUE INDEX changelog_entries_help_center_id_slug_key ON changelog_entries (help_center_id, slug)
    WHERE deleted_at IS NULL;
CREATE INDEX changelog_entries_workspace_id_help_center_id_updated_at_index
    ON changelog_entries (workspace_id, help_center_id, updated_at DESC);
CREATE INDEX changelog_entries_help_center_id_published_at_index
    ON changelog_entries (help_center_id, published_at DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX changelog_entries_scheduled_for_index
    ON changelog_entries (scheduled_for) WHERE status = 'scheduled' AND deleted_at IS NULL;

ALTER TABLE changelog_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_entries_tenant ON changelog_entries USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE changelog_entry_labels (
    entry_id uuid NOT NULL REFERENCES changelog_entries ON DELETE CASCADE,
    label_id uuid NOT NULL REFERENCES changelog_labels ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    PRIMARY KEY (entry_id, label_id)
);

CREATE INDEX changelog_entry_labels_label_id_index ON changelog_entry_labels (label_id);

ALTER TABLE changelog_entry_labels ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_entry_labels_tenant ON changelog_entry_labels
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE changelog_entry_people (
    entry_id uuid NOT NULL REFERENCES changelog_entries ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('editor', 'author')),
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (entry_id, role, member_id)
);

CREATE INDEX changelog_entry_people_member_id_index ON changelog_entry_people (member_id);

ALTER TABLE changelog_entry_people ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_entry_people_tenant ON changelog_entry_people
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE changelog_reactions (
    entry_id uuid NOT NULL REFERENCES changelog_entries ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    reactor_hash text NOT NULL,
    emoji text NOT NULL CHECK (emoji IN ('thumbsup', 'heart', 'tada', 'rocket', 'eyes')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (entry_id, reactor_hash, emoji)
);

ALTER TABLE changelog_reactions ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_reactions_tenant ON changelog_reactions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE changelog_reactions;
DROP TABLE changelog_entry_people;
DROP TABLE changelog_entry_labels;
DROP TABLE changelog_entries;
DROP TABLE changelog_labels;

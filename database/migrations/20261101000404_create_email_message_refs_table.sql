-- migrate: phase=expand
-- The threading index: every Message-ID seen or sent, pointing at the
-- conversation it belongs to. Ids mentioned in References before their
-- own mail arrives are kept too, so a reply that overtakes its original
-- still finds its thread. Rows are only ever added, and the same id may
-- point at several conversations of one workspace after a merge. Ids of
-- one workspace never answer another workspace's lookup.

-- +goose Up
CREATE TABLE email_message_refs (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    rfc_message_id text NOT NULL CHECK (char_length(rfc_message_id) BETWEEN 1 AND 998),
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    message_id uuid REFERENCES messages ON DELETE SET NULL,
    kind text NOT NULL CHECK (kind IN ('inbound', 'outbound', 'outbound_provider', 'referenced')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, rfc_message_id, conversation_id)
);

CREATE INDEX email_message_refs_workspace_id_conversation_id_index
    ON email_message_refs (workspace_id, conversation_id);

-- +goose Down
DROP TABLE email_message_refs;

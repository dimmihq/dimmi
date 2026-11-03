-- migrate: phase=expand
-- A thread ties a conversation to the address it is mailed from, with
-- the reply token that goes into the Reply-To address. A reply to that
-- address finds its conversation even when the customer's client drops
-- every threading header. Tokens are unique on the whole install and are
-- revoked, never reused, when their conversation goes.

-- +goose Up
CREATE TABLE email_threads (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    email_address_id uuid REFERENCES email_addresses ON DELETE SET NULL,
    reply_token text NOT NULL CHECK (reply_token ~ '^[0-9a-hjkmnp-tv-z]{16}$'),
    root_message_id text,
    normalized_subject text,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT email_threads_reply_token_key UNIQUE (reply_token),
    CONSTRAINT email_threads_conversation_id_email_address_id_key
        UNIQUE NULLS NOT DISTINCT (conversation_id, email_address_id)
);

CREATE INDEX email_threads_workspace_id_conversation_id_index ON email_threads (workspace_id, conversation_id);

-- +goose Down
DROP TABLE email_threads;

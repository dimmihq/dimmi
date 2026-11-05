-- migrate: phase=expand
-- A forwarding test is one real email sent to a public address, waited
-- for at the forwarding address. Its token rides in a header and in the
-- body's last line, since some forwards drop extra headers; a token is
-- unique on the install so a returning test finds its own row. A test
-- that is not back in time times out. Tests older than thirty days are
-- pruned across workspaces by a security definer function.

-- +goose Up
CREATE TABLE email_address_tests (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    email_address_id uuid NOT NULL REFERENCES email_addresses ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'sending'
        CHECK (status IN ('sending', 'waiting', 'received', 'failed', 'timed_out')),
    token text NOT NULL,
    outbound_email_id uuid REFERENCES outbound_emails ON DELETE SET NULL,
    inbound_email_id uuid REFERENCES inbound_emails ON DELETE SET NULL,
    findings jsonb NOT NULL DEFAULT '[]',
    latency_ms integer CHECK (latency_ms >= 0),
    expires_at timestamptz NOT NULL,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX email_address_tests_token_key ON email_address_tests (token);
CREATE INDEX email_address_tests_email_address_id_created_at_index
    ON email_address_tests (email_address_id, created_at DESC);
CREATE INDEX email_address_tests_workspace_id_created_at_index ON email_address_tests (workspace_id, created_at);
CREATE INDEX email_address_tests_outbound_email_id_index ON email_address_tests (outbound_email_id);
CREATE INDEX email_address_tests_created_at_index ON email_address_tests (created_at);

ALTER TABLE email_address_tests ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_address_tests_tenant ON email_address_tests
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE FUNCTION prune_email_address_tests(before timestamptz)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH pruned AS (DELETE FROM email_address_tests WHERE created_at < before RETURNING 1)
        SELECT count(*) FROM pruned
    $$;

-- +goose Down
DROP FUNCTION prune_email_address_tests(timestamptz);
DROP TABLE email_address_tests;

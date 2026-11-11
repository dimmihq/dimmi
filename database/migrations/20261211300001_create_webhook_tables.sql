-- migrate: phase=expand
-- Outbound webhooks: endpoints a workspace's developers register, one
-- delivery per event and endpoint holding the exact signed bytes, and
-- one attempt row per request sent, which is the delivery log. A global
-- sequence orders snapshots of the same object. Retries and health
-- checks run across workspaces, so the due deliveries and the endpoints
-- to check are read through definer functions that return ids only.

-- +goose Up
CREATE SEQUENCE webhook_event_sequence AS bigint;

CREATE TABLE webhook_endpoints (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 255),
    event_types text[] NOT NULL CHECK (cardinality(event_types) BETWEEN 1 AND 100),
    include_notes boolean NOT NULL DEFAULT false,
    inbox_ids uuid[],
    api_version text NOT NULL DEFAULT '2026-11-01',
    enabled boolean NOT NULL DEFAULT true,
    disabled_reason text CHECK (disabled_reason IN ('manual', 'auto_failure', 'gone')),
    disabled_at timestamptz,
    secret_encrypted bytea NOT NULL,
    previous_secret_encrypted bytea,
    previous_secret_expires_at timestamptz,
    health text NOT NULL DEFAULT 'healthy' CHECK (health IN ('healthy', 'failing')),
    failing_since timestamptz,
    failing_notified_at timestamptz,
    last_success_at timestamptz,
    last_failure_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX webhook_endpoints_workspace_id_url_key ON webhook_endpoints (workspace_id, url);
CREATE INDEX webhook_endpoints_workspace_id_enabled_index ON webhook_endpoints (workspace_id, enabled);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    endpoint_id uuid NOT NULL REFERENCES webhook_endpoints ON DELETE CASCADE,
    event_id uuid NOT NULL,
    event_type text NOT NULL,
    sequence bigint NOT NULL,
    object_id uuid,
    contact_id uuid,
    payload bytea NOT NULL,
    payload_bytes integer NOT NULL,
    thin boolean NOT NULL DEFAULT false,
    test boolean NOT NULL DEFAULT false,
    note boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'retrying', 'succeeded', 'failed', 'canceled')),
    attempt_count integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz,
    first_attempt_at timestamptz,
    last_attempt_at timestamptz,
    succeeded_at timestamptz,
    last_response_status integer,
    last_error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE webhook_deliveries ALTER COLUMN payload SET COMPRESSION lz4;

CREATE UNIQUE INDEX webhook_deliveries_endpoint_id_event_id_key ON webhook_deliveries (endpoint_id, event_id);
CREATE INDEX webhook_deliveries_endpoint_id_created_at_index
    ON webhook_deliveries (workspace_id, endpoint_id, created_at DESC, id DESC);
CREATE INDEX webhook_deliveries_event_id_index ON webhook_deliveries (workspace_id, event_id);
CREATE INDEX webhook_deliveries_contact_id_index ON webhook_deliveries (workspace_id, contact_id)
    WHERE contact_id IS NOT NULL;
CREATE INDEX webhook_deliveries_due_index ON webhook_deliveries (next_attempt_at)
    WHERE status IN ('pending', 'retrying');
CREATE INDEX webhook_deliveries_created_at_index ON webhook_deliveries (created_at);

CREATE TABLE webhook_delivery_attempts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    delivery_id uuid NOT NULL REFERENCES webhook_deliveries ON DELETE CASCADE,
    number integer NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('auto', 'manual', 'test')),
    requested_at timestamptz NOT NULL,
    duration_ms integer,
    request_headers jsonb NOT NULL DEFAULT '{}',
    response_status integer,
    response_headers jsonb,
    response_body text,
    error_code text,
    error_message text CHECK (char_length(error_message) <= 500),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX webhook_delivery_attempts_delivery_id_number_key
    ON webhook_delivery_attempts (delivery_id, number);
CREATE INDEX webhook_delivery_attempts_workspace_id_requested_at_index
    ON webhook_delivery_attempts (workspace_id, requested_at);

ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;
CREATE POLICY webhook_endpoints_tenant ON webhook_endpoints
    USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY webhook_deliveries_tenant ON webhook_deliveries
    USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE webhook_delivery_attempts ENABLE ROW LEVEL SECURITY;
CREATE POLICY webhook_delivery_attempts_tenant ON webhook_delivery_attempts
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE FUNCTION webhook_deliveries_due(due_at timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT d.workspace_id, d.id
        FROM webhook_deliveries d
        WHERE d.status IN ('pending', 'retrying') AND d.next_attempt_at <= due_at
        ORDER BY d.next_attempt_at
        LIMIT row_limit
    $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION webhook_workspaces_with_endpoints()
    RETURNS TABLE (workspace_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT DISTINCT e.workspace_id FROM webhook_endpoints e
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION webhook_workspaces_with_endpoints();
DROP FUNCTION webhook_deliveries_due(timestamptz, integer);
DROP TABLE webhook_delivery_attempts;
DROP TABLE webhook_deliveries;
DROP TABLE webhook_endpoints;
DROP SEQUENCE webhook_event_sequence;

-- migrate: phase=expand

-- +goose Up
CREATE TABLE knowledge_usage_periods (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    period_start date NOT NULL,
    source_id uuid,
    kind text NOT NULL CHECK (kind IN ('embedding', 'crawl')),
    model_key text NOT NULL DEFAULT '' CHECK (char_length(model_key) <= 200),
    input_tokens bigint NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    estimated_tokens bigint NOT NULL DEFAULT 0 CHECK (estimated_tokens >= 0),
    browser_ms bigint NOT NULL DEFAULT 0 CHECK (browser_ms >= 0),
    requests integer NOT NULL DEFAULT 0 CHECK (requests >= 0),
    cost_micros bigint NOT NULL DEFAULT 0 CHECK (cost_micros >= 0),
    unpriced boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX knowledge_usage_periods_key
    ON knowledge_usage_periods (
        workspace_id, period_start, kind, model_key,
        COALESCE(source_id, '00000000-0000-0000-0000-000000000000'::uuid)
    );

ALTER TABLE knowledge_usage_periods ENABLE ROW LEVEL SECURITY;
CREATE POLICY knowledge_usage_periods_tenant ON knowledge_usage_periods
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE knowledge_usage_periods;

-- migrate: phase=expand
-- The workspace's one agent: how it looks, when and where it answers,
-- what it may spend, and which connections its models run on.

-- +goose Up
CREATE TABLE ai_agents (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL DEFAULT 'Dimmi',
    avatar_attachment_id uuid REFERENCES attachments ON DELETE SET NULL,
    enabled boolean NOT NULL DEFAULT false,
    product_name text,
    tone text NOT NULL DEFAULT 'friendly' CHECK (tone IN ('friendly', 'professional', 'concise')),
    guidance text NOT NULL DEFAULT '',
    enabled_channels text[] NOT NULL DEFAULT '{widget}',
    excluded_channel_connection_ids uuid[] NOT NULL DEFAULT '{}',
    schedule text NOT NULL DEFAULT 'always' CHECK (schedule IN ('always', 'outside_office_hours', 'inside_office_hours')),
    office_hours jsonb,
    assigned_behavior text NOT NULL DEFAULT 'skip_when_human_active'
        CHECK (assigned_behavior IN ('skip_when_human_active', 'never_when_assigned')),
    answer_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    answer_model text,
    classifier_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    classifier_model text,
    fallback_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    fallback_model text,
    temperature numeric(3,2),
    max_output_tokens integer NOT NULL DEFAULT 600,
    max_ai_replies integer NOT NULL DEFAULT 5,
    unknown_behavior text NOT NULL DEFAULT 'offer_handoff' CHECK (unknown_behavior IN ('offer_handoff', 'handoff')),
    grounding_mode text NOT NULL DEFAULT 'citations' CHECK (grounding_mode IN ('citations', 'verify')),
    require_citations boolean NOT NULL DEFAULT true,
    min_similarity numeric(3,2) NOT NULL DEFAULT 0.35,
    languages text[] NOT NULL DEFAULT '{}',
    default_language text NOT NULL DEFAULT 'en',
    streaming boolean NOT NULL DEFAULT true,
    include_contact_context text NOT NULL DEFAULT 'basic' CHECK (include_contact_context IN ('none', 'basic', 'allowlist')),
    contact_attribute_allowlist text[] NOT NULL DEFAULT '{}',
    pii_redaction boolean NOT NULL DEFAULT true,
    link_allowlist text[] NOT NULL DEFAULT '{}',
    handoff jsonb NOT NULL DEFAULT '{}',
    messages jsonb NOT NULL DEFAULT '{}',
    prompt_version text,
    monthly_spend_cap_micros bigint DEFAULT 25000000,
    actions_enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX ai_agents_workspace_id_key ON ai_agents (workspace_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE ai_agents;

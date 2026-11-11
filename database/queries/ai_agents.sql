-- name: FindAgent :one
-- FindAgent returns the workspace's live agent, if it exists.
SELECT * FROM ai_agents
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: CreateAgent :one
-- CreateAgent makes the workspace's one agent with its defaults.
INSERT INTO ai_agents (workspace_id, monthly_spend_cap_micros, created_at, updated_at)
VALUES (@workspace_id, @monthly_spend_cap_micros, @now, @now)
ON CONFLICT (workspace_id) WHERE deleted_at IS NULL DO NOTHING
RETURNING *;

-- name: UpdateAgent :one
-- UpdateAgent replaces the agent's mutable settings.
UPDATE ai_agents SET
    name = @name, avatar_attachment_id = @avatar_attachment_id, enabled = @enabled,
    product_name = @product_name, tone = @tone, guidance = @guidance,
    enabled_channels = @enabled_channels, excluded_channel_connection_ids = @excluded_channel_connection_ids,
    schedule = @schedule, assigned_behavior = @assigned_behavior,
    answer_key_id = @answer_key_id, answer_model = @answer_model,
    classifier_key_id = @classifier_key_id, classifier_model = @classifier_model,
    fallback_key_id = @fallback_key_id, fallback_model = @fallback_model,
    temperature = @temperature, max_output_tokens = @max_output_tokens, max_ai_replies = @max_ai_replies,
    unknown_behavior = @unknown_behavior, grounding_mode = @grounding_mode,
    require_citations = @require_citations, min_similarity = @min_similarity,
    languages = @languages, default_language = @default_language, streaming = @streaming,
    include_contact_context = @include_contact_context,
    contact_attribute_allowlist = @contact_attribute_allowlist,
    pii_redaction = @pii_redaction, link_allowlist = @link_allowlist,
    handoff = @handoff, messages = @messages, prompt_version = @prompt_version,
    monthly_spend_cap_micros = @monthly_spend_cap_micros, actions_enabled = @actions_enabled,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

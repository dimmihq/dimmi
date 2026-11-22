-- name: FindDefaultAgent :one
-- FindDefaultAgent returns the workspace's default agent, which answers
-- every conversation no other agent takes.
SELECT * FROM ai_agents
WHERE workspace_id = @workspace_id AND is_default AND deleted_at IS NULL;

-- name: FindAgentByID :one
-- FindAgentByID returns one of the workspace's agents.
SELECT * FROM ai_agents
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ListAgents :many
-- ListAgents returns the workspace's agents, the default first, then by
-- name.
SELECT * FROM ai_agents
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
ORDER BY is_default DESC, lower(name), id;

-- name: ListAgentsByID :many
-- ListAgentsByID returns the workspace's agents among some ids.
SELECT * FROM ai_agents
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: CreateDefaultAgent :one
-- CreateDefaultAgent makes the workspace's default agent with its
-- defaults, unless it has one already.
INSERT INTO ai_agents (workspace_id, is_default, monthly_spend_cap_micros, created_at, updated_at)
VALUES (@workspace_id, true, NULL, @now, @now)
ON CONFLICT (workspace_id) WHERE is_default AND deleted_at IS NULL DO NOTHING
RETURNING *;

-- name: CreateAgent :one
-- CreateAgent makes a new agent as a draft that has never been
-- published: its columns hold the defaults and its draft what it starts
-- from.
INSERT INTO ai_agents (
    workspace_id, name, role, template_key, draft, monthly_spend_cap_micros,
    created_by_member_id, updated_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @name, @role, sqlc.narg(template_key), @draft, NULL,
    sqlc.narg(member_id), sqlc.narg(member_id), @now, @now
)
RETURNING *;

-- name: SaveAgentDraft :one
-- SaveAgentDraft keeps the working copy when the editor started from the
-- stored lock version, and moves the version on. A nil draft says the
-- working copy is the published one again.
UPDATE ai_agents SET name = @name, draft = sqlc.narg(draft), lock_version = lock_version + 1,
    updated_by_member_id = sqlc.narg(member_id), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND lock_version = @lock_version
RETURNING *;

-- name: UpdateAgent :one
-- UpdateAgent replaces the agent's published settings: what customers
-- get. Publishing writes it from the draft.
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
    knowledge_scope = @knowledge_scope, procedure_scope = @procedure_scope, action_scope = @action_scope,
    transfer_agent_ids = @transfer_agent_ids,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: PublishAgent :one
-- PublishAgent marks the agent live on a version, its draft folded in.
UPDATE ai_agents SET status = 'live', enabled = true, draft = NULL, live_version_id = @live_version_id,
    published_at = @now, lock_version = lock_version + 1, updated_by_member_id = sqlc.narg(member_id),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: PauseAgent :one
-- PauseAgent stops the agent answering, keeping its published settings
-- and its draft.
UPDATE ai_agents SET status = 'paused', enabled = false, updated_by_member_id = sqlc.narg(member_id),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND status = 'live'
RETURNING *;

-- name: DeleteAgent :exec
-- DeleteAgent removes an agent that is not the default.
UPDATE ai_agents SET deleted_at = @now, status = 'paused', enabled = false, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND NOT is_default AND deleted_at IS NULL;

-- name: AgentNameTaken :one
-- AgentNameTaken reports whether another agent has the name.
SELECT EXISTS (
    SELECT 1 FROM ai_agents
    WHERE workspace_id = @workspace_id AND lower(name) = lower(@name) AND id <> @id AND deleted_at IS NULL
)::boolean;

-- name: SetAgentToolLayer :exec
-- SetAgentToolLayer turns an agent's tool layer on or off.
UPDATE ai_agents SET tool_layer = @tool_layer, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetAgentCarryPassages :exec
-- SetAgentCarryPassages turns an agent's carried knowledge on or off.
UPDATE ai_agents SET carry_passages = @carry_passages, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: KeyUsedByAgents :one
-- KeyUsedByAgents reports whether any agent answers or falls back on a
-- connection, in its published settings or its draft.
SELECT EXISTS (
    SELECT 1 FROM ai_agents
    WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND (
        answer_key_id = @key_id OR fallback_key_id = @key_id OR classifier_key_id = @key_id
        OR draft->>'answer_key_id' = (@key_id::uuid)::text
        OR draft->>'fallback_key_id' = (@key_id::uuid)::text
    )
)::boolean;

-- name: ResolveAgent :one
-- ResolveAgent picks the agent that answers a conversation, first found
-- of: the agent a workflow step names, the agent already on the
-- conversation, the agent its Slack channel, its channel connection or
-- its inbox names, and last the workspace's default.
SELECT sqlc.embed(a) FROM ai_agents a
JOIN (
    SELECT 1 AS rank, sqlc.narg(workflow_agent_id)::uuid AS agent_id
    UNION ALL SELECT 2, sqlc.narg(conversation_agent_id)::uuid
    UNION ALL SELECT 3, (SELECT r.ai_agent_id FROM channel_rooms r
        WHERE r.workspace_id = @workspace_id AND r.id = sqlc.narg(channel_room_id)::uuid)
    UNION ALL SELECT 4, (SELECT c.ai_agent_id FROM channel_connections c
        WHERE c.workspace_id = @workspace_id AND c.id = sqlc.narg(channel_connection_id)::uuid)
    UNION ALL SELECT 5, (SELECT i.ai_agent_id FROM inboxes i
        WHERE i.workspace_id = @workspace_id AND i.id = sqlc.narg(inbox_id)::uuid)
    UNION ALL SELECT 6, (SELECT d.id FROM ai_agents d
        WHERE d.workspace_id = @workspace_id AND d.is_default AND d.deleted_at IS NULL)
) c ON a.id = c.agent_id
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL
ORDER BY c.rank
LIMIT 1;

-- name: PinConversationAgent :exec
-- PinConversationAgent puts an agent on a conversation that has none, so
-- the same agent keeps answering it.
UPDATE conversations SET ai_agent_id = @agent_id
WHERE workspace_id = @workspace_id AND id = @id AND ai_agent_id IS NULL;

-- name: ListAgentPlacements :many
-- ListAgentPlacements returns where each of the workspace's agents takes
-- new conversations: its channel connections, Slack channels and inboxes.
SELECT 'connection'::text AS kind, cc.id, cc.ai_agent_id::uuid AS agent_id FROM channel_connections cc
WHERE cc.workspace_id = @workspace_id AND cc.ai_agent_id IS NOT NULL
UNION ALL
SELECT 'room'::text, cr.id, cr.ai_agent_id::uuid FROM channel_rooms cr
WHERE cr.workspace_id = @workspace_id AND cr.ai_agent_id IS NOT NULL
UNION ALL
SELECT 'inbox'::text, ib.id, ib.ai_agent_id::uuid FROM inboxes ib
WHERE ib.workspace_id = @workspace_id AND ib.ai_agent_id IS NOT NULL;

-- name: ClearAgentConnections :exec
-- ClearAgentConnections takes an agent off every channel connection.
UPDATE channel_connections SET ai_agent_id = NULL
WHERE workspace_id = @workspace_id AND ai_agent_id = @agent_id;

-- name: ClearAgentRooms :exec
-- ClearAgentRooms takes an agent off every Slack channel.
UPDATE channel_rooms SET ai_agent_id = NULL
WHERE workspace_id = @workspace_id AND ai_agent_id = @agent_id;

-- name: ClearAgentInboxes :exec
-- ClearAgentInboxes takes an agent off every inbox.
UPDATE inboxes SET ai_agent_id = NULL
WHERE workspace_id = @workspace_id AND ai_agent_id = @agent_id;

-- name: PlaceAgentOnConnections :exec
-- PlaceAgentOnConnections puts an agent on channel connections, taking
-- them from whichever agent had them.
UPDATE channel_connections SET ai_agent_id = @agent_id
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: PlaceAgentOnRooms :exec
-- PlaceAgentOnRooms puts an agent on Slack channels.
UPDATE channel_rooms SET ai_agent_id = @agent_id
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: PlaceAgentOnInboxes :exec
-- PlaceAgentOnInboxes puts an agent on inboxes.
UPDATE inboxes SET ai_agent_id = @agent_id
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: ListPlacementTargets :many
-- ListPlacementTargets returns everywhere an agent can be placed: the
-- workspace's channel connections, Slack channels and inboxes, each
-- with its name, its channel and the agent placed there, if any.
SELECT 'connection'::text AS kind, cc.id, cc.name, cc.type AS channel, cc.ai_agent_id
FROM channel_connections cc
WHERE cc.workspace_id = @workspace_id AND cc.deleted_at IS NULL AND cc.status <> 'disconnected'
UNION ALL
SELECT 'room'::text, cr.id, cr.name, cr.kind, cr.ai_agent_id FROM channel_rooms cr
WHERE cr.workspace_id = @workspace_id AND cr.status <> 'removed'
UNION ALL
SELECT 'inbox'::text, ib.id, ib.name, 'inbox'::text, ib.ai_agent_id FROM inboxes ib
WHERE ib.workspace_id = @workspace_id AND ib.deleted_at IS NULL
ORDER BY kind, name;

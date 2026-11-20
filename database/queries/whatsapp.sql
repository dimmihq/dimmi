-- name: ListWhatsAppConnections :many
-- ListWhatsAppConnections returns the workspace's live WhatsApp
-- connections, oldest first.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND type = 'whatsapp' AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: SealChannelConnectionCredentials :exec
-- SealChannelConnectionCredentials stores the connection's sealed
-- credentials, which are bound to its row.
UPDATE channel_connections SET credentials_encrypted = @credentials_encrypted, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateChannelConnectionFacts :one
-- UpdateChannelConnectionFacts merges what the provider says about the
-- connection into its config.
UPDATE channel_connections SET config = config || @config::jsonb, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MoveChannelConnection :one
-- MoveChannelConnection points the connection's new conversations at
-- another inbox of the workspace.
UPDATE channel_connections SET inbox_id = @inbox_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DisconnectChannelConnection :one
-- DisconnectChannelConnection retires the connection: its credentials
-- go, its provider key is freed so the number can connect again, and its
-- conversations stay.
UPDATE channel_connections
SET status = 'disconnected', credentials_encrypted = NULL, external_id = NULL, deleted_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: RecordChannelContactInbound :one
-- RecordChannelContactInbound notes a customer's message on the
-- connection: when it came, which message it was, and until when the
-- business may write freely. An older message arriving late moves
-- nothing back.
INSERT INTO channel_contact_states (
    workspace_id, channel_connection_id, contact_id, last_inbound_at, window_expires_at, last_inbound_message_id,
    created_at, updated_at
)
VALUES (
    @workspace_id, @channel_connection_id, @contact_id, @sent_at::timestamptz, @window_expires_at::timestamptz,
    @message_id, @now::timestamptz, @now::timestamptz
)
ON CONFLICT (channel_connection_id, contact_id) DO UPDATE
SET last_inbound_at = greatest(channel_contact_states.last_inbound_at, excluded.last_inbound_at),
    window_expires_at = greatest(channel_contact_states.window_expires_at, excluded.window_expires_at),
    last_inbound_message_id = CASE
        WHEN channel_contact_states.last_inbound_at IS NULL
            OR excluded.last_inbound_at >= channel_contact_states.last_inbound_at
        THEN excluded.last_inbound_message_id
        ELSE channel_contact_states.last_inbound_message_id
    END,
    updated_at = excluded.updated_at
RETURNING *;

-- name: FindChannelContactState :one
-- FindChannelContactState returns the customer's state on the
-- connection.
SELECT * FROM channel_contact_states
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id AND contact_id = @contact_id;

-- name: UpsertWhatsAppTemplate :exec
-- UpsertWhatsAppTemplate copies one of Meta's templates onto the
-- connection, replacing what was copied before.
INSERT INTO whatsapp_templates (
    workspace_id, channel_connection_id, external_id, name, language, category, status, components,
    parameter_count, sendable, rejected_reason, synced_at, created_at, updated_at
)
VALUES (
    @workspace_id, @channel_connection_id, @external_id, @name, @language, @category, @status, @components,
    @parameter_count, @sendable, sqlc.narg(rejected_reason), @now::timestamptz, @now::timestamptz, @now::timestamptz
)
ON CONFLICT (channel_connection_id, name, language) DO UPDATE
SET external_id = excluded.external_id, category = excluded.category, status = excluded.status,
    components = excluded.components, parameter_count = excluded.parameter_count, sendable = excluded.sendable,
    rejected_reason = excluded.rejected_reason, synced_at = excluded.synced_at, updated_at = excluded.updated_at;

-- name: DeleteStaleWhatsAppTemplates :exec
-- DeleteStaleWhatsAppTemplates drops the connection's templates Meta no
-- longer lists, those a sync started at the time did not touch.
DELETE FROM whatsapp_templates
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND synced_at < @started_at::timestamptz;

-- name: SetWhatsAppTemplateStatus :exec
-- SetWhatsAppTemplateStatus records a status Meta reported for one of
-- the connection's templates.
UPDATE whatsapp_templates
SET status = @status, rejected_reason = sqlc.narg(rejected_reason), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @external_id;

-- name: ListWhatsAppTemplates :many
-- ListWhatsAppTemplates returns the connection's templates, sendable
-- ones first, by name and language.
SELECT * FROM whatsapp_templates
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
ORDER BY sendable DESC, name, language;

-- name: FindWhatsAppTemplate :one
-- FindWhatsAppTemplate returns the connection's template with the name
-- and language.
SELECT * FROM whatsapp_templates
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id AND name = @name
  AND language = @language;

-- name: UpdateChannelMessageRefStatus :one
-- UpdateChannelMessageRefStatus records how far one provider message
-- got, never moving it back: read stays read when a late delivered
-- report arrives.
UPDATE channel_message_refs
SET status = @status, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_message_id = @external_message_id
  AND (CASE status WHEN 'sent' THEN 1 WHEN 'delivered' THEN 2 WHEN 'read' THEN 3 ELSE 4 END)
      < (CASE @status::text WHEN 'sent' THEN 1 WHEN 'delivered' THEN 2 WHEN 'read' THEN 3 ELSE 4 END)
RETURNING *;

-- name: ExpireWhatsAppTemplates :exec
-- ExpireWhatsAppTemplates marks the connection's copied templates out of
-- date, so the next look at them asks Meta again.
UPDATE whatsapp_templates SET synced_at = 'epoch'::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id;

-- name: FindWhatsAppTemplateByID :one
-- FindWhatsAppTemplateByID returns the connection's template with the id.
SELECT * FROM whatsapp_templates
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id AND id = @id;

-- name: DeleteWhatsAppTemplate :exec
-- DeleteWhatsAppTemplate drops the connection's copy of a template Meta
-- deleted.
DELETE FROM whatsapp_templates
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id AND id = @id;

-- name: SetWhatsAppTemplateBindings :exec
-- SetWhatsAppTemplateBindings records which customer detail each variable
-- of a template written in dimmi stands for.
UPDATE whatsapp_templates SET bindings = @bindings, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_id = @external_id;

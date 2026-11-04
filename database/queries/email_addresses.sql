-- name: CreateEmailAddress :one
-- CreateEmailAddress stores the detail of an email connection. A
-- forwarding address any workspace holds fails on
-- email_addresses_forwarding_address_key, and a public address the
-- workspace already connected fails on
-- email_addresses_workspace_id_address_key.
INSERT INTO email_addresses (
    workspace_id, channel_connection_id, address, display_name, forwarding_address, is_default, settings,
    created_at, updated_at
) VALUES (
    @workspace_id, @channel_connection_id, @address, @display_name, @forwarding_address, @is_default, @settings,
    @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindEmailAddress :one
-- FindEmailAddress returns the live address with the id, with the
-- connection and inbox behind it.
SELECT sqlc.embed(a), c.inbox_id, c.status AS connection_status, i.name AS inbox_name
FROM email_addresses AS a
JOIN channel_connections AS c ON c.id = a.channel_connection_id AND c.workspace_id = a.workspace_id
JOIN inboxes AS i ON i.id = c.inbox_id AND i.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id AND a.id = @id AND a.deleted_at IS NULL;

-- name: ListEmailAddresses :many
-- ListEmailAddresses returns the workspace's live addresses, the
-- default first, then oldest first, with the connection and inbox
-- behind each.
SELECT sqlc.embed(a), c.inbox_id, c.status AS connection_status, i.name AS inbox_name
FROM email_addresses AS a
JOIN channel_connections AS c ON c.id = a.channel_connection_id AND c.workspace_id = a.workspace_id
JOIN inboxes AS i ON i.id = c.inbox_id AND i.workspace_id = a.workspace_id
WHERE a.workspace_id = @workspace_id AND a.deleted_at IS NULL
ORDER BY a.is_default DESC, a.created_at, a.id;

-- name: FindDefaultEmailAddress :one
-- FindDefaultEmailAddress returns the workspace's live default address.
SELECT * FROM email_addresses WHERE workspace_id = @workspace_id AND is_default AND deleted_at IS NULL;

-- name: FindEmailAddressByConnection :one
-- FindEmailAddressByConnection returns the address of the connection,
-- deleted or not.
SELECT * FROM email_addresses WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id;

-- name: FindEmailAddressByID :one
-- FindEmailAddressByID returns the address with the id, deleted or not.
SELECT * FROM email_addresses WHERE workspace_id = @workspace_id AND id = @id;

-- name: RouteEmailAddress :one
-- RouteEmailAddress finds whose forwarding address it is, across every
-- workspace, before any workspace is known, and whether the address was
-- deleted, recently or long ago, or its workspace is suspended.
SELECT
    r.workspace_id::uuid AS workspace_id,
    r.email_address_id::uuid AS email_address_id,
    r.channel_connection_id::uuid AS channel_connection_id,
    (r.deleted_at IS NOT NULL)::boolean AS deleted,
    coalesce(r.deleted_at >= @deleted_since::timestamptz, false)::boolean AS deleted_recently,
    (r.suspended_at IS NOT NULL)::boolean AS suspended
FROM email_route_address(@forwarding_address::citext) AS r;

-- name: UpdateEmailAddress :one
-- UpdateEmailAddress changes the address's own settings.
UPDATE email_addresses
SET display_name = @display_name, reply_to_mode = @reply_to_mode, from_name_style = @from_name_style,
    settings = @settings, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteEmailAddress :exec
-- DeleteEmailAddress disconnects the address and its connection. The
-- rows stay, so mail still forwarded to it can be explained, and the
-- address is no longer the default.
WITH address AS (
    UPDATE email_addresses
    SET deleted_at = @now::timestamptz, is_default = false, updated_at = @now::timestamptz
    WHERE email_addresses.workspace_id = @workspace_id AND email_addresses.id = @id
      AND email_addresses.deleted_at IS NULL
    RETURNING channel_connection_id
)
UPDATE channel_connections
SET status = 'disconnected', deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE channel_connections.workspace_id = @workspace_id AND channel_connections.id IN (SELECT channel_connection_id FROM address);

-- name: RecordEmailAddressInbound :exec
-- RecordEmailAddressInbound notes that mail reached the address, which
-- proves its forwarding works the first time.
UPDATE email_addresses
SET last_inbound_at = @now::timestamptz, inbound_verified_at = coalesce(inbound_verified_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordEmailAddressOutbound :exec
-- RecordEmailAddressOutbound notes that mail left from the address.
UPDATE email_addresses SET last_outbound_at = @now::timestamptz, updated_at = @now::timestamptz WHERE workspace_id = @workspace_id AND id = @id;

-- name: StoreForwardingConfirmation :exec
-- StoreForwardingConfirmation keeps the latest confirmation a mail
-- provider sent when forwarding was set up, so an admin can finish it.
UPDATE email_addresses
SET forwarding_confirmation = @forwarding_confirmation, last_inbound_at = @now::timestamptz,
    inbound_verified_at = coalesce(inbound_verified_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListWorkspaceEmailIdentities :many
-- ListWorkspaceEmailIdentities returns every address the workspace
-- itself receives or sends as, public and forwarding, so its own
-- addresses are never taken for customers.
SELECT DISTINCT unnest(ARRAY[address::text, forwarding_address::text])::text AS address
FROM email_addresses
WHERE workspace_id = @workspace_id;

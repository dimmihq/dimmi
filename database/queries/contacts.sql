-- name: CreateContact :one
-- CreateContact stores a customer in the workspace. An external id
-- another contact holds fails on contacts_workspace_id_external_id_key.
INSERT INTO contacts (
    workspace_id, kind, external_id, email, email_verified, name, phone, custom_attributes, first_seen_at,
    last_seen_at, created_at, updated_at
) VALUES (
    @workspace_id, @kind, @external_id, @email, @email_verified, @name, @phone, @custom_attributes,
    @now::timestamptz, @now::timestamptz, @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindContact :one
-- FindContact returns the contact with the id in the workspace, merged
-- or deleted ones included.
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindContactByExternalID :one
-- FindContactByExternalID returns the contact the host app knows by the
-- id.
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND external_id = @external_id;

-- name: FindContactByIdentity :one
-- FindContactByIdentity returns the contact that appears on a channel
-- with the identity, such as an address or a chat user id.
SELECT c.* FROM contacts AS c
JOIN contact_identities AS i ON i.contact_id = c.id AND i.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND i.type = @type AND i.value = @value;

-- name: CreateContactIdentity :one
-- CreateContactIdentity records how the contact appears on a channel.
-- An identity another contact of the workspace holds fails on
-- contact_identities_workspace_id_type_value_key.
INSERT INTO contact_identities (workspace_id, contact_id, type, value, verified, created_at, updated_at)
VALUES (@workspace_id, @contact_id, @type, @value, @verified, @now, @now)
RETURNING *;

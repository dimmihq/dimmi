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

-- name: ListContactIdentities :many
-- ListContactIdentities returns how the contact appears on channels, in
-- the order they were added.
SELECT * FROM contact_identities
WHERE workspace_id = @workspace_id AND contact_id = @contact_id
ORDER BY created_at, id;

-- name: DeleteContactIdentity :execrows
-- DeleteContactIdentity detaches an identity from the contact and reports
-- whether it was theirs.
DELETE FROM contact_identities
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND id = @id;

-- name: DeleteContactIdentityByValue :execrows
-- DeleteContactIdentityByValue detaches the identity of the type and
-- value from the contact, when it holds one.
DELETE FROM contact_identities
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND type = @type AND value = @value;

-- name: LockContactIdentity :exec
-- LockContactIdentity holds the identity until the transaction ends, so
-- two messages from one new sender, taken in at the same time, create
-- one contact between them instead of both trying to.
SELECT pg_advisory_xact_lock(hashtext('contact_identity:' || @workspace_id::text || ':' || @type::text || ':' || @value::text));

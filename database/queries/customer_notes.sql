-- name: ListCustomerNotes :many
-- ListCustomerNotes returns the notes about a customer or a company,
-- whichever is named, the pinned one first, then newest first, each with
-- who wrote it.
SELECT n.*, coalesce(u.name, '')::text AS author_name
FROM customer_notes n
LEFT JOIN workspace_members m ON m.id = n.author_member_id
LEFT JOIN users u ON u.id = m.user_id
WHERE n.workspace_id = @workspace_id
  AND n.contact_id IS NOT DISTINCT FROM sqlc.narg(contact_id)::uuid
  AND n.company_id IS NOT DISTINCT FROM sqlc.narg(company_id)::uuid
ORDER BY (n.pinned_at IS NULL), n.created_at DESC, n.id DESC;

-- name: FindCustomerNote :one
-- FindCustomerNote returns one note with who wrote it.
SELECT n.*, coalesce(u.name, '')::text AS author_name
FROM customer_notes n
LEFT JOIN workspace_members m ON m.id = n.author_member_id
LEFT JOIN users u ON u.id = m.user_id
WHERE n.workspace_id = @workspace_id AND n.id = @id;

-- name: CreateCustomerNote :one
-- CreateCustomerNote adds a note about a customer or a company.
INSERT INTO customer_notes (workspace_id, contact_id, company_id, author_member_id, body, created_at, updated_at)
VALUES (@workspace_id, sqlc.narg(contact_id), sqlc.narg(company_id), sqlc.narg(author_member_id), @body,
    @now::timestamptz, @now::timestamptz)
RETURNING id;

-- name: UpdateCustomerNoteBody :exec
-- UpdateCustomerNoteBody rewrites a note.
UPDATE customer_notes SET body = @body, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UnpinCustomerNotes :exec
-- UnpinCustomerNotes takes the pin off every note of a customer or a
-- company, whichever is named.
UPDATE customer_notes SET pinned_at = NULL
WHERE workspace_id = @workspace_id AND pinned_at IS NOT NULL
  AND contact_id IS NOT DISTINCT FROM sqlc.narg(contact_id)::uuid
  AND company_id IS NOT DISTINCT FROM sqlc.narg(company_id)::uuid;

-- name: PinCustomerNote :exec
-- PinCustomerNote pins one note.
UPDATE customer_notes SET pinned_at = @now::timestamptz WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteCustomerNote :exec
-- DeleteCustomerNote removes a note.
DELETE FROM customer_notes WHERE workspace_id = @workspace_id AND id = @id;

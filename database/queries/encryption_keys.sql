-- name: FindEncryptionKeyset :one
-- FindEncryptionKeyset returns the wrapped keyset of the scope for the
-- purpose.
SELECT wrapped FROM encryption_keys
WHERE scope_kind = @scope_kind AND scope_id = @scope_id AND purpose = @purpose;

-- name: CreateEncryptionKeyset :execrows
-- CreateEncryptionKeyset stores a new keyset unless another server
-- stored one first, and reports whether it did.
INSERT INTO encryption_keys (scope_kind, scope_id, purpose, wrapped, created_at, updated_at)
VALUES (@scope_kind, @scope_id, @purpose, @wrapped, now(), now())
ON CONFLICT DO NOTHING;

-- name: ReplaceEncryptionKeyset :execrows
-- ReplaceEncryptionKeyset swaps the keyset only while it still holds the
-- previous one, and reports whether it did.
UPDATE encryption_keys
SET wrapped = @next, updated_at = now()
WHERE scope_kind = @scope_kind AND scope_id = @scope_id AND purpose = @purpose AND wrapped = @previous;

-- name: ListEncryptionKeysets :many
-- ListEncryptionKeysets returns which keysets are stored, in a stable
-- order.
SELECT scope_kind, scope_id, purpose FROM encryption_keys ORDER BY scope_kind, scope_id, purpose;

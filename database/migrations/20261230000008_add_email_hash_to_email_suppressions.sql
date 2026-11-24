-- migrate: phase=expand
-- Erasing a customer must not undo their wish to hear nothing more. A
-- suppression made because they unsubscribed or complained outlives the
-- erasure as a hash of the address, salted with the workspace, in place
-- of the address itself: nobody can read whose it was, and a send to the
-- same address is still stopped. Every other suppression of an erased
-- customer is deleted. This adds the hash; a later migration lets the
-- address be empty.

-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION dimmi_email_suppression_hash(target_workspace_id uuid, address text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    SET search_path = pg_catalog
    AS $$
        SELECT encode(sha256(convert_to(target_workspace_id::text || ':' || lower(btrim(address)), 'UTF8')), 'hex')
    $$;
-- +goose StatementEnd

ALTER TABLE email_suppressions ADD COLUMN email_hash text;

CREATE UNIQUE INDEX CONCURRENTLY email_suppressions_workspace_id_email_hash_scope_key
    ON email_suppressions (workspace_id, email_hash, scope) WHERE email_hash IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS email_suppressions_workspace_id_email_hash_scope_key;
ALTER TABLE email_suppressions DROP COLUMN email_hash;
DROP FUNCTION dimmi_email_suppression_hash(uuid, text);

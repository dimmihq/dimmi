-- migrate: phase=contract
-- A person signing in through their company's identity provider gets an
-- identity like a social login's, under the provider name sso, tied to
-- the connection. Two companies' providers may hand out the same subject,
-- so the subject is unique per connection, and a person links each
-- connection once. Social logins have no connection; the null counts as
-- a value, so they keep their old uniqueness. The old keys go only once
-- the wider ones exist, so a write is never left unchecked. Deleting a
-- connection removes its identities: those people sign in another way.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE oauth_identities ADD COLUMN IF NOT EXISTS sso_connection_id uuid;
ALTER TABLE oauth_identities ADD CONSTRAINT oauth_identities_sso_connection_id_fkey
    FOREIGN KEY (sso_connection_id) REFERENCES sso_connections ON DELETE CASCADE NOT VALID;
ALTER TABLE oauth_identities VALIDATE CONSTRAINT oauth_identities_sso_connection_id_fkey;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS oauth_identities_provider_connection_subject_key
    ON oauth_identities (provider, sso_connection_id, provider_user_id) NULLS NOT DISTINCT;
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS oauth_identities_user_provider_connection_key
    ON oauth_identities (user_id, provider, sso_connection_id) NULLS NOT DISTINCT;

ALTER TABLE oauth_identities DROP CONSTRAINT IF EXISTS oauth_identities_provider_provider_user_id_key;
ALTER TABLE oauth_identities DROP CONSTRAINT IF EXISTS oauth_identities_user_id_provider_key;

-- +goose Down
DELETE FROM oauth_identities WHERE sso_connection_id IS NOT NULL;
ALTER TABLE oauth_identities ADD CONSTRAINT oauth_identities_provider_provider_user_id_key
    UNIQUE (provider, provider_user_id);
ALTER TABLE oauth_identities ADD CONSTRAINT oauth_identities_user_id_provider_key UNIQUE (user_id, provider);
DROP INDEX IF EXISTS oauth_identities_user_provider_connection_key;
DROP INDEX IF EXISTS oauth_identities_provider_connection_subject_key;
ALTER TABLE oauth_identities DROP COLUMN sso_connection_id;

-- migrate: phase=contract
-- Identity secrets may now hold an auth provider's key set address
-- instead of a key, and sign with ES256, which providers use. Exactly one
-- of a sealed secret, a public key or a key set address is stored, and
-- the address is https. The checks only widen what the previous release
-- writes: shared secrets and public keys still pass. Each new check is
-- added unvalidated and validated after, so writers are never blocked.
-- Going down restores the narrow checks unvalidated, so key set rows are
-- kept rather than refused.

-- +goose Up
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_algorithm_check;
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_check;
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_check1;

ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_algorithm_check
    CHECK (algorithm IN ('HS256', 'RS256', 'EdDSA', 'ES256')) NOT VALID;
ALTER TABLE identity_secrets VALIDATE CONSTRAINT identity_secrets_algorithm_check;
ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_shared_check
    CHECK ((algorithm = 'HS256') = (secret_encrypted IS NOT NULL)) NOT VALID;
ALTER TABLE identity_secrets VALIDATE CONSTRAINT identity_secrets_shared_check;
ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_one_key_check
    CHECK (num_nonnulls(secret_encrypted, public_key_jwk, jwks_url) = 1) NOT VALID;
ALTER TABLE identity_secrets VALIDATE CONSTRAINT identity_secrets_one_key_check;
ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_jwks_url_check
    CHECK (jwks_url IS NULL OR jwks_url LIKE 'https://%') NOT VALID;
ALTER TABLE identity_secrets VALIDATE CONSTRAINT identity_secrets_jwks_url_check;

-- +goose Down
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_jwks_url_check;
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_one_key_check;
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_shared_check;
ALTER TABLE identity_secrets DROP CONSTRAINT identity_secrets_algorithm_check;
ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_algorithm_check
    CHECK (algorithm IN ('HS256', 'RS256', 'EdDSA')) NOT VALID;
ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_check
    CHECK ((algorithm = 'HS256') = (secret_encrypted IS NOT NULL)) NOT VALID;
ALTER TABLE identity_secrets ADD CONSTRAINT identity_secrets_check1
    CHECK ((algorithm <> 'HS256') = (public_key_jwk IS NOT NULL)) NOT VALID;

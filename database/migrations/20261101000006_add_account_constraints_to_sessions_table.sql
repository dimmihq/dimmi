-- migrate: phase=expand
-- lint:user-scoped
-- The product stores only signed-in sessions: every one belongs to a
-- live account and records how its user got in, because step-up
-- checks, organization policies and the device list read it later and
-- none of it could be recovered afterwards. Both are added as validated
-- checks because the table already exists. The device list finds a
-- user's sessions on one device by the index.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users ON DELETE CASCADE,
    ADD CONSTRAINT sessions_user_id_not_null CHECK (user_id IS NOT NULL) NOT VALID,
    ADD CONSTRAINT sessions_auth_method_not_null CHECK (auth_method IS NOT NULL) NOT VALID;

ALTER TABLE sessions VALIDATE CONSTRAINT sessions_user_id_not_null;
ALTER TABLE sessions VALIDATE CONSTRAINT sessions_auth_method_not_null;

CREATE INDEX CONCURRENTLY IF NOT EXISTS sessions_user_id_device_id_index ON sessions (user_id, device_id);

-- +goose Down
DROP INDEX IF EXISTS sessions_user_id_device_id_index;
ALTER TABLE sessions
    DROP CONSTRAINT sessions_auth_method_not_null,
    DROP CONSTRAINT sessions_user_id_not_null,
    DROP CONSTRAINT sessions_user_id_fkey;

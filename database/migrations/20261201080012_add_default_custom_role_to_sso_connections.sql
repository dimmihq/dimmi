-- migrate: phase=expand
-- People a connection adds on their first sign-in may start with a custom
-- role, not only a built-in one. Custom roles belong to one workspace, so
-- the column is checked when it is set; a role that goes leaves the
-- connection on its built-in default.

-- +goose Up
ALTER TABLE sso_connections ADD COLUMN default_custom_role_id uuid;
ALTER TABLE sso_connections ADD CONSTRAINT sso_connections_default_custom_role_id_fkey
    FOREIGN KEY (default_custom_role_id) REFERENCES roles ON DELETE SET NULL NOT VALID;
ALTER TABLE sso_connections VALIDATE CONSTRAINT sso_connections_default_custom_role_id_fkey;

-- +goose Down
ALTER TABLE sso_connections DROP COLUMN default_custom_role_id;

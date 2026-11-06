-- migrate: phase=expand
-- lint:user-scoped
-- A sign-in vouched for by an outside service remembers which connection
-- it came through and which domain that service stands for, so rules read
-- later, on every request, can judge it without another query. Both stay
-- empty for every other way of signing in.

-- +goose Up
ALTER TABLE sessions ADD COLUMN sso_connection_id uuid;
ALTER TABLE sessions ADD COLUMN auth_domain text;

-- +goose Down
ALTER TABLE sessions DROP COLUMN auth_domain;
ALTER TABLE sessions DROP COLUMN sso_connection_id;

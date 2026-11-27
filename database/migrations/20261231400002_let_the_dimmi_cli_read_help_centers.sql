-- migrate: phase=expand
-- The dimmi CLI also builds help center themes: `dimmi theme` lists the
-- workspace's help centers to pick one, which takes articles:read, and
-- uploads, previews and publishes themes with settings:write, which it
-- already asks for.

-- +goose Up
UPDATE oauth_clients SET scopes = array_append(scopes, 'articles:read')
WHERE id = 'dimmi-cli' AND NOT 'articles:read' = ANY (scopes);

-- +goose Down
UPDATE oauth_clients SET scopes = array_remove(scopes, 'articles:read') WHERE id = 'dimmi-cli';

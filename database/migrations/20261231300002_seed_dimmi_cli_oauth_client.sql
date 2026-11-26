-- migrate: phase=expand
-- The dimmi CLI, which brings history in from other tools with dimmi
-- migrate, signs in as a registered app of dimmi's own: a public client
-- without a secret, proving each sign-in with PKCE, sent back to the
-- person's own computer on any loopback port. It asks only for
-- settings:write, which reaches the import permission, and offline
-- access, so a long sync keeps going. It is registered, not self-made,
-- so the consent screen shows it plainly, and each grant still belongs
-- to one workspace.

-- +goose Up
INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris, scopes, grant_types, first_party, dynamic, created_at)
VALUES (
    'dimmi-cli', NULL, 'dimmi CLI',
    ARRAY['http://127.0.0.1/callback', 'http://localhost/callback', 'http://[::1]/callback'],
    ARRAY['settings:write', 'offline_access'], ARRAY['authorization_code', 'refresh_token'], false, false, now()
)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM oauth_clients WHERE id = 'dimmi-cli';

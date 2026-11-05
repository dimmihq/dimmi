-- migrate: phase=expand
-- Accounts are found by one form of their address: compatibility
-- normalized and lowercase, so a fullwidth or decomposed spelling of an
-- address already in use names that account. Rows typed another way are
-- moved to that form, unless another account already holds it, which an
-- operator then resolves by hand. Unicode domains turn into their ASCII
-- form as people next sign in or change their address.

-- +goose Up
UPDATE users AS u SET email = lower(normalize(u.email::text, NFKC))
WHERE u.email::text <> lower(normalize(u.email::text, NFKC))
  AND NOT EXISTS (SELECT 1 FROM users AS o WHERE o.id <> u.id AND o.email = lower(normalize(u.email::text, NFKC)));

-- +goose Down
SELECT 1;

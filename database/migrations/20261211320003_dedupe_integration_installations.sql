-- migrate: phase=expand

-- +goose Up
WITH ranked AS (
    SELECT id, row_number() OVER (
        PARTITION BY workspace_id, provider, external_app_id, external_account_id
        ORDER BY (status = 'active') DESC, updated_at DESC, id DESC
    ) AS place
    FROM integration_installations
    WHERE deleted_at IS NULL AND status <> 'disconnected' AND external_account_id IS NOT NULL
)
UPDATE integration_installations i
SET status = 'disconnected', disconnected_at = now(), updated_at = now()
FROM ranked r
WHERE r.id = i.id AND r.place > 1;

CREATE UNIQUE INDEX integration_installations_workspace_account_key
    ON integration_installations (workspace_id, provider, external_app_id, external_account_id) NULLS NOT DISTINCT
    WHERE deleted_at IS NULL AND status <> 'disconnected' AND external_account_id IS NOT NULL;

-- +goose Down
DROP INDEX integration_installations_workspace_account_key;

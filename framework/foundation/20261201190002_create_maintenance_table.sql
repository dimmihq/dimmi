-- migrate: phase=expand
-- lint:global
-- Maintenance mode is one row shared by every replica, so a replica that
-- starts, restarts or replaces another finds the application down or up
-- as the operator left it. Containers keep nothing on local disk.

-- +goose Up
CREATE TABLE maintenance (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    enabled boolean NOT NULL,
    message text NOT NULL DEFAULT '',
    since timestamptz
);

-- +goose Down
DROP TABLE maintenance;

-- migrate: phase=expand
-- People tell tags apart by color and icon at a glance, so every tag has a
-- color and may carry an icon, named from the app's set. A new tag without
-- a chosen color takes the palette color its workspace uses least, its
-- name breaking ties, so tags made together still differ. Tags made before
-- colors were given out get one now.

-- +goose Up
ALTER TABLE tags ADD COLUMN IF NOT EXISTS icon text CHECK (icon ~ '^[a-z][a-z0-9-]{0,39}$');

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION dimmi_next_tag_color(workspace uuid, tag_name text) RETURNS text
LANGUAGE sql STABLE AS $$
    SELECT palette.color
    FROM unnest(ARRAY[
        '#2563eb', '#16a34a', '#dc2626', '#9333ea', '#ea580c', '#0891b2',
        '#db2777', '#ca8a04', '#0d9488'
    ]) WITH ORDINALITY AS palette(color, position)
    LEFT JOIN tags AS t ON t.workspace_id = workspace AND lower(t.color) = palette.color
    GROUP BY palette.color, palette.position
    ORDER BY count(t.id), (palette.position + abs(hashtext(tag_name))) % 9
    LIMIT 1
$$;
-- +goose StatementEnd

UPDATE tags AS t SET color = ordered.color
FROM (
    SELECT id, (ARRAY[
        '#2563eb', '#16a34a', '#dc2626', '#9333ea', '#ea580c', '#0891b2',
        '#db2777', '#ca8a04', '#0d9488'
    ])[1 + (row_number() OVER (PARTITION BY workspace_id ORDER BY created_at, id) - 1) % 9] AS color
    FROM tags WHERE color IS NULL
) AS ordered
WHERE t.id = ordered.id;

-- +goose Down
DROP FUNCTION IF EXISTS dimmi_next_tag_color(uuid, text);
ALTER TABLE tags DROP COLUMN IF EXISTS icon;

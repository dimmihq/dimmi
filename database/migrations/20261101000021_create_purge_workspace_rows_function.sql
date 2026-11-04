-- migrate: phase=expand
-- Purging a workspace removes every row it owns a batch at a time, so no
-- single statement holds locks for long and a purge that stops halfway
-- carries on where it left off. Every table with a workspace_id column
-- holds rows of one workspace, whichever part of the product added it,
-- so the function finds those tables itself rather than keeping a list
-- that new tables would have to join. Tables nothing else points at go
-- first, which keeps each batch from cascading into large deletes. A
-- table whose rows other rows still need is skipped until those are
-- gone, and whatever is left when no table moves goes with the
-- workspace row, whose foreign keys cascade. Each delete names the
-- workspace again, so no batch ever reaches another workspace's rows.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION dimmi_purge_workspace_rows(target_workspace_id uuid, batch_size integer)
    RETURNS bigint
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
    AS $$
DECLARE
    tenant_table regclass;
    removed bigint;
BEGIN
    FOR tenant_table IN
        SELECT c.oid::regclass
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'workspace_id' AND NOT a.attisdropped
        WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
        ORDER BY (
            SELECT count(*) FROM pg_constraint f WHERE f.contype = 'f' AND f.confrelid = c.oid AND f.conrelid <> c.oid
        ), c.relname
    LOOP
        BEGIN
            EXECUTE format(
                'DELETE FROM %s WHERE workspace_id = $1'
                    ' AND ctid = ANY (ARRAY(SELECT ctid FROM %s WHERE workspace_id = $1 LIMIT $2))',
                tenant_table, tenant_table
            ) USING target_workspace_id, batch_size;

            GET DIAGNOSTICS removed = ROW_COUNT;
        EXCEPTION WHEN foreign_key_violation THEN
            removed := 0;
        END;

        IF removed > 0 THEN
            RETURN removed;
        END IF;
    END LOOP;

    RETURN 0;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION dimmi_purge_workspace_rows(uuid, integer);

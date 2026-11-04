-- migrate: phase=expand
-- Audit entries are written once and never edited, whichever role is
-- connected. Revoking UPDATE only reached the role that ran the first
-- migration; in the cloud the application connects as another role,
-- whose grants come from default privileges and include UPDATE. A
-- trigger holds for every role, so the rule no longer depends on how
-- roles were provisioned. Deleting stays allowed: retention prunes old
-- entries and purging a workspace removes its own.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION dimmi_refuse_audit_log_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'audit log entries cannot be changed' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_logs_append_only
    BEFORE UPDATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION dimmi_refuse_audit_log_update();

-- +goose Down
DROP TRIGGER audit_logs_append_only ON audit_logs;
DROP FUNCTION dimmi_refuse_audit_log_update();

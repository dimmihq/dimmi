-- migrate: phase=expand
-- Reports read what happened to a conversation from conversation_events,
-- an append-only log of its changes: when it started, closed, reopened,
-- snoozed and woke, who and which team held it, when the AI agent handed
-- it to the team or took it back, when it was merged away, marked spam
-- or deleted. Triggers on conversations write each row in the same
-- transaction as the change, whichever code path made it, so no change
-- goes unlogged. A row is tagged live, import (history brought in from
-- another tool, written at the time it happened there) or backfill
-- (rebuilt from the timeline written before the log existed).
--
-- report_pending lists the conversations whose report facts must be
-- worked out again; the same triggers, and those on messages, ratings
-- and AI runs, add to it. report_states holds, per workspace, how far
-- the one-off backfill and any full rebuild have come.

-- +goose Up
CREATE TABLE conversation_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN (
        'created', 'closed', 'reopened', 'snoozed', 'unsnoozed', 'assigned', 'team_assigned',
        'ai_handed_off', 'ai_resumed', 'ai_stopped', 'merged', 'spam', 'unspam', 'deleted', 'restored'
    )),
    occurred_at timestamptz NOT NULL,
    source text NOT NULL CHECK (source IN ('live', 'import', 'backfill')),
    member_id uuid,
    team_id uuid,
    reason text,
    recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX conversation_events_conversation_id_index ON conversation_events (conversation_id, occurred_at, id);
CREATE INDEX conversation_events_workspace_id_occurred_at_index ON conversation_events (workspace_id, occurred_at);

ALTER TABLE conversation_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_events_tenant ON conversation_events
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE FUNCTION dimmi_refuse_conversation_event_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'conversation events cannot be changed' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER conversation_events_append_only
    BEFORE UPDATE ON conversation_events
    FOR EACH STATEMENT EXECUTE FUNCTION dimmi_refuse_conversation_event_update();

CREATE TABLE report_pending (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    due_at timestamptz NOT NULL,
    PRIMARY KEY (conversation_id)
);

CREATE INDEX report_pending_due_at_index ON report_pending (due_at);

ALTER TABLE report_pending ENABLE ROW LEVEL SECURITY;
CREATE POLICY report_pending_tenant ON report_pending USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE report_states (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    log_started_at timestamptz NOT NULL,
    backfilled_at timestamptz,
    rebuild_requested_at timestamptz,
    rebuild_cursor uuid,
    rebuilt_at timestamptz,
    snapshot_day date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE report_states ENABLE ROW LEVEL SECURITY;
CREATE POLICY report_states_tenant ON report_states USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- Every workspace that exists now needs its history backfilled and its
-- facts built; ones made later start with an empty log and need neither.
INSERT INTO report_states (workspace_id, log_started_at, rebuild_requested_at)
SELECT id, now(), now() FROM workspaces;

-- +goose StatementBegin
CREATE FUNCTION dimmi_report_event_source() RETURNS text
    LANGUAGE sql STABLE
    AS $$ SELECT COALESCE(nullif(current_setting('dimmi.event_source', true), ''), 'live') $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION dimmi_report_mark(ws uuid, conversation uuid) RETURNS void
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        INSERT INTO report_pending (workspace_id, conversation_id, due_at)
        VALUES (ws, conversation, clock_timestamp() + interval '2 seconds')
        ON CONFLICT (conversation_id) DO NOTHING;
    $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION dimmi_capture_conversation_events() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
DECLARE
    src text := dimmi_report_event_source();
    at timestamptz := NEW.updated_at;
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source, member_id, team_id)
        VALUES (NEW.workspace_id, NEW.id, 'created', NEW.created_at, src, NEW.assignee_member_id, NEW.assignee_team_id);
        IF NEW.status = 'closed' THEN
            INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source)
            VALUES (NEW.workspace_id, NEW.id, 'closed', COALESCE(NEW.closed_at, NEW.created_at), src);
        END IF;
        PERFORM dimmi_report_mark(NEW.workspace_id, NEW.id);
        RETURN NULL;
    END IF;

    IF NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source)
        VALUES (NEW.workspace_id, NEW.id, CASE
            WHEN NEW.status = 'closed' THEN 'closed'
            WHEN NEW.status = 'snoozed' THEN 'snoozed'
            WHEN OLD.status = 'snoozed' THEN 'unsnoozed'
            ELSE 'reopened'
        END, CASE WHEN NEW.status = 'closed' THEN COALESCE(NEW.closed_at, at) ELSE at END, src);
    END IF;

    IF NEW.assignee_member_id IS DISTINCT FROM OLD.assignee_member_id THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source, member_id)
        VALUES (NEW.workspace_id, NEW.id, 'assigned', at, src, NEW.assignee_member_id);
    END IF;

    IF NEW.assignee_team_id IS DISTINCT FROM OLD.assignee_team_id THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source, team_id)
        VALUES (NEW.workspace_id, NEW.id, 'team_assigned', at, src, NEW.assignee_team_id);
    END IF;

    IF NEW.ai_state IS DISTINCT FROM OLD.ai_state THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source, reason)
        VALUES (NEW.workspace_id, NEW.id, CASE
            WHEN NEW.ai_state = 'handed_off' THEN 'ai_handed_off'
            WHEN NEW.ai_state = 'active' THEN 'ai_resumed'
            ELSE 'ai_stopped'
        END, COALESCE(CASE WHEN NEW.ai_state = 'handed_off' THEN NEW.ai_handed_off_at END, at), src,
            CASE WHEN NEW.ai_state = 'handed_off' THEN NEW.ai_handoff_reason END);
    END IF;

    IF NEW.merged_into_id IS DISTINCT FROM OLD.merged_into_id AND NEW.merged_into_id IS NOT NULL THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source)
        VALUES (NEW.workspace_id, NEW.id, 'merged', at, src);
    END IF;

    IF NEW.spam IS DISTINCT FROM OLD.spam THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source)
        VALUES (NEW.workspace_id, NEW.id, CASE WHEN NEW.spam THEN 'spam' ELSE 'unspam' END, at, src);
    END IF;

    IF NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
        INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source)
        VALUES (NEW.workspace_id, NEW.id, CASE WHEN NEW.deleted_at IS NULL THEN 'restored' ELSE 'deleted' END,
            COALESCE(NEW.deleted_at, at), src);
    END IF;

    IF NEW.status IS DISTINCT FROM OLD.status
        OR NEW.assignee_member_id IS DISTINCT FROM OLD.assignee_member_id
        OR NEW.assignee_team_id IS DISTINCT FROM OLD.assignee_team_id
        OR NEW.ai_state IS DISTINCT FROM OLD.ai_state
        OR NEW.merged_into_id IS DISTINCT FROM OLD.merged_into_id
        OR NEW.spam IS DISTINCT FROM OLD.spam
        OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at
        OR NEW.is_test IS DISTINCT FROM OLD.is_test
        OR NEW.inbox_id IS DISTINCT FROM OLD.inbox_id
        OR NEW.priority IS DISTINCT FROM OLD.priority
        OR NEW.ai_cost_micros IS DISTINCT FROM OLD.ai_cost_micros
        OR NEW.channel_connection_id IS DISTINCT FROM OLD.channel_connection_id THEN
        PERFORM dimmi_report_mark(NEW.workspace_id, NEW.id);
    END IF;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER conversations_capture_events
    AFTER INSERT OR UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION dimmi_capture_conversation_events();

-- +goose StatementBegin
CREATE FUNCTION dimmi_mark_report_message() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
BEGIN
    IF NEW.type = 'message' THEN
        PERFORM dimmi_report_mark(NEW.workspace_id, NEW.conversation_id);
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER messages_mark_reports
    AFTER INSERT OR UPDATE OF deleted_at, undone_at, sent_at ON messages
    FOR EACH ROW EXECUTE FUNCTION dimmi_mark_report_message();

-- +goose StatementBegin
CREATE FUNCTION dimmi_mark_report_rating() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
BEGIN
    IF NEW.score IS NOT NULL THEN
        PERFORM dimmi_report_mark(NEW.workspace_id, NEW.conversation_id);
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER conversation_ratings_mark_reports
    AFTER INSERT OR UPDATE OF score ON conversation_ratings
    FOR EACH ROW EXECUTE FUNCTION dimmi_mark_report_rating();

-- +goose StatementBegin
CREATE FUNCTION dimmi_mark_report_ai_run() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
BEGIN
    IF NEW.conversation_id IS NOT NULL AND NEW.kind = 'conversation' THEN
        PERFORM dimmi_report_mark(NEW.workspace_id, NEW.conversation_id);
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ai_runs_mark_reports
    AFTER INSERT OR UPDATE OF outcome, feedback ON ai_runs
    FOR EACH ROW EXECUTE FUNCTION dimmi_mark_report_ai_run();

-- +goose StatementBegin
CREATE FUNCTION dimmi_report_rebuild_on_schedule() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
BEGIN
    IF TG_TABLE_NAME = 'schedules' THEN
        IF NEW.weekly IS NOT DISTINCT FROM OLD.weekly AND NEW.holidays IS NOT DISTINCT FROM OLD.holidays
            AND NEW.timezone IS NOT DISTINCT FROM OLD.timezone AND NEW.is_default IS NOT DISTINCT FROM OLD.is_default
            AND NEW.deleted_at IS NOT DISTINCT FROM OLD.deleted_at THEN
            RETURN NULL;
        END IF;
    ELSIF NEW.schedule_id IS NOT DISTINCT FROM OLD.schedule_id THEN
        RETURN NULL;
    END IF;
    INSERT INTO report_states (workspace_id, log_started_at, backfilled_at, rebuild_requested_at)
    VALUES (NEW.workspace_id, now(), now(), now())
    ON CONFLICT (workspace_id) DO UPDATE SET rebuild_requested_at = now(), rebuild_cursor = NULL, updated_at = now();
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER schedules_rebuild_reports
    AFTER UPDATE ON schedules
    FOR EACH ROW EXECUTE FUNCTION dimmi_report_rebuild_on_schedule();

CREATE TRIGGER inboxes_rebuild_reports
    AFTER UPDATE OF schedule_id ON inboxes
    FOR EACH ROW EXECUTE FUNCTION dimmi_report_rebuild_on_schedule();

-- The drain takes due conversations across workspaces, oldest first;
-- each is worked out again inside its own workspace.
-- +goose StatementBegin
CREATE FUNCTION dimmi_report_take(batch integer, due_before timestamptz)
    RETURNS TABLE (workspace_id uuid, conversation_id uuid)
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        DELETE FROM report_pending p
        WHERE p.conversation_id IN (
            SELECT q.conversation_id FROM report_pending q
            WHERE q.due_at <= due_before
            ORDER BY q.due_at
            LIMIT batch
            FOR UPDATE SKIP LOCKED
        )
        RETURNING p.workspace_id, p.conversation_id;
    $$;
-- +goose StatementEnd

-- The workspaces with backfill or rebuild work left, a few at a time.
-- +goose StatementBegin
CREATE FUNCTION dimmi_report_rebuilds(batch integer)
    RETURNS TABLE (workspace_id uuid, backfilled boolean)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT s.workspace_id, s.backfilled_at IS NOT NULL
        FROM report_states s
        WHERE s.backfilled_at IS NULL OR s.rebuild_requested_at IS NOT NULL
        ORDER BY s.updated_at
        LIMIT batch;
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION dimmi_report_rebuilds(integer);
DROP FUNCTION dimmi_report_take(integer, timestamptz);
DROP TRIGGER inboxes_rebuild_reports ON inboxes;
DROP TRIGGER schedules_rebuild_reports ON schedules;
DROP FUNCTION dimmi_report_rebuild_on_schedule();
DROP TRIGGER ai_runs_mark_reports ON ai_runs;
DROP FUNCTION dimmi_mark_report_ai_run();
DROP TRIGGER conversation_ratings_mark_reports ON conversation_ratings;
DROP FUNCTION dimmi_mark_report_rating();
DROP TRIGGER messages_mark_reports ON messages;
DROP FUNCTION dimmi_mark_report_message();
DROP TRIGGER conversations_capture_events ON conversations;
DROP FUNCTION dimmi_capture_conversation_events();
DROP FUNCTION dimmi_report_mark(uuid, uuid);
DROP FUNCTION dimmi_report_event_source();
DROP TABLE report_states;
DROP TABLE report_pending;
DROP TRIGGER conversation_events_append_only ON conversation_events;
DROP FUNCTION dimmi_refuse_conversation_event_update();
DROP TABLE conversation_events;

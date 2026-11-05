-- migrate: phase=expand
-- The one rule for which conversations a member sees, callable from any
-- query. It is a single statement without security definer, so the
-- planner inlines it, and for a member who sees everything the check
-- folds away and lists cost what they always did.

-- +goose Up
CREATE FUNCTION dimmi_conversation_visible(
    assignee_member uuid, assignee_team uuid, mode text, viewer uuid, teams uuid[], unassigned boolean
) RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT mode = 'all'
        OR assignee_member = viewer
        OR (mode = 'teams' AND assignee_team = ANY (teams))
        OR (mode = 'all_except_teams' AND (assignee_team IS NULL OR NOT assignee_team = ANY (teams)))
        OR (unassigned AND mode IN ('assigned', 'teams') AND assignee_member IS NULL AND assignee_team IS NULL)
$$;

-- +goose Down
DROP FUNCTION dimmi_conversation_visible(uuid, uuid, text, uuid, uuid[], boolean);

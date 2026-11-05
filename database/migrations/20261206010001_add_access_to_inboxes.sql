-- migrate: phase=expand
-- Who may open an inbox. An inbox is open to everyone, as every inbox was
-- before, or restricted to the teams and teammates on its list. Members
-- of a listed team open it for as long as they are in the team. Owners
-- and admins open every inbox; that is decided in the application.
--
-- The visibility rule gains the inboxes the viewer may not open: a
-- conversation in one of them is hidden, unless it is assigned to the
-- viewer, since assigning someone always grants them sight. The rule with
-- six arguments stays until no running code calls it.

-- +goose Up
ALTER TABLE inboxes
    ADD COLUMN access text NOT NULL DEFAULT 'everyone' CHECK (access IN ('everyone', 'restricted'));

CREATE TABLE inbox_access (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    inbox_id uuid NOT NULL REFERENCES inboxes ON DELETE CASCADE,
    team_id uuid REFERENCES teams ON DELETE CASCADE,
    member_id uuid REFERENCES workspace_members ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((team_id IS NULL) <> (member_id IS NULL))
);

CREATE UNIQUE INDEX inbox_access_inbox_id_team_id_key ON inbox_access (inbox_id, team_id) WHERE team_id IS NOT NULL;
CREATE UNIQUE INDEX inbox_access_inbox_id_member_id_key ON inbox_access (inbox_id, member_id)
    WHERE member_id IS NOT NULL;
CREATE INDEX inbox_access_workspace_id_index ON inbox_access (workspace_id);

ALTER TABLE inbox_access ENABLE ROW LEVEL SECURITY;
CREATE POLICY inbox_access_tenant ON inbox_access USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE FUNCTION dimmi_conversation_visible(
    assignee_member uuid, assignee_team uuid, inbox uuid, mode text, viewer uuid, teams uuid[], unassigned boolean,
    hidden_inboxes uuid[]
) RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT assignee_member = viewer
        OR (NOT inbox = ANY (hidden_inboxes) AND (
            mode = 'all'
            OR (mode = 'teams' AND assignee_team = ANY (teams))
            OR (mode = 'all_except_teams' AND (assignee_team IS NULL OR NOT assignee_team = ANY (teams)))
            OR (unassigned AND mode IN ('assigned', 'teams') AND assignee_member IS NULL AND assignee_team IS NULL)
        ))
$$;

-- +goose Down
DROP FUNCTION dimmi_conversation_visible(uuid, uuid, uuid, text, uuid, uuid[], boolean, uuid[]);
DROP TABLE inbox_access;
ALTER TABLE inboxes DROP COLUMN access;

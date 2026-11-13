-- name: ListReportStateEvents :many
-- ListReportStateEvents returns the creation and state changes of the
-- tickets the member may see created in the range, ticket by ticket,
-- oldest first, for time-in-state reports.
SELECT e.ticket_id, e.data, e.created_at, t.type_id
FROM ticket_events e
JOIN tickets t ON t.id = e.ticket_id
WHERE e.workspace_id = @workspace_id AND e.kind IN ('created', 'state')
  AND t.created_at >= @created_from AND t.created_at < @created_to AND t.deleted_at IS NULL
  AND dimmi_conversation_visible(
      t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
ORDER BY e.ticket_id, e.created_at, e.id;

-- name: CountReportTicketsByType :many
-- CountReportTicketsByType counts, per type, the tickets the member may
-- see created in the range, resolved in the range, and open now.
SELECT t.type_id,
    count(*) FILTER (WHERE t.created_at >= @range_from AND t.created_at < @range_to) AS created,
    count(*) FILTER (WHERE t.resolved_at >= @range_from AND t.resolved_at < @range_to) AS resolved,
    count(*) FILTER (WHERE t.state_group <> 'resolved') AS open
FROM tickets t
WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL
  AND dimmi_conversation_visible(
      t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY t.type_id;

-- name: CountReportTicketsByField :many
-- CountReportTicketsByField counts the tickets of the type the member
-- may see created in the range by the value of one field, each choice
-- of a multiple choice field counted once; null is no value.
SELECT coalesce(v.value, '')::text AS value, count(*) AS tickets,
    count(*) FILTER (WHERE t.state_group = 'resolved') AS resolved
FROM tickets t
CROSS JOIN LATERAL jsonb_array_elements_text(
    CASE WHEN jsonb_typeof(t.fields -> @field_key::text) = 'array' THEN t.fields -> @field_key::text
    ELSE jsonb_build_array(t.fields -> @field_key::text) END
) AS v(value)
WHERE t.workspace_id = @workspace_id AND t.type_id = @type_id AND t.deleted_at IS NULL
  AND t.created_at >= @range_from AND t.created_at < @range_to
  AND dimmi_conversation_visible(
      t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY v.value
ORDER BY count(*) DESC, v.value;

-- name: CountReportSLAClocks :many
-- CountReportSLAClocks counts, per policy and target, the clocks on
-- tickets and conversations the member may see that ended in the range,
-- met or breached, with how long met ones took on average.
SELECT k.policy_id, p.name AS policy_name, k.target,
    count(*) FILTER (WHERE k.state = 'met') AS met,
    count(*) FILTER (WHERE k.state = 'breached') AS breached,
    coalesce(avg(extract(epoch FROM k.stopped_at - k.started_at)) FILTER (WHERE k.state = 'met'), 0)::float8
        AS met_seconds
FROM sla_clocks k
JOIN sla_policies p ON p.id = k.policy_id
LEFT JOIN tickets t ON t.id = k.ticket_id
LEFT JOIN conversations c ON c.id = k.conversation_id
WHERE k.workspace_id = @workspace_id AND k.state IN ('met', 'breached')
  AND coalesce(k.stopped_at, k.breached_at) >= @range_from::timestamptz
  AND coalesce(k.stopped_at, k.breached_at) < @range_to::timestamptz
  AND coalesce(t.deleted_at, c.deleted_at) IS NULL
  AND dimmi_conversation_visible(
      coalesce(t.assignee_member_id, c.assignee_member_id), coalesce(t.assignee_team_id, c.assignee_team_id),
      coalesce(t.inbox_id, c.inbox_id), @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY k.policy_id, p.name, k.target
ORDER BY p.name, k.target;

-- name: CountReportSLADaily :many
-- CountReportSLADaily counts the clocks the member may see that ended
-- met or breached on each day of the range, in the time zone.
SELECT to_char(coalesce(k.stopped_at, k.breached_at) AT TIME ZONE @zone::text, 'YYYY-MM-DD') AS day,
    count(*) FILTER (WHERE k.state = 'met') AS met,
    count(*) FILTER (WHERE k.state = 'breached') AS breached
FROM sla_clocks k
LEFT JOIN tickets t ON t.id = k.ticket_id
LEFT JOIN conversations c ON c.id = k.conversation_id
WHERE k.workspace_id = @workspace_id AND k.state IN ('met', 'breached')
  AND coalesce(k.stopped_at, k.breached_at) >= @range_from::timestamptz
  AND coalesce(k.stopped_at, k.breached_at) < @range_to::timestamptz
  AND coalesce(t.deleted_at, c.deleted_at) IS NULL
  AND dimmi_conversation_visible(
      coalesce(t.assignee_member_id, c.assignee_member_id), coalesce(t.assignee_team_id, c.assignee_team_id),
      coalesce(t.inbox_id, c.inbox_id), @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY 1
ORDER BY 1;

-- name: CountReportTicketsDaily :many
-- CountReportTicketsDaily counts the tickets the member may see created
-- and resolved on each day of the range, in the time zone.
SELECT d.day, sum(d.created)::bigint AS created, sum(d.resolved)::bigint AS resolved FROM (
    SELECT to_char(t.created_at AT TIME ZONE @zone::text, 'YYYY-MM-DD') AS day, 1 AS created, 0 AS resolved
    FROM tickets t
    WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL
      AND t.created_at >= @range_from AND t.created_at < @range_to
      AND dimmi_conversation_visible(
          t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
          sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
          @access_hidden_inboxes::uuid[]
      )
    UNION ALL
    SELECT to_char(t.resolved_at AT TIME ZONE @zone::text, 'YYYY-MM-DD'), 0, 1
    FROM tickets t
    WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL
      AND t.resolved_at >= @range_from AND t.resolved_at < @range_to
      AND dimmi_conversation_visible(
          t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
          sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
          @access_hidden_inboxes::uuid[]
      )
) d
GROUP BY d.day
ORDER BY d.day;

-- name: ListReportTrackers :many
-- ListReportTrackers ranks the trackers the member may see by what they
-- reach: linked conversations, those linked in the range, distinct
-- customers and companies, and the companies' monthly revenue.
SELECT t.id, t.number, t.kind, t.title, t.state_group, t.created_at,
    count(DISTINCT c.id) AS conversations,
    count(DISTINCT c.id) FILTER (WHERE l.created_at >= @range_from AND l.created_at < @range_to) AS linked_in_range,
    count(DISTINCT c.contact_id) AS customers,
    count(DISTINCT ct.company_id) AS companies,
    coalesce((SELECT sum(dimmi_company_revenue(co.custom_attributes)) FROM companies co WHERE co.id IN (
        SELECT DISTINCT ct2.company_id FROM ticket_links l2
        JOIN conversations c2 ON c2.id = l2.conversation_id
        JOIN contacts ct2 ON ct2.id = c2.contact_id
        WHERE l2.from_ticket_id = t.id AND c2.deleted_at IS NULL)), 0)::float8 AS revenue
FROM tickets t
LEFT JOIN ticket_links l ON l.from_ticket_id = t.id AND l.conversation_id IS NOT NULL
LEFT JOIN conversations c ON c.id = l.conversation_id AND c.deleted_at IS NULL
LEFT JOIN contacts ct ON ct.id = c.contact_id
WHERE t.workspace_id = @workspace_id AND t.kind = 'tracker' AND t.deleted_at IS NULL
  AND (t.state_group <> 'resolved' OR t.resolved_at >= @range_from OR t.created_at >= @range_from)
  AND t.created_at < @range_to
  AND dimmi_conversation_visible(
      t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY t.id
ORDER BY revenue DESC, conversations DESC, t.number DESC
LIMIT @row_limit;

-- name: CountReportSLARunning :many
-- CountReportSLARunning counts, per policy and target, the clocks
-- running or paused now on tickets and conversations the member may
-- see.
SELECT k.policy_id, p.name AS policy_name, k.target, count(*) AS running
FROM sla_clocks k
JOIN sla_policies p ON p.id = k.policy_id
LEFT JOIN tickets t ON t.id = k.ticket_id
LEFT JOIN conversations c ON c.id = k.conversation_id
WHERE k.workspace_id = @workspace_id AND k.state IN ('running', 'paused')
  AND coalesce(t.deleted_at, c.deleted_at) IS NULL
  AND dimmi_conversation_visible(
      coalesce(t.assignee_member_id, c.assignee_member_id), coalesce(t.assignee_team_id, c.assignee_team_id),
      coalesce(t.inbox_id, c.inbox_id), @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY k.policy_id, p.name, k.target
ORDER BY p.name, k.target;

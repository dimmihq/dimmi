//go:build dev

/*
|--------------------------------------------------------------------------
| Reports Demo Rows
|--------------------------------------------------------------------------
|
| The statements reports:demo writes a batch with. A plan picks, for each
| conversation, when it starts (mostly in working hours on weekdays),
| its channel, customer, teammate and team, whether the AI answers and
| resolves or hands off, how fast the team replies over one or two
| turns, when it closes, whether it reopens, and its rating. The rows
| then go in as the live writes would leave them, status changes last,
| so the triggers log each close and reopen at its own time.
|
*/

package commands

/*
reportsDemoPlan builds the batch's plan from the workspace ($1), the
batch size ($2), the days back ($3) and the batch's offset ($4).
*/
const reportsDemoPlan = `
CREATE TEMP TABLE demo_plan ON COMMIT DROP AS
WITH people AS (
    SELECT array_agg(id ORDER BY id) AS ids FROM workspace_members WHERE workspace_id = $1 AND status = 'active'
), squads AS (
    SELECT COALESCE(array_agg(id ORDER BY id), '{}') AS ids FROM teams WHERE workspace_id = $1
), chans AS (
    SELECT array_agg(id ORDER BY type, id) AS ids, array_agg(inbox_id ORDER BY type, id) AS inboxes,
        array_agg(type ORDER BY type, id) AS types
    FROM channel_connections WHERE workspace_id = $1
), folks AS (
    SELECT array_agg(id) AS ids FROM (SELECT id FROM contacts WHERE workspace_id = $1 LIMIT 5000) x
), base AS (
    SELECT gs AS i, uuidv7() AS id, random() AS r1, random() AS r2, random() AS r3, random() AS r4,
        random() AS r5, random() AS r6, random() AS r7,
        date_trunc('day', now() - (power(random(), 0.8) * $3::int) * interval '1 day') AS day
    FROM generate_series(1, $2::int) gs
)
SELECT b.id, b.i + $4::int AS seq,
    p.ids[1 + (b.i * 7919 % cardinality(p.ids))] AS member_id,
    CASE WHEN cardinality(s.ids) > 0 AND b.r6 < 0.7 THEN s.ids[1 + (b.i % cardinality(s.ids))] END AS team_id,
    ch.ids[1 + (b.i * 31 % cardinality(ch.ids))] AS channel_id,
    ch.inboxes[1 + (b.i * 31 % cardinality(ch.ids))] AS inbox_id,
    ch.types[1 + (b.i * 31 % cardinality(ch.ids))] AS channel_type,
    f.ids[1 + (b.i * 104729 % cardinality(f.ids))] AS contact_id,
    -- weekdays mostly, in working hours mostly
    b.day + CASE WHEN extract(isodow FROM b.day) > 5 AND b.r7 < 0.7 THEN interval '2 days' ELSE interval '0' END
        + CASE WHEN b.r1 < 0.8 THEN interval '8 hours' + b.r2 * interval '10 hours' ELSE b.r2 * interval '24 hours' END
        AS started_at,
    b.r3 < 0.4 AS ai,
    b.r3 < 0.18 AS ai_resolves,
    b.r3 >= 0.18 AND b.r3 < 0.4 AS ai_hands_off,
    -- first reply after 20 seconds to about 8 hours, skewed fast
    (20 + exp(b.r4 * 10)) * interval '1 second' AS first_wait,
    b.r5 < 0.3 AS second_turn,
    (60 + exp(b.r5 * 9)) * interval '1 second' AS second_wait,
    b.r6 AS close_roll, b.r7 AS rate_roll
FROM base b, people p, squads s, chans ch, folks f;

ALTER TABLE demo_plan ADD COLUMN number bigint, ADD COLUMN closed_at timestamptz, ADD COLUMN reopen_at timestamptz,
    ADD COLUMN final_at timestamptz, ADD COLUMN reply_at timestamptz, ADD COLUMN reply2_at timestamptz;

UPDATE demo_plan SET
    reply_at = CASE WHEN NOT ai_resolves THEN started_at + first_wait
        + CASE WHEN ai THEN interval '1 minute' ELSE interval '0' END END,
    reply2_at = CASE WHEN NOT ai_resolves AND second_turn
        THEN started_at + first_wait + interval '30 minutes' + second_wait END;

UPDATE demo_plan SET closed_at = CASE
        WHEN close_roll < 0.9 THEN COALESCE(reply2_at, reply_at, started_at + interval '2 minutes')
            + close_roll * interval '20 hours'
    END;

UPDATE demo_plan SET
    reopen_at = CASE WHEN close_roll < 0.08 THEN closed_at + interval '1 day' END,
    final_at = CASE WHEN close_roll < 0.08 THEN closed_at + interval '1 day 3 hours' END;

-- Conversations still waiting are mostly recent ones.
UPDATE demo_plan SET closed_at = started_at + interval '6 hours', reopen_at = NULL, final_at = NULL
WHERE closed_at IS NULL AND started_at < now() - interval '21 days' AND rate_roll < 0.998;

UPDATE demo_plan SET closed_at = NULL, reopen_at = NULL, final_at = NULL WHERE closed_at > now();
UPDATE demo_plan SET reopen_at = NULL, final_at = NULL WHERE final_at > now();
DELETE FROM demo_plan WHERE started_at > now() - interval '5 minutes';
`

/*
reportsDemoRows writes the plan's rows for the workspace ($1).
*/
const reportsDemoRows = `
WITH numbered AS (
    SELECT id, row_number() OVER (ORDER BY started_at) AS n FROM demo_plan
), base AS (
    UPDATE workspaces SET next_conversation_number = next_conversation_number + (SELECT count(*) FROM demo_plan)
    WHERE id = $1
    RETURNING next_conversation_number - (SELECT count(*) FROM demo_plan) AS first
)
UPDATE demo_plan p SET number = base.first + numbered.n - 1 FROM numbered, base WHERE numbered.id = p.id;

INSERT INTO conversations (id, workspace_id, number, inbox_id, channel_connection_id, contact_id, subject, status,
    assignee_member_id, assignee_team_id, ai_state, first_customer_message_at, last_customer_message_at,
    first_reply_at, last_reply_at, last_message_at, last_activity_at, message_count, ai_reply_count, ai_cost_micros,
    created_at, updated_at)
SELECT id, $1::uuid, number, inbox_id, channel_id, contact_id,
    (ARRAY['Refund for my last order', 'Cannot sign in', 'Invoice question', 'Feature request: exports',
        'Shipping delay', 'Bug in the mobile app', 'How do I add a teammate?', 'Change my plan'])[1 + seq % 8],
    'open', CASE WHEN NOT ai_resolves THEN member_id END, team_id,
    CASE WHEN ai THEN 'active' ELSE 'off' END,
    started_at, COALESCE(CASE WHEN second_turn THEN started_at + first_wait + interval '20 minutes' END, started_at),
    COALESCE(CASE WHEN ai THEN started_at + interval '30 seconds' END, reply_at),
    COALESCE(reply2_at, reply_at, CASE WHEN ai THEN started_at + interval '30 seconds' END),
    COALESCE(reply2_at, reply_at, started_at), COALESCE(final_at, closed_at, reply2_at, reply_at, started_at),
    1 + (reply_at IS NOT NULL)::int + (reply2_at IS NOT NULL)::int * 2 + (CASE WHEN ai THEN 1 ELSE 0 END),
    ai::int, CASE WHEN ai THEN 2000 + seq % 9000 ELSE 0 END,
    started_at, started_at
FROM demo_plan;

INSERT INTO messages (workspace_id, conversation_id, type, author_type, author_id, body, substantive, created_at,
    updated_at)
SELECT $1::uuid, id, 'message', 'contact', contact_id, 'Hi, I need some help with this.', true, started_at, started_at
FROM demo_plan
UNION ALL
SELECT $1::uuid, id, 'message', 'agent', NULL, 'Here is what I found in our help center.', true,
    started_at + interval '30 seconds', started_at + interval '30 seconds' FROM demo_plan WHERE ai
UNION ALL
SELECT $1::uuid, id, 'message', 'member', member_id, 'Thanks for reaching out, looking into it now.', true,
    reply_at, reply_at
FROM demo_plan WHERE reply_at IS NOT NULL
UNION ALL
SELECT $1::uuid, id, 'message', 'contact', contact_id, 'One more thing, it still happens.', true,
    started_at + first_wait + interval '20 minutes', started_at + first_wait + interval '20 minutes'
FROM demo_plan WHERE reply2_at IS NOT NULL
UNION ALL
SELECT $1::uuid, id, 'message', 'member', member_id, 'Fixed now, sorry about that.', true, reply2_at, reply2_at
FROM demo_plan WHERE reply2_at IS NOT NULL;

UPDATE conversations c SET ai_state = 'handed_off', ai_handed_off_at = p.started_at + interval '45 seconds',
    ai_handoff_reason = (ARRAY['asked_for_human', 'no_answer', 'low_confidence', 'sensitive_topic'])[1 + p.seq % 4],
    updated_at = p.started_at + interval '45 seconds'
FROM demo_plan p WHERE c.id = p.id AND p.ai_hands_off;

UPDATE conversations c SET status = 'closed', closed_at = p.closed_at, updated_at = p.closed_at
FROM demo_plan p WHERE c.id = p.id AND p.closed_at IS NOT NULL;

UPDATE conversations c SET status = 'open', closed_at = NULL, updated_at = p.reopen_at
FROM demo_plan p WHERE c.id = p.id AND p.reopen_at IS NOT NULL;

UPDATE conversations c SET status = 'closed', closed_at = p.final_at, updated_at = p.final_at
FROM demo_plan p WHERE c.id = p.id AND p.final_at IS NOT NULL;

INSERT INTO conversation_ratings (workspace_id, conversation_id, contact_id, member_id, channel, closed_at, ask_at,
    asked_at, score, rated_at)
SELECT $1::uuid, id, contact_id, CASE WHEN NOT ai_resolves THEN member_id END,
    CASE WHEN channel_type = 'email' THEN 'email' ELSE 'widget' END, COALESCE(final_at, closed_at),
    COALESCE(final_at, closed_at), COALESCE(final_at, closed_at),
    CASE WHEN rate_roll < 0.12 THEN 5 WHEN rate_roll < 0.18 THEN 4 WHEN rate_roll < 0.2 THEN 3
        WHEN rate_roll < 0.215 THEN 2 ELSE 1 END,
    COALESCE(final_at, closed_at) + interval '1 hour'
FROM demo_plan
WHERE closed_at IS NOT NULL AND rate_roll < 0.23 AND COALESCE(final_at, closed_at) < now() - interval '1 hour';
`

/*
reportsDemoSQL returns the batch's statements: the plan, which reads
all four values, then the rows, which read the workspace.
*/
func reportsDemoSQL() (string, string) {
	return reportsDemoPlan, reportsDemoRows
}

/*
backlogHistorySQL counts what was waiting at the end of each past day
of the workspace ($1), over the days back ($2), from the facts; days
already counted are kept.
*/
const backlogHistorySQL = `
INSERT INTO backlog_snapshots (workspace_id, day, inbox_id, team_id, open, snoozed, unassigned,
    under_day, under_three_days, under_week, over_week, taken_at)
SELECT $1::uuid, d.day, f.inbox_id, f.assignee_team_id, count(*), 0,
    count(*) FILTER (WHERE f.assignee_member_id IS NULL),
    count(*) FILTER (WHERE f.started_at > d.ends - interval '1 day'),
    count(*) FILTER (WHERE f.started_at <= d.ends - interval '1 day' AND f.started_at > d.ends - interval '3 days'),
    count(*) FILTER (WHERE f.started_at <= d.ends - interval '3 days' AND f.started_at > d.ends - interval '7 days'),
    count(*) FILTER (WHERE f.started_at <= d.ends - interval '7 days'),
    d.ends
FROM (
    SELECT gs::date AS day, ((gs::date + 1)::timestamp AT TIME ZONE w.tz) AS ends
    FROM (SELECT COALESCE(nullif(timezone, ''), 'UTC') AS tz FROM workspaces WHERE id = $1) w,
        generate_series(current_date - $2::int, current_date - 1, interval '1 day') gs
) d
JOIN conversation_facts f ON f.workspace_id = $1 AND f.started_at < d.ends
    AND (f.closed_at IS NULL OR f.closed_at >= d.ends)
    AND (f.first_closed_at IS NULL OR f.first_closed_at >= d.ends OR f.closed_at IS NULL)
GROUP BY d.day, d.ends, f.inbox_id, f.assignee_team_id
ON CONFLICT (workspace_id, day, inbox_id, team_id) DO NOTHING
`

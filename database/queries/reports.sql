-- name: ReportNames :many
-- ReportNames returns the names reports show groups under: every inbox,
-- team and teammate of the workspace, removed ones included, so old
-- figures still read.
SELECT 'inbox'::text AS kind, i.id, i.name::text AS name FROM inboxes i WHERE i.workspace_id = @workspace_id
UNION ALL
SELECT 'team', t.id, t.name FROM teams t WHERE t.workspace_id = @workspace_id
UNION ALL
SELECT 'member', m.id, COALESCE(nullif(u.name, ''), u.email)::text
FROM workspace_members m JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id;

-- name: ReportBacklogDays :many
-- ReportBacklogDays returns the backlog counted at the end of each day of
-- a range, summed over the inboxes and teams asked for, all when none.
SELECT to_char(b.day, 'YYYY-MM-DD')::text AS day, sum(b.open)::bigint AS open, sum(b.snoozed)::bigint AS snoozed,
    sum(b.unassigned)::bigint AS unassigned, sum(b.over_week)::bigint AS over_week
FROM backlog_snapshots b
WHERE b.workspace_id = @workspace_id AND b.day >= CAST(@from_day::text AS date) AND b.day <= CAST(@to_day::text AS date)
  AND (cardinality(@inboxes::uuid[]) = 0 OR b.inbox_id = ANY (@inboxes::uuid[]))
  AND (cardinality(@teams::uuid[]) = 0 OR b.team_id = ANY (@teams::uuid[]))
GROUP BY b.day
ORDER BY b.day;

-- name: ReportHelpCenterDaily :many
-- ReportHelpCenterDaily returns article views per day across the
-- workspace's help centers.
SELECT to_char(day, 'YYYY-MM-DD')::text AS day, sum(views)::bigint AS views,
    sum(views) FILTER (WHERE source = 'messenger')::bigint AS messenger
FROM article_views
WHERE workspace_id = @workspace_id AND day >= CAST(@from_day::text AS date) AND day <= CAST(@to_day::text AS date)
GROUP BY day
ORDER BY day;

-- name: ReportHelpCenterTopArticles :many
-- ReportHelpCenterTopArticles returns the most read articles of a range
-- with how readers rated them.
SELECT a.id, a.title, sum(v.views)::bigint AS views,
    (SELECT count(*) FROM article_ratings r WHERE r.article_id = a.id AND r.helpful
        AND r.updated_at >= @since::timestamptz)::bigint AS helpful,
    (SELECT count(*) FROM article_ratings r WHERE r.article_id = a.id AND NOT r.helpful
        AND r.updated_at >= @since::timestamptz)::bigint AS unhelpful
FROM article_views v
JOIN articles a ON a.id = v.article_id
WHERE v.workspace_id = @workspace_id AND v.day >= CAST(@from_day::text AS date) AND v.day <= CAST(@to_day::text AS date)
GROUP BY a.id, a.title
ORDER BY views DESC, a.title
LIMIT @row_limit;

-- name: ReportHelpCenterSearches :one
-- ReportHelpCenterSearches sums the searches of a range and those that
-- found nothing.
SELECT COALESCE(sum(searches), 0)::bigint AS searches, COALESCE(sum(misses), 0)::bigint AS misses
FROM help_search_terms
WHERE workspace_id = @workspace_id AND day >= CAST(@from_day::text AS date) AND day <= CAST(@to_day::text AS date);

-- name: ReportHelpCenterGaps :many
-- ReportHelpCenterGaps returns the searches of a range that most often
-- found nothing.
SELECT term, sum(misses)::bigint AS misses, sum(searches)::bigint AS searches
FROM help_search_terms
WHERE workspace_id = @workspace_id AND day >= CAST(@from_day::text AS date) AND day <= CAST(@to_day::text AS date)
  AND misses > 0
GROUP BY term
ORDER BY misses DESC, term
LIMIT @row_limit;

-- name: ReportUnansweredQuestions :many
-- ReportUnansweredQuestions returns what customers asked the AI agent in
-- a range that it could not answer, most asked first.
SELECT lower(left(COALESCE(nullif(query_rewritten, ''), query_original), 120))::text AS question,
    count(*)::bigint AS asked
FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'conversation' AND outcome IN ('unknown', 'handoff')
  AND created_at >= @since::timestamptz AND created_at < @until::timestamptz
  AND COALESCE(nullif(query_rewritten, ''), query_original) IS NOT NULL
GROUP BY 1
ORDER BY asked DESC, 1
LIMIT @row_limit;

-- name: ReportAiSpend :one
-- ReportAiSpend sums what the AI agent cost in a range, every kind of
-- run included.
SELECT COALESCE(sum(cost_micros), 0)::bigint AS cost_micros, count(*)::bigint AS runs
FROM ai_runs
WHERE workspace_id = @workspace_id AND created_at >= @since::timestamptz AND created_at < @until::timestamptz;

-- name: FindMemberReportDigest :one
-- FindMemberReportDigest returns the teammate's report digest.
SELECT * FROM report_digests WHERE workspace_id = @workspace_id AND member_id = @member_id;

-- name: FindReportDigest :one
-- FindReportDigest returns a report digest by id.
SELECT * FROM report_digests WHERE workspace_id = @workspace_id AND id = @id;

-- name: SaveReportDigest :one
-- SaveReportDigest subscribes the teammate to a digest, or changes how
-- often it comes and what it covers.
INSERT INTO report_digests (id, workspace_id, member_id, frequency, filters, next_send_at, created_at, updated_at)
VALUES (@id, @workspace_id, @member_id, @frequency, @filters, @next_send_at, @now, @now)
ON CONFLICT (workspace_id, member_id) DO UPDATE SET frequency = EXCLUDED.frequency, filters = EXCLUDED.filters,
    next_send_at = EXCLUDED.next_send_at, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteMemberReportDigest :execrows
-- DeleteMemberReportDigest stops the teammate's digest.
DELETE FROM report_digests WHERE workspace_id = @workspace_id AND member_id = @member_id;

-- name: MarkReportDigestSent :exec
-- MarkReportDigestSent records a send and when the next one goes.
UPDATE report_digests SET last_sent_at = @now::timestamptz, next_send_at = @next_send_at, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ReportDigestsDue :many
-- ReportDigestsDue returns the digests due to go out, across workspaces.
SELECT workspace_id::uuid AS workspace_id, id::uuid AS id FROM dimmi_report_digests_due(@at::timestamptz, @batch::integer);

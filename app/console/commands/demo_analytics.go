//go:build dev

/*
|--------------------------------------------------------------------------
| Demo Analytics And Backlog History
|--------------------------------------------------------------------------
|
| Fills a seeded workspace with 365 days of historical backlog snapshots
| (derived from conversation_facts) and 90 days of help center & changelog
| analytics (article views, helpfulness ratings, search terms with misses
| and changelog views).
|
*/

package commands

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

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

/*
helpCenterDemoSQL writes 90 days of article views, helpfulness ratings,
search terms (with realistic miss rates) and changelog entry views.
$1 is the workspace id and $2 is how many days back they spread.
*/
const helpCenterDemoSQL = `
DELETE FROM article_views WHERE workspace_id = $1;

DELETE FROM help_search_terms WHERE workspace_id = $1;

DELETE FROM article_ratings WHERE workspace_id = $1 AND rater_hash LIKE 'demo-%';

INSERT INTO article_views (workspace_id, help_center_id, article_id, day, source, views)
SELECT a.workspace_id, a.help_center_id, a.id, d.day::date, s.source,
    greatest(1, round(
        (420.0 / power(a.rank, 0.85))
        * (CASE WHEN extract(isodow FROM d.day) >= 6 THEN 0.45 ELSE 1 END)
        * (0.6 + 0.4 * (1 - d.back::float8 / $2))
        * s.share
        * (0.75 + 0.5 * random())
    ))::int
FROM (
    SELECT id, workspace_id, help_center_id,
        row_number() OVER (PARTITION BY help_center_id ORDER BY md5(id::text)) AS rank
    FROM articles WHERE workspace_id = $1 AND status = 'published' AND deleted_at IS NULL
) a
CROSS JOIN (SELECT n AS back, (current_date - n) AS day FROM generate_series(0, $2 - 1) AS n) d
CROSS JOIN (VALUES ('web', 0.72), ('messenger', 0.28)) AS s(source, share)
WHERE a.rank <= 400;

INSERT INTO article_ratings (workspace_id, article_id, rater_hash, helpful, created_at, updated_at)
SELECT v.workspace_id, v.article_id, 'demo-' || g.n,
    random() < (0.62 + 0.33 * (abs(hashtext(v.article_id::text)) % 100) / 100.0), t.at, t.at
FROM (SELECT workspace_id, article_id, sum(views) AS views FROM article_views WHERE workspace_id = $1
      GROUP BY workspace_id, article_id) v
CROSS JOIN LATERAL generate_series(1, least(600, greatest(1, (v.views / 45)::int))) AS g(n)
CROSS JOIN LATERAL (SELECT now() - (random() * $2 || ' days')::interval AS at WHERE g.n > 0) t;

INSERT INTO help_search_terms (workspace_id, help_center_id, day, term, searches, misses)
SELECT h.workspace_id, h.id, (current_date - d.back)::date, w.term,
    greatest(1, round(w.weight * (CASE WHEN extract(isodow FROM current_date - d.back) >= 6 THEN 0.5 ELSE 1 END)
        * (0.6 + 0.8 * random())))::int,
    0
FROM help_centers h
CROSS JOIN generate_series(0, $2 - 1) AS d(back)
CROSS JOIN (VALUES
    ('print label', 38), ('tracking number', 31), ('refund', 27), ('change address', 22), ('customs form', 19),
    ('api key', 17), ('connect shopify', 15), ('pickup', 14), ('invoice', 12), ('thermal printer', 11),
    ('return label', 10), ('insurance claim', 9), ('webhook', 8), ('batch labels', 7), ('cancel shipment', 7),
    ('dhl express', 5), ('pallet shipping', 4), ('dangerous goods', 4), ('saturday delivery', 3),
    ('amazon fba', 3), ('ioss number', 3), ('packing slip template', 2), ('carbon offset', 2), ('zebra zd421', 2)
) AS w(term, weight)
WHERE h.workspace_id = $1;

UPDATE help_search_terms SET misses = greatest(1, round(searches * (0.7 + 0.3 * random())))::int
WHERE workspace_id = $1 AND term IN ('dhl express', 'pallet shipping', 'dangerous goods', 'saturday delivery',
    'amazon fba', 'ioss number', 'packing slip template', 'carbon offset', 'zebra zd421');

DELETE FROM changelog_entry_views WHERE workspace_id = $1;

INSERT INTO changelog_entry_views (workspace_id, help_center_id, entry_id, day, views)
SELECT e.workspace_id, e.help_center_id, e.id, d.day::date,
    greatest(1, round(
        (260.0 / power(e.rank, 0.7))
        * exp(-greatest(0, d.day::date - e.published_at::date) / 9.0)
        * (CASE WHEN extract(isodow FROM d.day) >= 6 THEN 0.5 ELSE 1 END)
        * (0.75 + 0.5 * random())
    ) + (random() * 6)::int)::int
FROM (
    SELECT id, workspace_id, help_center_id, published_at,
        row_number() OVER (PARTITION BY help_center_id ORDER BY published_at DESC) AS rank
    FROM changelog_entries
    WHERE workspace_id = $1 AND status = 'published' AND deleted_at IS NULL AND published_at IS NOT NULL
) e
CROSS JOIN (SELECT (current_date - n) AS day FROM generate_series(0, $2 - 1) AS n) d
WHERE d.day::date >= e.published_at::date;
`

var sqlParamPattern = regexp.MustCompile(`\$(\d+)`)

/*
seedAnalytics writes backlog snapshots and help center analytics inside one
transaction.
*/
func seedAnalytics(ctx context.Context, tx *database.Tx, ws ids.UUID) error {
	if _, err := tx.Exec(ctx, backlogHistorySQL, ws, 365); err != nil {
		return fmt.Errorf("seed backlog history: %w", err)
	}
	return runStatements(ctx, tx, helpCenterDemoSQL, []any{ws, 90})
}

/*
runStatements runs each statement of the script in turn, each with as
many of the values as it names.
*/
func runStatements(ctx context.Context, tx *database.Tx, script string, args []any) error {
	for stmt := range strings.SplitSeq(script, ";\n") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		n := 0
		for _, m := range sqlParamPattern.FindAllStringSubmatch(stmt, -1) {
			if k, _ := strconv.Atoi(m[1]); k > n {
				n = k
			}
		}

		if _, err := tx.Exec(ctx, stmt, args[:n]...); err != nil {
			return fmt.Errorf("seed step: %w\n%s", err, stmt)
		}
	}

	return nil
}

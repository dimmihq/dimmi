//go:build dev

/*
|--------------------------------------------------------------------------
| Help Center Demo
|--------------------------------------------------------------------------
|
| Fills a development workspace's help centers with ninety days of
| analytics, so the analytics page shows what a busy help center looks
| like: views of every published article on the web and in the
| messenger, more on weekdays and growing over the weeks, a few articles
| read far more than the rest; readers' yes and no answers; and searches,
| some of which found nothing. The changelog gets views of every
| published update too, most in the days after it went out. It needs
| published articles to exist, as the demo workspace leaves them, and can
| be run again: each run replaces what the last one wrote.
|
*/

package commands

import (
	"context"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
)

/*
helpCenterDemoSQL writes the views, answers and searches. $1 is the
workspace and $2 how many days back they spread.
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

/*
HelpCenterDemoCommand seeds a workspace's help center analytics.
*/
type HelpCenterDemoCommand struct {
	App *app.App
}

/*
Name returns help-center:demo.
*/
func (c HelpCenterDemoCommand) Name() string {
	return "help-center:demo"
}

/*
Description says what the command does.
*/
func (c HelpCenterDemoCommand) Description() string {
	return "Seed a development workspace's help centers with article views, answers and searches for the analytics page"
}

/*
Flags names the workspace and how far back to seed.
*/
func (c HelpCenterDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("member", "", "the owner's member public id, mem_...")
	flags.Int("days", 90, "how many days back the analytics spread")
}

/*
Handle replaces the workspace's help center analytics with seeded ones.
*/
func (c HelpCenterDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	owner, err := SeriesDemoCommand(c).membership(ctx, in.String("workspace"), in.String("member"))
	if err != nil {
		return err
	}

	ws, days := owner.Workspace.ID, in.Int("days")

	err = c.App.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		return runStatements(ctx, tx, helpCenterDemoSQL, []any{ws, days})
	})
	if err != nil {
		return err
	}

	out.Line("seeded %d days of help center analytics", days)

	return nil
}

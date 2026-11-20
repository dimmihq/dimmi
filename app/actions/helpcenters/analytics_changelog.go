/*
|--------------------------------------------------------------------------
| Changelog Analytics
|--------------------------------------------------------------------------
|
| What readers read in a changelog, counted per day like the help
| center's own: how many times an update's page was opened, and nothing
| about who opened it. The team reads the last days back as views over
| time, the most read updates with the reactions readers left, and how
| the list of subscribers moved.
|
*/

package helpcenters

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EntryViewed counts one view of the help center's changelog update.
*/
func (a *Analytics) EntryViewed(ctx context.Context, h models.HelpCenter, entry ids.UUID) {
	if a == nil {
		return
	}

	_ = a.write(ctx, h, func(q *models.Queries) error {
		return q.CountChangelogEntryView(ctx, models.CountChangelogEntryViewParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, EntryID: entry, CountedAt: dayOf(a.Clock.Now()),
		})
	})
}

/*
ChangelogReport is what readers did with a changelog over the last
days: views by day, the most read updates, the reactions left, and the
subscribers it has now with those who joined and left.
*/
type ChangelogReport struct {
	Days        int
	Since       time.Time
	Daily       []DailyViews
	Views       int64
	Top         []models.TopViewedChangelogEntriesRow
	Reactions   int64
	Subscribers int64
	Joined      int64
	Left        int64

	today time.Time
}

/*
ChangelogReport reads the last days of the help center's changelog, at
most ninety.
*/
func (a *Analytics) ChangelogReport(ctx context.Context, m authz.Membership, helpCenter ids.UUID, days int,
) (ChangelogReport, error) {
	if !m.Can("changelog:read") {
		return ChangelogReport{}, exceptions.ErrForbidden
	}

	days = min(max(days, 1), 90)
	today := dayOf(a.Clock.Now())
	since := today.AddDate(0, 0, -(days - 1)).Add(-12 * time.Hour)
	out := ChangelogReport{Days: days, Since: since, today: today}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		h, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter)
		if err != nil {
			return err
		}

		return a.readChangelog(ctx, q, h, &out)
	})

	return out, err
}

/*
readChangelog fills the report from the changelog's counts.
*/
func (*Analytics) readChangelog(ctx context.Context, q *models.Queries, h models.HelpCenter, out *ChangelogReport,
) error {
	since, days, today := out.Since, out.Days, out.today

	daily, err := q.DailyChangelogViews(ctx, models.DailyChangelogViewsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Since: since,
	})
	if err != nil {
		return fmt.Errorf("daily changelog views: %w", err)
	}

	rows := make([]models.DailyArticleViewsRow, len(daily))
	for i, d := range daily {
		rows[i] = models.DailyArticleViewsRow{Day: d.Day, Views: d.Views}
	}

	out.Daily, out.Views = fillDays(rows, today, days)

	out.Top, err = q.TopViewedChangelogEntries(ctx, models.TopViewedChangelogEntriesParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Since: since, RowLimit: 10,
	})
	if err != nil {
		return fmt.Errorf("top viewed updates: %w", err)
	}

	totals, err := q.ChangelogReaderTotals(ctx, models.ChangelogReaderTotalsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, Since: since,
	})
	if err != nil {
		return fmt.Errorf("changelog reader totals: %w", err)
	}

	out.Reactions, out.Subscribers = totals.Reactions, totals.Subscribers
	out.Joined, out.Left = totals.Joined, totals.LeftCount

	return nil
}

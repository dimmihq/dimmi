/*
|--------------------------------------------------------------------------
| Help Center Analytics
|--------------------------------------------------------------------------
|
| What readers read and look for, counted per day and nothing more: no
| reader, address or session is kept, only how many times an article was
| opened and how many searches for some words found nothing. Counting
| never fails a reader's page; a count that cannot be stored is dropped.
| The team reads the last days back as views over time, the most read
| articles with how helpful readers found them, and the searches that
| found nothing, which are the articles still to write.
|
*/

package helpcenters

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Where a view came from.
*/
const (
	ViewOnWeb       = "web"
	ViewInMessenger = "messenger"
)

/*
How long counts are kept: views for a little over a year, search words,
which readers typed, for three months.
*/
const (
	keepViews = 400 * 24 * time.Hour
	keepTerms = 90 * 24 * time.Hour
)

/*
maxTermRunes caps the words of a search kept.
*/
const maxTermRunes = 100

/*
Analytics counts what readers do and reports it.
*/
type Analytics struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Viewed counts one view of the help center's article from the source.
*/
func (a *Analytics) Viewed(ctx context.Context, h models.HelpCenter, article ids.UUID, source string) {
	if a == nil {
		return
	}
	_ = a.write(ctx, h, func(q *models.Queries) error {
		return q.CountArticleView(ctx, models.CountArticleViewParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, ArticleID: article, CountedAt: dayOf(a.Clock.Now()),
			Source: source,
		})
	})
}

/*
Searched counts one search of the help center and whether it found
anything. Words that look like an email address are not kept.
*/
func (a *Analytics) Searched(ctx context.Context, h models.HelpCenter, query string, results int) {
	term := normalizeTerm(query)
	if a == nil || term == "" {
		return
	}
	misses := int32(0)
	if results == 0 {
		misses = 1
	}
	_ = a.write(ctx, h, func(q *models.Queries) error {
		return q.CountHelpSearch(ctx, models.CountHelpSearchParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, CountedAt: dayOf(a.Clock.Now()), Term: term,
			Misses: misses,
		})
	})
}

/*
write runs a count inside the help center's workspace.
*/
func (a *Analytics) write(ctx context.Context, h models.HelpCenter, fn func(*models.Queries) error) error {
	scoped := tenancy.With(context.WithoutCancel(ctx), h.WorkspaceID.String())
	return a.DB.Transaction(scoped, func(_ context.Context, tx *database.Tx) error {
		return fn(models.New(tx))
	})
}

/*
Prune drops the counts older than they are kept, across workspaces.
*/
func (a *Analytics) Prune(ctx context.Context) error {
	now := a.Clock.Now()
	err := models.New(a.DB).PruneHelpAnalytics(ctx, models.PruneHelpAnalyticsParams{
		ViewsBefore: dayOf(now.Add(-keepViews)), TermsBefore: dayOf(now.Add(-keepTerms)),
	})
	if err != nil {
		return fmt.Errorf("prune help analytics: %w", err)
	}
	return nil
}

/*
normalizeTerm folds a search to the words it counts under: lower case,
single spaces, at most a hundred characters, empty for an address.
*/
func normalizeTerm(query string) string {
	term := strings.Join(strings.Fields(strings.ToLower(query)), " ")
	if strings.Contains(term, "@") {
		return ""
	}
	for utf8.RuneCountInString(term) > maxTermRunes {
		_, size := utf8.DecodeLastRuneInString(term)
		term = term[:len(term)-size]
	}
	return strings.TrimSpace(term)
}

/*
dayOf names the UTC day of a moment by its noon, which every database
time zone within twelve hours reads as the same date.
*/
func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

/*
Report is what readers did over the last days of a help center.
*/
type Report struct {
	Days        int
	Since       time.Time
	Daily       []DailyViews
	Views       int64
	Top         []models.TopViewedArticlesRow
	Helpful     int64
	Unhelpful   int64
	Searches    int64
	Misses      int64
	ContentGaps []models.TopMissedSearchesRow

	today time.Time
}

/*
DailyViews is one day's views, every day of the range present.
*/
type DailyViews struct {
	Day       string
	Views     int64
	Messenger int64
}

/*
Report reads the last days of the help center, at most ninety.
*/
func (a *Analytics) Report(ctx context.Context, m authz.Membership, helpCenter ids.UUID, days int) (Report, error) {
	if !m.Can("articles:read") {
		return Report{}, exceptions.ErrForbidden
	}
	days = min(max(days, 1), 90)
	today := dayOf(a.Clock.Now())
	since := today.AddDate(0, 0, -(days - 1)).Add(-12 * time.Hour)
	out := Report{Days: days, Since: since}
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter)
		if err != nil {
			return err
		}
		out.today = today
		return a.read(ctx, q, h, &out)
	})
	return out, err
}

/*
read fills the report from the help center's counts.
*/
func (a *Analytics) read(ctx context.Context, q *models.Queries, h models.HelpCenter, out *Report) error {
	ws, id, since := h.WorkspaceID, h.ID, out.Since
	daily, err := q.DailyArticleViews(ctx, models.DailyArticleViewsParams{
		WorkspaceID: ws, HelpCenterID: id, Since: since,
	})
	if err != nil {
		return fmt.Errorf("daily article views: %w", err)
	}
	out.Daily, out.Views = fillDays(daily, out.today, out.Days)
	out.Top, err = q.TopViewedArticles(ctx, models.TopViewedArticlesParams{
		WorkspaceID: ws, HelpCenterID: id, Since: since, RowLimit: 10,
	})
	if err != nil {
		return fmt.Errorf("top viewed articles: %w", err)
	}
	ratings, err := q.HelpCenterRatingTotals(ctx, models.HelpCenterRatingTotalsParams{
		WorkspaceID: ws, HelpCenterID: id, Since: since,
	})
	if err != nil {
		return fmt.Errorf("rating totals: %w", err)
	}
	out.Helpful, out.Unhelpful = ratings.Helpful, ratings.Unhelpful
	searches, err := q.HelpSearchTotals(ctx, models.HelpSearchTotalsParams{
		WorkspaceID: ws, HelpCenterID: id, Since: since,
	})
	if err != nil {
		return fmt.Errorf("search totals: %w", err)
	}
	out.Searches, out.Misses = searches.Searches, searches.Misses
	out.ContentGaps, err = q.TopMissedSearches(ctx, models.TopMissedSearchesParams{
		WorkspaceID: ws, HelpCenterID: id, Since: since, RowLimit: 20,
	})
	if err != nil {
		return fmt.Errorf("missed searches: %w", err)
	}
	return nil
}

/*
fillDays lays the stored days over every day of the range, a day with
no views at zero, and totals them.
*/
func fillDays(rows []models.DailyArticleViewsRow, today time.Time, days int) ([]DailyViews, int64) {
	stored := make(map[string]models.DailyArticleViewsRow, len(rows))
	for _, r := range rows {
		stored[r.Day] = r
	}
	out := make([]DailyViews, 0, days)
	var total int64
	for i := days - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i).Format(time.DateOnly)
		r := stored[day]
		out = append(out, DailyViews{Day: day, Views: r.Views, Messenger: r.MessengerViews})
		total += r.Views
	}
	return out, total
}

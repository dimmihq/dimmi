/*
|--------------------------------------------------------------------------
| Help Center Analytics Resources
|--------------------------------------------------------------------------
|
| How a help center's analytics read over the API: views per day, the
| most read articles with how helpful readers found them, and the
| searches that found nothing.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DailyViews is one day's article views.
*/
type DailyViews struct {
	Date      string `json:"date"`
	Views     int64  `json:"views"`
	Messenger int64  `json:"messenger"`
}

/*
TopArticle is one of the most read articles.
*/
type TopArticle struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Views     int64  `json:"views"`
	Helpful   int64  `json:"helpful"`
	Unhelpful int64  `json:"unhelpful"`
}

/*
ContentGap is words readers searched for that found nothing.
*/
type ContentGap struct {
	Query    string `json:"query"`
	Searches int64  `json:"searches"`
	LastDate string `json:"last_date"`
}

/*
HelpCenterAnalytics is what readers did over the last days. Helpful
percent is null without answers.
*/
type HelpCenterAnalytics struct {
	Object         string       `json:"object"`
	Days           int          `json:"days"`
	Since          string       `json:"since"`
	Views          int64        `json:"views"`
	Daily          []DailyViews `json:"daily"`
	TopArticles    []TopArticle `json:"top_articles"`
	Helpful        int64        `json:"helpful"`
	Unhelpful      int64        `json:"unhelpful"`
	HelpfulPercent *int         `json:"helpful_percent"`
	Searches       int64        `json:"searches"`
	EmptySearches  int64        `json:"empty_searches"`
	ContentGaps    []ContentGap `json:"content_gaps"`
}

/*
NewHelpCenterAnalytics shapes a help center's report.
*/
func NewHelpCenterAnalytics(r helpcenters.Report) HelpCenterAnalytics {
	out := HelpCenterAnalytics{
		Object: "help_center_analytics", Days: r.Days, Since: r.Since.Add(12 * time.Hour).Format(time.DateOnly),
		Views: r.Views, Daily: make([]DailyViews, len(r.Daily)), TopArticles: make([]TopArticle, len(r.Top)),
		Helpful: r.Helpful, Unhelpful: r.Unhelpful, Searches: r.Searches, EmptySearches: r.Misses,
		ContentGaps: make([]ContentGap, len(r.ContentGaps)),
	}
	for i, d := range r.Daily {
		out.Daily[i] = DailyViews{Date: d.Day, Views: d.Views, Messenger: d.Messenger}
	}
	for i, t := range r.Top {
		out.TopArticles[i] = TopArticle{
			ID: models.PublicID(models.PrefixArticle, t.ArticleID), Title: t.Title, Slug: t.Slug, Views: t.Views,
			Helpful: t.Helpful, Unhelpful: t.Unhelpful,
		}
	}
	for i, g := range r.ContentGaps {
		out.ContentGaps[i] = ContentGap{Query: g.Term, Searches: g.Misses, LastDate: g.LastDay}
	}
	if answers := r.Helpful + r.Unhelpful; answers > 0 {
		out.HelpfulPercent = new(int(r.Helpful * 100 / answers))
	}
	return out
}

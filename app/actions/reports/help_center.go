/*
|--------------------------------------------------------------------------
| Help Center
|--------------------------------------------------------------------------
|
| How readers used the workspace's help centers in the range: article
| views each day, in the messenger and on the web, the most read
| articles with how helpful readers found them, and the searches that
| found nothing. These are counted per day, so the conversation filters
| do not apply.
|
*/

package reports

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpCenter is the help center report.
*/
type HelpCenter struct{}

/*
Describe says what the report is.
*/
func (HelpCenter) Describe() Description {
	return Description{
		Key: "help_center", Title: "Help center", Order: 80, Permission: "articles:read",
		Summary: "Article views, helpfulness and searches that found nothing.",
	}
}

/*
Metrics returns the headline numbers.
*/
func (HelpCenter) Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error) {
	qs := models.New(tx)
	ws := q.Membership.Workspace.ID

	days, err := qs.ReportHelpCenterDaily(ctx, models.ReportHelpCenterDailyParams{
		WorkspaceID: ws, FromDay: q.Filters.From, ToDay: q.Filters.To,
	})
	if err != nil {
		return nil, err
	}

	var views int64
	for _, d := range days {
		views += d.Views
	}

	searches, err := qs.ReportHelpCenterSearches(ctx, models.ReportHelpCenterSearchesParams{
		WorkspaceID: ws, FromDay: q.Filters.From, ToDay: q.Filters.To,
	})
	if err != nil {
		return nil, err
	}

	return []Metric{
		{Key: "article_views", Label: "Article views", Unit: UnitCount, Value: num(views), Sample: views},
		{Key: "searches", Label: "Searches", Unit: UnitCount, Value: num(searches.Searches), Sample: searches.Searches},
		{
			Key: "empty_searches", Label: "Searches with no result", Unit: UnitPercent,
			Value: ratio(searches.Misses, searches.Searches), Sample: searches.Searches,
		},
	}, nil
}

/*
Charts returns the charts.
*/
func (HelpCenter) Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error) {
	qs := models.New(tx)
	ws := q.Membership.Workspace.ID

	days, err := qs.ReportHelpCenterDaily(ctx, models.ReportHelpCenterDailyParams{
		WorkspaceID: ws, FromDay: q.Filters.From, ToDay: q.Filters.To,
	})
	if err != nil {
		return nil, err
	}

	asValues := make([]values, len(days))
	for i, d := range days {
		asValues[i] = values{key: d.Day, nums: []*float64{num(d.Views - d.Messenger), num(d.Messenger)}}
	}

	articles, err := topArticles(ctx, qs, q)
	if err != nil {
		return nil, err
	}

	missing, err := contentGaps(ctx, qs, q)
	if err != nil {
		return nil, err
	}

	return []Chart{
		{
			Key: "views_daily", Title: "Article views", Kind: ChartStacked, Unit: UnitCount, Axis: AxisDay,
			Series: []Series{
				daily(q, Series{Key: "web", Label: "Web"}, asValues, at(0)),
				daily(q, Series{Key: "messenger", Label: "Messenger"}, asValues, at(1)),
			},
		},
		articles, missing,
	}, nil
}

/*
topArticles is the table of the most read articles and how helpful
readers found them.
*/
func topArticles(ctx context.Context, qs *models.Queries, q Query) (Chart, error) {
	top, err := qs.ReportHelpCenterTopArticles(ctx, models.ReportHelpCenterTopArticlesParams{
		WorkspaceID: q.Membership.Workspace.ID, FromDay: q.Filters.From, ToDay: q.Filters.To, Since: q.Start,
		RowLimit: listed,
	})
	if err != nil {
		return Chart{}, err
	}

	articles := Chart{
		Key: "top_articles", Title: "Most read articles", Kind: ChartTable, Axis: AxisCategory,
		Columns: []Column{
			{Key: "views", Label: "Views", Unit: UnitCount}, {Key: "helpful", Label: "Helpful", Unit: UnitPercent},
		},
	}
	for _, a := range top {
		articles.Rows = append(articles.Rows, Row{
			Key: models.PublicID(models.PrefixArticle, a.ID), Label: a.Title,
			Values: []*float64{num(a.Views), ratio(a.Helpful, a.Helpful+a.Unhelpful)},
		})
	}

	return articles, nil
}

/*
contentGaps is the table of searches that found nothing.
*/
func contentGaps(ctx context.Context, qs *models.Queries, q Query) (Chart, error) {
	gaps, err := qs.ReportHelpCenterGaps(ctx, models.ReportHelpCenterGapsParams{
		WorkspaceID: q.Membership.Workspace.ID, FromDay: q.Filters.From, ToDay: q.Filters.To, RowLimit: listed,
	})
	if err != nil {
		return Chart{}, err
	}

	missing := Chart{
		Key: "content_gaps", Title: "Searches that found nothing", Kind: ChartTable, Axis: AxisCategory,
		Columns: []Column{{Key: "misses", Label: "Found nothing", Unit: UnitCount}},
	}
	for _, g := range gaps {
		missing.Rows = append(missing.Rows, Row{Key: g.Term, Label: g.Term, Values: []*float64{num(g.Misses)}})
	}

	return missing, nil
}

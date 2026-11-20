/*
|--------------------------------------------------------------------------
| Changelog Analytics Resources
|--------------------------------------------------------------------------
|
| How a changelog's analytics read over the API: views by day, the most
| read updates, reactions and how the subscribers moved.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TopChangelogEntry is one of the most read updates.
*/
type TopChangelogEntry struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	PublishedAt *time.Time `json:"published_at"`
	Views       int64      `json:"views"`
	Reactions   int64      `json:"reactions"`
}

/*
ChangelogDailyViews is one day's views of updates.
*/
type ChangelogDailyViews struct {
	Date  string `json:"date"`
	Views int64  `json:"views"`
}

/*
ChangelogAnalytics is what readers did with a changelog over the last
days: views, the most read updates, reactions, and the subscribers it
has now with those who joined and left over the days.
*/
type ChangelogAnalytics struct {
	Object                  string                `json:"object"`
	Days                    int                   `json:"days"`
	Since                   string                `json:"since"`
	Views                   int64                 `json:"views"`
	Daily                   []ChangelogDailyViews `json:"daily"`
	TopEntries              []TopChangelogEntry   `json:"top_entries"`
	Reactions               int64                 `json:"reactions"`
	Subscribers             int64                 `json:"subscribers"`
	SubscribersJoined       int64                 `json:"subscribers_joined"`
	SubscribersUnsubscribed int64                 `json:"subscribers_unsubscribed"`
}

/*
NewChangelogAnalytics shapes a changelog's report.
*/
func NewChangelogAnalytics(r helpcenters.ChangelogReport) ChangelogAnalytics {
	out := ChangelogAnalytics{
		Object: "changelog_analytics", Days: r.Days, Since: r.Since.Add(12 * time.Hour).Format(time.DateOnly),
		Views: r.Views, Daily: make([]ChangelogDailyViews, len(r.Daily)),
		TopEntries: make([]TopChangelogEntry, len(r.Top)), Reactions: r.Reactions, Subscribers: r.Subscribers,
		SubscribersJoined: r.Joined, SubscribersUnsubscribed: r.Left,
	}
	for i, d := range r.Daily {
		out.Daily[i] = ChangelogDailyViews{Date: d.Day, Views: d.Views}
	}
	for i, t := range r.Top {
		out.TopEntries[i] = TopChangelogEntry{
			ID: models.PublicID(models.PrefixChangelogEntry, t.EntryID), Title: t.Title, Slug: t.Slug,
			PublishedAt: t.PublishedAt, Views: t.Views, Reactions: t.Reactions,
		}
	}
	return out
}

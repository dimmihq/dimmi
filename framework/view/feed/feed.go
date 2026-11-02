/*
|--------------------------------------------------------------------------
| RSS Feeds
|--------------------------------------------------------------------------
|
| Readers follow published content through RSS 2.0, which every feed
| reader understands. Each item keeps a stable id, so a reader that sees
| an edited item updates it instead of showing it twice, and full content
| rides along so it can be read without opening the site.
|
*/

package feed

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

/*
Feed is one channel of items.

Title, Link and Description describe the channel and Updated is when
anything in it last changed. Items are written in the order given,
newest first by convention.
*/
type Feed struct {
	Title       string
	Link        string
	Description string
	Updated     time.Time
	Items       []Item
}

/*
Item is one entry of a feed.

ID stays the same for the life of the entry. Description is the short
summary and Content the full HTML body. Categories label the entry.
Published is when it first appeared and Updated when it last changed;
the item's date is Published, or Updated when it was never published.
*/
type Item struct {
	ID          string
	Title       string
	Link        string
	Description string
	Content     string
	Categories  []string
	Published   time.Time
	Updated     time.Time
}

/*
RSS writes the feed as an RSS 2.0 document. An item's categories are
written as one category, separated by commas, because the library writes
at most one per item.
*/
func RSS(w io.Writer, f Feed) error {
	channel := (&feeds.Rss{Feed: &feeds.Feed{
		Title:       f.Title,
		Link:        &feeds.Link{Href: f.Link},
		Description: f.Description,
		Updated:     f.Updated,
	}}).RssFeed()

	for _, item := range f.Items {
		channel.Items = append(channel.Items, rssItem(item))
	}

	if err := feeds.WriteXML(channel, w); err != nil {
		return fmt.Errorf("write rss feed: %w", err)
	}

	return nil
}

func rssItem(item Item) *feeds.RssItem {
	out := &feeds.RssItem{
		Title:       item.Title,
		Link:        item.Link,
		Description: item.Description,
		Category:    strings.Join(item.Categories, ", "),
		Guid:        &feeds.RssGuid{Id: item.ID, IsPermaLink: "false"},
		PubDate:     date(item.Published, item.Updated),
	}

	if item.Content != "" {
		out.Content = &feeds.RssContent{Content: item.Content}
	}

	return out
}

func date(times ...time.Time) string {
	for _, t := range times {
		if !t.IsZero() {
			return t.Format(time.RFC1123Z)
		}
	}

	return ""
}

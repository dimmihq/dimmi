package feed_test

import (
	"bytes"
	"encoding/xml"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/view/feed"
)

type document struct {
	Version string `xml:"version,attr"`
	Channel struct {
		Title         string `xml:"title"`
		Link          string `xml:"link"`
		Description   string `xml:"description"`
		LastBuildDate string `xml:"lastBuildDate"`
		Items         []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			Category    string `xml:"category"`
			GUID        struct {
				ID          string `xml:",chardata"`
				IsPermaLink string `xml:"isPermaLink,attr"`
			} `xml:"guid"`
			PubDate string `xml:"pubDate"`
			Content string `xml:"encoded"`
		} `xml:"item"`
	} `xml:"channel"`
}

/*
failingWriter refuses every write, as a connection the client closed
does.
*/
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestRSS(t *testing.T) {
	t.Parallel()

	published := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	channel := feed.Feed{
		Title:       "Changes",
		Link:        "https://example.com/changes",
		Description: "What changed",
		Updated:     updated,
		Items: []feed.Item{
			{
				ID:          "entry_1",
				Title:       "Dark mode",
				Link:        "https://example.com/changes/dark-mode",
				Description: "Easier on the eyes.",
				Content:     "<p>Easier on the <b>eyes</b>.</p>",
				Categories:  []string{"Feature", "Design"},
				Published:   published,
				Updated:     updated,
			},
			{ID: "entry_2", Title: "Draft", Updated: updated},
		},
	}

	t.Run("writes an rss 2.0 document with every item", func(t *testing.T) {
		t.Parallel()
		var b bytes.Buffer
		require.NoError(t, feed.RSS(&b, channel))

		var doc document
		require.NoError(t, xml.Unmarshal(b.Bytes(), &doc))
		assert.Equal(t, "2.0", doc.Version)
		assert.Equal(t, "Changes", doc.Channel.Title)
		assert.Equal(t, "https://example.com/changes", doc.Channel.Link)
		assert.Equal(t, "What changed", doc.Channel.Description)
		assert.Equal(t, updated.Format(time.RFC1123Z), doc.Channel.LastBuildDate)
		require.Len(t, doc.Channel.Items, 2)

		first := doc.Channel.Items[0]
		assert.Equal(t, "Dark mode", first.Title)
		assert.Equal(t, "https://example.com/changes/dark-mode", first.Link)
		assert.Equal(t, "Easier on the eyes.", first.Description)
		assert.Equal(t, "<p>Easier on the <b>eyes</b>.</p>", first.Content)
		assert.Equal(t, "Feature, Design", first.Category)
		assert.Equal(t, "entry_1", first.GUID.ID)
		assert.Equal(t, "false", first.GUID.IsPermaLink)
		assert.Equal(t, published.Format(time.RFC1123Z), first.PubDate)
	})

	t.Run("dates an unpublished item by its update and omits empty content", func(t *testing.T) {
		t.Parallel()
		var b bytes.Buffer
		require.NoError(t, feed.RSS(&b, channel))

		var doc document
		require.NoError(t, xml.Unmarshal(b.Bytes(), &doc))
		second := doc.Channel.Items[1]
		assert.Equal(t, updated.Format(time.RFC1123Z), second.PubDate)
		assert.Empty(t, second.Content)
		assert.NotContains(t, b.String(), "<category></category>")
	})

	t.Run("leaves out the date of an item with none", func(t *testing.T) {
		t.Parallel()
		var b bytes.Buffer
		require.NoError(t, feed.RSS(&b, feed.Feed{Title: "Empty", Items: []feed.Item{{ID: "x"}}}))
		assert.NotContains(t, b.String(), "pubDate")
	})

	t.Run("returns the write error", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, feed.RSS(failingWriter{}, channel), io.ErrClosedPipe)
	})
}

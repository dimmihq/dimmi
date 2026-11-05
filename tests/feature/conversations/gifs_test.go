package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/tests"
)

/*
withGIFs turns GIF search on with a made-up GIPHY key.
*/
func withGIFs() tests.Option {
	return tests.Before(func(a *app.App) { a.Config.Inbox.GIF.GiphyAPIKey = "giphy-test-key" })
}

const gifBody = "Here you go ![Thumbs up](https://media1.giphy.com/media/abc123/giphy.gif) " +
	"and ![Tracker](https://tracker.example.com/pixel.gif) and ![Plain](http://media2.giphy.com/media/x/giphy.gif)"

func TestGIFs(t *testing.T) {
	t.Parallel()

	t.Run("gives the composer the key only while the instance and the workspace allow GIFs", func(t *testing.T) {
		t.Parallel()
		d := open(t, withGIFs())

		d.owner.GetJSON("/api/v1/gif-search").AssertOK().
			AssertJSONPath("data.provider", "giphy").AssertJSONPath("data.api_key", "giphy-test-key")
		d.owner.GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.gif_picker", true)

		d.owner.PatchJSON("/api/v1/workspace", testkit.JSON{"gif_picker": false}).AssertOK().
			AssertJSONPath("data.gif_picker", false)
		d.owner.GetJSON("/api/v1/gif-search").AssertOK().
			AssertJSONPath("data.enabled", false).AssertJSONPath("data.api_key", nil)

		open(t).owner.GetJSON("/api/v1/gif-search").AssertOK().AssertJSONPath("data.enabled", false)
	})

	t.Run("sends a GIPHY picture and drops every other remote one", func(t *testing.T) {
		t.Parallel()
		d := open(t, withGIFs())
		driver, path := fakeConversation(t, d)

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": gifBody}).AssertCreated()
		d.tc.Drain()

		require.Len(t, driver.Sent(), 2)
		html := driver.Sent()[1].BodyHTML
		assert.Contains(t, html, `src="https://media1.giphy.com/media/abc123/giphy.gif"`)
		assert.NotContains(t, html, "tracker.example.com")
		assert.NotContains(t, html, "http://media2.giphy.com")
		assert.Contains(t, html, "Tracker")
		d.tc.Table("messages").AssertCount(1, testkit.Row{"body": gifBody})

		var gif string
		require.NoError(t, d.tc.Operator().QueryRow(t.Context(),
			"SELECT metadata->'gif'->>'id' FROM messages WHERE body = $1", gifBody).Scan(&gif))
		assert.Equal(t, "abc123", gif)
	})

	t.Run("keeps GIPHY pictures out of replies while GIF search is off", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": gifBody}).AssertCreated()
		d.tc.Drain()

		require.Len(t, driver.Sent(), 2)
		assert.NotContains(t, driver.Sent()[1].BodyHTML, "giphy.com")
	})
}

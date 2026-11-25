package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

const picturesBody = "Here you go ![Thumbs up](https://media1.giphy.com/media/abc123/giphy.gif) " +
	"and ![Tracker](https://tracker.example.com/pixel.gif)"

func TestRemotePictures(t *testing.T) {
	t.Parallel()

	t.Run("keeps every remote picture out of a reply and its words in", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": picturesBody}).AssertCreated()
		d.tc.Drain()

		require.Len(t, driver.Sent(), 2)
		html := driver.Sent()[1].BodyHTML
		assert.NotContains(t, html, "giphy.com")
		assert.NotContains(t, html, "tracker.example.com")
		assert.Contains(t, html, "Thumbs up")
		assert.Contains(t, html, "Tracker")
		d.tc.Table("messages").AssertCount(1, testkit.Row{"body": picturesBody})
	})
}

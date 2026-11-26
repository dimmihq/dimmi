package helpcenters_test

import (
	"bytes"
	stdimage "image"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestHelpCenterLogo(t *testing.T) {
	t.Parallel()

	t.Run("shows the workspace's logo in the top bar and the tab, and the name without one", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)

		before := string(c.page("").AssertOK().Body)
		assert.NotContains(t, before, "theme-logo-light")
		assert.Contains(t, before, `<link rel="icon" href="data:,">`)

		var picture bytes.Buffer
		require.NoError(t, png.Encode(&picture, stdimage.NewRGBA(stdimage.Rect(0, 0, 40, 10))))
		path := c.d.Owner.PutFile("/api/v1/workspace/logo", "logo", "logo.png", picture.Bytes()).AssertOK().
			String("data.logo_url")
		logo := strings.TrimSuffix(c.d.TC.App.Application.Config.App.URL, "/") + path

		home := string(c.page("").AssertOK().Body)
		assert.Contains(t, home, `src="`+logo+`"`, "the logo loads from the app on every host")
		assert.Contains(t, home, `<link rel="icon" href="`+logo+`">`)
		article := c.article(t, testkit.JSON{"title": "Reset your password", "body_markdown": "Open settings."})
		id := article.String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		assert.Contains(t, string(c.page("/articles/reset-your-password").AssertOK().Body), `src="`+logo+`"`)

		c.d.Owner.Delete("/api/v1/workspace/logo").AssertNoContent()
		assert.NotContains(t, string(c.page("").AssertOK().Body), logo, "a removed logo leaves at once")
	})
}

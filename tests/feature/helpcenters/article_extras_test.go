package helpcenters_test

import (
	"bytes"
	stdimage "image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
picture is a PNG of the given size.
*/
func picture(t *testing.T, width, height int) []byte {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))))

	return out.Bytes()
}

func TestArticleExtras(t *testing.T) {
	t.Parallel()

	t.Run("restores a kept version into the working copy", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{"title": "Refunds", "body_markdown": "First words."}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		c.d.Owner.PatchJSON("/api/v1/articles/"+id, testkit.JSON{
			"title": "Refunds and returns", "body_markdown": "Second words.",
		}).AssertOK()
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		first := c.d.Owner.GetJSON("/api/v1/articles/"+id+"/versions").AssertOK().
			AssertJSONPath("data.#", 2).String("data.1.id")

		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/restorations", testkit.JSON{"version_id": first}).AssertOK().
			AssertJSONPath("data.title", "Refunds").AssertJSONPath("data.body_markdown", "First words.").
			AssertJSONPath("data.has_unpublished_changes", true)
		assert.Contains(t, string(c.page("/articles/refunds").Body), "Second words.")
	})

	t.Run("refuses a version of another article or one that does not exist", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		one := c.article(t, testkit.JSON{"title": "One", "body_markdown": "One."}).String("data.id")
		two := c.article(t, testkit.JSON{"title": "Two", "body_markdown": "Two."}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+one+"/publications", nil).AssertOK()
		kept := c.d.Owner.GetJSON("/api/v1/articles/" + one + "/versions").AssertOK().String("data.0.id")

		c.d.Owner.PostJSON("/api/v1/articles/"+two+"/restorations", testkit.JSON{"version_id": kept}).AssertNotFound()
		c.d.Owner.PostJSON("/api/v1/articles/"+two+"/restorations", testkit.JSON{"version_id": "artv_nope"}).
			AssertNotFound()
	})

	t.Run("keeps a picture redrawn and serves it on the help center's hosts", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_HELP_CENTERS__BASE_DOMAIN=help.example.test"))
		owner := tc.In(tc.Owner)
		sub := owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.subdomain")

		url := owner.PostForm("/api/v1/help-images",
			testkit.FormFile{Field: "image", Name: "shot.png", Contents: picture(t, 40, 20)}, nil).
			AssertStatus(201).String("data.url")
		require.Regexp(t, `^/help-images/[0-9a-f-]{36}\.png$`, url)

		tc.Client().Send("GET", url, nil).AssertOK().AssertHeader("Content-Type", "image/png")
		tc.Client().Send("GET", "https://"+sub+".help.example.test"+url, nil).AssertOK()
		tc.Client().Send("GET", "/help-images/nope.png", nil).AssertNotFound()
	})

	t.Run("refuses what is not a picture", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostForm("/api/v1/help-images",
			testkit.FormFile{Field: "image", Name: "notes.png", Contents: []byte("not a picture")}, nil).
			AssertStatus(422).AssertJSONPath("code", "invalid_image")
		other := testkit.FormFile{Field: "other", Name: "a.png", Contents: picture(t, 2, 2)}
		d.Owner.PostForm("/api/v1/help-images", other, nil).AssertStatus(422)
	})
}

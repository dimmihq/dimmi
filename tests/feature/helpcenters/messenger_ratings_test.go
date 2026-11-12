package helpcenters_test

import (
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestArticleRatingsFromTheMessenger(t *testing.T) {
	t.Parallel()

	t.Run("keeps one answer per visitor, changed when they answer again", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{"title": "Reset your password", "body_markdown": "Open Settings."}).
			String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		visitor := func() *testkit.Client {
			started := c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": c.app()}).AssertCreated()
			return c.d.TC.Client().WithHeader("Authorization", "Bearer "+started.String("data.token"))
		}
		ana, ben := visitor(), visitor()
		rate := func(v *testkit.Client, helpful bool) *testkit.Response {
			return v.PostJSON("/widget/v1/article-ratings",
				testkit.JSON{"article": "reset-your-password", "helpful": helpful})
		}

		rate(ana, false).AssertNoContent()
		rate(ben, true).AssertNoContent()
		c.feedbackOf(id).AssertJSONPath("data.helpful", float64(1)).AssertJSONPath("data.unhelpful", float64(1))

		rate(ana, true).AssertNoContent()
		c.feedbackOf(id).AssertJSONPath("data.helpful", float64(2)).AssertJSONPath("data.unhelpful", float64(0))
	})

	t.Run("asks for the visitor's session and a real article slug", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.d.TC.Client().PostJSON("/widget/v1/article-ratings", testkit.JSON{"article": "x", "helpful": true}).
			AssertStatus(fhttp.StatusUnauthorized)

		started := c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": c.app()}).AssertCreated()
		visitor := c.d.TC.Client().WithHeader("Authorization", "Bearer "+started.String("data.token"))
		visitor.PostJSON("/widget/v1/article-ratings", testkit.JSON{"article": "Not A Slug", "helpful": true}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		visitor.PostJSON("/widget/v1/article-ratings", testkit.JSON{"article": "missing", "helpful": true}).
			AssertNoContent()
	})
}

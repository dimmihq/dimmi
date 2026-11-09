package helpcenters_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
answer posts a reader's form on the help center and returns where the
portal sends them.
*/
func (c center) answer(path string, form url.Values) string {
	res := c.d.TC.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Send("POST", "/hc/"+c.subdomain+path, form.Encode())
	return res.Header.Get("Location")
}

/*
feedbackOf reads what readers said about the article, as the team sees it.
*/
func (c center) feedbackOf(id string) *testkit.Response {
	return c.d.Owner.GetJSON("/api/v1/articles/" + id + "/feedback").AssertOK()
}

func TestArticleRatings(t *testing.T) {
	t.Parallel()

	published := func(t *testing.T, c center) (string, string) {
		t.Helper()
		id := c.article(t, testkit.JSON{"title": "Reset your password", "body_markdown": "Open Settings."}).
			String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		return id, "/articles/reset-your-password"
	}

	t.Run("keeps a no and the note left with it for the team, and no note on a yes", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id, path := published(t, c)

		assert.Contains(t, c.answer(path+"/feedback", url.Values{"helpful": {"yes"}}), "helpful=yes")
		c.answer(path+"/feedback/comment", url.Values{"comment": {"A note on a yes"}})
		c.feedbackOf(id).AssertJSONPath("data.helpful", float64(1)).AssertJSONPath("data.comments", float64(0))

		assert.Contains(t, c.answer(path+"/feedback", url.Values{"helpful": {"no"}}), "helpful=no")
		assert.Contains(t, string(c.page(path+"?helpful=no").Body), "What were you looking for?")
		to := c.answer(path+"/feedback/comment", url.Values{"comment": {"  How do I reset with SSO?  "}})
		assert.Contains(t, to, "commented=1")
		assert.Contains(t, string(c.page(path+"?helpful=no&commented=1").Body), "The team reads every note")

		c.feedbackOf(id).AssertJSONPath("data.helpful", float64(0)).AssertJSONPath("data.unhelpful", float64(1)).
			AssertJSONPath("data.comments", float64(1)).
			AssertJSONPath("data.recent.0.comment", "How do I reset with SSO?").
			AssertJSONPath("data.recent.0.contact", nil)
	})

	t.Run("changes a reader's answer instead of counting it twice, and a yes clears their note", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id, path := published(t, c)

		c.answer(path+"/feedback", url.Values{"helpful": {"no"}})
		c.answer(path+"/feedback/comment", url.Values{"comment": {"Missing steps"}})
		c.answer(path+"/feedback", url.Values{"helpful": {"no"}})
		c.feedbackOf(id).AssertJSONPath("data.unhelpful", float64(1)).AssertJSONPath("data.comments", float64(1))

		c.answer(path+"/feedback", url.Values{"helpful": {"yes"}})
		c.feedbackOf(id).AssertJSONPath("data.helpful", float64(1)).AssertJSONPath("data.unhelpful", float64(0)).
			AssertJSONPath("data.comments", float64(0))
	})

	t.Run("leaves an empty note unsent and holds back a reader who sends too much", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		_, path := published(t, c)
		c.answer(path+"/feedback", url.Values{"helpful": {"no"}})

		assert.NotContains(t, c.answer(path+"/feedback/comment", url.Values{"comment": {"   "}}), "commented")
		var last string
		for range 11 {
			last = c.answer(path+"/feedback/comment", url.Values{"comment": {"again"}})
		}
		assert.Contains(t, last, "limited=1")
		assert.Contains(t, string(c.page(path+"?helpful=no&limited=1").Body), "sent a lot of feedback")
	})

	t.Run("shows an article's feedback only to its own workspace", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id, _ := published(t, c)
		other := newCenter(t)

		other.d.Owner.GetJSON("/api/v1/articles/" + id + "/feedback").AssertStatus(fhttp.StatusNotFound)
	})
}

package feedback_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

func TestCustomFeedbackStatuses(t *testing.T) {
	t.Parallel()

	t.Run("adds a status from its name, with a category, and moves requests to it", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		id := b.post(t, testkit.JSON{"title": "Dark mode"})

		b.d.Owner.PostJSON("/api/v1/feedback-statuses", testkit.JSON{
			"name": "In beta", "color": "#8B5CF6", "category": "in_progress",
		}).AssertCreated().AssertJSONPath("data.id", "in_beta").AssertJSONPath("data.color", "#8b5cf6").
			AssertJSONPath("data.roadmap", true)
		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+id, testkit.JSON{"status": "in_beta"}).AssertOK().
			AssertJSONPath("data.status", "in_beta")
		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+id, testkit.JSON{"status": "invented"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "status_invalid")

		assert.Contains(t, order(t, b.d.Owner), "in_beta")
		page := string(b.page("/feedback").AssertOK().Body)
		assert.Contains(t, page, `id="in-beta-title"`)
		assert.Contains(t, string(b.page("/feedback/1-dark-mode").AssertOK().Body), "In beta")
		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts").AssertOK().
			AssertJSONPath("counts.in_beta", float64(1))
	})

	t.Run("renames and recolors a status and keeps its key", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})

		b.d.Owner.PatchJSON("/api/v1/feedback-statuses/under_review", testkit.JSON{
			"name": "Gathering votes", "color": "#0ea5e9",
		}).AssertOK().AssertJSONPath("data.id", "under_review").AssertJSONPath("data.name", "Gathering votes")

		assert.Contains(t, string(b.page("/feedback").AssertOK().Body), `id="gathering-votes-title"`)
		b.d.Owner.PatchJSON("/api/v1/feedback-statuses/nowhere", testkit.JSON{"name": "x"}).AssertNotFound()
		b.d.Owner.PatchJSON("/api/v1/feedback-statuses/planned", testkit.JSON{"color": "blue"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("keeps statuses off the roadmap when asked and always for closed ones", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})

		b.d.Owner.PatchJSON("/api/v1/feedback-statuses/planned", testkit.JSON{"roadmap": false}).AssertOK()
		b.d.Owner.PostJSON("/api/v1/feedback-statuses", testkit.JSON{"name": "Won't do", "category": "closed"}).
			AssertCreated().AssertJSONPath("data.roadmap", false)

		page := string(b.page("/feedback").AssertOK().Body)
		assert.NotContains(t, page, `id="planned-title"`)
		assert.NotContains(t, page, `id="won-t-do-title"`)
		assert.Contains(t, page, `id="shipped-title"`)
	})

	t.Run("starts new requests in the first open status", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.d.Owner.PostJSON("/api/v1/feedback-statuses", testkit.JSON{"name": "New", "category": "open"}).
			AssertCreated()
		b.d.Owner.PatchJSON("/api/v1/feedback-statuses", testkit.JSON{
			"statuses": []string{"new", "under_review", "planned", "in_progress", "shipped", "closed"},
		}).AssertOK()

		id := b.post(t, testkit.JSON{"title": "Dark mode"})

		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id).AssertOK().AssertJSONPath("data.status", "new")
	})

	t.Run("moves a deleted status's requests and keeps the last status", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		id := b.post(t, testkit.JSON{"title": "Dark mode"})
		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+id, testkit.JSON{"status": "planned"}).AssertOK()

		b.d.Owner.Delete("/api/v1/feedback-statuses/planned?move_to=planned").
			AssertStatus(fhttp.StatusUnprocessableEntity)
		b.d.Owner.Delete("/api/v1/feedback-statuses/planned?move_to=in_progress").AssertStatus(fhttp.StatusNoContent)

		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id).AssertOK().AssertJSONPath("data.status", "in_progress")
		assert.NotContains(t, order(t, b.d.Owner), "planned")
		for _, key := range []string{"under_review", "in_progress", "shipped"} {
			b.d.Owner.Delete("/api/v1/feedback-statuses/" + key).AssertStatus(fhttp.StatusNoContent)
		}
		b.d.Owner.Delete("/api/v1/feedback-statuses/closed").AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "last_status")
		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id).AssertOK().AssertJSONPath("data.status", "closed")
	})

	t.Run("tells voters when a request moves to a custom done status", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		fake := b.d.TC.Mail()
		ana := b.reader(t, "ana@example.org")
		b.form(ana, "/feedback", url.Values{"title": {"Zapier"}})
		b.d.Owner.PostJSON("/api/v1/feedback-statuses", testkit.JSON{"name": "Live", "category": "done"}).
			AssertCreated()
		posts := b.d.Owner.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-posts").AssertOK()

		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+posts.String("data.0.id"), testkit.JSON{
			"status": "live", "notify": true,
		}).AssertOK()

		mails, _ := tests.Sent[appmail.FeedbackStatusMail](b.d.TC, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "Live", mails[0].Moved.Status)
		assert.True(t, strings.HasSuffix(mails[0].Moved.URL, "/feedback/1-zapier"))
	})
}

package feedback_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

func TestFeedbackFromConversations(t *testing.T) {
	t.Parallel()

	t.Run("links a conversation to a request and votes for its customer once", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		conv := b.d.Customer(t, "cy", "Need SAML").String("data.id")
		id := b.post(t, testkit.JSON{"title": "SAML"})
		path := "/api/v1/conversations/" + conv + "/feedback-posts"

		b.d.Owner.PostJSON(path, testkit.JSON{"post_id": id, "priority": "must"}).AssertOK().
			AssertJSONPath("data.vote_count", float64(1))
		b.d.Owner.PostJSON(path, testkit.JSON{"post_id": id}).AssertOK().AssertJSONPath("data.vote_count", float64(1))

		b.d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.id", id)
		linked := b.d.Owner.GetJSON("/api/v1/feedback-posts/" + id + "/conversations").AssertOK()
		linked.AssertJSONPath("data.0.conversation_id", conv).AssertJSONPath("data.0.email", "cy@example.com").
			AssertJSONPath("data.0.subject", "Need SAML").AssertJSONPath("data.0.linked_by_team", true)
		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id+"/votes").AssertOK().
			AssertJSONPath("data.0.priority", "must").AssertJSONPath("data.0.by_team", true)
	})

	t.Run("posts a new request from the conversation", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		conv := b.d.Customer(t, "dee", "Export please").String("data.id")

		res := b.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/feedback-posts", testkit.JSON{
			"title": "Export to CSV", "body_markdown": "Asked in a conversation.",
		}).AssertOK()

		res.AssertJSONPath("data.title", "Export to CSV").AssertJSONPath("data.vote_count", float64(1))
		assert.NotEmpty(t, res.String("data.board_id"))
		b.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/feedback-posts", testkit.JSON{}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "post_id_required")
	})

	t.Run("takes a link back and keeps the vote", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		conv := b.d.Customer(t, "eve", "Webhooks").String("data.id")
		id := b.post(t, testkit.JSON{"title": "Webhooks"})
		path := "/api/v1/conversations/" + conv + "/feedback-posts"
		b.d.Owner.PostJSON(path, testkit.JSON{"post_id": id}).AssertOK()

		b.d.Owner.Delete(path + "/" + id).AssertStatus(fhttp.StatusNoContent)

		b.d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.#", float64(0))
		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id).AssertOK().AssertJSONPath("data.vote_count", float64(1))
		b.d.Owner.Delete(path + "/" + id).AssertNotFound()
	})

	t.Run("tells the linked customers when the request ships", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		fake := b.d.TC.Mail()
		conv := b.d.Customer(t, "fay", "Zapier?").String("data.id")
		id := b.post(t, testkit.JSON{"title": "Zapier"})
		b.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/feedback-posts", testkit.JSON{"post_id": id}).AssertOK()

		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+id, testkit.JSON{"status": "shipped", "notify": true}).AssertOK()

		_, sent := tests.Sent[appmail.FeedbackStatusMail](b.d.TC, fake)
		require.Len(t, sent, 1)
		assert.Equal(t, "fay@example.com", sent[0].Message.To[0].Email)
	})

	t.Run("finds no conversation of another workspace", func(t *testing.T) {
		t.Parallel()
		b, other := newBoard(t, testkit.JSON{}), tests.NewDesk(t)
		conv := other.Customer(t, "gil", "Hi").String("data.id")
		id := b.post(t, testkit.JSON{"title": "SSO"})

		b.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/feedback-posts", testkit.JSON{"post_id": id}).
			AssertNotFound()
	})
}

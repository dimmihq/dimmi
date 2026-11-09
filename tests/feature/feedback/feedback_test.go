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

/*
board is a workspace's default help center with its board open.
*/
type board struct {
	d         tests.Desk
	id        string
	subdomain string
}

func newBoard(t *testing.T, settings testkit.JSON) board {
	t.Helper()
	d := tests.NewDesk(t)
	res := d.Owner.GetJSON("/api/v1/help-centers").AssertOK()
	b := board{d: d, id: res.String("data.0.id"), subdomain: res.String("data.0.subdomain")}
	settings["feedback_board"] = true
	d.Owner.PatchJSON("/api/v1/help-centers/"+b.id, testkit.JSON{"settings": settings}).AssertOK()
	return b
}

func (b board) post(t *testing.T, body testkit.JSON) string {
	t.Helper()
	return b.d.Owner.PostJSON("/api/v1/help-centers/"+b.id+"/feedback-posts", body).AssertCreated().String("data.id")
}

func (b board) page(path string) *testkit.Response {
	return b.d.TC.Client().WithHeader("Accept", "text/html").Send("GET", "/hc/"+b.subdomain+path, nil)
}

/*
reader signs a customer in to the help center with an emailed code and
returns their cookie.
*/
func (b board) reader(t *testing.T, email string) *fhttp.Cookie {
	t.Helper()
	fake := b.d.TC.Mail()
	b.form(nil, "/sso/codes", url.Values{"email": {email}}).AssertOK()
	mailed, _ := tests.LastSent[appmail.ReaderCodeMail](b.d.TC, fake)
	cookie := b.form(nil, "/sso/codes/check", url.Values{"email": {email}, "code": {mailed.Code}}).
		AssertStatus(fhttp.StatusSeeOther).Cookie("dimmi_help_reader")
	require.NotNil(t, cookie)
	return cookie
}

func (b board) form(cookie *fhttp.Cookie, path string, values url.Values) *testkit.Response {
	client := b.d.TC.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		WithHeader("Accept", "text/html")
	if cookie != nil {
		client = client.WithCookie(cookie)
	}
	return client.Send("POST", "/hc/"+b.subdomain+path, values.Encode())
}

func (b board) as(cookie *fhttp.Cookie, path string) *testkit.Response {
	return b.d.TC.Client().WithHeader("Accept", "text/html").WithCookie(cookie).
		Send("GET", "/hc/"+b.subdomain+path, nil)
}

func TestFeedbackPortal(t *testing.T) {
	t.Parallel()

	t.Run("keeps the board closed until the team opens it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		res := d.Owner.GetJSON("/api/v1/help-centers").AssertOK()
		subdomain := res.String("data.0.subdomain")

		d.TC.Client().WithHeader("Accept", "text/html").Send("GET", "/hc/"+subdomain+"/feedback", nil).
			AssertNotFound()
	})

	t.Run("lists requests by status and links the board from the header", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		id := b.post(t, testkit.JSON{"title": "Dark mode", "body_markdown": "Easier on the eyes."})
		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+id, testkit.JSON{"status": "planned"}).AssertOK()

		page := string(b.page("/feedback").AssertOK().Body)

		assert.Contains(t, page, "Dark mode")
		assert.Contains(t, page, `id="planned"`)
		assert.Contains(t, page, `href="/hc/`+b.subdomain+`/feedback"`)
		assert.Contains(t, string(b.page("/feedback/1-dark-mode").AssertOK().Body), "Easier on the eyes.")
	})

	t.Run("lets a signed-in customer post, vote once and comment", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		ana := b.reader(t, "ana@example.org")
		signIn := "/hc/" + b.subdomain + "/sso/sign-in?next=%2Fhc%2F" + b.subdomain + "%2Ffeedback%2Fnew"
		b.form(nil, "/feedback", url.Values{"title": {"Export to CSV"}}).
			AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", signIn)

		b.form(ana, "/feedback", url.Values{"title": {"Export to CSV"}, "details": {"Monthly reports."}}).
			AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", "/hc/"+b.subdomain+"/feedback/1-export-to-csv")
		bob := b.reader(t, "bob@example.org")
		b.form(bob, "/feedback/1-export-to-csv/votes", nil).AssertStatus(fhttp.StatusSeeOther)
		b.form(bob, "/feedback/1-export-to-csv/comments", url.Values{"body": {"We need this too"}}).
			AssertStatus(fhttp.StatusSeeOther)

		res := b.d.Owner.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-posts").AssertOK()
		res.AssertJSONPath("data.0.vote_count", 2).AssertJSONPath("data.0.comment_count", 1).
			AssertJSONPath("data.0.author", "ana@example.org")
		b.form(bob, "/feedback/1-export-to-csv/votes", nil).AssertStatus(fhttp.StatusSeeOther)
		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts").AssertOK().
			AssertJSONPath("data.0.vote_count", 1)
		assert.Contains(t, string(b.as(bob, "/feedback/1-export-to-csv").AssertOK().Body), "We need this too")
	})

	t.Run("shows similar requests before posting a new one", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.post(t, testkit.JSON{"title": "Export conversations to CSV"})
		ana := b.reader(t, "ana@example.org")

		res := b.form(ana, "/feedback", url.Values{"title": {"Export conversations as CSV"}}).
			AssertStatus(fhttp.StatusSeeOther)
		page := b.d.TC.Client().WithHeader("Accept", "text/html").WithCookie(ana).
			Send("GET", res.Header.Get("Location"), nil).AssertOK()
		assert.Contains(t, string(page.Body), "Is it one of these?")
		b.form(ana, "/feedback", url.Values{"title": {"Export conversations as CSV"}, "sure": {"1"}}).
			AssertStatus(fhttp.StatusSeeOther)

		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts").AssertOK().AssertJSONPath("meta.total", 2)
	})

	t.Run("holds customers' posts for approval when asked, shown only to their author", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{"feedback_approval": true})
		ana := b.reader(t, "ana@example.org")
		b.form(ana, "/feedback", url.Values{"title": {"Offline mode"}}).
			AssertHeader("Location", "/hc/"+b.subdomain+"/feedback?submitted=pending")

		assert.NotContains(t, string(b.page("/feedback").AssertOK().Body), "Offline mode")
		b.page("/feedback/1-offline-mode").AssertNotFound()
		assert.Contains(t, string(b.as(ana, "/feedback/1-offline-mode").AssertOK().Body), "Only you can see this")

		res := b.d.Owner.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-posts?moderation=pending").AssertOK()
		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+res.String("data.0.id"), testkit.JSON{"moderation": "approved"}).
			AssertOK()
		assert.Contains(t, string(b.page("/feedback").AssertOK().Body), "Offline mode")
	})
}

func TestFeedbackTeam(t *testing.T) {
	t.Parallel()

	t.Run("tells voters and the author once when a request ships, linking the update", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		fake := b.d.TC.Mail()
		ana := b.reader(t, "ana@example.org")
		b.form(ana, "/feedback", url.Values{"title": {"Zapier"}})
		bob := b.reader(t, "bob@example.org")
		b.form(bob, "/feedback/1-zapier/votes", nil)
		posts := b.d.Owner.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-posts").AssertOK()
		entry := b.d.Owner.PostJSON("/api/v1/help-centers/"+b.id+"/changelog-entries", testkit.JSON{"title": "Zapier"}).
			AssertCreated().String("data.id")
		b.d.Owner.PostJSON("/api/v1/changelog-entries/"+entry+"/publications", testkit.JSON{}).AssertOK()

		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+posts.String("data.0.id"), testkit.JSON{
			"status": "shipped", "notify": true, "message": "Live today.", "changelog_entry_id": entry,
		}).AssertOK().AssertJSONPath("data.status", "shipped")

		mails, sent := tests.Sent[appmail.FeedbackStatusMail](b.d.TC, fake)
		require.Len(t, mails, 2)
		to := []string{sent[0].Message.To[0].Email, sent[1].Message.To[0].Email}
		assert.ElementsMatch(t, []string{"ana@example.org", "bob@example.org"}, to)
		assert.Equal(t, "Live today.", mails[0].Moved.Message)
		assert.True(t, strings.HasSuffix(mails[0].Moved.Update, "/changelog/zapier"))
		assert.Contains(t, string(b.page("/feedback/1-zapier").AssertOK().Body), "/changelog/zapier")
	})

	t.Run("votes for a customer, with how much it matters, and counts each customer once", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		contact := b.d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "cy@example.org", "name": "Cy"}).
			AssertCreated().String("data.id")
		id := b.post(t, testkit.JSON{"title": "SAML", "contact_id": contact})
		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id).AssertOK().AssertJSONPath("data.author", "Cy")

		b.d.Owner.PostJSON("/api/v1/feedback-posts/"+id+"/votes", testkit.JSON{
			"contact_id": contact, "priority": "must",
		}).AssertOK().AssertJSONPath("data.vote_count", 1)

		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id+"/votes").AssertOK().
			AssertJSONPath("data.0.priority", "must").AssertJSONPath("data.0.by_team", true).
			AssertJSONPath("data.0.name", "Cy")
		b.d.Owner.Delete("/api/v1/feedback-posts/"+id+"/votes/"+contact).AssertOK().
			AssertJSONPath("data.vote_count", 0)
	})

	t.Run("merges a duplicate, carrying its votes once and sending its readers on", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		ana := b.reader(t, "ana@example.org")
		b.form(ana, "/feedback", url.Values{"title": {"Slack app"}})
		b.form(ana, "/feedback", url.Values{"title": {"Slack integration please"}, "sure": {"1"}})
		list := b.d.Owner.GetJSON("/api/v1/help-centers/" + b.id + "/feedback-posts?sort=new").AssertOK()
		dup, kept := list.String("data.0.id"), list.String("data.1.id")

		b.d.Owner.PatchJSON("/api/v1/feedback-posts/"+dup, testkit.JSON{"merge_into_id": kept}).AssertOK().
			AssertJSONPath("data.vote_count", 1)

		b.page("/feedback/2-slack-integration-please").AssertStatus(fhttp.StatusMovedPermanently).
			AssertHeader("Location", "/hc/"+b.subdomain+"/feedback/1-slack-app")
		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts").AssertOK().AssertJSONPath("meta.total", 1)
	})

	t.Run("keeps internal notes from customers", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		id := b.post(t, testkit.JSON{"title": "API rate limits"})
		b.d.Owner.PostJSON("/api/v1/feedback-posts/"+id+"/comments", testkit.JSON{
			"body": "Blocked on infra", "internal": true,
		}).AssertCreated().AssertJSONPath("data.internal", true).AssertJSONPath("data.from_team", true)
		b.d.Owner.PostJSON("/api/v1/feedback-posts/"+id+"/comments", testkit.JSON{"body": "Looking into it"}).
			AssertCreated()

		page := string(b.page("/feedback/1-api-rate-limits").AssertOK().Body)

		assert.Contains(t, page, "Looking into it")
		assert.NotContains(t, page, "Blocked on infra")
		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id+"/comments").AssertOK().AssertJSONPath("data.#", 2)
		b.d.Owner.GetJSON("/api/v1/feedback-posts/"+id).AssertOK().AssertJSONPath("data.comment_count", 1)
	})

	t.Run("sorts by top and finds requests by title", func(t *testing.T) {
		t.Parallel()
		b := newBoard(t, testkit.JSON{})
		b.post(t, testkit.JSON{"title": "Quiet one"})
		popular := b.post(t, testkit.JSON{"title": "Popular one"})
		contact := b.d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{"email": "dee@example.org"}).AssertCreated().
			String("data.id")
		b.d.Owner.PostJSON("/api/v1/feedback-posts/"+popular+"/votes", testkit.JSON{"contact_id": contact}).AssertOK()

		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts?sort=top").AssertOK().
			AssertJSONPath("data.0.title", "Popular one").AssertJSONPath("counts.under_review", 2)
		b.d.Owner.GetJSON("/api/v1/help-centers/"+b.id+"/feedback-posts?query=quiet").AssertOK().
			AssertJSONPath("data.#", 1)
		assert.Contains(t, string(b.page("/feedback?sort=top&q=popular").AssertOK().Body), "Popular one")
	})
}

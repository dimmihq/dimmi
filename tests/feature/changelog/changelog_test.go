package changelog_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
log is a workspace's default help center with its owner signed in.
*/
type log struct {
	d         tests.Desk
	id        string
	subdomain string
}

/*
newLog starts a workspace and finds its help center.
*/
func newLog(t *testing.T) log {
	t.Helper()
	d := tests.NewDesk(t)
	res := d.Owner.GetJSON("/api/v1/help-centers").AssertOK()
	return log{d: d, id: res.String("data.0.id"), subdomain: res.String("data.0.subdomain")}
}

/*
entry creates a draft and returns its id.
*/
func (l log) entry(t *testing.T, body testkit.JSON) string {
	t.Helper()
	return l.d.Owner.PostJSON("/api/v1/help-centers/"+l.id+"/changelog-entries", body).AssertCreated().
		String("data.id")
}

/*
published creates an entry and publishes it, returning its id.
*/
func (l log) published(t *testing.T, body testkit.JSON) string {
	t.Helper()
	id := l.entry(t, body)
	l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{}).AssertOK()
	return id
}

/*
page opens a public page of the changelog.
*/
func (l log) page(path string) *testkit.Response {
	return l.d.TC.Client().WithHeader("Accept", "text/html").Send("GET", "/hc/"+l.subdomain+path, nil)
}

/*
form posts a form of the public changelog.
*/
func (l log) form(path string, values url.Values) *testkit.Response {
	return l.d.TC.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		WithHeader("Accept", "text/html").Send("POST", "/hc/"+l.subdomain+path, values.Encode())
}

func TestChangelogEntries(t *testing.T) {
	t.Parallel()

	t.Run("starts a changelog with the default labels", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)

		res := l.d.Owner.GetJSON("/api/v1/help-centers/" + l.id + "/changelog-labels").AssertOK()

		res.AssertJSONPath("data.0.name", "New").AssertJSONPath("data.1.name", "Improved").
			AssertJSONPath("data.2.name", "Fixed")
	})

	t.Run("writes a draft with a cover, labels and the writer as author", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		labels := l.d.Owner.GetJSON("/api/v1/help-centers/" + l.id + "/changelog-labels").AssertOK()

		id := l.entry(t, testkit.JSON{
			"title": "Shared inboxes", "summary": "One inbox per team.", "body_markdown": "## Why\n\nTeams grow.",
			"cover_url": "/help-images/cover.png", "label_ids": []string{labels.String("data.0.id")},
		})

		res := l.d.Owner.GetJSON("/api/v1/changelog-entries/" + id).AssertOK()
		res.AssertJSONPath("data.slug", "shared-inboxes").AssertJSONPath("data.status", "draft").
			AssertJSONPath("data.labels.0.name", "New").AssertJSONPath("data.cover_url", "/help-images/cover.png").
			AssertJSONPath("data.authors.0.name", l.d.TC.Owner.Name).AssertJSONPath("data.authors_chosen", false)
	})

	t.Run("refuses a cover from anywhere but the help center's pictures", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)

		l.d.Owner.PostJSON("/api/v1/help-centers/"+l.id+"/changelog-entries", testkit.JSON{
			"title": "Tracking", "cover_url": "https://evil.example/pixel.gif",
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "cover_invalid")
	})

	t.Run("names editors as authors until authors are picked, and again once cleared", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		tc := l.d.TC
		ana := tc.CreateUser()
		member, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: ana, Role: enums.RoleAdmin,
		})
		require.NoError(t, err)
		id := l.entry(t, testkit.JSON{"title": "Faster search"})
		tc.In(ana).PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"body_markdown": "Twice as fast."}).
			AssertOK().AssertJSONPath("data.authors.#", 2).AssertJSONPath("data.authors.1.name", ana.Name)

		picked := models.PublicID(models.PrefixMember, member.ID)
		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"author_ids": []string{picked}}).AssertOK().
			AssertJSONPath("data.authors.#", 1).AssertJSONPath("data.authors.0.name", ana.Name).
			AssertJSONPath("data.authors_chosen", true).AssertJSONPath("data.editors.#", 2)

		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"author_ids": []string{}}).AssertOK().
			AssertJSONPath("data.authors.#", 2).AssertJSONPath("data.authors_chosen", false)
	})

	t.Run("refuses a save from an editor who started before someone else saved", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		id := l.entry(t, testkit.JSON{"title": "Exports"})
		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"title": "CSV exports", "lock_version": 1}).
			AssertOK()

		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"title": "Old", "lock_version": 1}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "changelog_entry_changed")
	})

	t.Run("moves a draft's address with its title until it is published or typed", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		id := l.entry(t, testkit.JSON{"title": "Untitled update"})

		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"title": "Dark mode"}).AssertOK().
			AssertJSONPath("data.slug", "dark-mode")
		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{}).AssertOK()
		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+id, testkit.JSON{"title": "Dark mode, everywhere"}).
			AssertOK().AssertJSONPath("data.slug", "dark-mode")
	})

	t.Run("gives a second entry with the same title a free slug", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.entry(t, testkit.JSON{"title": "Bug fixes"})

		id := l.entry(t, testkit.JSON{"title": "Bug fixes"})

		l.d.Owner.GetJSON("/api/v1/changelog-entries/"+id).AssertOK().AssertJSONPath("data.slug", "bug-fixes-2")
	})

	t.Run("lists a page of entries narrowed by state with counts of each", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.published(t, testkit.JSON{"title": "Live one"})
		l.entry(t, testkit.JSON{"title": "Draft one"})

		res := l.d.Owner.GetJSON("/api/v1/help-centers/" + l.id + "/changelog-entries?status=published").AssertOK()

		res.AssertJSONPath("data.#", 1).AssertJSONPath("data.0.title", "Live one").
			AssertJSONPath("counts.published", 1).AssertJSONPath("counts.draft", 1).AssertJSONPath("meta.total", 1)
	})

	t.Run("keeps labels to twenty and their entries when one goes", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		labels := l.d.Owner.GetJSON("/api/v1/help-centers/" + l.id + "/changelog-labels").AssertOK()
		id := l.entry(t, testkit.JSON{"title": "Filed", "label_ids": []string{labels.String("data.0.id")}})
		for i := range 17 {
			l.d.Owner.PostJSON("/api/v1/help-centers/"+l.id+"/changelog-labels", testkit.JSON{
				"name": "Label " + string(rune('a'+i)), "color": "#123456",
			}).AssertCreated()
		}

		l.d.Owner.PostJSON("/api/v1/help-centers/"+l.id+"/changelog-labels", testkit.JSON{"name": "One more"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "too_many_labels")
		l.d.Owner.Delete("/api/v1/changelog-labels/" + labels.String("data.0.id")).AssertNoContent()
		l.d.Owner.GetJSON("/api/v1/changelog-entries/"+id).AssertOK().AssertJSONPath("data.labels.#", 0)
	})

	t.Run("keeps a token without the changelog abilities out", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		reader := l.d.TC.ActingAsToken(l.d.TC.Workspace.Owner, "articles:read")

		reader.GetJSON("/api/v1/help-centers/" + l.id + "/changelog-entries").AssertStatus(fhttp.StatusForbidden)
	})
}

func TestChangelogPublishing(t *testing.T) {
	t.Parallel()

	t.Run("colors code in entries, also ones published before highlighting", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.published(t, testkit.JSON{"title": "SDK release", "body_markdown": "```js\nconst a = 1\n```\n"})
		_, err := l.d.TC.Operator().Exec(t.Context(),
			`UPDATE changelog_entries SET published_renderer_version = 1, published_body_html = '<p>plain</p>'
			 WHERE workspace_id = $1`, l.d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)

		page := string(l.page("/changelog/sdk-release").Body)
		assert.Contains(t, page, `class="chroma-kr"`)
		assert.NotContains(t, page, "<p>plain</p>")
	})

	t.Run("shows only published entries as last published, newest first", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		old := l.published(t, testkit.JSON{"title": "Older news", "body_markdown": "First."})
		l.d.TC.Clock.Travel(time.Hour)
		l.published(t, testkit.JSON{"title": "Newer news", "body_markdown": "Second."})
		l.entry(t, testkit.JSON{"title": "Secret plans"})
		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+old, testkit.JSON{"body_markdown": "Unsaid change."}).
			AssertOK().AssertJSONPath("data.has_unpublished_changes", true)

		page := string(l.page("/changelog").AssertOK().Body)

		assert.Less(t, strings.Index(page, "Newer news"), strings.Index(page, "Older news"))
		assert.NotContains(t, page, "Secret plans")
		assert.NotContains(t, page, "Unsaid change")
		l.page("/changelog/secret-plans").AssertStatus(fhttp.StatusNotFound)
	})

	t.Run("links the changelog from the help center once it has an entry", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.page("/changelog").AssertStatus(fhttp.StatusNotFound)
		assert.NotContains(t, string(l.page("").AssertOK().Body), `href="/hc/`+l.subdomain+`/changelog"`)

		l.published(t, testkit.JSON{"title": "Hello world"})

		home := string(l.page("").AssertOK().Body)
		assert.Contains(t, home, `href="/hc/`+l.subdomain+`/changelog"`)
		assert.Contains(t, home, "Hello world")
	})

	t.Run("filters by label and feeds the entries as RSS and Markdown", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		labels := l.d.Owner.GetJSON("/api/v1/help-centers/" + l.id + "/changelog-labels").AssertOK()
		l.published(t, testkit.JSON{
			"title": "Brand new", "body_markdown": "![Plans](/help-images/p.png#width=60)", "summary": "So new.",
			"label_ids": []string{labels.String("data.0.id")},
		})
		l.published(t, testkit.JSON{"title": "A fix", "label_ids": []string{labels.String("data.2.id")}})

		filtered := string(l.page("/changelog/labels/new").AssertOK().Body)
		feed := string(l.page("/changelog/feed.xml").AssertOK().Body)
		twin := string(l.page("/changelog/brand-new.md").AssertOK().Body)

		assert.Contains(t, filtered, "Brand new")
		assert.NotContains(t, filtered, "A fix")
		l.page("/changelog/labels/nope").AssertStatus(fhttp.StatusNotFound)
		assert.Contains(t, feed, "<rss")
		assert.Contains(t, feed, "/changelog/brand-new")
		assert.Contains(t, feed, "image-width-60")
		assert.Contains(t, twin, "# Brand new")
	})

	t.Run("publishes a scheduled entry when its time comes", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		id := l.entry(t, testkit.JSON{"title": "Coming soon"})
		at := l.d.TC.Clock.Now().Add(2 * time.Hour)
		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{"scheduled_for": at}).
			AssertOK().AssertJSONPath("data.status", "scheduled")
		l.page("/changelog/coming-soon").AssertStatus(fhttp.StatusNotFound)

		l.d.TC.Clock.Travel(3 * time.Hour)
		l.d.TC.DrainLater()

		l.d.Owner.GetJSON("/api/v1/changelog-entries/"+id).AssertOK().AssertJSONPath("data.status", "published")
		l.page("/changelog/coming-soon").AssertOK()
	})

	t.Run("refuses to schedule in the past", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		id := l.entry(t, testkit.JSON{"title": "Too late"})

		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{
			"scheduled_for": l.d.TC.Clock.Now().Add(-time.Minute),
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "schedule_in_past")
	})

	t.Run("takes an unpublished entry off the site", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		id := l.published(t, testkit.JSON{"title": "Oops"})
		l.page("/changelog/oops").AssertOK()

		l.d.Owner.Delete("/api/v1/changelog-entries/"+id+"/publications").AssertOK().
			AssertJSONPath("data.status", "draft")

		l.page("/changelog/oops").AssertStatus(fhttp.StatusNotFound)
	})

	t.Run("counts each reader's reaction once", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.published(t, testkit.JSON{"title": "Party"})

		l.form("/changelog/party/reactions", url.Values{"emoji": {"🎉"}}).AssertStatus(fhttp.StatusSeeOther)
		l.form("/changelog/party/reactions", url.Values{"emoji": {"🎉"}}).AssertStatus(fhttp.StatusSeeOther)

		l.d.TC.Table("changelog_reactions").AssertCount(1)
	})
}

func TestChangelogSubscribers(t *testing.T) {
	t.Parallel()

	t.Run("subscribes an address only once its owner confirms", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		fake := l.d.TC.Mail()
		l.form("/changelog/subscriptions", url.Values{"email": {"Bob@Example.org"}}).
			AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", "/hc/"+l.subdomain+"/changelog?subscribed#main")
		confirm, sent := tests.LastSent[appmail.ChangelogConfirmMail](l.d.TC, fake)
		assert.Equal(t, "bob@example.org", sent.Message.To[0].Email)
		path := strings.TrimPrefix(confirm.Link, l.d.TC.App.Application.Config.App.URL)

		opened := l.d.TC.Client().WithHeader("Accept", "text/html").Send("GET", path, nil).AssertOK()
		assert.Contains(t, string(opened.Body), "Confirm subscription")
		l.d.TC.Table("changelog_subscribers").AssertCount(1)
		l.d.Owner.GetJSON("/api/v1/help-centers/"+l.id+"/changelog-subscribers?status=pending").AssertOK().
			AssertJSONPath("data.#", 1)

		done := l.d.TC.Client().WithHeader("Accept", "text/html").Send("POST", path, nil).AssertOK()
		assert.Contains(t, string(done.Body), "subscribed")
		l.d.Owner.GetJSON("/api/v1/help-centers/"+l.id+"/changelog-subscribers?status=active").AssertOK().
			AssertJSONPath("data.0.email", "bob@example.org")
	})

	t.Run("answers the same and mails nothing for an address already active", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		fake := l.d.TC.Mail()
		l.form("/changelog/subscriptions", url.Values{"email": {"ana@example.org"}})
		confirm, _ := tests.LastSent[appmail.ChangelogConfirmMail](l.d.TC, fake)
		path := strings.TrimPrefix(confirm.Link, l.d.TC.App.Application.Config.App.URL)
		l.d.TC.Client().Send("POST", path, nil).AssertOK()

		l.form("/changelog/subscriptions", url.Values{"email": {"ana@example.org"}}).
			AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", "/hc/"+l.subdomain+"/changelog?subscribed#main")

		mails, _ := tests.Sent[appmail.ChangelogConfirmMail](l.d.TC, fake)
		assert.Len(t, mails, 1)
	})

	t.Run("refuses a link whose token was changed", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		fake := l.d.TC.Mail()
		l.form("/changelog/subscriptions", url.Values{"email": {"eve@example.org"}})
		confirm, _ := tests.LastSent[appmail.ChangelogConfirmMail](l.d.TC, fake)
		path := strings.TrimPrefix(confirm.Link, l.d.TC.App.Application.Config.App.URL)
		forged := path[:len(path)-4] + "AAAA"

		page := l.d.TC.Client().WithHeader("Accept", "text/html").Send("POST", forged, nil).AssertOK()

		assert.Contains(t, string(page.Body), "no longer works")
		l.d.Owner.GetJSON("/api/v1/help-centers/"+l.id+"/changelog-subscribers?status=active").AssertOK().
			AssertJSONPath("data.#", 0)
	})

	t.Run("mails a published entry to active subscribers once, with one-click unsubscribe", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		fake := l.d.TC.Mail()
		l.form("/changelog/subscriptions", url.Values{"email": {"sam@example.org"}})
		l.form("/changelog/subscriptions", url.Values{"email": {"pending@example.org"}})
		confirms, sentConfirms := tests.Sent[appmail.ChangelogConfirmMail](l.d.TC, fake)
		for i, m := range confirms {
			if sentConfirms[i].Message.To[0].Email == "sam@example.org" {
				path := strings.TrimPrefix(m.Link, l.d.TC.App.Application.Config.App.URL)
				l.d.TC.Client().Send("POST", path, nil).AssertOK()
			}
		}
		id := l.entry(t, testkit.JSON{"title": "Big launch", "body_markdown": "![x](/help-images/a.png)"})

		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{"notify": true}).AssertOK()
		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{"notify": true}).AssertOK()

		mails, sent := tests.Sent[appmail.ChangelogEntryMail](l.d.TC, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "sam@example.org", sent[0].Message.To[0].Email)
		assert.Equal(t, "Big launch", sent[0].Message.Subject)
		assert.Equal(t, "List-Unsubscribe=One-Click", sent[0].Message.Headers["List-Unsubscribe-Post"])
		assert.Contains(t, mails[0].Entry.BodyHTML, `src="http`)

		unsub := strings.TrimPrefix(mails[0].Entry.Unsubscribe, l.d.TC.App.Application.Config.App.URL)
		l.d.TC.Client().Send("POST", unsub, nil).AssertOK()
		l.d.Owner.GetJSON("/api/v1/help-centers/"+l.id+"/changelog-subscribers?status=unsubscribed").AssertOK().
			AssertJSONPath("data.0.email", "sam@example.org")
		l.d.TC.Client().Send("POST", unsub, nil).AssertOK()
	})

	t.Run("publishes without mail when nobody asked to notify", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		fake := l.d.TC.Mail()

		l.published(t, testkit.JSON{"title": "Quiet fix"})

		mails, _ := tests.Sent[appmail.ChangelogEntryMail](l.d.TC, fake)
		assert.Empty(t, mails)
	})
}

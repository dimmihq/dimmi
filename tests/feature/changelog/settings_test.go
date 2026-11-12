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

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

/*
set changes the help center's changelog settings.
*/
func (l log) set(t *testing.T, settings testkit.JSON) *testkit.Response {
	t.Helper()
	return l.d.Owner.PatchJSON("/api/v1/help-centers/"+l.id, testkit.JSON{"settings": settings})
}

/*
subscriber subscribes the address and confirms it.
*/
func (l log) subscriber(t *testing.T, email string) {
	t.Helper()
	fake := l.d.TC.Mail()
	l.form("/changelog/subscriptions", url.Values{"email": {email}})
	confirm, _ := tests.LastSent[appmail.ChangelogConfirmMail](l.d.TC, fake)
	l.d.TC.Client().Send("POST", strings.TrimPrefix(confirm.Link, l.d.TC.App.Application.Config.App.URL), nil).
		AssertOK()
}

func TestChangelogSettings(t *testing.T) {
	t.Parallel()

	t.Run("keeps every switch on until the team turns it off", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)

		res := l.d.Owner.GetJSON("/api/v1/help-centers/" + l.id).AssertOK()

		assert.Empty(t, res.String("data.settings.changelog"))
		l.published(t, testkit.JSON{"title": "Hello world"})
		page := string(l.page("/changelog").AssertOK().Body)
		assert.Contains(t, page, `action="/hc/`+l.subdomain+`/changelog/subscriptions"`)
		assert.Contains(t, page, "data-reactions")
	})

	t.Run("takes the changelog off the help center, its header and the messenger", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.published(t, testkit.JSON{"title": "Hello world"})
		widget := l.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		l.d.Owner.PatchJSON("/api/v1/widget-settings/"+widget, testkit.JSON{"changelog_tab_enabled": true}).AssertOK()

		l.set(t, testkit.JSON{"changelog": false}).AssertOK().AssertJSONPath("data.settings.changelog", false)

		l.page("/changelog").AssertNotFound()
		l.page("/changelog/hello-world").AssertNotFound()
		l.page("/changelog/feed.xml").AssertNotFound()
		assert.NotContains(t, string(l.page("").AssertOK().Body), `href="/hc/`+l.subdomain+`/changelog"`)
		l.d.TC.Client().GetJSON("/widget/v1/apps/"+widget+"/changelog").AssertOK().
			AssertJSONPath("data.entries.#", float64(0))
	})

	t.Run("titles the changelog and hides authors, reactions and subscribing when asked", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.published(t, testkit.JSON{"title": "Hello world"})

		l.set(t, testkit.JSON{
			"changelog_title": "What's new", "changelog_authors": false, "changelog_reactions": false,
			"changelog_subscriptions": false,
		}).AssertOK()

		page := string(l.page("/changelog").AssertOK().Body)
		assert.Contains(t, page, "What&#39;s new")
		assert.NotContains(t, page, "data-reactions")
		assert.NotContains(t, page, "/changelog/subscriptions")
		entry := string(l.page("/changelog/hello-world").AssertOK().Body)
		assert.NotContains(t, entry, "data-reactions")
		assert.NotContains(t, entry, "Get updates in your inbox")
		l.form("/changelog/hello-world/reactions", url.Values{"emoji": {"🎉"}}).AssertNotFound()
		l.form("/changelog/subscriptions", url.Values{"email": {"ana@example.org"}}).AssertNotFound()
		l.d.TC.Table("changelog_subscribers").AssertCount(0)
	})

	t.Run("refuses an email kind or undo window it does not offer", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)

		l.set(t, testkit.JSON{"changelog_email": "everything"}).AssertStatus(fhttp.StatusUnprocessableEntity)
		l.set(t, testkit.JSON{"changelog_undo_minutes": 7}).AssertStatus(fhttp.StatusUnprocessableEntity)
		l.set(t, testkit.JSON{"changelog_email": "summary", "changelog_undo_minutes": 15}).AssertOK().
			AssertJSONPath("data.settings.changelog_email", "summary").
			AssertJSONPath("data.settings.changelog_undo_minutes", float64(15))
	})

	t.Run("mails only the summary when asked", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.subscriber(t, "sam@example.org")
		l.set(t, testkit.JSON{"changelog_email": "summary"}).AssertOK()
		fake := l.d.TC.Mail()
		id := l.entry(t, testkit.JSON{"title": "Big launch", "summary": "In short.", "body_markdown": "Long story."})

		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+id+"/publications", testkit.JSON{"notify": true}).AssertOK()

		mails, _ := tests.Sent[appmail.ChangelogEntryMail](l.d.TC, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "In short.", mails[0].Entry.Summary)
		assert.Empty(t, mails[0].Entry.BodyHTML)
	})

	t.Run("waits out the undo window and mails nothing for an entry taken back", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		l.subscriber(t, "sam@example.org")
		l.set(t, testkit.JSON{"changelog_undo_minutes": 5}).AssertOK()
		fake := l.d.TC.Mail()
		kept := l.entry(t, testkit.JSON{"title": "Kept"})
		taken := l.entry(t, testkit.JSON{"title": "Taken back"})

		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+kept+"/publications", testkit.JSON{"notify": true}).AssertOK()
		l.d.Owner.PostJSON("/api/v1/changelog-entries/"+taken+"/publications", testkit.JSON{"notify": true}).AssertOK()
		l.d.TC.Drain()
		mails, _ := tests.Sent[appmail.ChangelogEntryMail](l.d.TC, fake)
		assert.Empty(t, mails)

		l.d.Owner.Delete("/api/v1/changelog-entries/" + taken + "/publications").AssertOK()
		l.d.TC.Clock.Travel(6 * time.Minute)
		l.d.TC.DrainLater()

		mails, _ = tests.Sent[appmail.ChangelogEntryMail](l.d.TC, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "Kept", mails[0].Entry.Title)
	})
}

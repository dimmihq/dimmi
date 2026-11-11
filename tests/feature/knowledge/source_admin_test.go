package knowledge_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSourceAdministration(t *testing.T) {
	t.Parallel()

	t.Run("members may sync but not see or change sources, and tokens need the settings abilities", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, nil)

		user := x.tc.CreateUser()
		_, err := factories.Member(t.Context(), x.tc.App, factories.Membership{
			Workspace: x.tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		as := x.tc.ActingAs(user)
		as.GetJSON("/api/v1/knowledge/sources").AssertForbidden()
		as.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)
		as.Delete("/api/v1/knowledge/sources/" + source).AssertForbidden()

		reader := x.tc.ActingAsToken(x.tc.Workspace.Owner, "settings:read")
		reader.GetJSON("/api/v1/knowledge/sources").AssertOK().AssertJSONPath("data.#", 1)
		reader.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertForbidden()
	})

	t.Run("a connected source keeps only its installation, never a token", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveIntercom(t, x)
		source := addIntercom(t, x)

		shown := x.owner.GetJSON("/api/v1/knowledge/sources/" + source).AssertOK()
		assert.Contains(t, shown.String("data.settings.installation"), "itg_")
		assert.NotContains(t, string(shown.Body), fakeKey())
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_sources WHERE config::text LIKE '%kkkk%'"))
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_sources WHERE credentials_encrypted IS NOT NULL"))

		x.owner.PatchJSON("/api/v1/knowledge/sources/"+source, testkit.JSON{"name": "Old help center"}).
			AssertOK().AssertJSONPath("data.name", "Old help center")
	})

	t.Run("a refused key fails the sync and tells the admins once", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		fake := serveIntercom(t, x)
		source := addIntercom(t, x)
		settle(t, x, source)
		x.tc.DrainLater()

		mail := x.tc.Mail()
		fake.Stub("GET "+intercomAPI+"/articles?page=1&per_page=50", httpclient.Response{Status: 401})

		for range 2 {
			x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)
			settle(t, x, source).AssertJSONPath("data.status", "error").
				AssertJSONPath("data.last_error_code", "auth_failed").AssertJSONPath("data.last_run.status", "failed")
		}

		sent, _ := tests.Sent[appmail.KnowledgeSourceFailingMail](x.tc, mail)
		require.Len(t, sent, 1, "admins hear once until the source works again")
		assert.Equal(t, "Intercom", sent[0].Source)
		assert.True(t, strings.HasSuffix(sent[0].SettingsURL, "/settings/knowledge/"+source))
	})

	t.Run("a wrong setting is refused before the source is saved", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "website", "name": "Bad", "settings": testkit.JSON{"start_url": "docs"},
		}).AssertValidationError("settings.start_url")
		x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "no_such_source", "name": "Bad",
		}).AssertValidationError("type")
		x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "website", "name": "Bad", "mode": "import", "settings": testkit.JSON{"start_url": site},
		}).AssertValidationError("mode")
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_sources"))
	})

	t.Run("the scheduler starts the syncs whose time came", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, testkit.JSON{"schedule": "daily"})
		settle(t, x, source)

		x.tc.Clock.Advance(25 * time.Hour)
		require.NoError(t, x.tc.App.Queue.Push(t.Context(), jobs.ScheduleKnowledgeSyncsArgs{}))
		settle(t, x, source)

		runs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/syncs").AssertOK()
		runs.AssertJSONPath("data.#", 2).AssertJSONPath("data.0.trigger", "schedule")
	})

	t.Run("a sync can be canceled and a single document synced again", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, nil)
		run := x.owner.GetJSON("/api/v1/knowledge/sources/" + source).String("data.last_run.id")

		x.owner.Delete("/api/v1/knowledge/sources/"+source+"/syncs/"+run).AssertOK().
			AssertJSONPath("data.status", "canceled")
		x.tc.DrainLater()
		x.owner.GetJSON("/api/v1/knowledge/sources/"+source).AssertJSONPath("data.status", "idle")

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)
		settle(t, x, source)
		doc := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents?status=active").String("data.0.id")
		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/documents/"+doc+"/syncs", nil).AssertStatus(202).
			AssertJSONPath("data.full_run", false)
		settle(t, x, source).AssertJSONPath("data.document_count", 3)
	})

	t.Run("an uploaded zip of markdown becomes documents", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		key := x.owner.PostForm("/api/v1/knowledge/uploads", testkit.FormFile{
			Field: "file", Name: "docs.zip", Contents: markdownZip(t),
		}, nil).AssertCreated().String("data.key")

		source := x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "markdown_upload", "name": "Handbook", "settings": testkit.JSON{"upload": key}, "sync_now": true,
		}).AssertCreated().String("data.id")
		settle(t, x, source).AssertJSONPath("data.document_count", 2)

		found := x.search(t, "parental leave weeks", "keyword")
		require.NotEmpty(t, found.Chunks)
		assert.Equal(t, "Parental leave", found.Chunks[0].Title)
		assert.Empty(t, found.Chunks[0].URL, "an internal document is never linked")

		x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "markdown_upload", "name": "Other", "settings": testkit.JSON{"upload": "knowledge-uploads/x/y.zip"},
		}).AssertValidationError("file")
	})
}

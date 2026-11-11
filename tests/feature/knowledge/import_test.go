package knowledge_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
intercomAPI is the Intercom API the import reads.
*/
const intercomAPI = "https://api.intercom.io"

/*
serveIntercom answers an Intercom help center of two articles in two
nested collections, one with a French translation, and a picture.
*/
func serveIntercom(t *testing.T, x index) *httpclienttest.Fake {
	t.Helper()

	refunds := map[string]any{
		"id": "101", "title": "Refund policy", "description": "When refunds are paid", "state": "published",
		"url": "https://help.hivy.test/en/articles/101-refund-policy", "parent_id": 11, "parent_type": "collection",
		"default_locale": "en", "updated_at": 1730000000,
		"body": `<p>Refunds are paid within <b>fourteen days</b>.</p><img src="https://downloads.intercomcdn.test/r.png" alt="Refund screen">`,
		"translated_content": map[string]any{
			"type": "group_translated_content",
			"fr": map[string]any{
				"title": "Politique de remboursement",
				"body":  "<p>Les remboursements sont versés sous quatorze jours.</p>",
				"state": "published", "url": "https://help.hivy.test/fr/articles/101-remboursement",
				"updated_at": 1730000000,
			},
		},
	}
	draft := map[string]any{
		"id": "102", "title": "Upcoming plans", "state": "draft", "url": nil, "parent_id": 12,
		"parent_type": "collection", "default_locale": "en", "updated_at": 1730000001,
		"body": "<p>New plans arrive next quarter.</p>",
	}

	fake := httpclienttest.New(x.tc.App.HTTP).PreventStrayRequests()
	me := map[string]any{"app": map[string]any{"name": "Hivy", "id_code": "hivy1"}}
	fake.Stub("GET "+intercomAPI+"/me", jsonReply(me))
	fake.Stub("GET "+intercomAPI+"/articles?per_page=1", jsonReply(map[string]any{"data": []any{}, "total_count": 2}))
	fake.Stub("GET "+intercomAPI+"/articles?page=1&per_page=50", jsonReply(map[string]any{
		"data": []any{refunds, draft}, "total_count": 2, "pages": map[string]any{"page": 1, "total_pages": 1},
	}))
	fake.Stub("GET "+intercomAPI+"/articles/101", jsonReply(refunds))
	fake.Stub("GET "+intercomAPI+"/articles/102", jsonReply(draft))
	fake.Stub("GET "+intercomAPI+"/help_center/collections/11", jsonReply(map[string]any{
		"id": "11", "name": "Payments", "url": "https://help.hivy.test/en/collections/11-payments", "parent_id": 10,
	}))
	fake.Stub("GET "+intercomAPI+"/help_center/collections/10", jsonReply(map[string]any{
		"id": "10", "name": "Billing", "url": "https://help.hivy.test/en/collections/10-billing", "parent_id": nil,
	}))
	fake.Stub("GET "+intercomAPI+"/help_center/collections/12", jsonReply(map[string]any{
		"id": "12", "name": "Roadmap", "parent_id": nil,
	}))
	fake.Stub("GET https://downloads.intercomcdn.test/r.png", httpclient.Response{
		Status: 200, Body: tinyPNG(), Header: textHeader("image/png"),
	})

	return fake
}

func jsonReply(v any) httpclient.Response {
	body, _ := json.Marshal(v)

	return httpclient.Response{Status: 200, Body: body, Header: textHeader("application/json")}
}

/*
tinyPNG is a small picture.
*/
func tinyPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = 200
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)

	return buf.Bytes()
}

/*
connectIntercom connects the workspace's Intercom account, as a token
pasted by an owner, and returns the installation's public id.
*/
func connectIntercom(t *testing.T, x index) string {
	t.Helper()

	ws := x.tc.Workspace
	owner := authz.NewMembership(ws.Workspace, ws.Owner, auth.RolePermissions(enums.RoleOwner))
	row, err := x.tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), integrations.ManualInstallationInput{
		Membership: owner, Provider: "intercom", Token: fakeKey(),
	})
	require.NoError(t, err)

	return models.PublicID(models.PrefixIntegrationInstallation, row.ID)
}

/*
fakeKey is the access token the faked Intercom accepts.
*/
func fakeKey() string {
	return strings.Repeat("k", 24)
}

/*
addIntercom adds an Intercom import into the workspace's help center,
through its connected account.
*/
func addIntercom(t *testing.T, x index) string {
	t.Helper()

	return x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
		"type": "intercom_help_center", "name": "Intercom", "mode": "import", "help_center_id": x.center,
		"settings": testkit.JSON{"installation": connectIntercom(t, x), "region": "us"}, "sync_now": true,
	}).AssertCreated().String("data.id")
}

func TestIntercomImport(t *testing.T) {
	t.Parallel()

	t.Run("an import writes articles in their collections and redirects their old addresses", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveIntercom(t, x)
		source := addIntercom(t, x)
		settle(t, x, source).AssertJSONPath("data.status", "idle").AssertJSONPath("data.last_run.status", "succeeded")
		x.tc.DrainLater()

		articles := x.owner.GetJSON("/api/v1/help-centers/" + x.center + "/articles").AssertOK()
		assert.ElementsMatch(t, []string{"Refund policy", "Upcoming plans"}, stringsOf(t, articles, "data.#.title"))

		refunds := articleNamed(t, articles, "Refund policy")
		assert.Equal(t, "published", refunds["status"])
		article := x.owner.GetJSON("/api/v1/articles/" + refunds["id"]).AssertOK()
		refunds["body_markdown"] = article.String("data.body_markdown")
		assert.Contains(t, refunds["body_markdown"], "**fourteen days**")
		assert.Contains(t, refunds["body_markdown"], "/help-images/", "the picture was copied")
		assert.NotContains(t, refunds["body_markdown"], "intercomcdn")
		assert.Equal(t, "draft", articleNamed(t, articles, "Upcoming plans")["status"])

		collections := x.owner.GetJSON("/api/v1/help-centers/" + x.center + "/collections").AssertOK()
		assert.Subset(t, stringsOf(t, collections, "data.#.name"), []string{"Billing", "Payments", "Roadmap"})

		assert.Equal(t, 1, x.count(t,
			"SELECT count(*) FROM help_center_redirects WHERE from_path = $1 AND source = 'import'",
			"/en/articles/101-refund-policy"))
		assert.Equal(t, 1, x.count(t, "SELECT count(*) FROM help_center_redirects WHERE from_path = $1",
			"/en/collections/11-payments"))

		docs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents").AssertOK()
		assert.Len(t, stringsOf(t, docs, "data.#.id"), 3, "each language is a document")
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_document_chunks"),
			"an import feeds the help center, not the source index")
	})

	t.Run("importing again updates the same articles instead of copying them", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveIntercom(t, x)
		source := addIntercom(t, x)
		settle(t, x, source)
		x.tc.DrainLater()

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)
		settle(t, x, source)
		x.tc.DrainLater()

		articles := x.owner.GetJSON("/api/v1/help-centers/" + x.center + "/articles").AssertOK()
		assert.Len(t, stringsOf(t, articles, "data.#.id"), 2)
	})
}

func TestTurnIntoArticles(t *testing.T) {
	t.Parallel()

	t.Run("a synced website becomes editable articles that redirect from the old pages", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, nil)
		settle(t, x, source)

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/imports", testkit.JSON{"help_center_id": x.center}).
			AssertStatus(202)
		x.tc.DrainLater()

		articles := x.owner.GetJSON("/api/v1/help-centers/" + x.center + "/articles").AssertOK()
		assert.ElementsMatch(t, []string{"Refund policy", "Change your plan", "Request time off"},
			stringsOf(t, articles, "data.#.title"))
		assert.Equal(t, 1, x.count(t, "SELECT count(*) FROM help_center_redirects WHERE from_path = '/docs/refunds'"))

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/imports", testkit.JSON{"help_center_id": x.center}).
			AssertStatus(202)
		x.tc.DrainLater()
		assert.Len(t, stringsOf(t, x.owner.GetJSON("/api/v1/help-centers/"+x.center+"/articles"), "data.#.id"), 3,
			"turning it into articles again copies nothing twice")
	})
}

/*
stringsOf reads a list of strings out of a response.
*/
func stringsOf(t *testing.T, res *testkit.Response, path string) []string {
	t.Helper()

	var out []string

	require.NoError(t, json.Unmarshal([]byte(res.String(path)), &out))

	return out
}

/*
articleNamed returns the listed article of the title, its fields as
strings.
*/
func articleNamed(t *testing.T, res *testkit.Response, title string) map[string]string {
	t.Helper()

	var list struct {
		Data []map[string]any `json:"data"`
	}

	res.Decode(&list)

	for _, a := range list.Data {
		if a["title"] == title {
			out := map[string]string{}
			for k, v := range a {
				if s, ok := v.(string); ok {
					out[k] = s
				}
			}

			return out
		}
	}

	require.Failf(t, "no article", "no article is titled %q", title)

	return nil
}

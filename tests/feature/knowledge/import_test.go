package knowledge_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func jsonReply(v any) httpclient.Response {
	body, _ := json.Marshal(v)

	return httpclient.Response{Status: 200, Body: body, Header: textHeader("application/json")}
}

func TestConnectedImport(t *testing.T) {
	t.Parallel()

	t.Run("an import writes the repository's Markdown files as help center articles", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveGitHub(t, x)
		source := addGitHub(t, x, "import")
		settle(t, x, source).AssertJSONPath("data.status", "idle").AssertJSONPath("data.last_run.status", "succeeded")
		x.tc.DrainLater()

		articles := x.owner.GetJSON("/api/v1/help-centers/" + x.center + "/articles").AssertOK()
		assert.ElementsMatch(t, []string{"Refund policy", "Change your plan", "Acme docs"},
			stringsOf(t, articles, "data.#.title"))

		article := x.owner.GetJSON("/api/v1/articles/" + articleNamed(t, articles, "Refund policy")["id"]).AssertOK()
		assert.Contains(t, article.String("data.body_markdown"), "fourteen days")
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_document_chunks"),
			"an import feeds the help center, not the source index")
	})

	t.Run("importing again updates the same articles instead of copying them", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveGitHub(t, x)
		source := addGitHub(t, x, "import")
		settle(t, x, source)
		x.tc.DrainLater()

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)
		settle(t, x, source)
		x.tc.DrainLater()

		articles := x.owner.GetJSON("/api/v1/help-centers/" + x.center + "/articles").AssertOK()
		assert.Len(t, stringsOf(t, articles, "data.#.id"), 3)
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

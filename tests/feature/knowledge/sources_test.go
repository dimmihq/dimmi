package knowledge_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
)

func TestWebsiteSource(t *testing.T) {
	t.Parallel()

	t.Run("a sync crawls the sitemap, indexes every page and answers searches", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, nil)

		done := settle(t, x, source)
		done.AssertJSONPath("data.status", "idle").AssertJSONPath("data.document_count", 3)
		done.AssertJSONPath("data.last_run.status", "succeeded").AssertJSONPath("data.last_run.stats.created", 3)

		docs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents?status=active").AssertOK()
		docs.AssertJSONPath("data.#", 3)
		chrome := x.count(t, "SELECT count(*) FROM knowledge_document_chunks WHERE body LIKE '%Copyright%'")
		assert.Zero(t, chrome, "page chrome is dropped")
		assert.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_document_chunks"))
		assert.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_embeddings"), "the chunks were embedded")

		found := x.search(t, "how fast are refunds paid", "keyword")
		require.NotEmpty(t, found.Chunks)
		assert.Equal(t, "website", found.Chunks[0].SourceType)
		assert.Equal(t, site+"/docs/refunds", found.Chunks[0].URL)
		assert.NotNil(t, found.Chunks[0].SourceID)

		hybrid := x.search(t, "refund", "hybrid")
		assert.Equal(t, "hybrid", hybrid.Mode)
		assert.Contains(t, titles(hybrid.Chunks), "Refund policy")
	})

	t.Run("a second sync only reindexes what changed and removes what is gone", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		pages := docsPages()
		serveSite(t, x, pages)
		source := addWebsite(t, x, nil)
		settle(t, x, source)

		pages[0].body = "Refunds are now paid within three days to the original card."
		serveSite(t, x, pages[:2])
		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)

		done := settle(t, x, source)
		done.AssertJSONPath("data.document_count", 2)
		done.AssertJSONPath("data.last_run.stats.updated", 1).AssertJSONPath("data.last_run.stats.unchanged", 1)
		removed := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents?status=removed").AssertOK()
		assert.Contains(t, removed.String("data.#.path"), "/docs/time-off")

		assert.Empty(t, x.search(t, "manager leave", "keyword").Chunks, "a removed page is never found")
		found := x.search(t, "refunds three days", "keyword")
		require.NotEmpty(t, found.Chunks)
		assert.Contains(t, found.Chunks[0].Text, "three days")

		runs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/syncs").AssertOK()
		runs.AssertJSONPath("data.#", 2)
	})

	t.Run("patterns and a member's exclusion keep pages out across syncs", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, testkit.JSON{"exclude": []string{"/docs/time-*"}})
		settle(t, x, source).AssertJSONPath("data.document_count", 2)

		docs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents?status=active").AssertOK()
		billing := ""
		for i := range 2 {
			if docs.String("data."+itoa(i)+".path") == "/docs/billing" {
				billing = docs.String("data." + itoa(i) + ".id")
			}
		}
		require.NotEmpty(t, billing)

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/documents/"+billing+"/exclusion", nil).AssertNoContent()
		assert.Empty(t, x.search(t, "upgrade downgrade plan billing", "keyword").Chunks)

		x.owner.PostJSON("/api/v1/knowledge/sources/"+source+"/syncs", nil).AssertStatus(202)
		settle(t, x, source).AssertJSONPath("data.document_count", 1)

		excluded := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents?status=excluded").AssertOK()
		excluded.AssertJSONPath("data.#", 2)
		reasons := []string{excluded.String("data.0.excluded_by"), excluded.String("data.1.excluded_by")}
		assert.True(t, slices.Contains(reasons, "member") && slices.Contains(reasons, "pattern"), reasons)
	})

	t.Run("a page that renders with javascript is set aside with its reason", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		fake := serveSite(t, x, docsPages()[:1])
		fake.Stub("GET "+site+"/docs/refunds", httpclient.Response{
			Status: 200, Header: textHeader("text/html"),
			Body: []byte(`<html><head><title>App</title></head><body><div id="root"></div>` +
				`<noscript>You need to enable JavaScript to run this app.</noscript></body></html>`),
		})
		source := addWebsite(t, x, nil)

		done := settle(t, x, source)
		done.AssertJSONPath("data.last_run.status", "partial")
		done.AssertJSONPath("data.last_error_code", "site_requires_javascript")
		x.owner.GetJSON("/api/v1/knowledge/sources/"+source+"/documents?status=excluded").AssertOK().
			AssertJSONPath("data.0.excluded_by", "js_rendered")
	})

	t.Run("the operator only reads sources that feed it", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, testkit.JSON{"targets": []string{"agent"}})
		settle(t, x, source)

		ws := x.tc.Workspace.Workspace.ID
		res, err := x.tc.App.Knowledge.Search(t.Context(), ws, "refunds", contracts.KnowledgeSearchOptions{
			Mode: "keyword", Surface: "operator", Visibility: []contracts.KnowledgeVisibility{"public", "agent_only"},
		})
		require.NoError(t, err)
		assert.Empty(t, res.Chunks)
		assert.NotEmpty(t, x.search(t, "refunds", "keyword").Chunks)
	})
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

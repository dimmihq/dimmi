package knowledge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
documentStatuses maps each of a source's documents, by title and
language, to its status.
*/
func documentStatuses(t *testing.T, x index, source string) map[string]string {
	t.Helper()

	docs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/documents").AssertOK()
	titles := stringsOf(t, docs, "data.#.title")
	statuses := stringsOf(t, docs, "data.#.status")
	require.Len(t, statuses, len(titles))

	out := map[string]string{}
	for i, title := range titles {
		out[title] = statuses[i]
	}

	return out
}

func TestChoosingContent(t *testing.T) {
	t.Parallel()

	t.Run("a source without a tree says so", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)

		x.owner.PostJSON("/api/v1/knowledge/sources/trees", testkit.JSON{
			"type": "markdown_upload", "settings": testkit.JSON{},
		}).AssertStatus(422)
	})

	t.Run("a website shows its pages by folder, each a path pattern", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())

		tree := x.owner.PostJSON("/api/v1/knowledge/sources/trees", testkit.JSON{
			"type": "website", "settings": testkit.JSON{"start_url": site + "/docs/refunds"},
		}).AssertOK()

		tree.AssertJSONPath("data.nodes.0.id", "/docs/**")
		assert.ElementsMatch(t, []string{"/docs/refunds", "/docs/billing", "/docs/time-off"},
			stringsOf(t, tree, "data.nodes.0.children.#.id"))
	})

	t.Run("ticking part of a website reads only that part", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveSite(t, x, docsPages())
		source := addWebsite(t, x, testkit.JSON{"include": []string{"/docs/refunds"}})
		settle(t, x, source)

		statuses := documentStatuses(t, x, source)
		assert.Equal(t, "active", statuses["Refund policy"])
		assert.NotContains(t, statuses, "Change your plan")
	})

	t.Run("changing the ticks syncs again and drops what was left out", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveGitHub(t, x)
		settings := testkit.JSON{
			"installation": connectGitHub(t, x), "repository": "acme/docs", "branch": "main",
			"selected": []string{"dir:guides"},
		}

		source := x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "github", "name": "Docs", "mode": "sync", "targets": []string{"agent"},
			"settings": settings, "sync_now": true,
		}).AssertCreated().String("data.id")
		settle(t, x, source)

		before := documentStatuses(t, x, source)
		assert.Equal(t, "active", before["Refund policy"])
		assert.NotContains(t, before, "Acme docs", "a file outside the ticks is not read")

		settings["selected"] = []string{"README.md"}
		x.owner.PatchJSON("/api/v1/knowledge/sources/"+source, testkit.JSON{"settings": settings}).AssertOK()
		settle(t, x, source).AssertJSONPath("data.last_run.status", "succeeded")

		after := documentStatuses(t, x, source)
		assert.Equal(t, "removed", after["Refund policy"])
		assert.Contains(t, after, "Acme docs")

		runs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/syncs").AssertOK()
		assert.Len(t, stringsOf(t, runs, "data.#.id"), 2, "saving new ticks started a sync")
	})
}

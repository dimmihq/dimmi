package knowledge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
serveIntercomTree answers the Intercom help center of serveIntercom
and its whole listing, which the tree reads.
*/
func serveIntercomTree(t *testing.T, x index) {
	t.Helper()

	fake := serveIntercom(t, x)
	fake.Stub("GET "+intercomAPI+"/help_center/collections?page=1&per_page=150", jsonReply(map[string]any{
		"data": []any{
			map[string]any{"id": "10", "name": "Billing", "parent_id": nil},
			map[string]any{"id": "11", "name": "Payments", "parent_id": "10"},
			map[string]any{"id": "12", "name": "Roadmap", "parent_id": nil},
		},
		"pages": map[string]any{"page": 1, "total_pages": 1},
	}))
	fake.Stub("GET "+intercomAPI+"/articles?page=1&per_page=150", jsonReply(map[string]any{
		"data": []any{
			map[string]any{"id": "101", "title": "Refund policy", "parent_id": 11, "parent_ids": []any{10, 11}},
			map[string]any{"id": "102", "title": "Upcoming plans", "parent_id": 12, "parent_ids": []any{12}},
		},
		"pages": map[string]any{"page": 1, "total_pages": 1},
	}))
}

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

	t.Run("the tree shows collections and articles as the service nests them", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveIntercomTree(t, x)

		tree := x.owner.PostJSON("/api/v1/knowledge/sources/trees", testkit.JSON{
			"type": "intercom_help_center", "settings": testkit.JSON{"installation": connectIntercom(t, x)},
		}).AssertOK()

		assert.Equal(t, []string{"Billing", "Roadmap"}, stringsOf(t, tree, "data.nodes.#.title"))
		assert.Equal(t, "Payments", tree.String("data.nodes.0.children.0.title"))
		assert.Equal(t, "Refund policy", tree.String("data.nodes.0.children.0.children.0.title"))
		assert.Equal(t, "article", tree.String("data.nodes.0.children.0.children.0.kind"))
		assert.Equal(t, []string{"Upcoming plans"}, stringsOf(t, tree, "data.nodes.1.children.#.title"))
	})

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
		serveIntercomTree(t, x)
		installation := connectIntercom(t, x)

		source := x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "intercom_help_center", "name": "Intercom", "mode": "sync", "targets": []string{"agent"},
			"settings": testkit.JSON{"installation": installation, "selected": []string{"11"}}, "sync_now": true,
		}).AssertCreated().String("data.id")
		settle(t, x, source)

		before := documentStatuses(t, x, source)
		assert.Equal(t, "active", before["Refund policy"])
		assert.NotContains(t, before, "Upcoming plans", "an article outside the ticks is not read")

		x.owner.PatchJSON("/api/v1/knowledge/sources/"+source, testkit.JSON{
			"settings": testkit.JSON{"installation": installation, "selected": []string{"12"}},
		}).AssertOK()
		settle(t, x, source).AssertJSONPath("data.last_run.status", "succeeded")

		after := documentStatuses(t, x, source)
		assert.Equal(t, "removed", after["Refund policy"])
		assert.Contains(t, after, "Upcoming plans")

		runs := x.owner.GetJSON("/api/v1/knowledge/sources/" + source + "/syncs").AssertOK()
		assert.Len(t, stringsOf(t, runs, "data.#.id"), 2, "saving new ticks started a sync")
	})
}

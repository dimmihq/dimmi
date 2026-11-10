package knowledge_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/fake"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
builtinModel is the built-in embedder's model key at its defaults.
*/
const builtinModel = "dimmi:Qwen/Qwen3-Embedding-8B@1024"

/*
index is one workspace with a scripted built-in embedder.
*/
type index struct {
	tc     *tests.TestCase
	owner  *testkit.Client
	center string
	embed  *fake.Embedder
}

/*
newIndex boots a workspace whose install has the built-in embedder,
answered by a fake.
*/
func newIndex(t *testing.T) index {
	t.Helper()
	return boot(t, tests.WithEnv("DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY=test-key"))
}

/*
newKeywordIndex boots a workspace whose install has no embedder.
*/
func newKeywordIndex(t *testing.T) index {
	t.Helper()
	return boot(t)
}

/*
boot boots the workspace with a fake standing in for the built-in
embedder.
*/
func boot(t *testing.T, opts ...tests.Option) index {
	t.Helper()
	tc := tests.New(t, opts...)
	embed := fake.New("dimmi", 1024)
	tc.App.Embeddings.Extend("dimmi", func(
		_ context.Context, _ *managers.EmbeddingsManager, _ *models.AiProviderKey, _ managers.Credentials,
		_ *httpclient.Client,
	) (contracts.Embeddings, error) {
		return embed, nil
	})
	owner := tc.In(tc.Owner)
	center := owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
	return index{tc: tc, owner: owner, center: center, embed: embed}
}

/*
publish creates and publishes an article and returns its id.
*/
func (x index) publish(t *testing.T, title, body string) string {
	t.Helper()
	id := x.owner.PostJSON("/api/v1/help-centers/"+x.center+"/articles", testkit.JSON{
		"title": title, "body_markdown": body,
	}).AssertCreated().String("data.id")
	x.owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
	return id
}

/*
search asks the knowledge index the way the agent does.
*/
func (x index) search(t *testing.T, query, mode string) contracts.KnowledgeSearchResult {
	t.Helper()
	res, err := x.tc.App.Knowledge.Search(t.Context(), x.tc.Workspace.Workspace.ID, query,
		contracts.KnowledgeSearchOptions{
			Mode: mode, Visibility: []contracts.KnowledgeVisibility{"public", "agent_only"},
		})
	require.NoError(t, err)
	return res
}

/*
count counts the rows the query returns, read as the operator.
*/
func (x index) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, x.tc.Operator().QueryRow(t.Context(), sql, args...).Scan(&n))
	return n
}

/*
titles are the titles of the chunks.
*/
func titles(chunks []contracts.KnowledgeChunk) []string {
	out := []string{}
	for _, c := range chunks {
		out = append(out, c.Title)
	}
	return out
}

/*
queries are the query requests the fake received.
*/
func (x index) queries() []contracts.EmbedRequest {
	return slices.DeleteFunc(x.embed.Sent(), func(r contracts.EmbedRequest) bool {
		return r.InputType != contracts.EmbedInputQuery
	})
}

func TestKnowledgeEmbeddings(t *testing.T) {
	t.Parallel()

	t.Run("embeds a published article in the background with the built-in model", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		id := x.publish(t, "Changing plans", "## Mid-cycle\n\nPlans can be changed any time; upgrades are prorated.")
		x.tc.Drain()

		x.owner.GetJSON("/api/v1/knowledge/settings").AssertOK().
			AssertJSONPath("data.active_model", builtinModel).
			AssertJSONPath("data.embeddings_status", "ready").
			AssertJSONPath("data.pending_model", nil).
			AssertJSONPath("data.builtin_model", builtinModel)
		assert.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_chunks c JOIN articles a ON a.id = c.article_id "+
			"WHERE a.published_version_id = c.article_version_id"))
		missing := x.count(t, `SELECT count(*) FROM knowledge_chunks c WHERE NOT EXISTS (
			SELECT 1 FROM knowledge_embeddings e WHERE e.embed_hash = c.embed_hash AND e.model_key = $1)`, builtinModel)
		assert.Zero(t, missing)
		for _, r := range x.embed.Sent() {
			assert.Equal(t, contracts.EmbedInputDocument, r.InputType)
			assert.Equal(t, 1024, r.Dimensions)
			assert.LessOrEqual(t, len(r.Inputs), contracts.MaxEmbedBatch)
		}

		sent := len(x.embed.Sent())
		x.owner.PatchJSON("/api/v1/articles/"+id, testkit.JSON{"status": "draft"}).AssertOK()
		x.owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		x.tc.Drain()
		assert.Equal(t, sent, len(x.embed.Sent()), "unchanged text is never embedded again")
	})

	t.Run("drops the chunks of an unpublished or trashed article", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		id := x.publish(t, "Gone soon", "This article will be trashed.")
		x.tc.Drain()
		where := "SELECT count(*) FROM knowledge_chunks c JOIN articles a ON a.id = c.article_id " +
			"WHERE a.slug = 'gone-soon'"
		require.Positive(t, x.count(t, where))
		x.owner.Delete("/api/v1/articles/" + id).AssertStatus(fhttp.StatusNoContent)
		assert.Zero(t, x.count(t, where))
	})

	t.Run("finds a rephrased question that keywords miss", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.embed.Script("switch tiers", fake.Axis(0, 1024)).Script("prorated", fake.Axis(0, 1024))
		x.publish(t, "Changing plans", "Plans can be changed any time; upgrades are prorated.")
		x.publish(t, "Refund policy", "Refunds take five business days to reach your card.")
		x.tc.Drain()
		query := "Will I be billed less when I switch tiers halfway?"

		keyword := x.search(t, query, "keyword")
		assert.NotContains(t, titles(keyword.Chunks), "Changing plans")

		res := x.search(t, query, "")
		require.Equal(t, "hybrid", res.Mode, res.Degraded)
		require.NotEmpty(t, res.Chunks)
		top := res.Chunks[0]
		assert.Equal(t, "Changing plans", top.Title)
		assert.Contains(t, top.MatchedBy, "vector")
		require.NotNil(t, top.Similarity)
		assert.InDelta(t, 1.0, *top.Similarity, 0.01)
		assert.Equal(t, builtinModel, res.EmbeddingModel)
		assert.Contains(t, top.URL, "/articles/changing-plans")

		queries := x.queries()
		require.Len(t, queries, 1)
		assert.Equal(t, "Qwen/Qwen3-Embedding-8B", queries[0].Model)
		assert.Equal(t, 1024, queries[0].Dimensions)

		x.search(t, "  will i be BILLED less when I switch tiers halfway?", "")
		assert.Len(t, x.queries(), 1, "a repeated query comes from the cache")
	})

	t.Run("answers from keywords when the embedder fails or times out", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.publish(t, "Refund policy", "Refunds take five business days.")
		x.tc.Drain()

		x.embed.Fail(&contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "down"})
		res := x.search(t, "how long do refunds take", "")
		assert.Equal(t, "fts_fallback", res.Mode)
		assert.Equal(t, "embedding_error", res.Degraded)
		assert.Contains(t, titles(res.Chunks), "Refund policy")

		x.embed.Fail(nil).Hang()
		res, err := x.tc.App.Knowledge.Search(t.Context(), x.tc.Workspace.Workspace.ID, "refunds for annual plans",
			contracts.KnowledgeSearchOptions{QueryEmbedTimeout: 20 * time.Millisecond})
		require.NoError(t, err)
		assert.Equal(t, "embedding_timeout", res.Degraded)
		assert.Contains(t, titles(res.Chunks), "Refund policy")
	})

	t.Run("searches by keywords alone without an embedder, as before", func(t *testing.T) {
		t.Parallel()
		x := newKeywordIndex(t)
		x.publish(t, "Refund policy", "Refunds take five business days.")
		x.tc.Drain()
		x.owner.GetJSON("/api/v1/knowledge/settings").AssertOK().
			AssertJSONPath("data.active_model", nil).
			AssertJSONPath("data.embeddings_status", "none").
			AssertJSONPath("data.builtin_model", nil)
		res := x.search(t, "how long do refunds take", "")
		assert.Equal(t, "keyword", res.Mode)
		assert.Equal(t, "no_embeddings_key", res.Degraded)
		require.NotEmpty(t, res.Chunks)
		assert.Equal(t, "Refund policy", res.Chunks[0].Title)
		assert.Empty(t, x.embed.Sent())
		x.owner.PatchJSON("/api/v1/knowledge/settings", testkit.JSON{}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "embedding_model_invalid")
	})
}

func TestKnowledgeModelSwitch(t *testing.T) {
	t.Parallel()

	t.Run("keeps searching the active model until the new one covers everything", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.publish(t, "Changing plans", "Plans can be changed any time; upgrades are prorated.")
		x.publish(t, "Refund policy", "Refunds take five business days.")
		x.tc.Drain()

		x.owner.PatchJSON("/api/v1/knowledge/settings", testkit.JSON{"dims": 512}).AssertOK().
			AssertJSONPath("data.active_model", builtinModel).
			AssertJSONPath("data.pending_model", "dimmi:Qwen/Qwen3-Embedding-8B@512").
			AssertJSONPath("data.embeddings_status", "ready")

		res := x.search(t, "how long do refunds take", "")
		assert.Equal(t, builtinModel, res.EmbeddingModel)
		for _, q := range x.queries() {
			assert.Equal(t, 1024, q.Dimensions, "queries use the active model while the new one builds")
		}

		x.tc.Drain()
		x.owner.GetJSON("/api/v1/knowledge/settings").AssertOK().
			AssertJSONPath("data.active_model", "dimmi:Qwen/Qwen3-Embedding-8B@512").
			AssertJSONPath("data.previous_model", builtinModel).
			AssertJSONPath("data.pending_model", nil)
		res = x.search(t, "how long do refunds take", "")
		assert.Equal(t, "dimmi:Qwen/Qwen3-Embedding-8B@512", res.EmbeddingModel)
		queries := x.queries()
		assert.Equal(t, 512, queries[len(queries)-1].Dimensions)
	})

	t.Run("embeds new content with both models while a switch builds", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.publish(t, "Refund policy", "Refunds take five business days.")
		x.tc.Drain()
		x.owner.PatchJSON("/api/v1/knowledge/settings", testkit.JSON{"dims": 512}).AssertOK()
		x.publish(t, "Invoices", "Invoices are mailed on the first of the month.")

		var jobs []string
		rows, err := x.tc.Operator().Query(t.Context(),
			"SELECT args->>'model_key' FROM river_job WHERE kind = 'embed_knowledge_chunks' AND state = 'available'")
		require.NoError(t, err)
		for rows.Next() {
			var m string
			require.NoError(t, rows.Scan(&m))
			jobs = append(jobs, m)
		}
		require.NoError(t, rows.Err())
		assert.Contains(t, jobs, builtinModel)
		assert.Contains(t, jobs, "dimmi:Qwen/Qwen3-Embedding-8B@512")
	})

	t.Run("refuses a model the test embedding cannot reach", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.embed.Fail(errors.New("unreachable"))
		x.owner.PatchJSON("/api/v1/knowledge/settings", testkit.JSON{"dims": 512}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "embedding_model_invalid")
		x.owner.PatchJSON("/api/v1/knowledge/settings", testkit.JSON{"embedding_key_id": "aik_nope"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}

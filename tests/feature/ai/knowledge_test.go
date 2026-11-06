package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/tests"
)

/*
TestSeedKnowledge searches the seed articles the way the agent does.
*/
func TestSeedKnowledge(t *testing.T) {
	t.Parallel()

	storage := tests.New(t).App.Storage
	seed, err := knowledge.LoadSeed(t.Context(), storage, seedPath)
	require.NoError(t, err)

	search := func(t *testing.T, query string, extra ...string) []contracts.KnowledgeChunk {
		t.Helper()
		res, err := seed.Search(t.Context(), ids.New(), query, contracts.KnowledgeSearchOptions{
			Limit: 5, Queries: extra,
			Visibility: []contracts.KnowledgeVisibility{contracts.KnowledgeVisibilityPublic},
		})
		require.NoError(t, err)
		return res.Chunks
	}

	t.Run("loads the articles", func(t *testing.T) {
		t.Parallel()
		status, err := seed.Status(t.Context(), ids.New())
		require.NoError(t, err)
		assert.True(t, status.HasContent)
		assert.GreaterOrEqual(t, status.DocumentCount, int64(8))
	})

	related := map[string]string{
		"How much does the Pro plan cost per seat?":  "https://help.lovelace.test/plans-and-pricing",
		"Can I get a refund on my annual plan?":      "https://help.lovelace.test/refunds",
		"Do you support SAML single sign-on?":        "https://help.lovelace.test/sso-saml",
		"Where is my data hosted?":                   "https://help.lovelace.test/export-and-data",
		"What is the API rate limit?":                "https://help.lovelace.test/api-rate-limits",
		"How do I invite teammates to my workspace?": "https://help.lovelace.test/invite-teammates",
	}
	for query, url := range related {
		t.Run("finds the article for "+query, func(t *testing.T) {
			t.Parallel()
			chunks := search(t, query)
			require.NotEmpty(t, chunks)
			assert.Equal(t, url, chunks[0].URL)
			require.NotNil(t, chunks[0].Similarity)
			assert.GreaterOrEqual(t, *chunks[0].Similarity, 0.35)
		})
	}

	t.Run("scores an unrelated question below the minimum similarity", func(t *testing.T) {
		t.Parallel()
		for _, c := range search(t, "What's the weather like in Paris tomorrow?") {
			assert.Less(t, *c.Similarity, 0.35, c.Title)
		}
	})

	t.Run("uses the extra queries", func(t *testing.T) {
		t.Parallel()
		chunks := search(t, "Wie kündige ich mein Abo?", "cancel subscription")
		require.NotEmpty(t, chunks)
		assert.Equal(t, "https://help.lovelace.test/cancel-subscription", chunks[0].URL)
	})

	t.Run("filters by visibility", func(t *testing.T) {
		t.Parallel()
		res, err := seed.Search(t.Context(), ids.New(), "refund", contracts.KnowledgeSearchOptions{
			Visibility: []contracts.KnowledgeVisibility{contracts.KnowledgeVisibilityAgentOnly},
		})
		require.NoError(t, err)
		assert.Empty(t, res.Chunks)
	})

	t.Run("an empty path is an empty index", func(t *testing.T) {
		t.Parallel()
		empty, err := knowledge.LoadSeed(t.Context(), storage, "")
		require.NoError(t, err)
		res, err := empty.Search(t.Context(), ids.New(), "refund", contracts.KnowledgeSearchOptions{})
		require.NoError(t, err)
		assert.Empty(t, res.Chunks)
	})

	t.Run("parses an article without front matter", func(t *testing.T) {
		t.Parallel()
		a := knowledge.ParseArticle("notes.md", "# Getting started\n\nHello.")
		assert.Equal(t, "Getting started", a.Title)
	})
}

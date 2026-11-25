package ai_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
publishedArticle publishes a help article with a description and
returns its id.
*/
func publishedArticle(t *testing.T, h *harness, title, description string) ids.UUID {
	t.Helper()
	center := h.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
	public := h.d.Owner.PostJSON("/api/v1/help-centers/"+center+"/articles", testkit.JSON{
		"title": title, "description": description, "body_markdown": "Print the label from the order page.",
	}).AssertCreated().String("data.id")
	h.d.Owner.PostJSON("/api/v1/articles/"+public+"/publications", nil).AssertOK()

	var id ids.UUID
	require.NoError(t, h.d.TC.Operator().QueryRow(t.Context(),
		"SELECT id FROM articles WHERE workspace_id = $1 AND title = $2",
		h.d.TC.Workspace.Workspace.ID, title).Scan(&id))
	return id
}

func TestSourceCards(t *testing.T) {
	t.Parallel()

	t.Run("gives cited sources an excerpt, and a public help article its slug", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		article := publishedArticle(t, h, "Shipping labels", "Print, void and reprint a label.")
		near := 0.8
		long := strings.Repeat("Carriers pick up parcels from the dock every weekday morning. ", 6)
		h.d.TC.App.AI.Knowledge = &scripted{Knowledge: h.d.TC.App.AI.Knowledge, chunks: []contracts.KnowledgeChunk{
			{
				ID: ids.New(), DocumentID: ids.New(), ArticleID: &article, Title: "Shipping labels",
				URL: "https://help.lovelace.test/shipping-labels", Text: "## Labels\n\nPrint it from the order.",
				Visibility: contracts.KnowledgeVisibilityPublic, Similarity: &near, MatchedBy: []string{"vector"},
			},
			{
				ID: ids.New(), DocumentID: ids.New(), Title: "Pickups", URL: "https://docs.lovelace.test/pickups",
				Text: "**Pickups**: " + long, Visibility: contracts.KnowledgeVisibilityPublic,
				Similarity: &near, MatchedBy: []string{"vector"},
			},
			{
				ID: ids.New(), DocumentID: ids.New(), Title: "Internal rates", Text: "Our margin is 40%.",
				Visibility: contracts.KnowledgeVisibilityAgentOnly, Similarity: &near, MatchedBy: []string{"vector"},
			},
		}}
		h.llm.answer.Script(search("shipping labels"),
			reply("Print the label from the order [1], pickups run daily [2] [3]."))

		conv, trigger := h.conversation("How do I print a shipping label?")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		sources, _ := aiMeta(t, h.load(conv, res.RunID).agentReplies()[0])["sources"].([]any)
		require.Len(t, sources, 3)
		byTitle := map[string]map[string]any{}
		for _, s := range sources {
			m, _ := s.(map[string]any)
			title, _ := m["title"].(string)
			byTitle[title] = m
		}

		label := byTitle["Shipping labels"]
		assert.Equal(t, "shipping-labels", label["slug"])
		assert.Equal(t, "Print, void and reprint a label.", label["excerpt"], "an article's description is its excerpt")

		pickups := byTitle["Pickups"]
		assert.Nil(t, pickups["slug"], "a source that is not a help article has no slug")
		excerpt, _ := pickups["excerpt"].(string)
		assert.True(t, strings.HasPrefix(excerpt, "Pickups: Carriers pick up parcels"), excerpt)
		assert.True(t, strings.HasSuffix(excerpt, "…"))
		assert.LessOrEqual(t, len([]rune(excerpt)), 161)

		internal := byTitle["Internal rates"]
		assert.Nil(t, internal["excerpt"], "agent-only text never reaches a card")
		assert.Nil(t, internal["slug"])
	})
}

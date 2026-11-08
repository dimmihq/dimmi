package ai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
spy wraps knowledge and keeps the options of the last search.
*/
type spy struct {
	contracts.Knowledge
	last contracts.KnowledgeSearchOptions
}

func (s *spy) Search(ctx context.Context, ws ids.UUID, query string, opts contracts.KnowledgeSearchOptions,
) (contracts.KnowledgeSearchResult, error) {
	s.last = opts

	return s.Knowledge.Search(ctx, ws, query, opts)
}

func TestAnswersByAudience(t *testing.T) {
	t.Parallel()

	t.Run("searches only what the customer could read, decided on each search", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		s := &spy{Knowledge: h.d.TC.App.AI.Knowledge}
		h.d.TC.App.AI.Knowledge = s

		pro := h.d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{"name": "Pro", "rules": testkit.JSON{
			"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"},
		}}).AssertCreated().String("data.id")
		center := h.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")
		article := h.d.Owner.PostJSON("/api/v1/help-centers/"+center+"/articles", testkit.JSON{
			"title": "Priority refunds", "body_markdown": "One day.",
		}).AssertCreated().String("data.id")
		h.d.Owner.PatchJSON("/api/v1/articles/"+article+"/audience", testkit.JSON{"audience_id": pro}).AssertOK()
		proID, err := h.d.TC.App.IDs.Decode(string(models.PrefixAudience), pro)
		require.NoError(t, err)

		conv, msg := h.conversation("How long do refunds take?")
		h.llm.answer.Script(search("refund time"), reply("About five days."))
		h.run(conv, msg)
		assert.True(t, s.last.AudienceFiltered)
		assert.NotContains(t, s.last.AllowedAudienceIDs, proID)

		contact := h.d.Owner.GetJSON("/api/v1/contacts?q=ana").AssertOK().String("data.0.id")
		h.d.Owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{
			"custom_attributes": testkit.JSON{"plan": "pro"},
		}).AssertOK()
		next := h.add(conv, "contact", "message", "And for Pro?")
		h.llm.answer.Script(search("pro refunds"), reply("One day."))
		h.run(conv, next)
		assert.Contains(t, s.last.AllowedAudienceIDs, proID)
	})
}

package knowledge_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/fake"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
exchange is a customer's question and the team's answer.
*/
type exchange struct {
	subject, question, answer string
}

/*
closedConversation opens a conversation from a customer, answers it
and closes it, returning its id.
*/
func (x index) closedConversation(t *testing.T, inbox string, e exchange) string {
	t.Helper()
	id := x.owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": inbox, "author": "contact", "subject": e.subject, "body": e.question,
		"contact": testkit.JSON{"email": testkit.UniqueName("c") + "@example.com", "name": "Ana"},
	}).AssertCreated().String("data.id")
	x.owner.PostJSON("/api/v1/conversations/"+id+"/messages", testkit.JSON{"body": e.answer}).AssertCreated()
	x.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()
	return id
}

func TestSimilarConversations(t *testing.T) {
	t.Parallel()

	t.Run("remembers closed conversations and finds similar ones by meaning", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		inbox, err := factories.DefaultInbox(t.Context(), x.tc.App, x.tc.Workspace.Workspace)
		require.NoError(t, err)
		inboxID := models.PublicID(models.PrefixInbox, inbox.Inbox.ID)
		x.embed.Script("card was declined", fake.Axis(3, 1024)).Script("payment bounced", fake.Axis(3, 1024))

		first := x.closedConversation(t, inboxID, exchange{
			"Checkout", "My card was declined at checkout", "Ask your bank to allow the charge.",
		})
		second := x.closedConversation(t, inboxID, exchange{"Billing", "The payment bounced again", "We retried it."})
		x.closedConversation(t, inboxID, exchange{"Export", "How do I export contacts?", "Use the CSV export."})
		x.tc.Drain()

		assert.Equal(t, 3, x.count(t, "SELECT count(*) FROM knowledge_conversations"))
		assert.Zero(t, x.count(t, `SELECT count(*) FROM knowledge_conversations k WHERE NOT EXISTS (
			SELECT 1 FROM knowledge_embeddings e
			WHERE e.embed_hash = k.embed_hash AND e.model_key = $1)`, builtinModel))

		firstID, err := x.tc.App.IDs.Decode(string(models.PrefixConversation), first)
		require.NoError(t, err)
		secondID, err := x.tc.App.IDs.Decode(string(models.PrefixConversation), second)
		require.NoError(t, err)
		m := authz.NewMembership(x.tc.Workspace.Workspace, x.tc.Workspace.Owner, []string{"conversations:read"})
		similar, err := x.tc.App.Vectors.SimilarConversations(t.Context(), m, firstID, 5)
		require.NoError(t, err)
		require.NotEmpty(t, similar)
		assert.Equal(t, secondID, similar[0].ID)
		assert.Equal(t, "Billing", similar[0].Subject)
		for _, s := range similar {
			assert.NotEqual(t, firstID, s.ID)
		}

		none := authz.NewMembership(x.tc.Workspace.Workspace, x.tc.Workspace.Owner, nil)
		_, err = x.tc.App.Vectors.SimilarConversations(t.Context(), none, firstID, 5)
		require.Error(t, err)
	})
}

func TestKnowledgeSweep(t *testing.T) {
	t.Parallel()

	t.Run("binds old workspaces, indexes their articles and forgets models out of use", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.publish(t, "Refund policy", "Refunds take five business days.")
		x.tc.Drain()
		ws := x.tc.Workspace.Workspace.ID
		_, err := x.tc.Operator().Exec(t.Context(), "DELETE FROM knowledge_chunks WHERE workspace_id = $1", ws)
		require.NoError(t, err)
		_, err = x.tc.Operator().Exec(t.Context(), "DELETE FROM knowledge_settings WHERE workspace_id = $1", ws)
		require.NoError(t, err)
		_, err = x.tc.Operator().Exec(t.Context(), `INSERT INTO knowledge_embeddings
			(workspace_id, model_key, embed_hash, dims, embedding) VALUES ($1, 'old:model@3', 'h', 3, '[1,0,0]')`, ws)
		require.NoError(t, err)

		require.NoError(t, x.tc.App.Vectors.Sweep(t.Context(), ws))
		x.tc.Drain()

		x.owner.GetJSON("/api/v1/knowledge/settings").AssertOK().AssertJSONPath("data.active_model", builtinModel)
		assert.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_chunks WHERE workspace_id = $1", ws))
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_embeddings WHERE model_key = 'old:model@3'"))
		assert.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_embeddings WHERE model_key = $1", builtinModel))
	})

	t.Run("keeps the model switched away from for a week", func(t *testing.T) {
		t.Parallel()
		x := newIndex(t)
		x.publish(t, "Refund policy", "Refunds take five business days.")
		x.tc.Drain()
		x.owner.PatchJSON("/api/v1/knowledge/settings", testkit.JSON{"dims": 512}).AssertOK()
		x.tc.Drain()
		ws := x.tc.Workspace.Workspace.ID

		require.NoError(t, x.tc.App.Vectors.Sweep(t.Context(), ws))
		assert.Positive(t, x.count(t, "SELECT count(*) FROM knowledge_embeddings WHERE model_key = $1", builtinModel))

		x.tc.Clock.Travel(8 * 24 * time.Hour)
		require.NoError(t, x.tc.App.Vectors.Sweep(t.Context(), ws))
		assert.Zero(t, x.count(t, "SELECT count(*) FROM knowledge_embeddings WHERE model_key = $1", builtinModel))
	})
}

/*
|--------------------------------------------------------------------------
| Knowledge Index
|--------------------------------------------------------------------------
|
| Semantic search's wiring: the embeddings manager with the install's
| built-in embedder, the workspace vector index, the article hook that
| keeps chunks in step with publishing, the default binding every new
| workspace gets, the embedding jobs, and the listener that remembers
| each closed conversation.
|
*/

package providers

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/knowledge"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/managers"
)

/*
bootKnowledge builds the vector index and hooks it into publishing,
new workspaces and closed conversations.
*/
func bootKnowledge(a *app.App) {
	if a.Embeddings == nil {
		a.Embeddings = managers.NewEmbeddingsManager(managers.EmbeddingsDeps{
			Keys: a.Keys, HTTP: a.HTTP, Config: a.Config.AI, Clock: a.Clock,
		})
	}
	a.Vectors = &knowledge.Vectors{
		DB: a.DB, Embeddings: a.Embeddings, Cache: a.Cache, Clock: a.Clock, Queue: a.Queue, Config: a.Config.AI,
		Prices: loadPrices(a),
	}
	a.Actions.Articles.Index = a.Vectors
	a.Actions.KnowledgeSettings = &knowledge.Settings{Vectors: a.Vectors}
	a.WorkspaceInitializers.Register(&knowledge.EnsureKnowledgeSettings{Vectors: a.Vectors})
	bootKnowledgeSources(a)

	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.ConversationClosed) error {
		return a.Queue.Push(context.WithoutCancel(ctx), jobs.EmbedConversationArgs{
			WorkspaceID: e.WorkspaceID, ConversationID: e.ConversationID,
		})
	})
}

/*
registerKnowledgeJobs registers the embedding workers.
*/
func registerKnowledgeJobs(a *app.App) error {
	return errors.Join(
		queue.Register(a.Queue, &knowledge.EmbedKnowledgeChunks{Vectors: a.Vectors}),
		queue.Register(a.Queue, &knowledge.ReembedWorkspaceKnowledge{Vectors: a.Vectors}),
		queue.Register(a.Queue, &knowledge.SweepKnowledgeEmbeddings{Vectors: a.Vectors}),
		queue.Register(a.Queue, &knowledge.EmbedConversation{Vectors: a.Vectors}),
		registerKnowledgeSourceJobs(a),
	)
}

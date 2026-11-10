/*
|--------------------------------------------------------------------------
| Knowledge Embedding Jobs
|--------------------------------------------------------------------------
|
| The jobs that keep a workspace's vectors in step with its content:
| a batch of chunk hashes embedded with one model, a whole workspace
| built for a model it is switching to, the nightly sweep that retries
| refusals, fills holes and forgets models out of use, and a closed
| conversation remembered for similar-conversation search. Every one
| is safe to repeat: a vector is keyed by workspace, model and hash,
| and stored once. The knowledge actions do the work.
|
*/

package jobs

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
embedAttempts is how often a batch is tried before it is given up.
*/
const embedAttempts = 8

/*
EmbedKnowledgeChunksArgs is one batch of at most 64 hashes of the
workspace to embed with the model.
*/
type EmbedKnowledgeChunksArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ModelKey    string   `json:"model_key"`
	Hashes      []string `json:"hashes"`
}

/*
Kind names the job.
*/
func (EmbedKnowledgeChunksArgs) Kind() string { return "embed_knowledge_chunks" }

/*
Options retry a batch a few times; the same batch waits once.
*/
func (EmbedKnowledgeChunksArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: embedAttempts, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
ReembedWorkspaceKnowledgeArgs builds the workspace's vectors for the
model.
*/
type ReembedWorkspaceKnowledgeArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ModelKey    string   `json:"model_key"`
}

/*
Kind names the job.
*/
func (ReembedWorkspaceKnowledgeArgs) Kind() string { return "reembed_workspace_knowledge" }

/*
Options keep one build per workspace and model waiting at a time.
*/
func (ReembedWorkspaceKnowledgeArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: embedAttempts, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
SweepKnowledgeEmbeddingsArgs names the nightly sweep; it takes no
arguments.
*/
type SweepKnowledgeEmbeddingsArgs struct{}

/*
Kind names the job.
*/
func (SweepKnowledgeEmbeddingsArgs) Kind() string { return "sweep_knowledge_embeddings" }

/*
Options run the sweep once a day at most.
*/
func (SweepKnowledgeEmbeddingsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: 24 * time.Hour}}
}

/*
EmbedConversationArgs remembers one closed conversation.
*/
type EmbedConversationArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	ConversationID ids.UUID `json:"conversation_id"`
}

/*
Kind names the job.
*/
func (EmbedConversationArgs) Kind() string { return "embed_conversation" }

/*
Options retry a few times.
*/
func (EmbedConversationArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: embedAttempts}
}

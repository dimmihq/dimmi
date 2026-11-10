/*
|--------------------------------------------------------------------------
| Built-In Embedder
|--------------------------------------------------------------------------
|
| Every workspace gets semantic search with no setup, from the model
| the install configured, by default Qwen3-Embedding-8B on DeepInfra's
| OpenAI-compatible endpoint. The model is trained so a prefix of its
| vector keeps its meaning: the size is asked for, and a longer vector
| is cut and normalized again whether or not the provider honored it.
| Questions carry the model's retrieval instruction; stored documents
| are embedded as they are.
|
*/

package dimmi

import (
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openaicompat"
)

/*
QueryPrefix is the instruction a question is embedded behind.
*/
const QueryPrefix = "Instruct: Given a customer support question, retrieve help articles that answer it\nQuery: "

/*
Provider names the built-in embedder in model keys.
*/
const Provider = "dimmi"

/*
Options configure the built-in embedder: the upstream endpoint, its
key, the model and the size its vectors are shortened to.
*/
type Options struct {
	BaseURL string
	APIKey  string
	Model   string
	Dims    int
	HTTP    *httpclient.Client
	Clock   clock.Clock
}

/*
New returns the built-in embedder.
*/
func New(opts Options) *openaicompat.Driver {
	return openaicompat.New(openaicompat.Options{
		Provider: Provider, BaseURL: opts.BaseURL, APIKey: opts.APIKey, HTTP: opts.HTTP, Clock: opts.Clock,
		SendDimensions: true, Truncate: true, QueryPrefix: QueryPrefix,
		Models: []contracts.EmbeddingModel{{
			ID: opts.Model, NativeDims: 4096, AllowedDims: []int{384, 512, 768, 1024, 1536, opts.Dims},
			MaxInputTokens: 32_768, MaxBatch: contracts.MaxEmbedBatch,
		}},
	})
}

/*
|--------------------------------------------------------------------------
| Built-In Embedder
|--------------------------------------------------------------------------
|
| Every workspace gets semantic search with no setup, from the model
| the install configured: by default voyage-4-lite on Voyage's API,
| chosen because a search waits on one call to it and it answers a
| short question quickly and steadily. Questions and stored passages
| are embedded as what they are, a query or a document, so a question
| lands near the passage that answers it.
|
*/

package dimmi

import (
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/voyage"
)

/*
Provider names the built-in embedder in model keys.
*/
const Provider = "dimmi"

/*
Options configure the built-in embedder: the upstream endpoint, its
key, the model and the size of its vectors.
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
func New(opts Options) *voyage.Driver {
	return voyage.New(voyage.Options{
		Provider: Provider, BaseURL: opts.BaseURL, APIKey: opts.APIKey, HTTP: opts.HTTP, Clock: opts.Clock,
		Models: []contracts.EmbeddingModel{{
			ID: opts.Model, NativeDims: 1024, AllowedDims: sizes(opts.Dims),
			MaxInputTokens: 32_000, MaxBatch: contracts.MaxEmbedBatch,
		}},
	})
}

/*
sizes are the vector sizes the 4 series can be asked for, with the
install's own when it is another.
*/
func sizes(configured int) []int {
	out := []int{256, 512, 1024, 2048}
	if slices.Contains(out, configured) {
		return out
	}
	return append(out, configured)
}

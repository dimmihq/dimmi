/*
|--------------------------------------------------------------------------
| OpenAI Embeddings
|--------------------------------------------------------------------------
|
| OpenAI's text-embedding-3 models shorten their own vectors to the
| size asked for, so the request always names it and the driver never
| cuts a vector itself.
|
*/

package openai

import (
	"cmp"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openaicompat"
)

/*
DefaultBaseURL is OpenAI's API.
*/
const DefaultBaseURL = "https://api.openai.com/v1"

/*
Options configure the driver; an empty BaseURL is OpenAI's own.
*/
type Options struct {
	BaseURL string
	APIKey  string
	Headers map[string]string
	HTTP    *httpclient.Client
	Clock   clock.Clock
}

/*
New returns the OpenAI embeddings driver.
*/
func New(opts Options) *openaicompat.Driver {
	return openaicompat.New(openaicompat.Options{
		Provider: "openai", BaseURL: cmp.Or(opts.BaseURL, DefaultBaseURL), APIKey: opts.APIKey,
		Headers: opts.Headers, HTTP: opts.HTTP, Clock: opts.Clock, SendDimensions: true, Models: Models(),
	})
}

/*
Models are OpenAI's embedding models.
*/
func Models() []contracts.EmbeddingModel {
	dims := []int{256, 512, 768, 1024, 1536}
	return []contracts.EmbeddingModel{
		{ID: "text-embedding-3-small", NativeDims: 1536, AllowedDims: dims, MaxInputTokens: 8192, MaxBatch: 64},
		{
			ID: "text-embedding-3-large", NativeDims: 3072, AllowedDims: append(dims, 3072), MaxInputTokens: 8192,
			MaxBatch: 64,
		},
	}
}

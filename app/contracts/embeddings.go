/*
|--------------------------------------------------------------------------
| Embeddings Contract
|--------------------------------------------------------------------------
|
| Semantic search turns text into vectors, and every provider that can
| does it through this one interface. A driver returns one unit-length
| vector per input in input order, at the dimensions asked for, so
| vectors of one model always compare by cosine. Failures speak the
| same error kinds as the language models.
|
*/

package contracts

import "context"

/*
Embeddings turns texts into vectors with one provider's models.
*/
type Embeddings interface {
	/*
		Provider names the provider family behind the driver.
	*/
	Provider() string

	/*
		Models lists the embedding models the driver knows.
	*/
	Models(ctx context.Context) ([]EmbeddingModel, error)

	/*
		Embed returns one vector per input, in the same order, each
		normalized to unit length. Inputs already fit the model's
		limits; the caller chunks.
	*/
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

/*
EmbedInputType says what an input is for: a stored document or a
search query. Some models embed a query differently.
*/
type EmbedInputType string

/*
The input types.
*/
const (
	EmbedInputDocument EmbedInputType = "document"
	EmbedInputQuery    EmbedInputType = "query"
)

/*
EmbedRequest asks for vectors of the inputs, from one to MaxEmbedBatch,
at Dimensions, or the model's native size when zero.
*/
type EmbedRequest struct {
	Model      string
	Inputs     []string
	Dimensions int
	InputType  EmbedInputType
}

/*
MaxEmbedBatch is the most inputs one request carries.
*/
const MaxEmbedBatch = 64

/*
EmbedResponse is the vectors, their size, the tokens the provider
counted and the model that answered.
*/
type EmbedResponse struct {
	Vectors [][]float32
	Dims    int
	Usage   EmbedUsage
	Model   string
}

/*
EmbedUsage is the input tokens an embedding call used; Reported is
false when the provider did not say.
*/
type EmbedUsage struct {
	InputTokens int64
	Reported    bool
}

/*
EmbeddingModel describes one model: its native size, the sizes it may
be shortened to, its input limit and the most inputs per request.
*/
type EmbeddingModel struct {
	ID             string
	NativeDims     int
	AllowedDims    []int
	MaxInputTokens int
	MaxBatch       int
}

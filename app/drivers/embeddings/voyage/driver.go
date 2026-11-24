/*
|--------------------------------------------------------------------------
| Voyage Embeddings
|--------------------------------------------------------------------------
|
| Voyage's embeddings endpoint. It is close to OpenAI's but not the
| same: the vector size is asked for as output_dimension, and every
| request says whether its inputs are queries or documents, which
| Voyage embeds differently so a question lands near the passage that
| answers it. Inputs longer than the model reads are cut by Voyage
| rather than refused, since a search must not fail on a long message.
|
| The 4 series shares one vector space, so documents embedded with one
| of its models can be searched with another.
|
*/

package voyage

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openaicompat"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
DefaultBaseURL is Voyage's public API.
*/
const DefaultBaseURL = "https://api.voyageai.com/v1"

/*
requestTimeout bounds one embeddings request; a caller's context may
end it sooner.
*/
const requestTimeout = 30 * time.Second

/*
Options configure the driver. Provider names the family it reports,
Voyage's own unless the built-in embedder says otherwise. Models is the
static table Models returns; Clock reads a Retry-After.
*/
type Options struct {
	Provider string
	BaseURL  string
	APIKey   string
	HTTP     *httpclient.Client
	Models   []contracts.EmbeddingModel
	Clock    clock.Clock
}

/*
Driver embeds through Voyage.
*/
type Driver struct {
	opts Options
}

/*
New returns the driver.
*/
func New(opts Options) *Driver {
	opts.BaseURL = strings.TrimSuffix(cmp.Or(opts.BaseURL, DefaultBaseURL), "/")
	opts.Provider = cmp.Or(opts.Provider, "voyage")
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	return &Driver{opts: opts}
}

/*
Provider names the provider family.
*/
func (d *Driver) Provider() string {
	return d.opts.Provider
}

/*
Models lists the models the driver was given.
*/
func (d *Driver) Models(context.Context) ([]contracts.EmbeddingModel, error) {
	return d.opts.Models, nil
}

/*
body is the request Voyage reads.
*/
type body struct {
	Input           []string `json:"input"`
	Model           string   `json:"model"`
	InputType       string   `json:"input_type,omitempty"`
	OutputDimension int      `json:"output_dimension,omitempty"`
	Truncation      bool     `json:"truncation"`
}

/*
reply is Voyage's answer: one vector per input, numbered, and the
tokens it counted.
*/
type reply struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage *struct {
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
}

/*
Embed returns the vectors of the inputs in input order, at the size
asked for and of unit length.
*/
func (d *Driver) Embed(ctx context.Context, req contracts.EmbedRequest) (*contracts.EmbedResponse, error) {
	if len(req.Inputs) == 0 || len(req.Inputs) > contracts.MaxEmbedBatch {
		return nil, invalid(fmt.Sprintf("between 1 and %d inputs per request", contracts.MaxEmbedBatch))
	}
	b := body{
		Input: req.Inputs, Model: req.Model, InputType: inputType(req.InputType),
		OutputDimension: req.Dimensions, Truncation: true,
	}
	call := d.opts.HTTP.New().Timeout(requestTimeout).Accept("application/json")
	if d.opts.APIKey != "" {
		call = call.WithToken(d.opts.APIKey)
	}
	resp, err := call.Post(ctx, d.opts.BaseURL+"/embeddings", b)
	if err != nil {
		return nil, networkError(ctx, err)
	}
	if resp.Status >= 300 {
		return nil, statusError(resp, d.opts.Clock.Now())
	}
	var r reply
	if err := resp.JSON(&r); err != nil {
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "unreadable embeddings response"}
	}
	return response(req, r)
}

/*
inputType is what Voyage calls the kind of input: a question searching
for passages, or a passage to be found. An input of neither kind is
embedded as it is.
*/
func inputType(kind contracts.EmbedInputType) string {
	switch kind {
	case contracts.EmbedInputQuery:
		return "query"
	case contracts.EmbedInputDocument:
		return "document"
	}
	return ""
}

/*
response orders the vectors by input and normalizes them. A vector of
another size than asked for is refused: Voyage returns the size it was
asked for, and a silent mismatch would corrupt the index.
*/
func response(req contracts.EmbedRequest, r reply) (*contracts.EmbedResponse, error) {
	if len(r.Data) != len(req.Inputs) {
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "embeddings response lost inputs"}
	}
	vectors := make([][]float32, len(req.Inputs))
	for _, item := range r.Data {
		if item.Index < 0 || item.Index >= len(vectors) || vectors[item.Index] != nil {
			return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "embeddings response misnumbered"}
		}
		if req.Dimensions > 0 && len(item.Embedding) != req.Dimensions {
			return nil, invalid(fmt.Sprintf(
				"model returns %d dimensions, not %d", len(item.Embedding), req.Dimensions))
		}
		v, err := openaicompat.Normalize(item.Embedding)
		if err != nil {
			return nil, err
		}
		vectors[item.Index] = v
	}
	out := &contracts.EmbedResponse{Vectors: vectors, Dims: len(vectors[0]), Model: cmp.Or(r.Model, req.Model)}
	if r.Usage != nil {
		out.Usage = contracts.EmbedUsage{InputTokens: r.Usage.TotalTokens, Reported: true}
	}
	return out, nil
}

/*
statusError maps a failed response to the error kinds. Voyage says
what went wrong in a detail field.
*/
func statusError(resp *httpclient.Response, now time.Time) error {
	var b struct {
		Detail string `json:"detail"`
	}
	_ = resp.JSON(&b)
	return &contracts.LLMError{
		Kind: openai.KindForStatus(resp.Status, ""), HTTPStatus: resp.Status,
		Message:    cmp.Or(b.Detail, fmt.Sprintf("embeddings request answered %d", resp.Status)),
		RetryAfter: httpclient.RetryAfter(resp.Header.Get("Retry-After"), now),
	}
}

/*
networkError maps a request that got no answer to the error kinds.
*/
func networkError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		return &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: "embeddings request canceled"}
	case errors.Is(err, context.DeadlineExceeded):
		return &contracts.LLMError{Kind: contracts.LLMErrorTimeout, Message: "embeddings request timed out"}
	case errors.Is(err, httpclient.ErrBlockedHost):
		return &contracts.LLMError{Kind: contracts.LLMErrorBlockedHost, Message: "embeddings host not allowed"}
	}
	return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: "embeddings request failed"}
}

/*
invalid is a refused request.
*/
func invalid(message string) error {
	return &contracts.LLMError{Kind: contracts.LLMErrorInvalidRequest, Message: message}
}

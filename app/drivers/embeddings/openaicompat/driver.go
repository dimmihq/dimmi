/*
|--------------------------------------------------------------------------
| OpenAI-Compatible Embeddings
|--------------------------------------------------------------------------
|
| Most providers embed through the same POST /embeddings shape, so one
| driver serves them all and the others only set its options. Vectors
| always come back at the size asked for and of unit length: a
| provider that ignores the requested size has its vectors shortened
| here, when the model was trained so a prefix keeps its meaning, and
| every vector is normalized again, since cosine search assumes it. A
| model that cannot be shortened and answers at another size is a
| refused request, never a silent mismatch.
|
*/

package openaicompat

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
requestTimeout bounds one embeddings request; a caller's context may
end it sooner.
*/
const requestTimeout = 30 * time.Second

/*
Options configure the driver. Provider names the family it reports.
SendDimensions sends the requested size to the provider; Truncate lets
the driver shorten longer vectors itself. QueryPrefix is put before
every query input, for models that embed questions with an
instruction. Models is the static table Models returns; Clock reads
a provider's Retry-After.
*/
type Options struct {
	Provider       string
	BaseURL        string
	APIKey         string
	Headers        map[string]string
	HTTP           *httpclient.Client
	SendDimensions bool
	Truncate       bool
	QueryPrefix    string
	Models         []contracts.EmbeddingModel
	Clock          clock.Clock
}

/*
Driver embeds through an OpenAI-shaped endpoint.
*/
type Driver struct {
	opts Options
}

/*
New returns the driver.
*/
func New(opts Options) *Driver {
	opts.BaseURL = strings.TrimSuffix(opts.BaseURL, "/")
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
Models returns the driver's static model table.
*/
func (d *Driver) Models(context.Context) ([]contracts.EmbeddingModel, error) {
	return d.opts.Models, nil
}

/*
body is the request as the endpoint reads it.
*/
type body struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

/*
reply is the endpoint's answer.
*/
type reply struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage *struct {
		PromptTokens int64 `json:"prompt_tokens"`
	} `json:"usage"`
}

/*
Embed sends the inputs, query inputs behind the prefix, and returns
the vectors in input order at the requested size, of unit length.
*/
func (d *Driver) Embed(ctx context.Context, req contracts.EmbedRequest) (*contracts.EmbedResponse, error) {
	if len(req.Inputs) == 0 || len(req.Inputs) > contracts.MaxEmbedBatch {
		return nil, invalid(fmt.Sprintf("between 1 and %d inputs per request", contracts.MaxEmbedBatch))
	}
	b := body{Model: req.Model, Input: d.inputs(req), EncodingFormat: "float"}
	if d.opts.SendDimensions {
		b.Dimensions = req.Dimensions
	}
	call := d.opts.HTTP.New().Timeout(requestTimeout).Accept("application/json")
	if d.opts.APIKey != "" {
		call = call.WithToken(d.opts.APIKey)
	}
	for name, value := range d.opts.Headers {
		call = call.WithHeader(name, value)
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
	return d.response(req, r)
}

/*
inputs puts the query prefix before every query input.
*/
func (d *Driver) inputs(req contracts.EmbedRequest) []string {
	if req.InputType != contracts.EmbedInputQuery || d.opts.QueryPrefix == "" {
		return req.Inputs
	}
	out := make([]string, len(req.Inputs))
	for i, in := range req.Inputs {
		out[i] = d.opts.QueryPrefix + in
	}
	return out
}

/*
response orders the vectors by input, fits them to the requested size
and normalizes them.
*/
func (d *Driver) response(req contracts.EmbedRequest, r reply) (*contracts.EmbedResponse, error) {
	if len(r.Data) != len(req.Inputs) {
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "embeddings response lost inputs"}
	}
	vectors := make([][]float32, len(req.Inputs))
	for _, item := range r.Data {
		if item.Index < 0 || item.Index >= len(vectors) || vectors[item.Index] != nil {
			return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "embeddings response misnumbered"}
		}
		v, err := d.fit(item.Embedding, req.Dimensions)
		if err != nil {
			return nil, err
		}
		vectors[item.Index] = v
	}
	out := &contracts.EmbedResponse{Vectors: vectors, Dims: len(vectors[0]), Model: cmp.Or(r.Model, req.Model)}
	if r.Usage != nil {
		out.Usage = contracts.EmbedUsage{InputTokens: r.Usage.PromptTokens, Reported: true}
	}
	return out, nil
}

/*
fit shortens a vector to the requested size when the driver may, and
normalizes it.
*/
func (d *Driver) fit(v []float32, dims int) ([]float32, error) {
	if dims > 0 && len(v) != dims {
		if !d.opts.Truncate || len(v) < dims {
			return nil, invalid(fmt.Sprintf("model returns %d dimensions, not %d", len(v), dims))
		}
		v = v[:dims]
	}
	return Normalize(v)
}

/*
Normalize scales a vector to unit length; a zero vector has no
direction and is refused.
*/
func Normalize(v []float32) ([]float32, error) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return nil, invalid("model returned a zero vector")
	}
	norm := math.Sqrt(sum)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out, nil
}

/*
statusError maps a failed response to the error kinds.
*/
func statusError(resp *httpclient.Response, now time.Time) error {
	var b struct {
		Error *struct {
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = resp.JSON(&b)
	code, message := "", fmt.Sprintf("embeddings request answered %d", resp.Status)
	if b.Error != nil {
		code = fmt.Sprint(b.Error.Code)
		message = cmp.Or(b.Error.Message, message)
	}
	return &contracts.LLMError{
		Kind: openai.KindForStatus(resp.Status, code), HTTPStatus: resp.Status, ProviderCode: code,
		Message:    message,
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

/*
|--------------------------------------------------------------------------
| Voyage Reranker
|--------------------------------------------------------------------------
|
| Voyage's rerank endpoint: one request carries the question and the
| passages, and the answer numbers the passages best first with a
| relevance each. Passages longer than the model reads are cut by
| Voyage rather than refused.
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
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
DefaultBaseURL is Voyage's public API.
*/
const DefaultBaseURL = "https://api.voyageai.com/v1"

/*
requestTimeout bounds one rerank request; a caller's context may end
it sooner.
*/
const requestTimeout = 15 * time.Second

/*
Options configure the driver. Model is used when a request names none.
*/
type Options struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *httpclient.Client
	Clock   clock.Clock
}

/*
Driver reranks through Voyage.
*/
type Driver struct {
	opts Options
}

/*
New returns the driver.
*/
func New(opts Options) *Driver {
	opts.BaseURL = strings.TrimSuffix(cmp.Or(opts.BaseURL, DefaultBaseURL), "/")
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	return &Driver{opts: opts}
}

/*
body is the request Voyage reads.
*/
type body struct {
	Query      string   `json:"query"`
	Documents  []string `json:"documents"`
	Model      string   `json:"model"`
	TopK       int      `json:"top_k,omitempty"`
	Truncation bool     `json:"truncation"`
}

/*
reply is Voyage's answer: the passages best first, each with its index
in the request, and the tokens it read.
*/
type reply struct {
	Data []struct {
		Index int     `json:"index"`
		Score float64 `json:"relevance_score"`
	} `json:"data"`
	Model string `json:"model"`
	Usage *struct {
		TotalTokens int64 `json:"total_tokens"`
	} `json:"usage"`
}

/*
Rerank orders the documents by how well each answers the query.
*/
func (d *Driver) Rerank(ctx context.Context, req contracts.RerankRequest) (*contracts.RerankResponse, error) {
	if len(req.Documents) == 0 || len(req.Documents) > contracts.MaxRerankDocuments {
		return nil, &contracts.LLMError{
			Kind:    contracts.LLMErrorInvalidRequest,
			Message: fmt.Sprintf("between 1 and %d documents per request", contracts.MaxRerankDocuments),
		}
	}
	model := cmp.Or(req.Model, d.opts.Model)
	b := body{Query: req.Query, Documents: req.Documents, Model: model, TopK: req.TopK, Truncation: true}
	call := d.opts.HTTP.New().Timeout(requestTimeout).Accept("application/json")
	if d.opts.APIKey != "" {
		call = call.WithToken(d.opts.APIKey)
	}
	resp, err := call.Post(ctx, d.opts.BaseURL+"/rerank", b)
	if err != nil {
		return nil, networkError(ctx, err)
	}
	if resp.Status >= 300 {
		return nil, statusError(resp, d.opts.Clock.Now())
	}
	var r reply
	if err := resp.JSON(&r); err != nil {
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "unreadable rerank response"}
	}
	return response(len(req.Documents), model, r)
}

/*
response checks every result names a document of the request, once.
*/
func response(documents int, model string, r reply) (*contracts.RerankResponse, error) {
	out := &contracts.RerankResponse{Model: cmp.Or(r.Model, model)}
	seen := make([]bool, documents)
	for _, item := range r.Data {
		if item.Index < 0 || item.Index >= documents || seen[item.Index] {
			return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "rerank response misnumbered"}
		}
		seen[item.Index] = true
		out.Results = append(out.Results, contracts.RerankResult{Index: item.Index, Score: item.Score})
	}
	if len(out.Results) == 0 {
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "rerank response is empty"}
	}
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
		Message:    cmp.Or(b.Detail, fmt.Sprintf("rerank request answered %d", resp.Status)),
		RetryAfter: httpclient.RetryAfter(resp.Header.Get("Retry-After"), now),
	}
}

/*
networkError maps a request that got no answer to the error kinds.
*/
func networkError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		return &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: "rerank request canceled"}
	case errors.Is(err, context.DeadlineExceeded):
		return &contracts.LLMError{Kind: contracts.LLMErrorTimeout, Message: "rerank request timed out"}
	case errors.Is(err, httpclient.ErrBlockedHost):
		return &contracts.LLMError{Kind: contracts.LLMErrorBlockedHost, Message: "rerank host not allowed"}
	}
	return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: "rerank request failed"}
}

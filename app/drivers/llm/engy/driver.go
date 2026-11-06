/*
|--------------------------------------------------------------------------
| Engy Driver
|--------------------------------------------------------------------------
|
| Engy serves the open-weight models over the OpenAI wire at
| api.engy.ai, asking only for a base URL change. On top of the
| compatible mapping it adds two provider facts: ZDR workers by
| default, and the real charge it reports per request.
|
*/

package engy

import (
	"context"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openaicompat"
)

/*
BaseURL is Engy's endpoint.
*/
const BaseURL = "https://api.engy.ai/v1"

/*
Driver is a contracts.LLM over Engy.
*/
type Driver struct {
	clk   clock.Clock
	inner *openaicompat.Driver
}

/*
New builds an Engy driver for a key.
*/
func New(apiKey string, httpClient *httpclient.StdClient, clk clock.Clock) *Driver {
	return &Driver{clk: clk, inner: openaicompat.New(BaseURL, apiKey, nil, httpClient, openaicompat.Config{
		MaxTokensField:      "max_tokens",
		SupportsTools:       true,
		SupportsStreamUsage: true,
		JSONMode:            true,
		ContextTokens:       200_000,
	})}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderEngy }

/*
Chat runs one non-streaming completion.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	return d.inner.Chat(ctx, req)
}

/*
Stream runs one streaming completion.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	return d.inner.Stream(ctx, req)
}

/*
ListModels returns Engy's model catalog.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	return d.inner.ListModels(ctx)
}

/*
Probe sends the smallest request to a model.
*/
func (d *Driver) Probe(ctx context.Context, model string) (*contracts.ProbeResult, error) {
	return d.inner.Probe(ctx, model)
}

/*
Capabilities answers from Engy's table.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return d.inner.Capabilities(model)
}

/*
ChargedMicros parses the per-request charge Engy reports, in
micro-USD, from a response header or extension field.
*/
func ChargedMicros(headers fhttp.Header, body string) int64 {
	if v := headers.Get("X-Engy-Charged-Micro"); v != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	if _, after, ok := strings.Cut(body, `"charged_micro":`); ok {
		rest := after
		end := strings.IndexAny(rest, ",}")
		if end > 0 {
			if n, err := strconv.ParseInt(strings.TrimSpace(rest[:end]), 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

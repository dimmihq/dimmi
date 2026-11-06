/*
|--------------------------------------------------------------------------
| Anthropic Driver
|--------------------------------------------------------------------------
|
| The Messages API over the official SDK: system blocks carry cache
| markers, streaming accumulates through the SDK, and usage counts
| the cache reads and writes Anthropic reports.
|
*/

package anthropic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	anth "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Options is what one Anthropic connection needs.
*/
type Options struct {
	BaseURL string
	APIKey  string
	Version string
	HTTP    *httpclient.StdClient
	Clock   clock.Clock
}

/*
Driver is a contracts.LLM over the Messages API.
*/
type Driver struct {
	client anth.Client
	clk    clock.Clock
}

/*
New builds the SDK client for one connection.
*/
func New(opts Options) *Driver {
	params := []option.RequestOption{option.WithMaxRetries(0)}
	if opts.HTTP != nil {
		params = append(params, option.WithHTTPClient(opts.HTTP))
	}
	if opts.BaseURL != "" {
		params = append(params, option.WithBaseURL(opts.BaseURL))
	}
	if opts.APIKey != "" {
		params = append(params, option.WithAPIKey(opts.APIKey))
	}
	if opts.Version != "" {
		params = append(params, option.WithHeader("anthropic-version", opts.Version))
	}
	return &Driver{client: anth.NewClient(params...)}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderAnthropic }

/*
Chat runs one non-streaming completion.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	started := d.clock().Now()

	params, err := messageParams(req)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Messages.New(ctx, params)
	if err != nil {
		return nil, MapError(err)
	}

	return mapResponse(res, d.clock().Since(started)), nil
}

/*
Stream runs one streaming completion.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	params, err := messageParams(req)
	if err != nil {
		return nil, err
	}
	return newChatStream(d.client.Messages.NewStreaming(ctx, params)), nil
}

/*
ListModels returns the models the key can call.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	pager := d.client.Models.ListAutoPaging(ctx, anth.ModelListParams{})
	var models []contracts.LLMModel
	for pager.Next() {
		m := pager.Current()
		models = append(models, contracts.LLMModel{ID: m.ID, DisplayName: string(m.DisplayName)})
	}
	if err := pager.Err(); err != nil {
		return nil, MapError(err)
	}
	return models, nil
}

/*
Probe sends the smallest possible request to a model.
*/
func (d *Driver) Probe(ctx context.Context, model string) (*contracts.ProbeResult, error) {
	started := d.clock().Now()

	res, err := d.Chat(ctx, contracts.ChatRequest{
		Model:           model,
		Messages:        []contracts.ChatMessage{{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "Say OK"}}}},
		MaxOutputTokens: 8,
	})
	if err != nil {
		return nil, err
	}
	return &contracts.ProbeResult{Model: res.Model, Latency: d.clock().Since(started), Usage: res.Usage}, nil
}

/*
Capabilities answers from the static table.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return Capabilities(model)
}

/*
messageParams maps our request to the SDK's params.
*/
func messageParams(req contracts.ChatRequest) (anth.MessageNewParams, error) {
	params := anth.MessageNewParams{
		Model:     anth.Model(req.Model),
		MaxTokens: int64(req.MaxOutputTokens),
	}

	for _, block := range req.System {
		sys := anth.TextBlockParam{Text: block.Text}
		if block.Cacheable {
			sys.CacheControl = anth.NewCacheControlEphemeralParam()
		}
		params.System = append(params.System, sys)
	}

	for _, msg := range req.Messages {
		params.Messages = append(params.Messages, mapMessage(msg))
	}

	for _, tool := range req.Tools {
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema, &schema); err != nil {
			return params, contracts.ErrLLMNotSupported
		}
		params.Tools = append(params.Tools, anth.ToolUnionParam{
			OfTool: &anth.ToolParam{
				Name:        tool.Name,
				Description: anth.String(tool.Description),
				InputSchema: anth.ToolInputSchemaParam{Properties: schema["properties"], Required: nil, ExtraFields: schema},
			},
		})
	}

	if req.ToolChoice == contracts.ToolChoiceNone {
		params.ToolChoice = anth.ToolChoiceUnionParam{OfNone: &anth.ToolChoiceNoneParam{}}
	}
	if req.Temperature != nil {
		params.Temperature = anth.Float(*req.Temperature)
	}
	if len(req.StopSequences) > 0 {
		params.StopSequences = req.StopSequences
	}

	return params, nil
}

/*
mapMessage maps one message to SDK params.
*/
func mapMessage(msg contracts.ChatMessage) anth.MessageParam {
	var text strings.Builder
	for _, part := range msg.Parts {
		text.WriteString(part.Text)
	}

	switch msg.Role {
	case contracts.ChatRoleUser:
		return anth.NewUserMessage(anth.NewTextBlock(text.String()))
	case contracts.ChatRoleAssistant:
		blocks := []anth.ContentBlockParamUnion{}
		if text.String() != "" {
			blocks = append(blocks, anth.NewTextBlock(text.String()))
		}
		for _, call := range msg.ToolCalls {
			var input any
			if err := json.Unmarshal(call.Arguments, &input); err != nil {
				input = map[string]any{}
			}
			blocks = append(blocks, anth.NewToolUseBlock(call.ID, input, call.Name))
		}
		return anth.NewAssistantMessage(blocks...)
	case contracts.ChatRoleTool:
		return anth.NewUserMessage(anth.NewToolResultBlock(msg.ToolCallID, text.String(), msg.ToolError))
	}

	return anth.NewUserMessage(anth.NewTextBlock(text.String()))
}

/*
mapResponse maps the SDK response to ours.
*/
func mapResponse(res *anth.Message, latency time.Duration) *contracts.ChatResponse {
	out := &contracts.ChatResponse{
		Model:   string(res.Model),
		Latency: latency,
		Usage: contracts.LLMUsage{
			InputTokens:       res.Usage.InputTokens,
			CachedInputTokens: res.Usage.CacheReadInputTokens,
			CacheWriteTokens:  res.Usage.CacheCreationInputTokens,
			OutputTokens:      res.Usage.OutputTokens,
			Reported:          true,
		},
		StopReason: mapStop(res.StopReason),
	}

	for _, block := range res.Content {
		switch v := block.AsAny().(type) {
		case anth.TextBlock:
			out.Text += v.Text
		case anth.ToolUseBlock:
			args, _ := json.Marshal(v.Input)
			out.ToolCalls = append(out.ToolCalls, contracts.ToolCall{
				ID: v.ID, Name: v.Name, Arguments: args,
			})
		}
	}

	return out
}

/*
mapStop folds Anthropic's stop reason into ours.
*/
func mapStop(reason anth.StopReason) contracts.StopReason {
	switch reason {
	case anth.StopReasonEndTurn, anth.StopReasonStopSequence:
		return contracts.StopReasonEnd
	case anth.StopReasonMaxTokens:
		return contracts.StopReasonMaxTokens
	case anth.StopReasonToolUse:
		return contracts.StopReasonToolUse
	case anth.StopReasonRefusal:
		return contracts.StopReasonContentFilter
	case anth.StopReasonPauseTurn, anth.StopReasonModelContextWindowExceeded:
		return contracts.StopReasonEnd
	default:
		return contracts.StopReasonOther
	}
}

/*
clock returns the driver's clock, the system one when unset.
*/
func (d *Driver) clock() clock.Clock {
	if d.clk != nil {
		return d.clk
	}
	return clock.System()
}

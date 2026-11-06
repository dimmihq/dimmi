/*
|--------------------------------------------------------------------------
| Anthropic Driver
|--------------------------------------------------------------------------
|
| The Messages API over the official SDK: system blocks carry cache
| markers, streaming yields each event as it lands, and usage counts
| the cache reads and writes Anthropic reports. A stream is only handed
| back once its first event arrived, so a refusal at request time —
| a bad key, a throttle, an overload — is an error the retry layer can
| still act on, not a broken stream.
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
defaultMaxTokens bounds a reply when the request names no bound, as
the API requires one.
*/
const defaultMaxTokens = 4096

/*
jsonInstruction asks for JSON, which the API has no simple mode for.
*/
const jsonInstruction = "Respond with a single JSON object and nothing else."

/*
Driver is a contracts.LLM over the Messages API.
*/
type Driver struct {
	client anth.Client
	clock  clock.Clock
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
	params = append(params, option.WithAPIKey(opts.APIKey))
	if opts.Version != "" {
		params = append(params, option.WithHeader("anthropic-version", opts.Version))
	}
	clk := opts.Clock
	if clk == nil {
		clk = clock.System()
	}
	return &Driver{client: anth.NewClient(params...), clock: clk}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderAnthropic }

/*
Chat runs one non-streaming completion.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	started := d.clock.Now()

	params, err := messageParams(req)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Messages.New(ctx, params)
	if err != nil {
		return nil, MapError(err, d.clock.Now())
	}

	return mapResponse(res, d.clock.Since(started)), nil
}

/*
Stream runs one streaming completion and waits for its first event,
so request-time failures come back from here.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	params, err := messageParams(req)
	if err != nil {
		return nil, err
	}

	stream := newChatStream(d.client.Messages.NewStreaming(ctx, params), d.clock)
	if err := stream.prime(); err != nil {
		_ = stream.Close()
		return nil, err
	}
	return stream, nil
}

/*
ListModels returns the models the key can call.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	pager := d.client.Models.ListAutoPaging(ctx, anth.ModelListParams{})
	var out []contracts.LLMModel
	for pager.Next() {
		m := pager.Current()
		out = append(out, contracts.LLMModel{
			ID: m.ID, DisplayName: m.DisplayName, ContextTokens: Capabilities(m.ID).ContextTokens,
		})
	}
	if err := pager.Err(); err != nil {
		return nil, MapError(err, d.clock.Now())
	}
	return out, nil
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
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	params := anth.MessageNewParams{Model: req.Model, MaxTokens: int64(maxTokens)}

	for _, block := range req.System {
		sys := anth.TextBlockParam{Text: block.Text}
		if block.Cacheable {
			sys.CacheControl = anth.NewCacheControlEphemeralParam()
		}
		params.System = append(params.System, sys)
	}
	if req.ResponseFormat == contracts.ResponseFormatJSON {
		params.System = append(params.System, anth.TextBlockParam{Text: jsonInstruction})
	}

	for _, msg := range req.Messages {
		params.Messages = append(params.Messages, mapMessage(msg))
	}

	tools, err := mapTools(req.Tools)
	if err != nil {
		return params, err
	}
	params.Tools = tools
	if len(tools) > 0 && req.ToolChoice == contracts.ToolChoiceNone {
		params.ToolChoice = anth.ToolChoiceUnionParam{OfNone: &anth.ToolChoiceNoneParam{}}
	}

	if req.Temperature != nil && acceptsSampling(req.Model) {
		params.Temperature = anth.Float(*req.Temperature)
	}
	if len(req.StopSequences) > 0 {
		params.StopSequences = req.StopSequences
	}

	return params, nil
}

/*
mapTools maps tool specs; a schema that is not a JSON object is a
caller bug and refuses the request.
*/
func mapTools(specs []contracts.ToolSpec) ([]anth.ToolUnionParam, error) {
	var tools []anth.ToolUnionParam
	for _, tool := range specs {
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema, &schema); err != nil {
			return nil, &contracts.LLMError{
				Kind: contracts.LLMErrorInvalidRequest, Message: "tool " + tool.Name + " has an invalid schema",
			}
		}
		param := &anth.ToolParam{
			Name:        tool.Name,
			InputSchema: anth.ToolInputSchemaParam{Properties: schema["properties"], ExtraFields: schema},
		}
		if tool.Description != "" {
			param.Description = anth.String(tool.Description)
		}
		tools = append(tools, anth.ToolUnionParam{OfTool: param})
	}
	return tools, nil
}

/*
mapMessage maps one message to SDK params.
*/
func mapMessage(msg contracts.ChatMessage) anth.MessageParam {
	var b strings.Builder
	for _, part := range msg.Parts {
		b.WriteString(part.Text)
	}
	text := b.String()

	switch msg.Role {
	case contracts.ChatRoleAssistant:
		blocks := []anth.ContentBlockParamUnion{}
		if text != "" {
			blocks = append(blocks, anth.NewTextBlock(text))
		}
		for _, call := range msg.ToolCalls {
			var input any
			if err := json.Unmarshal(call.Arguments, &input); err != nil || input == nil {
				input = map[string]any{}
			}
			blocks = append(blocks, anth.NewToolUseBlock(call.ID, input, call.Name))
		}
		return anth.NewAssistantMessage(blocks...)
	case contracts.ChatRoleTool:
		return anth.NewUserMessage(anth.NewToolResultBlock(msg.ToolCallID, text, msg.ToolError))
	case contracts.ChatRoleUser:
		return anth.NewUserMessage(anth.NewTextBlock(text))
	}
	return anth.NewUserMessage(anth.NewTextBlock(text))
}

/*
mapResponse maps the SDK response to ours.
*/
func mapResponse(res *anth.Message, latency time.Duration) *contracts.ChatResponse {
	out := &contracts.ChatResponse{
		Model:      res.Model,
		RequestID:  res.ID,
		Latency:    latency,
		Usage:      usageOf(res.Usage),
		StopReason: mapStop(res.StopReason),
	}

	for _, block := range res.Content {
		switch block.Type {
		case "text":
			out.Text += block.Text
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, toolCall(block))
		}
	}

	return out
}

/*
toolCall maps a finished tool_use block.
*/
func toolCall(block anth.ContentBlockUnion) contracts.ToolCall {
	args := block.Input
	if len(args) == 0 || !json.Valid(args) {
		args = json.RawMessage("{}")
	}
	return contracts.ToolCall{ID: block.ID, Name: block.Name, Arguments: args}
}

/*
usageOf maps Anthropic usage, whose input count already excludes the
cache reads and writes it reports beside it.
*/
func usageOf(u anth.Usage) contracts.LLMUsage {
	return contracts.LLMUsage{
		InputTokens:       u.InputTokens,
		CachedInputTokens: u.CacheReadInputTokens,
		CacheWriteTokens:  u.CacheCreationInputTokens,
		OutputTokens:      u.OutputTokens,
		Reported:          true,
	}
}

/*
mapStop folds Anthropic's stop reason into ours.
*/
func mapStop(reason anth.StopReason) contracts.StopReason {
	switch reason {
	case anth.StopReasonEndTurn, anth.StopReasonPauseTurn:
		return contracts.StopReasonEnd
	case anth.StopReasonStopSequence:
		return contracts.StopReasonStopSequence
	case anth.StopReasonMaxTokens, anth.StopReasonModelContextWindowExceeded:
		return contracts.StopReasonMaxTokens
	case anth.StopReasonToolUse:
		return contracts.StopReasonToolUse
	case anth.StopReasonRefusal:
		return contracts.StopReasonContentFilter
	default:
		return contracts.StopReasonOther
	}
}

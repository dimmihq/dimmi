/*
|--------------------------------------------------------------------------
| OpenAI Driver
|--------------------------------------------------------------------------
|
| One thin wrapper over the official SDK: our request in, their types
| out, errors folded into the shared kinds, usage counted including
| cached and reasoning tokens. The same mapping serves the compatible
| providers, so it lives where every sibling can reach it.
|
*/

package openai

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Options carries what a connection needs to reach an OpenAI-shaped
endpoint: a base URL, optional key and headers, and the HTTP client
the app's egress guard built.
*/
type Options struct {
	BaseURL     string
	APIKey      string
	Headers     map[string]string
	HTTP        *httpclient.StdClient
	Clock       clock.Clock
	Context     map[string]string
	ModelParams func(model string) []option.RequestOption
}

/*
Driver is a contracts.LLM over the OpenAI chat-completions surface.
*/
type Driver struct {
	client  oai.Client
	clk     clock.Clock
	options Options
}

/*
New builds the SDK client for one connection.
*/
func New(opts Options) *Driver {
	params := []option.RequestOption{
		option.WithMaxRetries(0),
	}
	if opts.HTTP != nil {
		params = append(params, option.WithHTTPClient(opts.HTTP))
	}
	if opts.BaseURL != "" {
		params = append(params, option.WithBaseURL(opts.BaseURL))
	}
	if opts.APIKey != "" {
		params = append(params, option.WithAPIKey(opts.APIKey))
	}
	for k, v := range opts.Headers {
		params = append(params, option.WithHeader(k, v))
	}

	return &Driver{client: oai.NewClient(params...), options: opts}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderOpenAI }

/*
Chat runs one non-streaming completion.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	started := d.clock().Now()

	params, err := completionParams(req)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Chat.Completions.New(ctx, params, d.options.ModelParams(req.Model)...)
	if err != nil {
		return nil, MapError(err)
	}

	return mapResponse(res, req.Model, d.clock().Since(started)), nil
}

/*
Stream runs one streaming completion.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	params, err := completionParams(req)
	if err != nil {
		return nil, err
	}
	params.StreamOptions = openaiStreamOptions()

	stream := d.client.Chat.Completions.NewStreaming(ctx, params, d.options.ModelParams(req.Model)...)
	if err := stream.Err(); err != nil {
		return nil, MapError(err)
	}

	return newChatStream(stream, req.Model), nil
}

/*
ListModels returns the models the key can call.
*/
func (d *Driver) ListModels(ctx context.Context) ([]contracts.LLMModel, error) {
	pager := d.client.Models.ListAutoPaging(ctx)
	var models []contracts.LLMModel
	for pager.Next() {
		m := pager.Current()
		models = append(models, contracts.LLMModel{ID: m.ID, DisplayName: m.ID})
	}
	if err := pager.Err(); err != nil {
		return nil, MapError(err)
	}
	return models, nil
}

/*
Capabilities answers what a model supports from the static table.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return Capabilities(model)
}

/*
completionParams maps our request to the SDK's params.
*/
func completionParams(req contracts.ChatRequest) (oai.ChatCompletionNewParams, error) {
	params := oai.ChatCompletionNewParams{
		Model: req.Model,
	}

	for _, block := range req.System {
		params.Messages = append(params.Messages, oai.SystemMessage(block.Text))
	}
	for _, msg := range req.Messages {
		params.Messages = append(params.Messages, mapMessage(msg)...)
	}

	for _, tool := range req.Tools {
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema, &schema); err != nil {
			return params, contracts.ErrLLMNotSupported
		}
		params.Tools = append(params.Tools, oai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: oai.String(tool.Description),
			Parameters:  shared.FunctionParameters(schema),
		}))
	}

	if req.ToolChoice == contracts.ToolChoiceNone {
		params.ToolChoice = oai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: oai.String("none")}
	}

	params.MaxCompletionTokens = oai.Int(int64(req.MaxOutputTokens))

	if req.Temperature != nil {
		params.Temperature = oai.Float(*req.Temperature)
	}
	if len(req.StopSequences) > 0 {
		params.Stop = oai.ChatCompletionNewParamsStopUnion{OfStringArray: req.StopSequences}
	}
	if req.ResponseFormat == contracts.ResponseFormatJSON {
		params.ResponseFormat = oai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		}
	}

	return params, nil
}

/*
openaiStreamOptions asks for usage inside the stream.
*/
func openaiStreamOptions() oai.ChatCompletionStreamOptionsParam {
	return oai.ChatCompletionStreamOptionsParam{IncludeUsage: oai.Bool(true)}
}

/*
mapMessage maps one message to the SDK params.
*/
func mapMessage(msg contracts.ChatMessage) []oai.ChatCompletionMessageParamUnion {
	var out []oai.ChatCompletionMessageParamUnion

	var text strings.Builder
	for _, part := range msg.Parts {
		text.WriteString(part.Text)
	}

	switch msg.Role {
	case contracts.ChatRoleUser:
		out = append(out, oai.UserMessage(text.String()))
	case contracts.ChatRoleAssistant:
		assistant := oai.ChatCompletionAssistantMessageParam{}
		if text.String() != "" {
			assistant.Content = oai.ChatCompletionAssistantMessageParamContentUnion{OfString: oai.String(text.String())}
		}
		for _, call := range msg.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal(call.Arguments, &args); err != nil {
				args = map[string]any{}
			}
			assistant.ToolCalls = append(assistant.ToolCalls, oai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &oai.ChatCompletionMessageFunctionToolCallParam{
					ID: call.ID,
					Function: oai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      call.Name,
						Arguments: string(call.Arguments),
					},
				},
			})
		}
		out = append(out, oai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
	case contracts.ChatRoleTool:
		out = append(out, oai.ToolMessage(text.String(), msg.ToolCallID))
	}

	return out
}

/*
mapResponse maps the SDK response to ours.
*/
func mapResponse(res *oai.ChatCompletion, _ string, latency time.Duration) *contracts.ChatResponse {
	out := &contracts.ChatResponse{
		Model:   res.Model,
		Latency: latency,
		Usage: contracts.LLMUsage{
			InputTokens:       res.Usage.PromptTokens,
			CachedInputTokens: res.Usage.PromptTokensDetails.CachedTokens,
			OutputTokens:      res.Usage.CompletionTokens,
			ReasoningTokens:   res.Usage.CompletionTokensDetails.ReasoningTokens,
			Reported:          true,
		},
	}

	if len(res.Choices) == 0 {
		return out
	}

	choice := res.Choices[0]
	out.Text = choice.Message.Content
	out.StopReason = mapFinish(choice.FinishReason)

	for _, call := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, contracts.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(call.Function.Arguments),
		})
		if out.StopReason != contracts.StopReasonEnd {
			continue
		}
		out.StopReason = contracts.StopReasonToolUse
	}

	return out
}

/*
mapFinish folds a provider finish reason into ours.
*/
func mapFinish(reason string) contracts.StopReason {
	switch reason {
	case "stop":
		return contracts.StopReasonEnd
	case "length":
		return contracts.StopReasonMaxTokens
	case "tool_calls", "function_call":
		return contracts.StopReasonToolUse
	case "content_filter":
		return contracts.StopReasonContentFilter
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

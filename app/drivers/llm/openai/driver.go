/*
|--------------------------------------------------------------------------
| OpenAI Driver
|--------------------------------------------------------------------------
|
| One thin wrapper over the official SDK: our request in, their types
| out, errors folded into the shared kinds, usage counted with cached
| prompt tokens kept apart from fresh ones so nothing is billed twice.
| The same mapping serves every OpenAI-shaped endpoint, so the quirks
| a compatible server has — the max-tokens field it reads, whether it
| reports usage in a stream, whether it has a JSON mode — are options.
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
the app's egress guard built. Provider names the connection, OpenAI
when empty. MaxTokensField is "max_tokens" for servers that ignore
max_completion_tokens. NoStreamUsage stops asking for usage inside a
stream; NoJSONMode replaces the native JSON mode with an instruction.
Capabilities overrides the static table; ModelParams adds per-model
request options.
*/
type Options struct {
	Provider       contracts.LLMProvider
	BaseURL        string
	APIKey         string
	Headers        map[string]string
	HTTP           *httpclient.StdClient
	Clock          clock.Clock
	MaxTokensField string
	NoStreamUsage  bool
	NoJSONMode     bool
	Capabilities   func(model string) contracts.LLMCapabilities
	ModelParams    func(model string) []option.RequestOption
}

/*
MaxTokensLegacy is the field older and compatible servers read.
*/
const MaxTokensLegacy = "max_tokens"

/*
jsonInstruction asks for JSON where a server has no JSON mode.
*/
const jsonInstruction = "Respond with a single JSON object and nothing else."

/*
Driver is a contracts.LLM over the OpenAI chat-completions surface.
*/
type Driver struct {
	client  oai.Client
	options Options
}

/*
New builds the SDK client for one connection.
*/
func New(opts Options) *Driver {
	if opts.Provider == "" {
		opts.Provider = contracts.LLMProviderOpenAI
	}
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	if opts.Capabilities == nil {
		opts.Capabilities = Capabilities
	}
	if opts.ModelParams == nil {
		opts.ModelParams = func(string) []option.RequestOption { return nil }
	}

	params := []option.RequestOption{option.WithMaxRetries(0)}
	if opts.HTTP != nil {
		params = append(params, option.WithHTTPClient(opts.HTTP))
	}
	if opts.BaseURL != "" {
		params = append(params, option.WithBaseURL(opts.BaseURL))
	}
	params = append(params, option.WithAPIKey(opts.APIKey))
	for k, v := range opts.Headers {
		params = append(params, option.WithHeader(k, v))
	}

	return &Driver{client: oai.NewClient(params...), options: opts}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return d.options.Provider }

/*
Chat runs one non-streaming completion.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	started := d.options.Clock.Now()

	params, err := d.completionParams(req)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Chat.Completions.New(ctx, params, d.requestOptions(req)...)
	if err != nil {
		return nil, MapError(err, d.options.Clock.Now())
	}

	return mapResponse(res, d.options.Clock.Since(started)), nil
}

/*
Stream runs one streaming completion. A refused request fails here,
before any event, so the retry layer can still act on it.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	params, err := d.completionParams(req)
	if err != nil {
		return nil, err
	}
	if !d.options.NoStreamUsage {
		params.StreamOptions = oai.ChatCompletionStreamOptionsParam{IncludeUsage: oai.Bool(true)}
	}

	stream := d.client.Chat.Completions.NewStreaming(ctx, params, d.requestOptions(req)...)
	if err := stream.Err(); err != nil {
		_ = stream.Close()
		return nil, MapError(err, d.options.Clock.Now())
	}

	return newChatStream(stream, d.options.Clock), nil
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
		return nil, MapError(err, d.options.Clock.Now())
	}
	return models, nil
}

/*
Capabilities answers what a model supports from the static table.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return d.options.Capabilities(model)
}

/*
requestOptions adds the per-model options and, for servers that read
the legacy field, moves the output bound there.
*/
func (d *Driver) requestOptions(req contracts.ChatRequest) []option.RequestOption {
	opts := d.options.ModelParams(req.Model)
	if d.options.MaxTokensField == MaxTokensLegacy && req.MaxOutputTokens > 0 {
		opts = append(opts,
			option.WithJSONDel("max_completion_tokens"),
			option.WithJSONSet(MaxTokensLegacy, req.MaxOutputTokens),
		)
	}
	return opts
}

/*
completionParams maps our request to the SDK's params.
*/
func (d *Driver) completionParams(req contracts.ChatRequest) (oai.ChatCompletionNewParams, error) {
	params := oai.ChatCompletionNewParams{Model: req.Model}

	for _, block := range req.System {
		params.Messages = append(params.Messages, oai.SystemMessage(block.Text))
	}
	jsonMode := req.ResponseFormat == contracts.ResponseFormatJSON
	if jsonMode && d.options.NoJSONMode {
		params.Messages = append(params.Messages, oai.SystemMessage(jsonInstruction))
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
		params.ToolChoice = oai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: oai.String("none")}
	}

	if req.MaxOutputTokens > 0 {
		params.MaxCompletionTokens = oai.Int(int64(req.MaxOutputTokens))
	}
	if effort := reasoningEffort(req); effort != "" {
		params.ReasoningEffort = effort
	}
	if req.Temperature != nil {
		params.Temperature = oai.Float(*req.Temperature)
	}
	if len(req.StopSequences) > 0 {
		params.Stop = oai.ChatCompletionNewParamsStopUnion{OfStringArray: req.StopSequences}
	}
	if jsonMode && !d.options.NoJSONMode {
		params.ResponseFormat = oai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		}
	}

	return params, nil
}

/*
mapTools maps tool specs to function tools; a schema that is not a
JSON object is a caller bug and refuses the request.
*/
func mapTools(specs []contracts.ToolSpec) ([]oai.ChatCompletionToolUnionParam, error) {
	var tools []oai.ChatCompletionToolUnionParam
	for _, tool := range specs {
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema, &schema); err != nil {
			return nil, &contracts.LLMError{
				Kind: contracts.LLMErrorInvalidRequest, Message: "tool " + tool.Name + " has an invalid schema",
			}
		}
		fn := shared.FunctionDefinitionParam{Name: tool.Name, Parameters: shared.FunctionParameters(schema)}
		if tool.Description != "" {
			fn.Description = oai.String(tool.Description)
		}
		tools = append(tools, oai.ChatCompletionFunctionTool(fn))
	}
	return tools, nil
}

/*
mapMessage maps one message to the SDK params.
*/
func mapMessage(msg contracts.ChatMessage) oai.ChatCompletionMessageParamUnion {
	text := joinParts(msg.Parts)

	switch msg.Role {
	case contracts.ChatRoleAssistant:
		assistant := oai.ChatCompletionAssistantMessageParam{}
		if text != "" {
			assistant.Content = oai.ChatCompletionAssistantMessageParamContentUnion{OfString: oai.String(text)}
		}
		for _, call := range msg.ToolCalls {
			args := string(call.Arguments)
			if args == "" {
				args = "{}"
			}
			assistant.ToolCalls = append(assistant.ToolCalls, oai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &oai.ChatCompletionMessageFunctionToolCallParam{
					ID:       call.ID,
					Function: oai.ChatCompletionMessageFunctionToolCallFunctionParam{Name: call.Name, Arguments: args},
				},
			})
		}
		return oai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
	case contracts.ChatRoleTool:
		return oai.ToolMessage(text, msg.ToolCallID)
	case contracts.ChatRoleUser:
		return oai.UserMessage(text)
	}
	return oai.UserMessage(text)
}

/*
joinParts concatenates a message's text parts.
*/
func joinParts(parts []contracts.ContentPart) string {
	var text strings.Builder
	for _, part := range parts {
		text.WriteString(part.Text)
	}
	return text.String()
}

/*
mapResponse maps the SDK response to ours.
*/
func mapResponse(res *oai.ChatCompletion, latency time.Duration) *contracts.ChatResponse {
	out := &contracts.ChatResponse{
		Model:     res.Model,
		RequestID: res.ID,
		Latency:   latency,
		Usage:     usageOf(res.Usage),
	}

	if len(res.Choices) == 0 {
		out.StopReason = contracts.StopReasonOther
		return out
	}

	choice := res.Choices[0]
	out.Text = choice.Message.Content
	out.StopReason = MapFinish(choice.FinishReason)

	for _, call := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, contracts.ToolCall{
			ID: call.ID, Name: call.Function.Name, Arguments: rawArgs(call.Function.Arguments),
		})
	}
	if len(out.ToolCalls) > 0 && out.StopReason == contracts.StopReasonEnd {
		out.StopReason = contracts.StopReasonToolUse
	}

	return out
}

/*
usageOf maps the SDK usage. OpenAI counts cached tokens inside
prompt_tokens, so fresh input is the difference.
*/
func usageOf(u oai.CompletionUsage) contracts.LLMUsage {
	cached := u.PromptTokensDetails.CachedTokens
	return contracts.LLMUsage{
		InputTokens:       max(u.PromptTokens-cached, 0),
		CachedInputTokens: cached,
		OutputTokens:      u.CompletionTokens,
		ReasoningTokens:   u.CompletionTokensDetails.ReasoningTokens,
		Reported:          u.PromptTokens > 0 || u.CompletionTokens > 0,
	}
}

/*
rawArgs keeps a tool call's arguments, an empty object when none.
*/
func rawArgs(args string) json.RawMessage {
	if strings.TrimSpace(args) == "" {
		return json.RawMessage("{}")
	}
	return json.RawMessage(args)
}

/*
MapFinish folds a provider finish reason into ours.
*/
func MapFinish(reason string) contracts.StopReason {
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
reasoningEffort is the effort to send for the request. Chat Completions
refuses function tools alongside any reasoning on OpenAI's reasoning
models, so a request with tools turns reasoning off; one without asks
for the effort the caller wanted.
*/
func reasoningEffort(req contracts.ChatRequest) shared.ReasoningEffort {
	if !reasons(req.Model) {
		return ""
	}
	if len(req.Tools) > 0 {
		return shared.ReasoningEffort("none")
	}
	return shared.ReasoningEffort(req.Effort)
}

/*
reasons reports whether the model is one of OpenAI's reasoning models,
the only ones that take a reasoning effort; open-weight models served
through the same API reject it.
*/
func reasons(model string) bool {
	return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o")
}

/*
|--------------------------------------------------------------------------
| ChatGPT Driver
|--------------------------------------------------------------------------
|
| A subscription connection spends the member's ChatGPT plan through
| the Responses endpoint the ChatGPT coding tools use. It takes an
| OAuth bearer and the plan's account id instead of a key, always
| streams, never stores the conversation, and refuses an output bound
| or sampling settings, so the driver sends neither. A bearer that
| stopped working is renewed once and the call tried again.
|
*/

package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
BaseURL is the subscription's Responses endpoint root.
*/
const BaseURL = "https://chatgpt.com/backend-api/codex"

/*
defaultInstructions stands in when a request has no system text, as
the endpoint requires instructions.
*/
const defaultInstructions = "You are a helpful assistant."

/*
Token is a bearer and the plan account it spends.
*/
type Token struct {
	AccessToken string
	AccountID   string
}

/*
TokenSource hands the driver a bearer, refreshing and re-sealing the
connection's credential before expiry.
*/
type TokenSource interface {
	/*
		Token returns a bearer valid for the next call.
	*/
	Token(ctx context.Context) (Token, error)

	/*
		Renew returns a new bearer because the endpoint refused the
		last one.
	*/
	Renew(ctx context.Context) (Token, error)
}

/*
Options configures one subscription connection. Originator names this
app to the endpoint.
*/
type Options struct {
	BaseURL    string
	Originator string
	HTTP       *httpclient.StdClient
	Source     TokenSource
	Clock      clock.Clock
}

/*
Driver is a contracts.LLM over the subscription Responses API.
*/
type Driver struct {
	opts Options
}

/*
New builds the driver for one connection.
*/
func New(opts Options) *Driver {
	if opts.BaseURL == "" {
		opts.BaseURL = BaseURL
	}
	if opts.Originator == "" {
		opts.Originator = "dimmi"
	}
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	return &Driver{opts: opts}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderChatGPT }

/*
Chat runs one completion as a stream read to its end, since the
endpoint only streams.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	started := d.opts.Clock.Now()

	stream, err := d.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	out := &contracts.ChatResponse{Model: req.Model}
	var text strings.Builder
	for {
		ev, err := stream.Next(ctx)
		if err != nil {
			return nil, err
		}
		switch ev.Kind {
		case contracts.StreamEventTextDelta:
			text.WriteString(ev.Text)
		case contracts.StreamEventToolCallDone:
			out.ToolCalls = append(out.ToolCalls, *ev.ToolCall)
		case contracts.StreamEventUsage:
			out.Usage = *ev.Usage
		case contracts.StreamEventDone:
			out.Text = text.String()
			out.StopReason = ev.StopReason
			out.Latency = d.opts.Clock.Since(started)
			return out, nil
		}
	}
}

/*
Stream runs one streamed completion, renewing a refused bearer once.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	params, err := responseParams(req)
	if err != nil {
		return nil, err
	}

	token, err := d.opts.Source.Token(ctx)
	if err != nil {
		return nil, err
	}

	stream, err := d.open(ctx, req, params, token)
	if !isAuth(err) {
		return stream, err
	}

	token, err = d.opts.Source.Renew(ctx)
	if err != nil {
		return nil, err
	}
	return d.open(ctx, req, params, token)
}

/*
open sends the request with the plan's headers.
*/
func (d *Driver) open(ctx context.Context, req contracts.ChatRequest,
	params responses.ResponseNewParams, token Token,
) (contracts.ChatStream, error) {
	client := oai.NewClient(
		option.WithBaseURL(d.opts.BaseURL),
		option.WithAPIKey(token.AccessToken),
		option.WithHTTPClient(d.opts.HTTP),
		option.WithMaxRetries(0),
	)

	headers := []option.RequestOption{
		option.WithHeader("originator", d.opts.Originator),
		option.WithHeader("OpenAI-Beta", "responses=experimental"),
		option.WithHeader("session-id", sessionID(req)),
		option.WithHeader("Accept", "text/event-stream"),
	}
	if token.AccountID != "" {
		headers = append(headers, option.WithHeader("chatgpt-account-id", token.AccountID))
	}

	stream := client.Responses.NewStreaming(ctx, params, headers...)
	if err := stream.Err(); err != nil {
		_ = stream.Close()
		return nil, openai.MapError(err, d.opts.Clock.Now())
	}
	return newChatStream(stream, d.opts.Clock), nil
}

/*
ListModels is not offered to plans; callers use the curated list.
*/
func (d *Driver) ListModels(_ context.Context) ([]contracts.LLMModel, error) {
	return nil, contracts.ErrLLMNotSupported
}

/*
Capabilities answers what a plan model supports. Plans cap a model's
window below the API's, and the output bound is the plan's own.
*/
func (d *Driver) Capabilities(_ string) contracts.LLMCapabilities {
	return contracts.LLMCapabilities{
		Tools:           true,
		Streaming:       true,
		SystemPrompt:    true,
		JSONSchema:      true,
		PromptCache:     true,
		ContextTokens:   272_000,
		MaxOutputTokens: 128_000,
	}
}

/*
responseParams maps our request to the Responses API: system blocks
join into instructions, messages and tool traffic into input items,
store always off, and a response schema into a strict json_schema
text format.
*/
func responseParams(req contracts.ChatRequest) (responses.ResponseNewParams, error) {
	params := responses.ResponseNewParams{
		Model:        req.Model,
		Store:        oai.Bool(false),
		Instructions: oai.String(instructions(req.System)),
		Input:        responses.ResponseNewParamsInputUnion{OfInputItemList: inputItems(req.Messages)},
	}

	for _, tool := range req.Tools {
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema, &schema); err != nil {
			return params, &contracts.LLMError{
				Kind: contracts.LLMErrorInvalidRequest, Message: "tool " + tool.Name + " has an invalid schema",
			}
		}
		fn := &responses.FunctionToolParam{Name: tool.Name, Parameters: schema, Strict: oai.Bool(false)}
		if tool.Description != "" {
			fn.Description = oai.String(tool.Description)
		}
		params.Tools = append(params.Tools, responses.ToolUnionParam{OfFunction: fn})
	}
	if len(params.Tools) > 0 {
		choice := responses.ToolChoiceOptionsAuto
		if req.ToolChoice == contracts.ToolChoiceNone {
			choice = responses.ToolChoiceOptionsNone
		}
		params.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{OfToolChoiceMode: oai.Opt(choice)}
	}

	if req.Schema != nil {
		schema, err := openai.StrictSchema(req.Schema)
		if err != nil {
			return params, err
		}
		params.Text = responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
				Name: req.Schema.Name, Schema: schema, Strict: oai.Bool(true),
			},
		}}
	}

	return params, nil
}

/*
instructions joins the system blocks.
*/
func instructions(blocks []contracts.SystemBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return defaultInstructions
	}
	return strings.Join(parts, "\n\n")
}

/*
inputItems maps the transcript: text turns become messages, an
assistant's tool calls become function_call items and tool results
function_call_output items, linked by call id.
*/
func inputItems(messages []contracts.ChatMessage) responses.ResponseInputParam {
	items := responses.ResponseInputParam{}
	for _, msg := range messages {
		text := joinParts(msg.Parts)
		switch msg.Role {
		case contracts.ChatRoleTool:
			item := responses.ResponseInputItemParamOfFunctionCallOutput(text)
			item.OfFunctionCallOutput.CallID = oai.String(msg.ToolCallID)
			items = append(items, item)
		case contracts.ChatRoleAssistant:
			if text != "" {
				items = append(items,
					responses.ResponseInputItemParamOfMessage(text, responses.EasyInputMessageRoleAssistant))
			}
			for _, call := range msg.ToolCalls {
				args := string(call.Arguments)
				if args == "" {
					args = "{}"
				}
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(args, call.ID, call.Name))
			}
		case contracts.ChatRoleUser:
			items = append(items, responses.ResponseInputItemParamOfMessage(text, responses.EasyInputMessageRoleUser))
		}
	}
	return items
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
sessionID keys the endpoint's prompt cache: the conversation's id
when the caller passes one, so its turns share a cache.
*/
func sessionID(req contracts.ChatRequest) string {
	if id := req.Metadata["session_id"]; id != "" {
		return id
	}
	return ids.New().String()
}

/*
isAuth reports a refused bearer.
*/
func isAuth(err error) bool {
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	return ok && llmErr.Kind == contracts.LLMErrorAuth
}

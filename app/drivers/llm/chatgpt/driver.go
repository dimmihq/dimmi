/*
|--------------------------------------------------------------------------
| ChatGPT Driver
|--------------------------------------------------------------------------
|
| A subscription connection speaks the public Responses API with an
| OAuth bearer instead of a key: the user's own plan answers, with no
| stored state and streaming always on. The token source refreshes
| and re-seals the credential before it ever expires.
|
*/

package chatgpt

import (
	"github.com/dimmihq/dimmi/framework/httpclient"
	"context"
	"encoding/json"
		"strings"
	"time"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
TokenSource hands the driver a bearer token, refreshing and re-sealing
the connection's credential before expiry.
*/
type TokenSource interface {
	/*
		Token returns a bearer valid for the next call.
	*/
	Token(ctx context.Context) (string, error)
}

/*
Tokens is one sealed credential set as stored on the connection.
*/
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	AccountID    string    `json:"account_id"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes"`
}

/*
Driver is a contracts.LLM over the subscription Responses API.
*/
type Driver struct {
	baseURL string
	http    *httpclient.StdClient
	source  TokenSource
}

/*
New builds the driver for one connection.
*/
func New(baseURL string, httpClient *httpclient.StdClient, source TokenSource) *Driver {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Driver{baseURL: baseURL, http: httpClient, source: source}
}

/*
Provider names the driver.
*/
func (d *Driver) Provider() contracts.LLMProvider { return contracts.LLMProviderChatGPT }

/*
Chat runs one non-streaming completion, as a streamed request read to
completion since the contract requires stream:true.
*/
func (d *Driver) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	stream, err := d.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	out := &contracts.ChatResponse{Model: req.Model}
	for {
		ev, err := stream.Next(ctx)
		if err != nil {
			return nil, err
		}
		switch ev.Kind {
		case contracts.StreamEventTextDelta:
			out.Text += ev.Text
		case contracts.StreamEventToolCallDone:
			out.ToolCalls = append(out.ToolCalls, *ev.ToolCall)
		case contracts.StreamEventUsage:
			out.Usage = *ev.Usage
		case contracts.StreamEventDone:
			out.StopReason = ev.StopReason
			return out, nil
		}
	}
}

/*
Stream runs one streamed completion.
*/
func (d *Driver) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	token, err := d.source.Token(ctx)
	if err != nil {
		return nil, &contracts.LLMError{Kind: contracts.LLMErrorAuth, Message: "subscription token: " + err.Error()}
	}

	client := oai.NewClient(
		option.WithBaseURL(d.baseURL),
		option.WithAPIKey(token),
		option.WithHTTPClient(d.http),
		option.WithMaxRetries(0),
	)

	params, err := responseParams(req)
	if err != nil {
		return nil, err
	}

	stream := client.Responses.NewStreaming(ctx, params)
	if err := stream.Err(); err != nil {
		return nil, mapError(err)
	}
	return newChatStream(stream, req.Model), nil
}

/*
ListModels is not offered by the plan endpoint; callers use the
curated list or Probe.
*/
func (d *Driver) ListModels(_ context.Context) ([]contracts.LLMModel, error) {
	return nil, contracts.ErrLLMNotSupported
}

/*
Probe sends the smallest request to a model.
*/
func (d *Driver) Probe(ctx context.Context, model string) (*contracts.ProbeResult, error) {
	started := time.Now()
	res, err := d.Chat(ctx, contracts.ChatRequest{
		Model:           model,
		Messages:        []contracts.ChatMessage{{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: "Say OK"}}}},
		MaxOutputTokens: 8,
	})
	if err != nil {
		return nil, err
	}
	return &contracts.ProbeResult{Model: res.Model, Latency: time.Since(started), Usage: res.Usage}, nil
}

/*
Capabilities answers what a plan model supports.
*/
func (d *Driver) Capabilities(model string) contracts.LLMCapabilities {
	return contracts.LLMCapabilities{
		Tools:           true,
		Streaming:       true,
		SystemPrompt:    true,
		JSONMode:        true,
		PromptCache:     true,
		ContextTokens:   400_000,
		MaxOutputTokens: 128_000,
	}
}

/*
responseParams maps our request to the Responses API: system blocks
join into instructions, messages into input items, store always
false, stream always on.
*/
func responseParams(req contracts.ChatRequest) (responses.ResponseNewParams, error) {
	params := responses.ResponseNewParams{
		Model:           shared.ResponsesModel(req.Model),
		Store:           oai.Bool(false),
		MaxOutputTokens: oai.Int(int64(req.MaxOutputTokens)),
	}

	var instructions []string
	for _, block := range req.System {
		instructions = append(instructions, block.Text)
	}
	params.Instructions = oai.String(strings.Join(instructions, "\n\n"))

	items := responses.ResponseInputParam{}
	for _, msg := range req.Messages {
		items = append(items, inputItem(msg))
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: items}

	for _, tool := range req.Tools {
		var schema map[string]any
		if err := json.Unmarshal(tool.Schema, &schema); err != nil {
			return params, contracts.ErrLLMNotSupported
		}
		params.Tools = append(params.Tools, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:       tool.Name,
				Parameters: schema,
			},
		})
	}

	if req.Temperature != nil {
		params.Temperature = oai.Float(*req.Temperature)
	}

	return params, nil
}

/*
inputItem maps one message to a Responses input item.
*/
func inputItem(msg contracts.ChatMessage) responses.ResponseInputItemUnionParam {
	text := ""
	for _, part := range msg.Parts {
		text += part.Text
	}

	role := responses.EasyInputMessageRoleUser
	if msg.Role == contracts.ChatRoleAssistant {
		role = responses.EasyInputMessageRoleAssistant
	}

	return responses.ResponseInputItemUnionParam{
		OfMessage: &responses.EasyInputMessageParam{
			Role: role,
			Content: responses.EasyInputMessageContentUnionParam{
				OfString: oai.String(text),
			},
		},
	}
}

/*
mapError folds errors into the shared kinds.
*/
func mapError(err error) error {
	var apiErr *oai.Error
	if asError(err, &apiErr) {
		kind := contracts.LLMErrorServer
		switch apiErr.StatusCode {
		case 401:
			kind = contracts.LLMErrorAuth
		case 403:
			kind = contracts.LLMErrorPermission
		case 404:
			kind = contracts.LLMErrorModelNotFound
		case 400, 422:
			kind = contracts.LLMErrorInvalidRequest
		case 402:
			kind = contracts.LLMErrorQuota
		case 429:
			kind = contracts.LLMErrorRateLimited
		case 503:
			kind = contracts.LLMErrorOverloaded
		}
		return &contracts.LLMError{Kind: kind, HTTPStatus: apiErr.StatusCode, Message: apiErr.Error()}
	}
	return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: err.Error()}
}

func asError(err error, target **oai.Error) bool {
	for err != nil {
		if e, ok := err.(*oai.Error); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

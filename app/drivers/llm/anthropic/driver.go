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
	"slices"
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
the API requires one. It is generous on purpose: thinking spends from
the same budget, and a reply cut short is worse than a long one.
*/
const defaultMaxTokens = 16000

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

	out := mapResponse(res, d.clock.Since(started))
	if answer := schemaName(req); answer != "" {
		asAnswer(out, answer)
	}
	return out, nil
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

	stream := newChatStream(d.client.Messages.NewStreaming(ctx, params), d.clock, schemaName(req))
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
messageParams maps our request to the SDK's params; a response schema
becomes one forced tool.
*/
func messageParams(req contracts.ChatRequest) (anth.MessageNewParams, error) {
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	params := anth.MessageNewParams{Model: req.Model, MaxTokens: int64(maxTokens)}
	if req.Effort != "" {
		params.OutputConfig = anth.OutputConfigParam{Effort: anth.OutputConfigEffort(req.Effort)}
	}

	params.System = systemBlocks(req.System)
	params.Messages = historyOf(req)

	tools, err := mapTools(req.Tools)
	if err != nil {
		return params, err
	}
	params.Tools = tools
	if len(tools) > 0 && req.ToolChoice == contracts.ToolChoiceNone {
		params.ToolChoice = anth.ToolChoiceUnionParam{OfNone: &anth.ToolChoiceNoneParam{}}
	}
	if req.Schema != nil {
		if err := applySchema(&params, req); err != nil {
			return params, err
		}
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
historyOf maps the messages and marks where they may be cached, within
the breakpoints the system blocks leave.
*/
func historyOf(req contracts.ChatRequest) []anth.MessageParam {
	breakpoints := maxBreakpoints
	for _, block := range req.System {
		if block.Cacheable {
			breakpoints--
		}
	}
	msgs, cacheable := mapMessages(req.Messages)
	if req.CacheHint && len(cacheable) > 0 {
		cacheable[len(cacheable)-1] = true
	}
	markHistory(msgs, cacheable, breakpoints)
	return msgs
}

/*
systemBlocks maps the system blocks, a cacheable one marked so.
*/
func systemBlocks(blocks []contracts.SystemBlock) []anth.TextBlockParam {
	out := make([]anth.TextBlockParam, 0, len(blocks))
	for _, block := range blocks {
		sys := anth.TextBlockParam{Text: block.Text}
		if block.Cacheable {
			sys.CacheControl = anth.NewCacheControlEphemeralParam()
		}
		out = append(out, sys)
	}
	return out
}

/*
maxBreakpoints is the most cache breakpoints one request may carry.
*/
const maxBreakpoints = 4

/*
markHistory marks where the history may be cached, newest first, within
the breakpoints the system blocks left: the messages marked cacheable,
which with a cache hint include the last one, so the next call of a
tool loop reads everything before it from the cache.
*/
func markHistory(msgs []anth.MessageParam, cacheable []bool, left int) {
	for i, msg := range slices.Backward(msgs) {
		if left <= 0 {
			return
		}
		if !cacheable[i] || len(msg.Content) == 0 {
			continue
		}
		if cc := msg.Content[len(msg.Content)-1].GetCacheControl(); cc != nil {
			*cc = anth.NewCacheControlEphemeralParam()
			left--
		}
	}
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
mapMessages maps the conversation to SDK params. The results of the
tool calls one reply made go back together in a single user message:
split across several, the model learns to stop calling tools in
parallel. Beside each mapped message it reports whether the caller
marked it, or a message folded into it, as cacheable.
*/
func mapMessages(msgs []contracts.ChatMessage) ([]anth.MessageParam, []bool) {
	var out []anth.MessageParam
	var cacheable []bool
	for i, msg := range msgs {
		mapped := mapMessage(msg)
		if toolResult(msg) && i > 0 && toolResult(msgs[i-1]) {
			last := &out[len(out)-1]
			last.Content = append(last.Content, mapped.Content...)
			cacheable[len(cacheable)-1] = cacheable[len(cacheable)-1] || msg.Cacheable
			continue
		}
		out = append(out, mapped)
		cacheable = append(cacheable, msg.Cacheable)
	}
	return out, cacheable
}

/*
toolResult reports whether the message carries a tool's result.
*/
func toolResult(msg contracts.ChatMessage) bool {
	switch msg.Role {
	case contracts.ChatRoleTool:
		return true
	case contracts.ChatRoleUser, contracts.ChatRoleAssistant:
		return false
	}
	return false
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
		if len(msg.Sources) > 0 && !msg.ToolError {
			content := make([]anth.ToolResultBlockParamContentUnion, 0, len(msg.Sources))
			for _, s := range msg.Sources {
				block := searchResult(s)
				content = append(content, anth.ToolResultBlockParamContentUnion{OfSearchResult: &block})
			}
			return anth.NewUserMessage(anth.ContentBlockParamUnion{OfToolResult: &anth.ToolResultBlockParam{
				ToolUseID: msg.ToolCallID, Content: content,
			}})
		}
		return anth.NewUserMessage(anth.NewToolResultBlock(msg.ToolCallID, text, msg.ToolError))
	case contracts.ChatRoleUser:
		blocks := make([]anth.ContentBlockParamUnion, 0, len(msg.Sources)+1)
		for _, s := range msg.Sources {
			block := searchResult(s)
			blocks = append(blocks, anth.ContentBlockParamUnion{OfSearchResult: &block})
		}
		return anth.NewUserMessage(append(blocks, anth.NewTextBlock(text))...)
	}
	return anth.NewUserMessage(anth.NewTextBlock(text))
}

/*
sourcePrefix marks a search result's source as a passage the pipeline
numbered, so a citation of it maps back to that number.
*/
const sourcePrefix = "kb:"

/*
searchResult is one passage as a citable search result. Its text is
cut into sentences, the smallest piece Claude cites, so a citation
names the sentence a claim rests on rather than the whole passage.
*/
func searchResult(s contracts.ChatSource) anth.SearchResultBlockParam {
	var content []anth.TextBlockParam
	for _, piece := range citablePieces(s.Text) {
		content = append(content, anth.TextBlockParam{Text: piece})
	}
	if len(content) == 0 {
		content = []anth.TextBlockParam{{Text: s.Title}}
	}
	title := s.Title
	if s.URL != "" {
		title += " (" + s.URL + ")"
	}
	return anth.SearchResultBlockParam{
		Source: sourcePrefix + s.ID, Title: title, Content: content,
		Citations: anth.CitationsConfigParam{Enabled: anth.Bool(true)},
	}
}

/*
citablePieces cuts a passage into paragraphs and each prose paragraph
into sentences. A fenced code block stays whole, since a sentence of
code means nothing on its own.
*/
func citablePieces(text string) []string {
	var out []string
	var fence []string
	for _, para := range strings.Split(strings.TrimSpace(text), "\n\n") {
		if fence != nil || strings.HasPrefix(strings.TrimSpace(para), "```") {
			fence = append(fence, para)
			if strings.Count(strings.Join(fence, "\n\n"), "```")%2 == 0 {
				out = append(out, strings.Join(fence, "\n\n"))
				fence = nil
			}
			continue
		}
		start := 0
		for i := 0; i < len(para); i++ {
			if (para[i] == '.' || para[i] == '?' || para[i] == '!') && (i+1 == len(para) || para[i+1] == ' ') {
				if piece := strings.TrimSpace(para[start : i+1]); piece != "" {
					out = append(out, piece)
				}
				start = i + 1
			}
		}
		if piece := strings.TrimSpace(para[start:]); piece != "" {
			out = append(out, piece)
		}
	}
	if fence != nil {
		out = append(out, strings.Join(fence, "\n\n"))
	}
	return out
}

/*
citationMarkers are the "[ID]" markers of the passages a text block
cites, each once, in the order the provider listed them. Citations of
anything other than a numbered passage add nothing.
*/
func citationMarkers(citations []anth.TextCitationUnion) string {
	var b strings.Builder
	seen := map[string]bool{}
	for _, c := range citations {
		id, ok := strings.CutPrefix(c.Source, sourcePrefix)
		if c.Type != "search_result_location" || !ok || id == "" || seen[id] {
			continue
		}
		seen[id] = true
		b.WriteString(" [" + id + "]")
	}
	return b.String()
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
			out.Text += block.Text + citationMarkers(block.Citations)
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

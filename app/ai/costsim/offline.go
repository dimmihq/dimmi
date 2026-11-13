/*
|--------------------------------------------------------------------------
| Offline Model
|--------------------------------------------------------------------------
|
| A dry run exercises the whole harness without a single real model
| call. The scripted fake driver refuses to answer outside test
| binaries, so the command line's dry run uses this model instead: it
| behaves the way the agent's model is used, searching the knowledge
| first, answering from what the search returned, and filling any
| response schema with the smallest valid answer, and it reports usage
| counted from the request's and the answer's size, so the recorded
| costs are a rough projection rather than zeros. As the simulated
| customer it asks one follow-up and then says it is done.
|
*/

package costsim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Offline is a model that answers in process.
*/
type Offline struct {
	provider contracts.LLMProvider
	mu       sync.Mutex
	seen     map[string]bool
}

/*
NewOffline returns an offline model for the provider name.
*/
func NewOffline(provider contracts.LLMProvider) *Offline {
	return &Offline{provider: provider, seen: map[string]bool{}}
}

/*
Provider names the provider the model stands in for.
*/
func (o *Offline) Provider() contracts.LLMProvider { return o.provider }

/*
ListModels lists nothing.
*/
func (o *Offline) ListModels(context.Context) ([]contracts.LLMModel, error) { return nil, nil }

/*
Capabilities says the model can do what the agent asks of models.
*/
func (o *Offline) Capabilities(string) contracts.LLMCapabilities {
	return contracts.LLMCapabilities{
		Tools: true, Streaming: true, SystemPrompt: true, JSONSchema: true,
		PromptCache: true, ContextTokens: 200_000, MaxOutputTokens: 8_192,
	}
}

/*
Chat answers the request.
*/
func (o *Offline) Chat(_ context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	text, calls := o.answer(req)
	return &contracts.ChatResponse{
		Text: text, ToolCalls: calls, StopReason: contracts.StopReasonEnd,
		Usage: o.usage(req, text, calls), Model: req.Model,
	}, nil
}

/*
Stream answers the request as one delta.
*/
func (o *Offline) Stream(_ context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	text, calls := o.answer(req)
	var events []contracts.StreamEvent
	if text != "" {
		events = append(events, contracts.StreamEvent{Kind: contracts.StreamEventTextDelta, Text: text})
	}
	for i := range calls {
		events = append(events, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &calls[i]})
	}
	u := o.usage(req, text, calls)
	events = append(events, contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: &u},
		contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: contracts.StopReasonEnd})
	return &offlineStream{events: events}, nil
}

/*
answer decides what the model says: a schema's smallest answer, a
customer's next line, a search, or a reply from the search's results.
*/
func (o *Offline) answer(req contracts.ChatRequest) (string, []contracts.ToolCall) {
	if req.Schema != nil {
		var schema map[string]any
		_ = json.Unmarshal(req.Schema.Schema, &schema)
		out, _ := json.Marshal(smallest(schema))
		return string(out), nil
	}
	if req.Metadata[metaRole] == roleCustomer {
		if replied(req.Messages) {
			return DoneMarker, nil
		}
		return "Thanks. Could you explain that in a bit more detail for my setup?", nil
	}

	lastUser, results := "", []string{}
	for _, m := range req.Messages {
		switch m.Role {
		case contracts.ChatRoleUser:
			lastUser, results = joinParts(m.Parts), nil
		case contracts.ChatRoleTool:
			results = append(results, joinParts(m.Parts))
		case contracts.ChatRoleAssistant:
		}
	}
	if len(results) == 0 && hasTool(req.Tools, "search_knowledge") {
		args, _ := json.Marshal(map[string]string{"query": clip(customerText(lastUser), 200)})
		return "", []contracts.ToolCall{{ID: "call_offline_search", Name: "search_knowledge", Arguments: args}}
	}
	found := "the help center"
	if len(results) > 0 {
		found = clip(results[0], 400)
	}
	return fmt.Sprintf("Here is what I found in our help center: %s [1]", found), nil
}

/*
replied reports whether the customer has already written a reply of
its own, which the customer's model sees as an assistant message.
*/
func replied(msgs []contracts.ChatMessage) bool {
	for _, m := range msgs {
		switch m.Role {
		case contracts.ChatRoleAssistant:
			return true
		case contracts.ChatRoleUser, contracts.ChatRoleTool:
		}
	}
	return false
}

/*
usage counts tokens from characters, four to a token, and treats a
system prompt it has seen before as read from the cache.
*/
func (o *Offline) usage(req contracts.ChatRequest, text string, calls []contracts.ToolCall) contracts.LLMUsage {
	system, rest := 0, 0
	var key strings.Builder
	for _, s := range req.System {
		system += len(s.Text)
		key.WriteString(s.Text)
	}
	for _, m := range req.Messages {
		rest += len(joinParts(m.Parts))
		for _, c := range m.ToolCalls {
			rest += len(c.Arguments)
		}
	}
	for _, t := range req.Tools {
		system += len(t.Description) + len(t.Schema) + len(t.Name)
	}
	out := len(text)
	for _, c := range calls {
		out += len(c.Arguments) + len(c.Name)
	}
	o.mu.Lock()
	cached := req.CacheHint && o.seen[key.String()]
	o.seen[key.String()] = true
	o.mu.Unlock()
	u := contracts.LLMUsage{InputTokens: int64((system + rest) / 4), OutputTokens: int64(out/4 + 1), Reported: true}
	if cached {
		u.CachedInputTokens = int64(system / 4)
		u.InputTokens -= u.CachedInputTokens
	}
	return u
}

/*
smallest returns the smallest value a JSON schema accepts.
*/
func smallest(schema map[string]any) any {
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}
	if c, ok := schema["const"]; ok {
		return c
	}
	kind, _ := schema["type"].(string)
	if kinds, ok := schema["type"].([]any); ok && len(kinds) > 0 {
		kind, _ = kinds[0].(string)
	}
	switch kind {
	case "object":
		return smallestObject(schema)
	case "array":
		return []any{}
	case "boolean":
		return false
	case "integer", "number":
		if m, ok := schema["minimum"].(float64); ok {
			return m
		}
		return 0
	case "null":
		return nil
	}
	if m, ok := schema["minLength"].(float64); ok && m > 0 {
		return strings.Repeat("x", int(m))
	}
	return ""
}

/*
smallestObject fills an object schema's required properties.
*/
func smallestObject(schema map[string]any) map[string]any {
	out := map[string]any{}
	props, _ := schema["properties"].(map[string]any)
	required, _ := schema["required"].([]any)
	for _, r := range required {
		name, _ := r.(string)
		sub, _ := props[name].(map[string]any)
		out[name] = smallest(sub)
	}
	return out
}

/*
customerText is what the customer wrote, out of the message the agent
reads it in.
*/
func customerText(prompt string) string {
	const open, end = "<customer_message>", "</customer_message>"
	i := strings.LastIndex(prompt, open)
	if i < 0 {
		return prompt
	}
	text, _, _ := strings.Cut(prompt[i+len(open):], end)
	return text
}

func hasTool(tools []contracts.ToolSpec, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func joinParts(parts []contracts.ContentPart) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

/*
offlineStream serves prepared events.
*/
type offlineStream struct {
	events []contracts.StreamEvent
	at     int
}

/*
Next serves the next prepared event.
*/
func (s *offlineStream) Next(context.Context) (contracts.StreamEvent, error) {
	if s.at >= len(s.events) {
		return contracts.StreamEvent{}, io.EOF
	}
	ev := s.events[s.at]
	s.at++
	return ev, nil
}

/*
Close has nothing to release.
*/
func (s *offlineStream) Close() error { return nil }

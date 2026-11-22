/*
|--------------------------------------------------------------------------
| Agent Loop
|--------------------------------------------------------------------------
|
| The agent's turn is a tool loop: the answer model reads the
| conversation, calls tools — search the knowledge, hand off, tag,
| assign, prioritize — reads their results, and calls more until it
| writes its reply. Its last text is the reply, in free Markdown.
|
| Each model call runs under three clocks: time to the first token,
| idle time between tokens, and the whole call. A tool that fails tells
| the model so in its result instead of ending the turn. After
| maxModelCalls calls the tools are withdrawn and the model must
| answer with what it has, told in a note that it can no longer act, so
| it does not say it did what it had no tool left to do. A turn that
| follows a procedure gets more calls, since each step it finishes is
| one.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/ledger"
	"github.com/dimmihq/dimmi/app/ai/toolset"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/llmretry"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
maxModelCalls caps the answer model calls of one turn.
*/
const maxModelCalls = 6

/*
procedureModelCalls caps the answer model calls of a turn that follows
a procedure: looking something up, saying which steps are finished and
handing over each take a call.
*/
const procedureModelCalls = 14

/*
outOfTools is the note the model reads before its last call of a turn,
when the tools are withdrawn.
*/
const outOfTools = "<system_note>You have no tools left in this reply. Write the reply now from what the tool " +
	"results above show. Say nothing was done, passed on or handed over unless a result above shows it, and " +
	"do not say you are doing it now. If a step is left unfinished, end by asking the customer what you still " +
	"need, or tell them plainly what you could not finish.</system_note>"

/*
loop is the turn's conversation with the answer model: the driver, the
rendered system prompt, the brief of the conversation's action
requests, the turns so far and why the last call ended. When set, the
limit caps the model calls of the next tool loop below maxModelCalls.
*/
type loop struct {
	llm      contracts.LLM
	system   string
	brief    string
	messages []contracts.ChatMessage
	stop     contracts.StopReason
	newest   int
	limit    int
}

/*
streamed is what one model call produced.
*/
type streamed struct {
	text       string
	calls      []contracts.ToolCall
	usage      contracts.LLMUsage
	stop       contracts.StopReason
	firstToken time.Duration
	requestID  string
	model      string
}

/*
agentLoop runs model calls and tools until the model writes its reply,
which it returns.
*/
func (r *Runner) agentLoop(ctx context.Context, st *run) (string, error) {
	lp := st.loop
	for calls := 1; ; calls++ {
		most := callLimit(st)
		var tools []contracts.ToolSpec
		if calls < most && !st.actions.closed {
			tools = r.toolSpecs(st)
		}
		if calls == most && calls > 1 {
			lp.messages = append(lp.messages, userMessage(outOfTools))
		}

		out, err := r.generate(ctx, st, tools)
		if err != nil {
			return "", err
		}
		lp.stop = out.stop
		if len(out.calls) == 0 || len(tools) == 0 {
			return out.text, nil
		}

		r.runCalls(ctx, st, out)
		if st.transfer != nil {
			return "", nil
		}
	}
}

/*
runCalls runs the tools the model asked for and adds the call and each
result to the conversation the model reads next.
*/
func (r *Runner) runCalls(ctx context.Context, st *run, out streamed) {
	lp := st.loop
	lp.messages = append(lp.messages, contracts.ChatMessage{
		Role: contracts.ChatRoleAssistant, Parts: textParts(out.text), ToolCalls: out.calls,
	})
	for _, call := range out.calls {
		result, failed := r.callTool(ctx, st, call)
		lp.messages = append(lp.messages, contracts.ChatMessage{
			Role: contracts.ChatRoleTool, ToolCallID: call.ID, ToolError: failed,
			Parts: []contracts.ContentPart{{Text: result}},
		})
	}
}

/*
callLimit is how many model calls the turn's tool loop gets: the limit
set for it, or else more for a turn that follows a procedure.
*/
func callLimit(st *run) int {
	switch {
	case st.loop.limit > 0:
		return st.loop.limit
	case st.proc.active():
		return procedureModelCalls
	}
	return maxModelCalls
}

/*
callTool runs one tool the model asked for, records it, and returns
the result the model reads, which says what went wrong when it failed.
*/
func (r *Runner) callTool(ctx context.Context, st *run, call contracts.ToolCall) (string, bool) {
	started := r.Clock.Now()
	t, ok := r.registry(st)[call.Name]
	allowed := ok && st.settings.allows(call.Name, t.action)
	if st.layer != nil {
		allowed = allowed && slices.Contains(st.layer.fixed, call.Name)
	}
	st.actions.toolCallID = call.ID

	var result string
	ran := call.Name
	err := errors.New("there is no tool named " + call.Name + " in this conversation. Nothing was done.")
	if st.layer != nil && st.layer.set.Has(call.Name) {
		err = errors.New(st.layer.set.Misdirected(call.Name))
	}
	blocked, refused := "", false
	if allowed {
		blocked, refused = r.gateProcedure(st, call.Name, t)
	}
	switch {
	case st.layer != nil && (call.Name == toolset.LoadTool || call.Name == toolset.CallTool):
		ran, result, err = r.layerCall(ctx, st, call)
	case refused:
		result, err = blocked, nil
	case allowed:
		result, err = t.handle(ctx, st, call.Arguments)
	}

	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindToolCall, model: ran, started: started,
		request: r.redactFor(st.env, string(call.Arguments)), response: withTrace(st, ran, result), err: err,
	})
	r.publishPreview(ctx, st, finished{
		tool: ran, args: call.Arguments, result: result, failed: err != nil, refused: refused,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}
	return result, refused
}

/*
withTrace adds to a search's recorded result how the search went, so
the trace shows when it answered from keywords alone and why; the model
reads the result without it.
*/
func withTrace(st *run, tool, result string) string {
	if tool != toolSearchKnowledge {
		return result
	}

	var body map[string]any
	if json.Unmarshal([]byte(result), &body) != nil {
		return result
	}

	body["trace"] = st.lastSearch
	out, err := json.Marshal(body)
	if err != nil {
		return result
	}

	return string(out)
}

/*
generate streams one model call and records its step, whatever the
outcome.
*/
func (r *Runner) generate(ctx context.Context, st *run, tools []contracts.ToolSpec) (streamed, error) {
	e, lp := st.env, st.loop
	model := strv(e.Agent.AnswerModel)
	system := []contracts.SystemBlock{{Text: lp.system, Cacheable: true}}
	if lp.brief != "" {
		system = append(system, contracts.SystemBlock{Text: lp.brief})
	}
	if block := r.procedureBlock(st); block != "" {
		system = append(system, contracts.SystemBlock{Text: block})
	}
	req := contracts.ChatRequest{
		Model: model, System: system,
		Messages: lp.messages, Tools: tools, CacheHint: true, Effort: r.answerEffort(),
		Temperature: temperature(e), Timeout: llmretry.StepTimeouts(r.Timeouts, llmretry.StepGenerate),
	}
	if e.Conversation.ID != (ids.UUID{}) {
		req.Metadata = map[string]string{"session_id": e.Conversation.ID.String()}
	}
	if len(tools) > 0 {
		req.ToolChoice = contracts.ToolChoiceAuto
	}

	started := r.Clock.Now()
	out, err := r.stream(ctx, lp.llm, req, started)
	calls, _ := json.Marshal(out.calls)
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindGenerate, llm: lp.llm,
		key: &keyRef{id: e.AnswerKey.ID, provider: e.AnswerKey.Provider}, model: model,
		started: started, firstToken: out.firstToken, request: transcriptOf(req),
		response: r.redactFor(e, out.text+"\n"+string(calls)), usage: out.usage, err: err,
		requestID: out.requestID, responseModel: out.model,
		ledger: ledger.Encode(ledger.Of(req, lp.newest)),
	})
	return out, err
}

/*
stream runs the call under its three clocks and collects the text,
tool calls, usage and stop reason.
*/
func (r *Runner) stream(ctx context.Context, llm contracts.LLM, req contracts.ChatRequest,
	started time.Time,
) (streamed, error) {
	ctx, cancel := r.timeout(ctx, r.Timeouts.GenerateTotal)
	defer cancel()
	ctx, stall := context.WithCancelCause(ctx)
	defer stall(context.Canceled)

	watchdog := r.Clock.AfterFunc(firstOr(r.Timeouts.GenerateFirst), func() { stall(context.DeadlineExceeded) })
	defer watchdog.Stop()

	s, err := llm.Stream(ctx, req)
	if err != nil {
		return streamed{}, err
	}
	defer func() { _ = s.Close() }()

	out := streamed{stop: contracts.StopReasonEnd}
	var b strings.Builder
	for {
		ev, err := s.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			out.text = b.String()
			return out, err
		}
		watchdog.Reset(firstOr(r.Timeouts.GenerateIdle))
		r.collect(&out, &b, ev, started)
	}
	out.text = strings.TrimSpace(b.String())
	return out, nil
}

/*
collect folds one stream event into the result.
*/
func (r *Runner) collect(out *streamed, b *strings.Builder, ev contracts.StreamEvent, started time.Time) {
	switch ev.Kind {
	case contracts.StreamEventTextDelta:
		if out.firstToken == 0 {
			out.firstToken = max(r.Clock.Now().Sub(started), time.Millisecond)
		}
		b.WriteString(ev.Text)
	case contracts.StreamEventToolCallDone:
		if ev.ToolCall != nil {
			out.calls = append(out.calls, *ev.ToolCall)
		}
	case contracts.StreamEventUsage:
		if ev.Usage != nil {
			out.usage = *ev.Usage
		}
		out.requestID, out.model = ev.RequestID, ev.Model
	case contracts.StreamEventDone:
		if ev.StopReason != "" {
			out.stop = ev.StopReason
		}
	}
}

/*
answerVars fills the answer prompt's placeholders.
*/
func (r *Runner) answerVars(st *run) prompts.Vars {
	e := st.env
	channel := "chat"
	if e.ChannelType == "email" {
		channel = "email"
	}
	product := strv(e.Agent.ProductName)
	if product == "" {
		product = e.Workspace.Name
	}
	return prompts.Vars{
		"agent_name": e.Agent.Name, "product_name": product, "rules": rulesText(e, st.settings),
		"tone":     r.render(e, "tone_"+toneOf(e.Agent.Tone)+".md", nil),
		"language": languageName(e.Agent.DefaultLanguage), "guidance": guidanceText(st),
		"channel": channel, "handoff": e.availability(),
	}
}

/*
render renders one prompt file of the run's version for the answer
model's family; a render failure renders empty, since the boot check
already proved every file exists.
*/
func (r *Runner) render(e *env, name string, vars prompts.Vars) string {
	if r.Prompts == nil {
		return ""
	}
	file, err := r.Prompts.Render(e.Version, r.family(e.AnswerKey, e.Agent.AnswerModel), name, vars)
	if err != nil {
		return ""
	}
	return file.Content
}

/*
family resolves a model's prompt family from its connection.
*/
func (r *Runner) family(key *models.AiProviderKey, model *string) enums.AIModelFamily {
	provider := enums.AIProviderDimmi
	if key != nil {
		provider = enums.AIProvider(key.Provider)
	}
	return aimodels.FamilyOf(provider, strv(model))
}

/*
keyRef is the connection a call spends.
*/
type keyRef struct {
	id       ids.UUID
	provider string
}

/*
transcriptOf is the text of a request's turns, for its step row.
*/
func transcriptOf(req contracts.ChatRequest) string {
	var parts []string
	for _, m := range req.Messages {
		for _, p := range m.Parts {
			parts = append(parts, string(m.Role)+": "+p.Text)
		}
		for _, c := range m.ToolCalls {
			parts = append(parts, string(m.Role)+" calls "+c.Name+": "+string(c.Arguments))
		}
	}
	return joinParagraphs(parts)
}

/*
textParts wraps a text as message parts, none when empty.
*/
func textParts(text string) []contracts.ContentPart {
	if text == "" {
		return nil
	}
	return []contracts.ContentPart{{Text: text}}
}

/*
temperature reads the agent's sampling temperature, nil when unset.
*/
func temperature(e *env) *float64 {
	v, err := e.Agent.Temperature.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	return &v.Float64
}

/*
firstOr is a stream clock, defaulting a zero setting to a generous one.
*/
func firstOr(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Minute
	}
	return d
}

/*
toneOf maps the agent's tone to a tone file, friendly by default.
*/
func toneOf(tone string) string {
	switch tone {
	case "professional", "concise":
		return tone
	}
	return "friendly"
}

/*
answerEffort is the configured reasoning effort for answer calls, low
when unset.
*/
func (r *Runner) answerEffort() contracts.Effort {
	if r.AnswerEffort == "" {
		return contracts.EffortLow
	}
	return r.AnswerEffort
}

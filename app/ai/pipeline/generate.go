/*
|--------------------------------------------------------------------------
| Generate
|--------------------------------------------------------------------------
|
| The answer model writes the reply in a fixed shape: a STATUS line
| saying answered, unknown or handoff, a blank line, then the reply.
| The stream is read under three clocks — time to the first token, idle
| time between tokens, and the whole call — and the customer sees the
| agent typing meanwhile. A reply that breaks the shape gets one short
| corrective turn; a second failure is a failed run that hands off.
|
*/

package pipeline

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
protocolRetry is the corrective turn after a reply without a valid
STATUS line.
*/
const protocolRetry = "Your reply must start with a line that is exactly STATUS: answered, " +
	"STATUS: unknown or STATUS: handoff, then a blank line, then the reply. Write it again in that format."

/*
errProtocol marks an answer that twice ignored the output protocol.
*/
var errProtocol = errors.New("answer did not follow the output protocol")

/*
streamed is what one generate call produced.
*/
type streamed struct {
	text       string
	usage      contracts.LLMUsage
	stop       contracts.StopReason
	firstToken time.Duration
}

/*
answer runs the answer model and post-processes its reply.
*/
func (r *Runner) answer(ctx context.Context, st *run) *decision {
	e := st.env
	llm, err := r.LLM.ForKey(ctx, *e.AnswerKey)
	if err != nil {
		return failed("provider_unavailable", err)
	}

	stopTyping := r.typing(ctx, e)
	defer stopTyping()

	call := answerCall{
		llm: llm, system: r.render(e, "answer.system.md", r.answerVars(st)),
		messages: []contracts.ChatMessage{userMessage(r.answerEnvelope(e, st.sources))},
	}

	out, err := r.generate(ctx, st, call)
	if err != nil {
		return failed("provider_error", err)
	}

	p, ok := parseStatus(out)
	if !ok {
		call.messages = append(call.messages, assistantMessage(out.text), userMessage(protocolRetry))
		if out, err = r.generate(ctx, st, call); err != nil {
			return failed("provider_error", err)
		}
		if p, ok = parseStatus(out); !ok {
			return failed("protocol_error", errProtocol)
		}
	}

	return r.postProcess(ctx, st, p)
}

/*
answerCall is one answer model call: the driver, the rendered system
prompt and the turns so far.
*/
type answerCall struct {
	llm      contracts.LLM
	system   string
	messages []contracts.ChatMessage
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
		"agent_name": e.Agent.Name, "product_name": product,
		"tone":     r.render(e, "tone_"+toneOf(e.Agent.Tone)+".md", nil),
		"language": languageName(replyLanguage(e, st.c)), "guidance": e.Agent.Guidance,
		"channel": channel, "handoff": e.availability(),
	}
}

/*
generate streams one answer call and records its step, whatever the
outcome.
*/
func (r *Runner) generate(ctx context.Context, st *run, call answerCall) (streamed, error) {
	e := st.env
	llm, system, messages := call.llm, call.system, call.messages
	model := strv(e.Agent.AnswerModel)
	req := contracts.ChatRequest{
		Model: model, System: []contracts.SystemBlock{{Text: system, Cacheable: true}},
		Messages: messages, MaxOutputTokens: int(e.Agent.MaxOutputTokens), CacheHint: true,
		Temperature: temperature(e), Timeout: contracts.StepTimeouts{
			FirstToken: r.Timeouts.GenerateFirst, Idle: r.Timeouts.GenerateIdle, Total: r.Timeouts.GenerateTotal,
		},
	}

	started := r.Clock.Now()
	out, err := r.stream(ctx, llm, req, started)
	r.step(ctx, st, stepRecord{
		kind: enums.AIRunStepKindGenerate, llm: llm, key: &keyRef{id: e.AnswerKey.ID, provider: e.AnswerKey.Provider},
		model: model, attempt: (len(messages) + 1) / 2, started: started, firstToken: out.firstToken,
		request: system + "\n\n" + transcriptOf(req), response: out.text, usage: out.usage, err: err,
	})
	return out, err
}

/*
stream runs the call under its three clocks and collects the text,
usage and stop reason.
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
	case contracts.StreamEventUsage:
		if ev.Usage != nil {
			out.usage = *ev.Usage
		}
	case contracts.StreamEventDone:
		if ev.StopReason != "" {
			out.stop = ev.StopReason
		}
	case contracts.StreamEventToolCallDone:
	}
}

/*
statusLine matches the protocol's first line.
*/
func statusLine() *regexp.Regexp {
	return regexp.MustCompile(`(?i)^\**\s*status\s*:\s*(answered|unknown|handoff)\s*\**$`)
}

/*
parseStatus splits a reply into its status and the customer-facing
text.
*/
func parseStatus(out streamed) (parsed, bool) {
	first, rest, _ := strings.Cut(strings.TrimSpace(out.text), "\n")
	m := statusLine().FindStringSubmatch(strings.TrimSpace(first))
	if m == nil {
		return parsed{}, false
	}
	return parsed{status: strings.ToLower(m[1]), reply: strings.TrimSpace(rest), stop: out.stop}, true
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

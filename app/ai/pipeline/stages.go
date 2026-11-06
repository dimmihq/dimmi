/*
|--------------------------------------------------------------------------
| Pipeline Stages
|--------------------------------------------------------------------------
|
| Each stage does one thing and records one or more steps: retrieve
| asks the knowledge index, generate writes the answer on the answer
| model, and the commits write the outcome transactionally, after a
| last look for anything newer.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/linkguard"
)

/*
classification is the run's decision state: no model fills it yet —
the dedicated decisions model is out of scope — so the pipeline
defaults it from the trigger and the agent's settings.
*/
type classification struct {
	Intent         string `json:"intent"`
	Language       string `json:"language"`
	QueryRewritten string `json:"query_rewritten"`
	Safety         string `json:"safety"`
	Summary        string `json:"summary"`
}

/*
retrieval is what the index returned plus the answerability verdict.
*/
type retrieval struct {
	Chunks     []contracts.KnowledgeChunk
	Query      string
	Answerable bool
}

/*
flow carries what a stage needs: the environment, the recorder and
the artifacts earlier stages produced.
*/
type flow struct {
	env      *env
	recorder *Recorder
	c        *classification
	found    *retrieval
}

/*
answer is the generated reply.
*/
type answer struct {
	Text       string
	handoff    string
	unknown    bool
	FirstToken time.Duration
	Latency    time.Duration
}

/*
retrieve asks the knowledge index and decides answerability.
*/
func (r *Runner) retrieve(ctx context.Context, f *flow) (*retrieval, error) {
	query := f.c.QueryRewritten
	if query == "" {
		query = triggerText(f.env.Trigger)
	}

	started := r.Clock.Now()
	res, err := r.Knowledge.Search(ctx, f.env.Workspace.ID, f.env.redact(query),
		contracts.KnowledgeSearchOptions{
			Limit:      6,
			Visibility: []contracts.KnowledgeVisibility{contracts.KnowledgeVisibilityPublic},
			Surface:    "agent",
		})
	if err != nil {
		r.stepErr(ctx, f, stepError{kind: enums.AIRunStepKindRetrieve, started: started, err: err})
		return nil, fmt.Errorf("retrieve: %w", err)
	}

	chunks := res.Chunks
	answerable := len(chunks) > 0
	if f8, err := f.env.Agent.MinSimilarity.Float64Value(); err == nil && f8.Valid {
		threshold := f8.Float64
		answerable = false
		for _, ch := range chunks {
			if ch.Similarity == nil || *ch.Similarity >= threshold {
				answerable = true
				break
			}
		}
	}

	payload, _ := json.Marshal(chunks)
	if _, err := f.recorder.Step(ctx, f.env.tx, StepInput{
		Kind: enums.AIRunStepKindRetrieve, Status: "ok",
		Latency: r.Clock.Now().Sub(started), StartedAt: started, CompletedAt: r.Clock.Now(),
		RequestPayload: r.payload(f.env, query), ResponsePayload: r.payload(f.env, string(payload)),
	}); err != nil {
		return nil, fmt.Errorf("record retrieve step: %w", err)
	}

	return &retrieval{Chunks: chunks, Query: query, Answerable: answerable}, nil
}

/*
generate writes the answer on the answer model, streaming into a
buffer; the stream's first token is timed.
*/
func (r *Runner) generate(ctx context.Context, f *flow) (*answer, error) {
	llm, err := r.driverFor(ctx, f.env.AnswerKey, f.env.Agent.AnswerModel)
	if err != nil {
		return nil, err
	}
	model := strv(f.env.Agent.AnswerModel)

	system, sources, err := r.answerPrompt(f.env, f.c, f.found)
	if err != nil {
		return nil, err
	}

	started := r.Clock.Now()

	stream, err := llm.Stream(ctx, contracts.ChatRequest{
		Model:  model,
		System: []contracts.SystemBlock{{Text: system, Cacheable: true}},
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: sources}}},
		},
		MaxOutputTokens: int(f.env.Agent.MaxOutputTokens),
		CacheHint:       true,
	})
	if err != nil {
		r.stepErr(ctx, f, stepError{kind: enums.AIRunStepKindGenerate, model: model, started: started, err: err})
		return nil, fmt.Errorf("generate: %w", err)
	}
	defer func() { _ = stream.Close() }()

	got, err := r.streamAnswer(ctx, stream, started)
	if err != nil {
		r.stepErr(ctx, f, stepError{kind: enums.AIRunStepKindGenerate, model: model, started: started, err: err})
		return nil, fmt.Errorf("generate stream: %w", err)
	}
	latency := r.Clock.Now().Sub(started)
	text, usage, firstToken := got.text, got.usage, got.firstToken

	if _, err := f.recorder.Step(ctx, f.env.tx, StepInput{
		Kind: enums.AIRunStepKindGenerate, Provider: string(llm.Provider()),
		ProviderKeyID: keyID(f.env.AnswerKey), Model: model, Status: "ok",
		Usage: usage, Latency: latency, FirstToken: firstToken,
		StartedAt: started, CompletedAt: r.Clock.Now(),
		RequestPayload: r.payload(f.env, sources), ResponsePayload: r.payload(f.env, text),
	}); err != nil {
		return nil, fmt.Errorf("record generate step: %w", err)
	}

	out := &answer{Text: text, FirstToken: firstToken, Latency: latency}

	if saysUnknown(text) {
		out.unknown = true
	}

	return out, nil
}

/*
answerPrompt renders the family prompt and the numbered source list.
*/
func (r *Runner) answerPrompt(env *env, c *classification, found *retrieval) (string, string, error) {
	vars := map[string]string{
		"agent_name":   env.Agent.Name,
		"product_name": product(env),
		"guidance":     env.Agent.Guidance,
		"rules":        standingRules(env),
		"tone":         env.Agent.Tone,
		"language":     languageFor(env, c),
	}

	system, err := r.prompt(env, "answer.system.md", vars)
	if err != nil {
		return "", "", err
	}

	var b strings.Builder
	b.WriteString("SOURCES\n")
	for i, ch := range found.Chunks {
		fmt.Fprintf(&b, "[%d] %s\n%s\nURL: %s\n\n", i+1, ch.Title, ch.Text, ch.URL)
	}
	fmt.Fprintf(&b, "TRANSCRIPT\n%s", transcriptForLLM(env, 12))

	return system, b.String(), nil
}

/*
postProcess applies the deterministic rewrites: link allowlist and
the citation requirement.
*/
func postProcess(env *env, text string, _ *retrieval) string {
	allowed := linkHosts(env)
	return linkguard.Filter(text, allowed)
}

/*
linkHosts builds the allowlist from the index's public hosts plus the
agent's own.
*/
func linkHosts(env *env) []string {
	return append([]string{}, env.Agent.LinkAllowlist...)
}

/*
writer builds the writer acting as the agent.
*/
func (r *Runner) writer(env *env) *conversations.Writer {
	return &conversations.Writer{
		Q: models.New(env.tx), Tx: env.tx, Events: r.Events,
		Now:   r.Clock.Now(),
		Actor: conversations.Actor{Type: "agent", ID: env.Agent.ID, Name: env.Agent.Name},
	}
}

/*
driverFor resolves a role's driver.
*/
func (r *Runner) driverFor(ctx context.Context, key *models.AiProviderKey, model *string) (llm, error) {
	if key == nil || model == nil {
		return nil, fmt.Errorf("model not configured")
	}
	return r.LLM.ForKey(ctx, *key)
}

/*
llm is the contracts interface.
*/
type llm = contracts.LLM

/*
prompt renders one file for the agent's family.
*/
func (r *Runner) prompt(env *env, name string, vars map[string]string) (string, error) {
	if r.Prompts == nil {
		return "", nil
	}
	family := enums.AIModelFamilyOpenWeight
	if env.Agent.AnswerModel != nil {
		family = familyForKey(env.AnswerKey, *env.Agent.AnswerModel)
	}
	file, err := r.Prompts.RenderFile(family, name, vars)
	if err != nil {
		return "", err
	}
	return file.Content, nil
}

/*
stepError describes one failed stage for its step row.
*/
type stepError struct {
	kind    enums.AIRunStepKind
	model   string
	started time.Time
	err     error
}

/*
stepErr records a failed step.
*/
func (r *Runner) stepErr(ctx context.Context, f *flow, fail stepError) {
	code := ""
	httpStatus := 0
	if llmErr, ok := errors.AsType[*contracts.LLMError](fail.err); ok {
		code = string(llmErr.Kind)
		httpStatus = llmErr.HTTPStatus
	}
	_, _ = f.recorder.Step(ctx, f.env.tx, StepInput{
		Kind: fail.kind, Model: fail.model, Status: "error", ErrorKind: code, HTTPStatus: httpStatus,
		Latency: r.Clock.Now().Sub(fail.started), StartedAt: fail.started,
		CompletedAt: r.Clock.Now(),
	})
}

/*
payload trims a stored request/response payload.
*/
func (r *Runner) payload(_ *env, text string) []byte {
	if len(text) > 64_000 {
		text = text[:64_000]
	}
	out, _ := json.Marshal(text)
	return out
}

/*
keyID maps a key row to its id pointer.
*/
func keyID(key *models.AiProviderKey) *ids.UUID {
	if key == nil {
		return nil
	}
	return &key.ID
}

/*
product names the product the agent speaks for.
*/
func product(env *env) string {
	if env.Agent.ProductName != nil && *env.Agent.ProductName != "" {
		return *env.Agent.ProductName
	}
	return env.Workspace.Name
}

/*
standingRules is the workspace's rules block.
*/
func standingRules(_ *env) string {
	return "Always offer a real person when the customer asks or when you cannot answer."
}

/*
languageFor picks the reply language.
*/
func languageFor(env *env, c *classification) string {
	if c != nil && c.Language != "" {
		return c.Language
	}
	return env.Agent.DefaultLanguage
}

/*
messageFor reads one configurable line, with its default.
*/
func messageFor(env *env, key string) string {
	defaults := map[string]string{
		"unknown_offer": "I don't have an answer for that. Want me to bring in someone from the team?",
		"handoff_line":  "I'm bringing in someone from the team.",
	}
	var msgs map[string]any
	if err := json.Unmarshal(env.Agent.Messages, &msgs); err == nil {
		if v, ok := msgs[key].(string); ok && v != "" {
			return v
		}
	}
	return defaults[key]
}

/*
saysUnknown reports whether the answer declines to answer.
*/
func saysUnknown(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "don't have an answer") ||
		strings.Contains(lower, "do not have an answer") ||
		strings.Contains(lower, "i don't know") ||
		strings.Contains(lower, "cannot find")
}

/*
transcriptForLLM renders the tail of the conversation.
*/
func transcriptForLLM(env *env, count int) string {
	msgs := env.Messages
	if len(msgs) > count {
		msgs = msgs[len(msgs)-count:]
	}

	var b strings.Builder
	for _, m := range msgs {
		who := "Customer"
		switch m.AuthorType {
		case "member":
			who = "Teammate"
		case "agent":
			who = env.Agent.Name
		case "system":
			who = "System"
		}
		text := triggerText(m)
		text = env.redact(text)
		if len(text) > 2000 {
			text = text[:2000]
		}
		fmt.Fprintf(&b, "%s: %s\n\n", who, text)
	}
	return b.String()
}

/*
streamAnswer consumes the generate stream into text and usage.
*/
/*
Streamed is what the generate stream produced.
*/
type streamed struct {
	text       string
	usage      contracts.LLMUsage
	firstToken time.Duration
}

/*
streamAnswer consumes the generate stream into text and usage.
*/
func (r *Runner) streamAnswer(ctx context.Context, stream contracts.ChatStream,
	started time.Time,
) (streamed, error) {
	out := streamed{}
	var buf strings.Builder

	for {
		ev, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return out, err
		}
		switch ev.Kind {
		case contracts.StreamEventTextDelta:
			if out.firstToken == 0 {
				out.firstToken = r.Clock.Now().Sub(started)
			}
			buf.WriteString(ev.Text)
		case contracts.StreamEventUsage:
			if ev.Usage != nil {
				out.usage = *ev.Usage
			}
		case contracts.StreamEventToolCallDone, contracts.StreamEventDone:
		}
	}
	out.text = strings.TrimSpace(buf.String())
	return out, nil
}

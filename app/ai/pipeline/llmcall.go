/*
|--------------------------------------------------------------------------
| Model Calls
|--------------------------------------------------------------------------
|
| The small non-streaming calls — classify, verify, summarize — share
| one shape: a rendered system prompt, one envelope, a timeout on the
| runner's clock, and a step row written the moment the call returns.
| A call that must answer JSON gets one corrective retry before its
| caller falls back.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
keyRef is the connection a call spends.
*/
type keyRef struct {
	id       ids.UUID
	provider string
}

/*
jsonCall is one non-streaming call: its step kind, driver, connection,
model, prompts, timeout and output cap, and for a JSON answer the
check a decoded answer must pass.
*/
type jsonCall struct {
	kind      enums.AIRunStepKind
	llm       contracts.LLM
	key       *keyRef
	model     string
	system    string
	user      string
	timeout   time.Duration
	maxTokens int
	valid     func(any) bool
}

/*
jsonRetry is the corrective turn after a reply that is not valid JSON.
*/
const jsonRetry = "Return valid JSON only, exactly in the shape the instructions describe, with no other text."

/*
callJSON runs the call and decodes its JSON answer into out, retrying
once with a corrective turn. It reports whether out holds a valid
answer.
*/
func (r *Runner) callJSON(ctx context.Context, st *run, call jsonCall, out any) bool {
	messages := []contracts.ChatMessage{userMessage(call.user)}
	for range 2 {
		res, err := r.chat(ctx, st, call, messages)
		if err != nil {
			return false
		}
		if decodeJSON(res.Text, out) && call.valid(out) {
			return true
		}
		messages = append(messages, assistantMessage(res.Text), userMessage(jsonRetry))
	}
	return false
}

/*
chat runs one non-streaming call and records its step.
*/
func (r *Runner) chat(ctx context.Context, st *run, call jsonCall, messages []contracts.ChatMessage,
) (*contracts.ChatResponse, error) {
	ctx, cancel := r.timeout(ctx, call.timeout)
	defer cancel()

	req := contracts.ChatRequest{
		Model: call.model, System: []contracts.SystemBlock{{Text: call.system, Cacheable: true}},
		Messages: messages, MaxOutputTokens: call.maxTokens, CacheHint: true,
		Timeout: contracts.StepTimeouts{Total: call.timeout},
	}
	if call.kind != enums.AIRunStepKindSummarize && call.llm.Capabilities(call.model).JSONMode {
		req.ResponseFormat = contracts.ResponseFormatJSON
	}

	started := r.Clock.Now()
	res, err := call.llm.Chat(ctx, req)
	rec := stepRecord{
		kind: call.kind, llm: call.llm, key: call.key, model: call.model, attempt: (len(messages) + 1) / 2,
		started: started, request: transcriptOf(req), err: err,
	}
	if res != nil {
		rec.usage, rec.response, rec.responseModel, rec.requestID = res.Usage, res.Text, res.Model, res.RequestID
	}
	r.step(ctx, st, rec)
	if err == nil && res == nil {
		err = errors.New("empty response")
	}
	return res, err
}

/*
userMessage is one user turn.
*/
func userMessage(text string) contracts.ChatMessage {
	return contracts.ChatMessage{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: text}}}
}

/*
assistantMessage is one assistant turn.
*/
func assistantMessage(text string) contracts.ChatMessage {
	return contracts.ChatMessage{Role: contracts.ChatRoleAssistant, Parts: []contracts.ContentPart{{Text: text}}}
}

/*
transcriptOf is the text of a request's messages, for its step row.
*/
func transcriptOf(req contracts.ChatRequest) string {
	var parts []string
	for _, m := range req.Messages {
		for _, p := range m.Parts {
			parts = append(parts, string(m.Role)+": "+p.Text)
		}
	}
	return joinParagraphs(parts)
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
cheapLLM is the answer connection with its provider's cheap model, for
the generative side tasks: the handoff note and the grounding check.
*/
func (r *Runner) cheapLLM(ctx context.Context, e *env) (contracts.LLM, string, *keyRef) {
	key := e.AnswerKey
	if key == nil || r.LLM == nil {
		return nil, "", nil
	}
	llm, err := r.LLM.ForKey(ctx, *key)
	if err != nil {
		return nil, "", nil
	}
	return llm, cheapModel(strv(e.Agent.AnswerModel)), &keyRef{id: key.ID, provider: key.Provider}
}

/*
cheapModel is the small model of an answer model's family, or the
answer model itself when its family has none known.
*/
func cheapModel(answer string) string {
	switch {
	case strings.HasPrefix(answer, "claude"):
		return "claude-haiku-5-5"
	case strings.HasPrefix(answer, "gpt"):
		return "gpt-6-luna"
	case strings.HasPrefix(answer, "glm"):
		return "glm-5.3-flash"
	}
	return answer
}

/*
decodeJSON reads the first JSON object in a text, tolerating a code
fence or stray prose around it.
*/
func decodeJSON(text string, out any) bool {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return false
	}
	return json.Unmarshal([]byte(text[start:end+1]), out) == nil
}

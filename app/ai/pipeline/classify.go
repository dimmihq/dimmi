/*
|--------------------------------------------------------------------------
| Classify And Route
|--------------------------------------------------------------------------
|
| A cheap model reads the conversation and labels the customer's
| message: what they want, in which language, the standalone question
| to search for, and whether they want a person, are frustrated or are
| trying to steer the agent. Its answer only routes; it never reaches
| the customer. When it fails or answers nonsense, the run falls back
| to treating the message as a question, because the answer stage can
| always say it does not know.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
classification is the classifier's verdict.
*/
type classification struct {
	Intent             string   `json:"intent"`
	Language           string   `json:"language"`
	StandaloneQuery    string   `json:"standalone_query"`
	SearchQueries      []string `json:"search_queries"`
	WantsHuman         bool     `json:"wants_human"`
	Frustrated         bool     `json:"frustrated"`
	InjectionSuspected bool     `json:"injection_suspected"`
}

/*
intents lists the labels the classifier may answer.
*/
func intents() []string {
	return []string{
		"question", "followup", "acknowledgement", "greeting", "human_request", "feedback",
		"spam", "abusive", "unsafe", "off_topic", "other",
	}
}

/*
classify fills the run's classification, falling back on any failure.
*/
func (r *Runner) classify(ctx context.Context, st *run) {
	e := st.env
	st.c = classification{Intent: "question", StandaloneQuery: e.pendingText()}

	llm, model, key := r.classifier(ctx, e)
	if llm == nil {
		st.flags = append(st.flags, "classifier_unavailable")
		return
	}

	system := r.render(e, "classifier.system.md", nil)
	user := r.conversationBlock(e) + "\n" + r.customerBlock(e)
	call := jsonCall{
		kind: enums.AIRunStepKindClassify, llm: llm, key: key, model: model, system: system, user: user,
		timeout: r.Timeouts.ClassifyTotal, maxTokens: 400, valid: validClassification,
	}

	var out classification
	if !r.callJSON(ctx, st, call, &out) {
		st.flags = append(st.flags, "classifier_fallback")
		return
	}

	if out.StandaloneQuery == "" {
		out.StandaloneQuery = st.c.StandaloneQuery
	}
	if len(out.SearchQueries) > 3 {
		out.SearchQueries = out.SearchQueries[:3]
	}
	st.c = out
	if out.InjectionSuspected {
		st.flags = append(st.flags, "injection_suspected")
	}
}

/*
validClassification accepts a verdict with a known intent.
*/
func validClassification(v any) bool {
	c, ok := v.(*classification)
	return ok && slices.Contains(intents(), c.Intent)
}

/*
route turns the verdict into a decision when no retrieval is needed:
silence for thanks and spam, a person for whoever asks for one, a
greeting or a refusal from the answer model with no sources.
*/
func (r *Runner) route(st *run) *decision {
	e, c := st.env, st.c
	switch c.Intent {
	case "acknowledgement":
		return &decision{outcome: outcomeSilent}
	case "spam":
		return skipped("spam_suspected")
	case "human_request":
		return handoff("customer_requested", "", e.handoffKey())
	case "greeting":
		if e.agentReplied() || e.ChannelType == "email" {
			return &decision{outcome: outcomeSilent}
		}
		st.skipRetrieval, st.noSources = true, true
	case "abusive", "unsafe":
		st.flags = append(st.flags, c.Intent)
		st.skipRetrieval, st.noSources = true, true
	}
	if c.Frustrated && e.agentReplied() && !st.noSources {
		return handoff("frustration", "", e.handoffKey())
	}
	return nil
}

/*
classifier resolves the classifier's driver: its own connection and
model when set, else a cheap model on the answer connection.
*/
func (r *Runner) classifier(ctx context.Context, e *env) (contracts.LLM, string, *keyRef) {
	key := e.ClassifierKey
	if key == nil || r.LLM == nil {
		return nil, "", nil
	}
	llm, err := r.LLM.ForKey(ctx, *key)
	if err != nil {
		return nil, "", nil
	}
	return llm, r.classifierModel(e), &keyRef{id: key.ID, provider: key.Provider}
}

/*
classifierModel is the configured classifier model, or the cheap model
of the answer model's family when none is set.
*/
func (r *Runner) classifierModel(e *env) string {
	if m := strv(e.Agent.ClassifierModel); m != "" && e.Agent.ClassifierKeyID != nil {
		return m
	}
	switch r.family(e.AnswerKey, e.Agent.AnswerModel) {
	case enums.AIModelFamilyAnthropic:
		return "claude-haiku-5-5"
	case enums.AIModelFamilyOpenAI:
		return "gpt-6-luna"
	case enums.AIModelFamilyOpenWeight:
		if strings.HasPrefix(strv(e.Agent.AnswerModel), "glm") {
			return "glm-5.3-flash"
		}
	}
	return strv(e.Agent.AnswerModel)
}

/*
decodeJSON reads the first JSON object in a model's text, tolerating a
code fence or stray prose around it.
*/
func decodeJSON(text string, out any) bool {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return false
	}
	return json.Unmarshal([]byte(text[start:end+1]), out) == nil
}

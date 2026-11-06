/*
|--------------------------------------------------------------------------
| Classify And Route
|--------------------------------------------------------------------------
|
| A decision model, not a language model, labels the customer's message:
| what they want, which language they write in, and whether they want a
| person, are frustrated or are trying to steer the agent. All of it is
| one request of parallel questions answered with probabilities, so the
| pipeline routes on confidence instead of parsing prose. Its answer
| only routes; it never reaches the customer. Without a decision model,
| or when it fails, the message is treated as a question in the agent's
| default language, because the answer stage can always say it does
| not know.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
Confidence thresholds: an intent or language below its minimum
confidence is not trusted, and a yes-or-no question counts as yes from
noulTrue up.
*/
const (
	intentMinConfidence   = 0.5
	languageMinConfidence = 0.6
	noulTrue              = 0.7
)

/*
classification is the run's routing verdict.
*/
type classification struct {
	Intent             string
	Language           string
	WantsHuman         bool
	Frustrated         bool
	InjectionSuspected bool
}

/*
classify fills the run's classification, falling back on any failure.
*/
func (r *Runner) classify(ctx context.Context, st *run) {
	e := st.env
	st.c = classification{Intent: "question"}
	if r.Decider == nil {
		st.flags = append(st.flags, "decider_unavailable")
		return
	}

	req := contracts.DecisionRequest{
		State: r.conversationBlock(e) + "\n" + r.customerBlock(e), Questions: decisionQuestions(),
	}
	callCtx, cancel := r.timeout(ctx, r.Timeouts.ClassifyTotal)
	started := r.Clock.Now()
	res, err := r.Decider.Decide(callCtx, req)
	cancel()

	r.step(ctx, st, r.decisionStep(req, res, started, err))
	if err != nil {
		st.flags = append(st.flags, "classifier_fallback")
		return
	}

	st.c = verdictOf(res)
	if st.c.InjectionSuspected {
		st.flags = append(st.flags, "injection_suspected")
	}
}

/*
decisionStep is the classify step's record, priced from the configured
input price; output tokens are free.
*/
func (r *Runner) decisionStep(req contracts.DecisionRequest, res *contracts.DecisionResponse,
	started time.Time, err error,
) stepRecord {
	rec := stepRecord{
		kind: enums.AIRunStepKindClassify, provider: "typesafe", model: r.DecisionModel,
		started: started, request: req.State, err: err,
	}
	if res == nil {
		return rec
	}
	answers, _ := json.Marshal(res.Answers)
	rec.response, rec.responseModel = string(answers), res.Model
	rec.usage = contracts.LLMUsage{
		InputTokens: res.Usage.InputTokens, OutputTokens: res.Usage.OutputTokens, Reported: true,
	}
	rec.cost = new(int64(math.Round(float64(res.Usage.InputTokens) * r.DecisionPricePerMtok)))
	return rec
}

/*
verdictOf reads the answers through the confidence thresholds.
*/
func verdictOf(res *contracts.DecisionResponse) classification {
	c := classification{Intent: "question"}
	if a := res.Answers["intent"]; a.Confidence >= intentMinConfidence && slices.Contains(intents(), a.Choice) {
		c.Intent = a.Choice
	}
	if a := res.Answers["language"]; a.Confidence >= languageMinConfidence && a.Choice != "other" {
		c.Language = a.Choice
	}
	c.WantsHuman = res.Answers["wants_human"].Noul >= noulTrue
	c.Frustrated = res.Answers["frustrated"].Noul >= noulTrue
	c.InjectionSuspected = res.Answers["injection_suspected"].Noul >= noulTrue
	return c
}

/*
route turns the verdict into a decision when no retrieval is needed:
silence for thanks and spam, a person for whoever mainly asks for one,
a greeting or a careful reply from the answer model with no sources.
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

/*
|--------------------------------------------------------------------------
| Decisions
|--------------------------------------------------------------------------
|
| Every stage resolves to one decision: what the customer sees, why,
| and how the run is recorded. The commit turns the decision into rows;
| nothing before it writes a message.
|
*/

package pipeline

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
The run outcomes a decision can carry.
*/
const (
	outcomeAnswered   = "answered"
	outcomeUnknown    = "unknown"
	outcomeHandoff    = "handoff"
	outcomeSilent     = "silent"
	outcomeGreeted    = "greeted"
	outcomeRefused    = "refused"
	outcomeSkipped    = "skipped"
	outcomeSuperseded = "superseded"
	outcomeError      = "error"
	outcomeDrafted    = "drafted"
)

/*
The conversation's agent states.
*/
const (
	aiStateOff       = "off"
	aiStateActive    = "active"
	aiStateHandedOff = "handed_off"
)

/*
decision is what a run resolved to. The reply is the customer-facing
text; when it is empty, the replyKey names the default line used
instead. The citations are the sources the reply cites; substantive
marks a real answer and offerHandoff a reply the widget offers a
person under. An errorCode and errorMessage mark a failed run, which
still hands off.
*/
type decision struct {
	outcome       string
	skipReason    string
	handoffReason string
	reply         string
	replyKey      string
	citations     []citation
	substantive   bool
	offerHandoff  bool
	errorCode     string
	errorMessage  string
	answerID      *ids.UUID
}

/*
citation is one source an answer cites, as the message metadata keeps
it for the widget and the channels to render.
*/
type citation struct {
	N          int     `json:"n"`
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity"`
}

/*
skipped is a decision to stay quiet for a reason.
*/
func skipped(reason string) *decision {
	return &decision{outcome: outcomeSkipped, skipReason: reason}
}

/*
handoff is a decision to hand the conversation to the team, with the
model's reply or, when there is none, the default line for the key.
*/
func handoff(reason, reply, key string) *decision {
	return &decision{outcome: outcomeHandoff, handoffReason: reason, reply: reply, replyKey: key}
}

/*
unknown is the honest no-answer, with the model's reply or the
default line.
*/
func unknown(reply string) *decision {
	return &decision{outcome: outcomeUnknown, reply: reply, replyKey: "unknown"}
}

/*
failed is a run the pipeline could not finish: it is recorded as an
error and hands the conversation to the team.
*/
func failed(code string, err error) *decision {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &decision{
		outcome: outcomeError, handoffReason: "provider_error", errorCode: code, errorMessage: msg,
	}
}

/*
status is the run row status for the decision.
*/
func (d *decision) status() string {
	switch d.outcome {
	case outcomeSkipped:
		return "skipped"
	case outcomeError:
		return "failed"
	}
	return "completed"
}

/*
result maps the decision to the runner's result.
*/
func (d *decision) result(runID ids.UUID) *Result {
	return &Result{
		Outcome: d.outcome, SkipReason: d.skipReason, HandoffReason: d.handoffReason,
		AnswerID: d.answerID, RunID: runID,
	}
}

/*
writesMessage reports whether the decision posts a reply.
*/
func (d *decision) writesMessage() bool {
	switch d.outcome {
	case outcomeSkipped, outcomeSilent, outcomeSuperseded:
		return false
	}
	return true
}

/*
retrievalJSON describes the sources a run kept, for the run row.
*/
func retrievalJSON(sources []source) json.RawMessage {
	out := make([]map[string]any, 0, len(sources))
	for _, s := range sources {
		out = append(out, map[string]any{
			"n": s.n, "chunk_id": s.chunk.ID.String(), "title": s.chunk.Title,
			"url": s.chunk.URL, "similarity": similarityOf(s.chunk.Similarity),
		})
	}
	data, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage("[]")
	}
	return data
}

/*
similarityOf reads a nullable similarity.
*/
func similarityOf(s *float64) float64 {
	if s == nil {
		return 0
	}
	return *s
}

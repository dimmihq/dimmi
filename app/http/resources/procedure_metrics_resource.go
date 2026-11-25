/*
|--------------------------------------------------------------------------
| Procedure Scorecard Resources
|--------------------------------------------------------------------------
|
| A procedure's numbers as the API reads them, and the report row of
| every procedure side by side. A rate is null when nothing was counted.
| Handovers are split by why: the procedure asking for one, the customer
| asking, a guardrail stepping in, or something failing.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ProcedureMetrics is a procedure's scorecard.
*/
type ProcedureMetrics struct {
	Object               string                    `json:"object"`
	ProcedureID          string                    `json:"procedure_id"`
	Days                 int                       `json:"days"`
	Version              *int32                    `json:"version"`
	MatchedConversations int32                     `json:"matched_conversations"`
	AIInvolved           int32                     `json:"ai_involved"`
	AIResolved           int32                     `json:"ai_resolved"`
	AIConfirmed          int32                     `json:"ai_confirmed"`
	AIAssumed            int32                     `json:"ai_assumed"`
	AIResolutionRate     *float64                  `json:"ai_resolution_rate"`
	HandedOff            int32                     `json:"handed_off"`
	HandoverRate         *float64                  `json:"handover_rate"`
	HandoverReasons      []ProcedureHandoverReason `json:"handover_reasons"`
	CSAT                 ProcedureCSAT             `json:"csat"`
	Guardrails           map[string]int32          `json:"guardrails"`
	Steps                []ProcedureStepFigure     `json:"steps"`
	Actions              []ProcedureActionFigure   `json:"actions"`
	Runs                 []ProcedureRunFigure      `json:"runs"`
}

/*
ProcedureHandoverReason is how many conversations were handed over for
one reason, and what kind of reason it is.
*/
type ProcedureHandoverReason struct {
	Reason        string `json:"reason"`
	Kind          string `json:"kind"`
	Conversations int32  `json:"conversations"`
}

/*
ProcedureCSAT is how customers rated the conversations.
*/
type ProcedureCSAT struct {
	Rated        int32    `json:"rated"`
	Positive     int32    `json:"positive"`
	PositiveRate *float64 `json:"positive_rate"`
	Mean         *float64 `json:"mean"`
}

/*
ProcedureStepFigure is how far conversations got through one step.
*/
type ProcedureStepFigure struct {
	StepID         string   `json:"step_id"`
	Reached        int32    `json:"reached"`
	Done           int32    `json:"done"`
	CompletionRate *float64 `json:"completion_rate"`
	TookThen       int32    `json:"took_then"`
	TookElse       int32    `json:"took_else"`
}

/*
ProcedureActionFigure is how one action fared in the procedure's runs.
*/
type ProcedureActionFigure struct {
	ActionID      string   `json:"action_id"`
	Name          string   `json:"name"`
	Calls         int32    `json:"calls"`
	Failed        int32    `json:"failed"`
	ErrorRate     *float64 `json:"error_rate"`
	MeanLatencyMS float64  `json:"mean_latency_ms"`
}

/*
NewProcedureMetrics shapes a procedure's scorecard.
*/
func NewProcedureMetrics(id ids.UUID, m procedures.Metrics) ProcedureMetrics {
	f := m.Figures
	out := ProcedureMetrics{
		Object: "procedure_metrics", ProcedureID: models.PublicID(models.PrefixProcedure, id), Days: m.Days,
		MatchedConversations: f.Matched, AIInvolved: f.Involved, AIResolved: f.Resolved, AIConfirmed: f.Confirmed,
		AIAssumed: f.Resolved - f.Confirmed, AIResolutionRate: ratio(f.Resolved, f.Involved),
		HandedOff: f.HandedOff, HandoverRate: ratio(f.HandedOff, f.Involved),
		HandoverReasons: []ProcedureHandoverReason{}, Guardrails: map[string]int32{},
		CSAT:  ProcedureCSAT{Rated: f.Rated, Positive: f.Positive, PositiveRate: ratio(f.Positive, f.Rated)},
		Steps: []ProcedureStepFigure{}, Actions: []ProcedureActionFigure{}, Runs: []ProcedureRunFigure{},
	}
	if m.Version > 0 {
		out.Version = &m.Version
	}
	if f.Rated > 0 {
		out.CSAT.Mean = &f.MeanRating
	}
	for _, h := range m.Handoffs {
		out.HandoverReasons = append(out.HandoverReasons, ProcedureHandoverReason{
			Reason: h.Reason, Kind: handoverKind(h.Reason), Conversations: h.Conversations,
		})
	}
	for _, g := range m.Guards {
		out.Guardrails[g.Type] = g.Events
	}
	for _, s := range m.Steps {
		out.Steps = append(out.Steps, ProcedureStepFigure{
			StepID: s.StepID, Reached: s.Reached, Done: s.Done, CompletionRate: ratio(s.Done, s.Reached),
			TookThen: s.TookThen, TookElse: s.TookElse,
		})
	}
	for _, a := range m.Actions {
		out.Actions = append(out.Actions, ProcedureActionFigure{
			ActionID: models.PublicID(models.PrefixAiAction, a.ActionID), Name: m.ActionNames[a.ActionID],
			Calls: a.Calls, Failed: a.Failed, ErrorRate: ratio(a.Failed, a.Calls), MeanLatencyMS: a.MeanLatencyMs,
		})
	}
	for _, r := range m.Runs {
		out.Runs = append(out.Runs, ProcedureRunFigure{
			RunID: models.PublicID(models.PrefixAiRun, r.ID), ConversationID: publicPtr(models.PrefixConversation,
				r.ConversationID), ConversationNumber: r.Number, Outcome: r.Outcome, HandoffReason: r.HandoffReason,
			Rating: r.Rating, Reopens: r.Reopens, ProcedureTrace: r.ProcedureTrace, CreatedAt: r.CreatedAt,
		})
	}
	return out
}

/*
handoverKind sorts a handover's reason: a guardrail stepping in, the
customer asking, something failing, or the procedure's own handover.
*/
func handoverKind(reason string) string {
	switch reason {
	case "procedure_guard":
		return "guardrail"
	case "customer_request", "customer_requested":
		return "customer"
	case "provider_error", "provider_unavailable", "action_error", "unknown", "unknown_streak":
		return "failure"
	}
	return "intentional"
}

/*
ratio is part over whole, nil when the whole is zero.
*/
func ratio(part, whole int32) *float64 {
	if whole == 0 {
		return nil
	}
	v := float64(part) / float64(whole)
	return &v
}

/*
|--------------------------------------------------------------------------
| Procedure Report Resources
|--------------------------------------------------------------------------
|
| One procedure in the procedures report, and one run behind a
| scorecard's numbers, for drilling in.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ProcedureRunFigure is one run behind the numbers, for drilling in.
*/
type ProcedureRunFigure struct {
	RunID              string          `json:"run_id"`
	ConversationID     *string         `json:"conversation_id"`
	ConversationNumber int64           `json:"conversation_number"`
	Outcome            *string         `json:"outcome"`
	HandoffReason      *string         `json:"handoff_reason"`
	Rating             *int16          `json:"rating"`
	Reopens            int32           `json:"reopens"`
	ProcedureTrace     json.RawMessage `json:"procedure_trace,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

/*
ProcedureReportRow is one procedure in the report.
*/
type ProcedureReportRow struct {
	ProcedureID      string   `json:"procedure_id"`
	Name             string   `json:"name"`
	Matched          int32    `json:"matched_conversations"`
	AIInvolved       int32    `json:"ai_involved"`
	AIResolved       int32    `json:"ai_resolved"`
	AIResolutionRate *float64 `json:"ai_resolution_rate"`
	HandedOff        int32    `json:"handed_off"`
	HandoverRate     *float64 `json:"handover_rate"`
	Rated            int32    `json:"rated"`
	MeanRating       *float64 `json:"mean_rating"`
	GuardEvents      int32    `json:"guard_events"`
}

/*
NewProcedureReportRow shapes one procedure of the report.
*/
func NewProcedureReportRow(r models.ReportProceduresRow) ProcedureReportRow {
	out := ProcedureReportRow{
		ProcedureID: models.PublicID(models.PrefixProcedure, r.ID), Name: r.Name, Matched: r.Matched,
		AIInvolved: r.Involved, AIResolved: r.Resolved, AIResolutionRate: ratio(r.Resolved, r.Involved),
		HandedOff: r.HandedOff, HandoverRate: ratio(r.HandedOff, r.Involved), Rated: r.Rated,
		GuardEvents: r.GuardEvents,
	}
	if r.Rated > 0 {
		out.MeanRating = &r.MeanRating
	}
	return out
}

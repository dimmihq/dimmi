/*
|--------------------------------------------------------------------------
| Fact Row
|--------------------------------------------------------------------------
|
| One conversation's row of facts, named like the columns of
| conversation_facts so it is written as JSON in one statement.
|
*/

package reports

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
FactRow is one conversation's row of facts.
*/
type FactRow struct {
	ConversationID               ids.UUID   `json:"conversation_id"`
	WorkspaceID                  ids.UUID   `json:"workspace_id"`
	Number                       int64      `json:"number"`
	InboxID                      ids.UUID   `json:"inbox_id"`
	Channel                      string     `json:"channel"`
	AssigneeMemberID             *ids.UUID  `json:"assignee_member_id"`
	AssigneeTeamID               *ids.UUID  `json:"assignee_team_id"`
	Priority                     int16      `json:"priority"`
	Status                       string     `json:"status"`
	Imported                     bool       `json:"imported"`
	StartedAt                    time.Time  `json:"started_at"`
	CustomerMessages             int        `json:"customer_messages"`
	LastCustomerMessageAt        *time.Time `json:"last_customer_message_at"`
	FirstResponseAt              *time.Time `json:"first_response_at"`
	FirstResponseSeconds         *int64     `json:"first_response_seconds"`
	FirstResponseBusinessSeconds *int64     `json:"first_response_business_seconds"`
	FirstResponderID             *ids.UUID  `json:"first_responder_id"`
	FirstAnswerAt                *time.Time `json:"first_answer_at"`
	FirstAnswerSeconds           *int64     `json:"first_answer_seconds"`
	FirstAnswerBusinessSeconds   *int64     `json:"first_answer_business_seconds"`
	HumanReplies                 int        `json:"human_replies"`
	FirstClosedAt                *time.Time `json:"first_closed_at"`
	FirstCloseSeconds            *int64     `json:"first_close_seconds"`
	FirstCloseBusinessSeconds    *int64     `json:"first_close_business_seconds"`
	ClosedAt                     *time.Time `json:"closed_at"`
	ResolutionSeconds            *int64     `json:"resolution_seconds"`
	ResolutionBusinessSeconds    *int64     `json:"resolution_business_seconds"`
	Closes                       int        `json:"closes"`
	Reopens                      int        `json:"reopens"`
	ClosedByMemberID             *ids.UUID  `json:"closed_by_member_id"`
	ClosedByTeamID               *ids.UUID  `json:"closed_by_team_id"`
	FirstContactResolved         bool       `json:"first_contact_resolved"`
	Rating                       *int16     `json:"rating"`
	RatedAt                      *time.Time `json:"rated_at"`
	RatedMemberID                *ids.UUID  `json:"rated_member_id"`
	AiInvolved                   bool       `json:"ai_involved"`
	AiAnswers                    int        `json:"ai_answers"`
	AiFirstAnswerAt              *time.Time `json:"ai_first_answer_at"`
	AiLastAnswerAt               *time.Time `json:"ai_last_answer_at"`
	AiHandedOffAt                *time.Time `json:"ai_handed_off_at"`
	AiHandoffReason              *string    `json:"ai_handoff_reason"`
	AiConfirmed                  bool       `json:"ai_confirmed"`
	AiCostMicros                 int64      `json:"ai_cost_micros"`
	ComputedAt                   time.Time  `json:"computed_at"`
}

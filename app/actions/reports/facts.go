/*
|--------------------------------------------------------------------------
| Conversation Facts
|--------------------------------------------------------------------------
|
| A conversation's facts are worked out from scratch from what happened
| to it: its logged changes and its public messages, its latest rating
| and whether a customer said an AI answer helped. The same inputs give
| the same facts, so a reopen, a backdated import or a merge is handled
| by working the conversation out again.
|
| The rules every report follows:
|
|   - A conversation starts at its first customer message, or when it
|     was made when the customer never wrote.
|   - First response runs from the first customer message to the first
|     public reply by a teammate. AI answers, automatic messages, notes
|     and events never count; first answer counts the AI too.
|   - A customer turn runs from the first customer message nobody has
|     answered to the next teammate reply. A close ends a turn nobody
|     answered.
|   - Resolution runs from the start to the last close; a reopened
|     conversation is resolved again only when it closes again.
|   - First contact resolution is a closed conversation that never
|     reopened and had at most one teammate reply.
|   - Spam, test, deleted and merged-away conversations have no facts.
|
*/

package reports

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
The authors a report tells apart.
*/
const (
	authorContact = "contact"
	authorMember  = "member"
	authorAgent   = "agent"
)

/*
Input is everything one conversation's facts are worked out from.
Schedule is the inbox's office hours, nil when it has none, and Now the
time open spans are counted up to.
*/
type Input struct {
	Conversation models.ReportConversationInputsRow
	Messages     []models.ReportMessageInputsRow
	Events       []models.ReportEventInputsRow
	Rating       *models.ReportRatingInputsRow
	Helped       bool
	Schedule     *officehours.Schedule
	Now          time.Time
}

/*
Facts is what a conversation adds to reports: its row, one row per
customer turn a teammate answered, and one per teammate who took part.
Counted is false for a conversation reports leave out.
*/
type Facts struct {
	Counted bool
	Row     FactRow
	Replies []ReplyRow
	Members []MemberRow
}

/*
ReplyRow is one answered customer turn.
*/
type ReplyRow struct {
	ConversationID  ids.UUID  `json:"conversation_id"`
	Turn            int       `json:"turn"`
	WorkspaceID     ids.UUID  `json:"workspace_id"`
	AskedAt         time.Time `json:"asked_at"`
	RepliedAt       time.Time `json:"replied_at"`
	MemberID        ids.UUID  `json:"member_id"`
	Seconds         int64     `json:"seconds"`
	BusinessSeconds int64     `json:"business_seconds"`
}

/*
MemberRow is one teammate's part in a conversation: their replies and
how long they held it while it was open.
*/
type MemberRow struct {
	ConversationID      ids.UUID   `json:"conversation_id"`
	MemberID            ids.UUID   `json:"member_id"`
	WorkspaceID         ids.UUID   `json:"workspace_id"`
	Replies             int        `json:"replies"`
	FirstRepliedAt      *time.Time `json:"first_replied_at"`
	LastRepliedAt       *time.Time `json:"last_replied_at"`
	AssignedOpenSeconds int64      `json:"assigned_open_seconds"`
	LastActiveAt        time.Time  `json:"last_active_at"`
}

/*
Compute works out a conversation's facts.
*/
func Compute(in Input) Facts {
	c := in.Conversation
	if c.Spam || c.IsTest || c.DeletedAt != nil || c.MergedIntoID != nil {
		return Facts{}
	}

	hist := walkEvents(in)
	msgs := readMessages(in.Messages)

	row := baseRow(in)
	row.StartedAt = c.CreatedAt
	if msgs.firstCustomer != nil {
		row.StartedAt = *msgs.firstCustomer
	}

	clock := durations{schedule: in.Schedule}
	applyMessages(&row, msgs, clock)
	applyCloses(&row, hist, clock)
	applyRating(&row, in)
	applyAI(&row, in, msgs, hist)

	return Facts{
		Counted: true, Row: row,
		Replies: turns(in, hist, clock),
		Members: memberRows(in, msgs, hist),
	}
}

/*
baseRow fills what the conversation row says about itself.
*/
func baseRow(in Input) FactRow {
	c := in.Conversation

	return FactRow{
		ConversationID: c.ID, WorkspaceID: c.WorkspaceID, Number: c.Number, InboxID: c.InboxID,
		Channel: c.Channel, AssigneeMemberID: c.AssigneeMemberID, AssigneeTeamID: c.AssigneeTeamID,
		Priority: c.Priority, Status: c.Status, Imported: c.Imported, AiCostMicros: c.AiCostMicros,
		ComputedAt: in.Now,
	}
}

/*
durations measures spans in calendar seconds and in the seconds the
schedule was open; without a schedule every second is open.
*/
type durations struct {
	schedule *officehours.Schedule
}

/*
between returns the calendar and open seconds from one moment to
another.
*/
func (d durations) between(from, to time.Time) (int64, int64) {
	if !to.After(from) {
		return 0, 0
	}

	calendar := int64(to.Sub(from) / time.Second)
	if d.schedule == nil {
		return calendar, calendar
	}

	return calendar, int64(d.schedule.BusinessBetween(from, to) / time.Second)
}

/*
span returns a pointer pair of the seconds between two moments, both
nil when there is no end.
*/
func (d durations) span(from time.Time, to *time.Time) (*int64, *int64) {
	if to == nil {
		return nil, nil
	}

	cal, open := d.between(from, *to)

	return &cal, &open
}

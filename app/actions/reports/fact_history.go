/*
|--------------------------------------------------------------------------
| Conversation History
|--------------------------------------------------------------------------
|
| The logged changes of a conversation read as spans of time: from each
| change to the next, its status, its assignee and its team. Closes,
| reopens and the AI's hand-off are picked out on the way. A
| conversation logged before the log existed and never backfilled is
| read from its row: open from its start, closed when it says so.
|
*/

package reports

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
The logged kinds a report reads.
*/
const (
	kindCreated      = "created"
	kindClosed       = "closed"
	kindReopened     = "reopened"
	kindSnoozed      = "snoozed"
	kindUnsnoozed    = "unsnoozed"
	kindAssigned     = "assigned"
	kindTeamAssigned = "team_assigned"
	kindHandedOff    = "ai_handed_off"
	statusOpen       = "open"
	statusClosed     = "closed"
	statusSnoozed    = "snoozed"
)

/*
span is a stretch of time with one status, assignee and team.
*/
type span struct {
	from, to time.Time
	status   string
	member   *ids.UUID
	team     *ids.UUID
}

/*
history is what the log says happened.
*/
type history struct {
	spans         []span
	closes        []time.Time
	reopens       int
	handoffAt     *time.Time
	handoffReason *string
}

/*
walkEvents reads the conversation's log into spans.
*/
func walkEvents(in Input) history {
	c := in.Conversation
	if len(in.Events) == 0 {
		return rowHistory(in)
	}

	h := history{}
	cur := span{from: c.CreatedAt, status: statusOpen}

	for _, e := range in.Events {
		next := cur
		next.from = e.OccurredAt
		h.apply(&next, e.Kind, e.MemberID, e.TeamID)

		if e.Kind == kindHandedOff && h.handoffAt == nil {
			h.handoffAt, h.handoffReason = new(e.OccurredAt), e.Reason
		}

		if e.Kind == kindCreated {
			cur.member, cur.team = e.MemberID, e.TeamID

			continue
		}

		h.push(cur, e.OccurredAt)
		cur = next
	}

	h.push(cur, in.Now)

	return h
}

/*
apply moves the state by one logged change.
*/
func (h *history) apply(s *span, kind string, member, team *ids.UUID) {
	switch kind {
	case kindClosed:
		s.status = statusClosed
		h.closes = append(h.closes, s.from)
	case kindReopened:
		if len(h.closes) > 0 {
			h.reopens++
		}

		s.status = statusOpen
	case kindSnoozed:
		s.status = statusSnoozed
	case kindUnsnoozed:
		s.status = statusOpen
	case kindAssigned:
		s.member = member
	case kindTeamAssigned:
		s.team = team
	}
}

/*
push ends the span at the moment, dropping one with no length.
*/
func (h *history) push(s span, to time.Time) {
	if to.After(s.from) {
		s.to = to
		h.spans = append(h.spans, s)
	}
}

/*
rowHistory reads a conversation with no log from its row.
*/
func rowHistory(in Input) history {
	c := in.Conversation
	h := history{handoffAt: c.AiHandedOffAt, handoffReason: c.AiHandoffReason}
	end := in.Now

	if c.Status == statusClosed && c.ClosedAt != nil {
		h.closes = []time.Time{*c.ClosedAt}
		end = *c.ClosedAt
	}

	h.push(span{from: c.CreatedAt, status: statusOpen, member: c.AssigneeMemberID, team: c.AssigneeTeamID}, end)

	return h
}

/*
at returns the span the moment falls in, the last one past the end.
*/
func (h *history) at(t time.Time) span {
	for _, s := range h.spans {
		if !t.Before(s.from) && t.Before(s.to) {
			return s
		}
	}

	if len(h.spans) == 0 {
		return span{}
	}

	return h.spans[len(h.spans)-1]
}

/*
openSeconds sums, per member, how long they held the conversation
while it was open.
*/
func (h *history) openSeconds() map[ids.UUID]int64 {
	out := map[ids.UUID]int64{}

	for _, s := range h.spans {
		if s.member == nil || s.status != statusOpen {
			continue
		}

		out[*s.member] += int64(s.to.Sub(s.from) / time.Second)
	}

	return out
}

/*
holders returns every member who held the conversation while it was
waiting, with when they last did.
*/
func (h *history) holders() map[ids.UUID]time.Time {
	out := map[ids.UUID]time.Time{}

	for _, s := range h.spans {
		if s.member != nil && s.status != statusClosed && s.to.After(out[*s.member]) {
			out[*s.member] = s.to
		}
	}

	return out
}

/*
|--------------------------------------------------------------------------
| Messages and Turns
|--------------------------------------------------------------------------
|
| What the public messages say: when the customer first and last wrote,
| when a teammate first replied and who, when anyone, the AI included,
| first answered, and each customer turn a teammate answered.
|
*/

package reports

import (
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
messageSummary is what the messages say, read once.
*/
type messageSummary struct {
	all            []models.ReportMessageInputsRow
	firstCustomer  *time.Time
	lastCustomer   *time.Time
	customer       int
	firstHuman     *models.ReportMessageInputsRow
	firstAnswer    *time.Time
	humanReplies   int
	aiAnswers      int
	aiFirst        *time.Time
	aiLast         *time.Time
	memberReplies  map[ids.UUID]int
	memberFirst    map[ids.UUID]time.Time
	memberLastSeen map[ids.UUID]time.Time
}

/*
readMessages summarizes the messages, oldest first.
*/
func readMessages(msgs []models.ReportMessageInputsRow) messageSummary {
	out := messageSummary{
		all: msgs, memberReplies: map[ids.UUID]int{}, memberFirst: map[ids.UUID]time.Time{},
		memberLastSeen: map[ids.UUID]time.Time{},
	}

	for i := range msgs {
		m := msgs[i]
		switch {
		case m.AuthorType == authorContact:
			out.customerMessage(m.At)
		case m.AuthorType == authorMember && m.AuthorID != nil:
			out.memberMessage(&msgs[i])
		case m.AuthorType == authorAgent && m.Substantive:
			out.aiAnswers++
			out.aiLast = new(m.At)
			if out.aiFirst == nil {
				out.aiFirst = new(m.At)
			}
			out.answered(m.At)
		}
	}

	return out
}

/*
customerMessage counts a message from the customer.
*/
func (s *messageSummary) customerMessage(at time.Time) {
	s.customer++
	s.lastCustomer = new(at)
	if s.firstCustomer == nil {
		s.firstCustomer = new(at)
	}
}

/*
memberMessage counts a teammate's public reply.
*/
func (s *messageSummary) memberMessage(m *models.ReportMessageInputsRow) {
	id := *m.AuthorID
	s.humanReplies++
	s.memberReplies[id]++
	s.memberLastSeen[id] = m.At
	if _, ok := s.memberFirst[id]; !ok {
		s.memberFirst[id] = m.At
	}

	if s.firstCustomer != nil && s.firstHuman == nil {
		s.firstHuman = m
	}

	s.answered(m.At)
}

/*
answered notes the first answer after the customer first wrote.
*/
func (s *messageSummary) answered(at time.Time) {
	if s.firstCustomer != nil && s.firstAnswer == nil {
		s.firstAnswer = new(at)
	}
}

/*
applyMessages fills the response facts.
*/
func applyMessages(row *FactRow, s messageSummary, d durations) {
	row.CustomerMessages = s.customer
	row.LastCustomerMessageAt = s.lastCustomer
	row.HumanReplies = s.humanReplies

	if s.firstHuman != nil {
		row.FirstResponseAt = new(s.firstHuman.At)
		row.FirstResponderID = s.firstHuman.AuthorID
		row.FirstResponseSeconds, row.FirstResponseBusinessSeconds = d.span(row.StartedAt, row.FirstResponseAt)
	}

	if s.firstAnswer != nil {
		row.FirstAnswerAt = s.firstAnswer
		row.FirstAnswerSeconds, row.FirstAnswerBusinessSeconds = d.span(row.StartedAt, row.FirstAnswerAt)
	}
}

/*
turnItem is a message or a close in time order.
*/
type turnItem struct {
	at     time.Time
	close  bool
	author string
	member *ids.UUID
}

/*
turns returns each customer turn a teammate answered, numbered from
zero in order.
*/
func turns(in Input, h history, d durations) []ReplyRow {
	items := make([]turnItem, 0, len(in.Messages)+len(h.closes))
	for _, m := range in.Messages {
		items = append(items, turnItem{at: m.At, author: m.AuthorType, member: m.AuthorID})
	}

	for _, t := range h.closes {
		items = append(items, turnItem{at: t, close: true})
	}

	slices.SortStableFunc(items, func(a, b turnItem) int { return a.at.Compare(b.at) })

	var (
		out   []ReplyRow
		asked *time.Time
	)

	for _, it := range items {
		switch {
		case it.close:
			asked = nil
		case it.author == authorContact && asked == nil:
			asked = new(it.at)
		case it.author == authorMember && it.member != nil && asked != nil:
			cal, open := d.between(*asked, it.at)
			out = append(out, ReplyRow{
				ConversationID: in.Conversation.ID, Turn: len(out), WorkspaceID: in.Conversation.WorkspaceID,
				AskedAt: *asked, RepliedAt: it.at, MemberID: *it.member, Seconds: cal, BusinessSeconds: open,
			})
			asked = nil
		}
	}

	return out
}

/*
memberRows returns each teammate's part: those who replied and those
who held the conversation while it waited.
*/
func memberRows(in Input, s messageSummary, h history) []MemberRow {
	open := h.openSeconds()
	held := h.holders()

	seen := map[ids.UUID]bool{}
	var order []ids.UUID
	for _, m := range in.Messages {
		if m.AuthorType == authorMember && m.AuthorID != nil && !seen[*m.AuthorID] {
			seen[*m.AuthorID] = true
			order = append(order, *m.AuthorID)
		}
	}

	for _, sp := range h.spans {
		if sp.member != nil && !seen[*sp.member] {
			if _, ok := held[*sp.member]; ok {
				seen[*sp.member] = true
				order = append(order, *sp.member)
			}
		}
	}

	out := make([]MemberRow, 0, len(order))
	for _, id := range order {
		out = append(out, s.memberRow(in, id, open[id], held[id]))
	}

	return out
}

/*
memberRow is one teammate's part.
*/
func (s *messageSummary) memberRow(in Input, id ids.UUID, open int64, held time.Time) MemberRow {
	row := MemberRow{
		ConversationID: in.Conversation.ID, MemberID: id, WorkspaceID: in.Conversation.WorkspaceID,
		Replies: s.memberReplies[id], AssignedOpenSeconds: open, LastActiveAt: held,
	}

	if first, ok := s.memberFirst[id]; ok {
		last := s.memberLastSeen[id]
		row.FirstRepliedAt, row.LastRepliedAt = &first, &last
		if last.After(row.LastActiveAt) {
			row.LastActiveAt = last
		}
	}

	return row
}

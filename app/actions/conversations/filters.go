/*
|--------------------------------------------------------------------------
| List Filters
|--------------------------------------------------------------------------
|
| Every list of conversations, its neighbors and the quick search read
| the same filters: a view, statuses, an inbox, a customer, a holder,
| priority, unread state and a search. A list shows open conversations
| unless asked otherwise; spam and the trash only show in their own
| views. A search for a number finds that conversation whatever its
| status.
|
*/

package conversations

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
	"github.com/dimmihq/dimmi/app/support/searchquery"
)

/*
The views of the inbox.
*/
const (
	ViewMine       = "mine"
	ViewUnassigned = "unassigned"
	ViewAll        = "all"
	ViewMember     = "member"
	ViewSpam       = "spam"
	ViewDeleted    = "deleted"
	ViewMentions   = "mentions"
)

/*
Filters narrow a list. View is one of the views, with ViewMember naming
the member and ViewID the saved view. Statuses are the statuses shown,
none meaning open, unless AnyStatus. Assignee narrows to one member's
conversations, Unassigned to nobody's and AssigneeMe to the caller's.
NoTeam narrows to those no team holds. AssignmentWaiting narrows to
those automatic assignment could not place yet. The pointers and lists
narrow only when set. Search is what the caller typed.
*/
type Filters struct {
	View          string
	ViewMember    ids.UUID
	ViewID        ids.UUID
	Statuses      []string
	AnyStatus     bool
	Assignee      *ids.UUID
	AssigneeMe    bool
	Unassigned    bool
	TeamID        *ids.UUID
	NoTeam        bool
	InboxID       *ids.UUID
	ContactID     *ids.UUID
	CompanyID     *ids.UUID
	Connections   []ids.UUID
	ChannelTypes  []string
	Priorities    []int16
	TagsAll       []ids.UUID
	TagsAny       []ids.UUID
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Waiting       bool
	Unread        bool
	Search        string

	AssignmentWaiting bool
	Ai                []string
}

/*
listOf turns the filters into the list's query, and reports false for a
list that is empty whatever the data: a search for an id, which only the
search endpoint jumps to.
*/
func listOf(m authz.Membership, f Filters, s querybuilder.Sort) (querybuilder.Conversations, bool) {
	l := querybuilder.Conversations{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, Sort: s,
		Statuses: f.Statuses, Unassigned: f.Unassigned, AssigneeID: f.Assignee, TeamID: f.TeamID, NoTeam: f.NoTeam,
		InboxID: f.InboxID, ContactID: f.ContactID, CompanyID: f.CompanyID, ConnectionIDs: f.Connections,
		ChannelTypes: f.ChannelTypes, Priorities: f.Priorities, TagsAll: f.TagsAll, TagsAny: f.TagsAny,
		CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore, Waiting: f.Waiting, Unread: f.Unread,
		Access: access(m),
	}

	if f.AssignmentWaiting {
		l.Extra = append(l.Extra, assignmentWaiting)
	}

	if clause := aiClause(f.Ai); clause != "" {
		l.Extra = append(l.Extra, func(int) (string, []any) { return clause, nil })
	}

	applyView(&l, m, f)

	sq := searchquery.Parse(f.Search)
	if sq.ID != "" {
		return l, false
	}

	if sq.Number != nil {
		l.Number, l.Statuses = sq.Number, nil

		return l, true
	}

	if !applySearch(&l, m, sq) {
		return l, false
	}

	if len(l.Statuses) == 0 && !f.AnyStatus && f.View != ViewSpam && f.View != ViewDeleted {
		l.Statuses = []string{string(enums.ConversationStatusOpen)}
	}

	return l, true
}

/*
applyView narrows the list to the view's conversations, an assignee
filter winning over the view's holder.
*/
func applyView(l *querybuilder.Conversations, m authz.Membership, f Filters) {
	switch f.View {
	case ViewMine:
		l.AssigneeID = new(m.Member.ID)
	case ViewUnassigned:
		l.Unassigned = true
	case ViewMember:
		l.AssigneeID = new(f.ViewMember)
	case ViewMentions:
		l.Mentioned = true
	case ViewSpam:
		l.Spam = true
	case ViewDeleted:
		l.Deleted = true
	}

	if f.Assignee != nil {
		l.AssigneeID = f.Assignee
	}

	if f.AssigneeMe {
		l.AssigneeID = new(m.Member.ID)
	}
}

/*
applySearch narrows the list to what the search finds: its filters and
its words. It reports false when the search names a priority that does
not exist, which finds nothing.
*/
func applySearch(l *querybuilder.Conversations, m authz.Membership, sq searchquery.Query) bool {
	if len(sq.Statuses) > 0 {
		l.Statuses = sq.Statuses
	}

	l.Unread = l.Unread || sq.Unread
	l.Unassigned = l.Unassigned || sq.Unassigned

	if sq.Mine {
		l.AssigneeID = new(m.Member.ID)
	}

	if sq.Priority {
		l.PriorityMin = new(int16(1))
	}

	if sq.PriorityLevel != "" {
		level, ok := enums.ConversationPriority(sq.PriorityLevel).Level()
		if !ok {
			return false
		}

		l.Priorities = append(l.Priorities, level)
	}

	if sq.PriorityMin != "" {
		level, ok := enums.ConversationPriority(sq.PriorityMin).Level()
		if !ok {
			return false
		}

		l.PriorityMin = &level
	}

	if sq.TSQuery != "" {
		l.TSQuery = new(sq.TSQuery)
	}

	return true
}

/*
access returns what the caller may see, as a list takes it.
*/
func access(m authz.Membership) querybuilder.Access {
	p := VisibleScope(m).Params()

	return querybuilder.Access{
		Mode: p.Mode, Viewer: p.Viewer, Teams: p.Teams, Unassigned: p.Unassigned, HiddenInboxes: p.HiddenInboxes,
	}
}

/*
aiClause maps the ai filter to its predicate: handed_off for the
conversations with the team, active for those the agent still
answers, and any for every conversation the agent touched.
*/
func aiClause(values []string) string {
	for _, v := range values {
		switch v {
		case "handed_off":
			return "c.ai_state = 'handed_off'"
		case "active":
			return "c.ai_state = 'active' AND c.ai_reply_count > 0"
		case "any":
			return "(c.ai_reply_count > 0 OR c.ai_state <> 'off')"
		}
	}
	return ""
}

/*
assignmentWaiting narrows a list to the conversations waiting for
automatic assignment.
*/
func assignmentWaiting(int) (string, []any) {
	return "c.assignment_wait_reason IS NOT NULL", nil
}

/*
|--------------------------------------------------------------------------
| List Filters
|--------------------------------------------------------------------------
|
| Every list of conversations, its neighbors and the quick search read
| the same filters: a view, statuses, an inbox, a customer, a holder,
| priority, unread state and a search. A list shows open conversations
| unless asked otherwise; spam only shows in its own view. A search for
| a number finds that conversation whatever its status.
|
*/

package conversations

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
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
	ViewMentions   = "mentions"
)

/*
Filters narrow a list. View is one of the views, with ViewMember naming
the member. Statuses are the statuses shown, none meaning open, unless
AnyStatus. Assignee narrows to one member's conversations and
Unassigned to nobody's. The pointers narrow only when set. TagIDs keeps
conversations carrying every tag and AnyTagIDs those carrying one of
them. Search is what the caller typed.
*/
type Filters struct {
	View       string
	ViewMember ids.UUID
	Statuses   []string
	AnyStatus  bool
	Assignee   *ids.UUID
	Unassigned bool
	TeamID     *ids.UUID
	InboxID    *ids.UUID
	ContactID  *ids.UUID
	CompanyID  *ids.UUID
	Priority   *bool
	Unread     bool
	TagIDs     []ids.UUID
	AnyTagIDs  []ids.UUID
	Search     string
}

/*
Sort orders a list: by creation time when Created, else by the latest
message, newest first when Descending.
*/
type Sort struct {
	Created    bool
	Descending bool
}

/*
Cursor is where a page starts: after the row with the sort key and id.
*/
type Cursor struct {
	At time.Time
	ID ids.UUID
}

/*
listParams turns the filters into the query's parameters, and reports
false for a list that is empty whatever the data: mentions, and a
search for an id, which only the search endpoint jumps to.
*/
func listParams(m authz.Membership, f Filters, s Sort) (models.FilterConversationsParams, bool) {
	p := models.FilterConversationsParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, SortCreated: s.Created, Descending: s.Descending,
		Statuses: f.Statuses, Unassigned: f.Unassigned, AssigneeID: f.Assignee, TeamID: f.TeamID,
		InboxID: f.InboxID, ContactID: f.ContactID, CompanyID: f.CompanyID, Priority: f.Priority, OnlyUnread: f.Unread,
		TagIds: f.TagIDs, AnyTagIds: f.AnyTagIDs,
	}

	if f.View == ViewMentions {
		return p, false
	}

	applyView(&p, m, f)

	sq := searchquery.Parse(f.Search)
	if sq.ID != "" {
		return p, false
	}

	if sq.Number != nil {
		p.Number, p.Statuses = sq.Number, nil

		return p, true
	}

	applySearch(&p, m, sq)

	if len(p.Statuses) == 0 && !f.AnyStatus && f.View != ViewSpam {
		p.Statuses = []string{"open"}
	}

	return p, true
}

/*
applyView narrows the list to the view's conversations, an assignee
filter winning over the view's holder.
*/
func applyView(p *models.FilterConversationsParams, m authz.Membership, f Filters) {
	switch f.View {
	case ViewMine:
		p.AssigneeID = new(m.Member.ID)
	case ViewUnassigned:
		p.Unassigned = true
	case ViewMember:
		p.AssigneeID = new(f.ViewMember)
	case ViewSpam:
		p.Spam = true
	}

	if f.Assignee != nil {
		p.AssigneeID = f.Assignee
	}
}

/*
applySearch narrows the list to what the search finds: its is: filters
and its words.
*/
func applySearch(p *models.FilterConversationsParams, m authz.Membership, sq searchquery.Query) {
	if len(sq.Statuses) > 0 {
		p.Statuses = sq.Statuses
	}

	p.OnlyUnread = p.OnlyUnread || sq.Unread
	p.Unassigned = p.Unassigned || sq.Unassigned

	if sq.Mine {
		p.AssigneeID = new(m.Member.ID)
	}

	if sq.Priority {
		p.Priority = new(true)
	}

	if sq.TSQuery != "" {
		p.Query = new(sq.TSQuery)
	}
}

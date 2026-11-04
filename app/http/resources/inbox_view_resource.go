/*
|--------------------------------------------------------------------------
| Inbox View Resources
|--------------------------------------------------------------------------
|
| What the sidebar and the quick search show: how many conversations
| wait in each view, capped so a busy workspace costs the same to count
| as a quiet one, and what a search found.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationCounts is how many conversations wait in each view.
*/
type ConversationCounts struct {
	Object     string     `json:"object"`
	Views      CountViews `json:"views"`
	ComputedAt time.Time  `json:"computed_at"`
}

/*
CountViews are the counts by view. Capped says a count reached the cap
and shows the cap.
*/
type CountViews struct {
	Mine struct {
		Open    int64 `json:"open"`
		Unread  int64 `json:"unread"`
		Snoozed int64 `json:"snoozed"`
	} `json:"mine"`
	Unassigned struct {
		Open   int64 `json:"open"`
		Unread int64 `json:"unread"`
	} `json:"unassigned"`
	All struct {
		Open   int64 `json:"open"`
		Capped bool  `json:"capped"`
	} `json:"all"`
	Mentions struct {
		Unread int64 `json:"unread"`
	} `json:"mentions"`
	Spam struct {
		Total int64 `json:"total"`
	} `json:"spam"`
	Inboxes map[string]InboxCount `json:"inboxes"`
	Teams   map[string]InboxCount `json:"teams"`
	Members map[string]InboxCount `json:"members"`
	Custom  map[string]InboxCount `json:"custom"`
}

/*
InboxCount is how many open conversations an inbox, team or member
holds.
*/
type InboxCount struct {
	Open int64 `json:"open"`
}

/*
NewConversationCounts shapes the counts, capped, as of the time.
*/
func NewConversationCounts(c conversations.Counts, at time.Time) ConversationCounts {
	out := ConversationCounts{Object: "conversation_counts", ComputedAt: at}
	v := &out.Views

	v.Mine.Open, v.Mine.Unread, v.Mine.Snoozed = capped(c.MineOpen), capped(c.MineUnread), capped(c.MineSnoozed)
	v.Unassigned.Open, v.Unassigned.Unread = capped(c.UnassignedOpen), capped(c.UnassignedUnread)
	v.All.Open, v.All.Capped = capped(c.AllOpen), c.AllOpen > conversations.CountCap
	v.Spam.Total = capped(c.SpamTotal)
	v.Inboxes = openCounts(models.PrefixInbox, c.Inboxes)
	v.Teams = openCounts(models.PrefixTeam, c.Teams)
	v.Members = openCounts(models.PrefixMember, c.Members)
	v.Custom = openCounts(models.PrefixConversationView, c.Views)

	return out
}

/*
openCounts keys the open counts by public id.
*/
func openCounts(prefix models.Prefix, counts map[ids.UUID]int64) map[string]InboxCount {
	out := make(map[string]InboxCount, len(counts))
	for id, open := range counts {
		out[models.PublicID(prefix, id)] = InboxCount{Open: capped(open)}
	}

	return out
}

/*
capped returns the count, or the cap when it is larger.
*/
func capped(n int64) int64 {
	return min(n, conversations.CountCap)
}

/*
SearchResults are what a quick search found.
*/
type SearchResults struct {
	Object        string           `json:"object"`
	Jump          *ConversationRef `json:"jump"`
	Conversations []SearchHit      `json:"conversations"`
	Contacts      []ContactSummary `json:"contacts"`
	Companies     []any            `json:"companies"`
}

/*
SearchHit is a conversation a search found, with an excerpt of the
matching message, marks around the matched words.
*/
type SearchHit struct {
	Conversation ConversationRef `json:"conversation"`
	Contact      ContactSummary  `json:"contact"`
	Highlight    *string         `json:"highlight"`
	MessageID    *string         `json:"message_id"`
}

/*
NewSearchResults shapes what the search found.
*/
func NewSearchResults(r conversations.SearchResults) SearchResults {
	out := SearchResults{
		Object: "search_results", Conversations: []SearchHit{}, Contacts: []ContactSummary{}, Companies: []any{},
	}

	if r.Jump != nil {
		out.Jump = new(NewConversationRef(r.Jump.Conversation))
	}

	for _, h := range r.Conversations {
		hit := SearchHit{
			Conversation: NewConversationRef(h.Conversation.Conversation),
			Contact:      NewContactSummary(h.Conversation.Contact),
		}
		if h.MessageID != nil {
			hit.Highlight, hit.MessageID = &h.Highlight, new(models.PublicID(models.PrefixMessage, *h.MessageID))
		}

		out.Conversations = append(out.Conversations, hit)
	}

	for _, c := range r.Contacts {
		out.Contacts = append(out.Contacts, NewContactSummary(c))
	}

	return out
}

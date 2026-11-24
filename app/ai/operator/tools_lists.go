/*
|--------------------------------------------------------------------------
| Operator Conversation Lists
|--------------------------------------------------------------------------
|
| A teammate asks "what is open?", "what is waiting on me?" or "what did
| the agent hand over?" far more often than they search for words. The
| list tool answers those straight from the inbox's own lists, so it
| sees exactly the conversations the teammate may see in the inbox and
| nothing else, newest activity first.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
listLimit is the most conversations one list returns in full, and
scanLimit the most it returns as a scan: each as one short line, so a
whole week fits in one result.
*/
const (
	listLimit = 25
	scanLimit = 200
)

/*
listSchema is the schema of list_conversations.
*/
const listSchema = `{"type":"object","properties":{` +
	`"status":{"type":"string","enum":["open","snoozed","closed","any"],` +
	`"description":"Which conversations by status. Defaults to open."},` +
	`"assignee":{"type":"string","enum":["anyone","me","unassigned"],` +
	`"description":"Whose conversations. Defaults to anyone."},` +
	`"ai_agent":{"type":"string","enum":["any","handed_off","answered_by_ai"],` +
	`"description":"Only those the AI agent handed to the team, or ones it is answering."},` +
	`"tag":{"type":"string","description":"Only those with this tag, by its name as list_tags shows it, ` +
	`such as billing."},` +
	`"customer_waiting":{"type":"boolean","description":"true for only those where the customer wrote last ` +
	`and is waiting for the team."},` +
	`"started_since":{"type":"string","format":"date","description":"Only conversations started on or ` +
	`after this day, YYYY-MM-DD."},` +
	`"started_until":{"type":"string","format":"date","description":"Only conversations started on or ` +
	`before this day, YYYY-MM-DD."},` +
	`"scan":{"type":"boolean","description":"true to look across many at once: up to 200, each as only its ` +
	`number, subject, link and why the AI agent handed it over. Use it to see a whole period, then read the few ` +
	`that matter."},` +
	`"order":{"type":"string","enum":["latest_activity","longest_waiting"],` +
	`"description":"latest_activity lists the most recently active first, the default. longest_waiting lists ` +
	`the customers who have waited longest for the team first."},` +
	`"limit":{"type":"integer","minimum":1,"maximum":200,"description":"How many: at most 25, or 200 ` +
	`with scan. Defaults to 15, or 200 with scan."}}}`

/*
orderLongestWaiting lists the customers who waited longest first, which
only makes sense among those still waiting.
*/
const orderLongestWaiting = "longest_waiting"

/*
listArgs are list_conversations' filters.
*/
type listArgs struct {
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
	AI       string `json:"ai_agent"`
	Tag      string `json:"tag"`
	Waiting  bool   `json:"customer_waiting"`
	Since    string `json:"started_since"`
	Until    string `json:"started_until"`
	Scan     bool   `json:"scan"`
	Order    string `json:"order"`
	Limit    int    `json:"limit"`
}

/*
listTool is the tool that lists conversations by status, owner and the
agent's part in them.
*/
func (e *Engine) listTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name: "list_conversations",
		Description: "List conversations by status, by who holds them, by tag, by when they started and by the " +
			"AI agent's part, newest " +
			"activity first: open ones, ones waiting on the teammate, unassigned ones, ones the agent " +
			"handed to the team, ones where the customer is waiting. Each line already says who wrote " +
			"last and since when the customer has waited, so read a conversation only when a line is " +
			"not enough. Use search_conversations only to find conversations by their words or a customer.",
		Schema: schema(listSchema),
	}, handle: e.listConversations, was: map[string]string{"ai": "ai_agent", "waiting": "customer_waiting"}}
}

/*
listConversations lists the conversations the filters ask for.
*/
func (e *Engine) listConversations(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in listArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}

	listing, refusal, err := e.listingOf(ctx, s, in)
	if err != nil || refusal != "" {
		return result{text: refusal}, err
	}

	page, err := e.Services.List.Handle(ctx, listing)
	if err != nil {
		return result{}, err
	}

	s.tainted = true

	var b strings.Builder
	for _, l := range page.Conversations {
		c := l.Conversation
		if in.Scan {
			fmt.Fprintf(&b, "#%d %s · %s · started %s%s · link %s\n", c.Number, untrusted(deref(c.Subject)),
				c.Status, c.CreatedAt.UTC().Format(time.DateOnly), handedOver(c.AiHandoffReason),
				e.conversationLink(s, c.ID))

			continue
		}

		fmt.Fprintf(&b, "#%d %s · %s · %s · AI %s · %s · last from %s · %s · link %s\n", c.Number,
			untrusted(deref(c.Subject)), c.Status, assigneeOf(l), c.AiState, untrusted(deref(l.Contact.Name)),
			lastFrom(l), waiting(l), e.conversationLink(s, c.ID))
	}

	if b.Len() == 0 {
		b.WriteString("No conversations match.")
	}

	if page.More {
		b.WriteString("(More match; narrow the filters or raise the limit to see them.)\n")
	}

	summary := fmt.Sprintf("Listed %d %s conversations", len(page.Conversations), describe(in))

	return result{text: b.String(), step: Step{Tool: "list_conversations", Summary: summary}}, nil
}

/*
listingOf is the list the arguments ask for: their filters, the tag
looked up by name, and the order. It returns words for the model
instead when the tag does not exist.
*/
func (e *Engine) listingOf(
	ctx context.Context, s *session, in listArgs,
) (conversations.ListConversationsInput, string, error) {
	listing := conversations.ListConversationsInput{
		Membership: s.m, Filters: filtersOf(in), DefaultSort: true, Limit: limitOf(in),
	}

	if refusal := between(in, &listing.Filters); refusal != "" {
		return listing, refusal, nil
	}

	if name := strings.TrimSpace(in.Tag); name != "" {
		tag, found, err := e.tagNamed(ctx, s, name)
		if err != nil {
			return listing, "", err
		}

		if !found {
			return listing, fmt.Sprintf("No tag is named %q. list_tags shows the tags there are.", name), nil
		}

		listing.Filters.TagsAny = []ids.UUID{tag}
	}

	if in.Order == orderLongestWaiting {
		listing.Filters.Waiting, listing.Sort, listing.DefaultSort = true, querybuilder.SortWaiting, false
	}

	return listing, "", nil
}

/*
filtersOf turns the tool's arguments into the inbox's filters.
*/
func filtersOf(in listArgs) conversations.Filters {
	f := conversations.Filters{}

	switch in.Status {
	case "any":
		f.AnyStatus = true
	case "snoozed", "closed":
		f.Statuses = []string{in.Status}
	default:
		f.Statuses = []string{"open"}
	}

	switch in.Assignee {
	case "me":
		f.AssigneeMe = true
	case "unassigned":
		f.Unassigned = true
	}

	f.Waiting = in.Waiting

	switch in.AI {
	case "handed_off":
		f.Ai = []string{"handed_off"}
	case "answered_by_ai":
		f.Ai = []string{"active"}
	}

	return f
}

/*
limitOf is the list's size, the default when none is asked: a scan
returns many short lines, a full list a few long ones.
*/
func limitOf(in listArgs) int {
	if in.Scan {
		if in.Limit <= 0 {
			return scanLimit
		}

		return min(in.Limit, scanLimit)
	}

	if in.Limit <= 0 {
		return 15
	}

	return min(in.Limit, listLimit)
}

/*
between narrows the filters to the days the arguments name, and returns
words for the model when a day is not a date.
*/
func between(in listArgs, f *conversations.Filters) string {
	if in.Since != "" {
		day, err := time.Parse(time.DateOnly, in.Since)
		if err != nil {
			return "started_since must be a day like 2026-01-31."
		}

		f.CreatedAfter = &day
	}

	if in.Until != "" {
		day, err := time.Parse(time.DateOnly, in.Until)
		if err != nil {
			return "started_until must be a day like 2026-01-31."
		}

		end := day.AddDate(0, 0, 1)
		f.CreatedBefore = &end
	}

	return ""
}

/*
handedOver says why the AI agent handed a conversation over, for a
scan's line, and nothing when it did not.
*/
func handedOver(reason *string) string {
	if reason == nil || *reason == "" {
		return ""
	}

	return " · handed over: " + *reason
}

/*
describe names the filters for the run's steps.
*/
func describe(in listArgs) string {
	parts := []string{cmpOr(in.Status, "open")}
	if in.Assignee != "" && in.Assignee != "anyone" {
		parts = append(parts, in.Assignee)
	}

	if in.AI != "" && in.AI != "any" {
		parts = append(parts, in.AI)
	}

	if in.Tag != "" {
		parts = append(parts, "tagged "+in.Tag)
	}

	if in.Since != "" {
		parts = append(parts, "since "+in.Since)
	}

	if in.Waiting || in.Order == orderLongestWaiting {
		parts = append(parts, "waiting")
	}

	if in.Order == orderLongestWaiting {
		parts = append(parts, "longest wait first")
	}

	return strings.Join(parts, ", ")
}

/*
cmpOr returns the value, or the fallback when it is empty.
*/
func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

/*
lastFrom names who wrote the conversation's latest message, so the
list shows whose turn it is without reading each conversation.
*/
func lastFrom(l conversations.Loaded) string {
	if l.LastMessage == nil {
		return "nobody yet"
	}

	switch l.LastMessage.AuthorType {
	case "contact":
		return "the customer"
	case "agent":
		return "the AI agent"
	case "member":
		return "a teammate"
	}

	return l.LastMessage.AuthorType
}

/*
waiting says since when the customer has waited for an answer, or when
the conversation last moved when nobody is waiting.
*/
func waiting(l conversations.Loaded) string {
	c := l.Conversation
	if c.WaitingSince != nil {
		return "customer waiting since " + c.WaitingSince.UTC().Format("2006-01-02 15:04")
	}

	at := c.CreatedAt
	if c.LastActivityAt != nil {
		at = *c.LastActivityAt
	}

	return "last activity " + at.UTC().Format("2006-01-02 15:04")
}

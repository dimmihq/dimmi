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

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
)

/*
listLimit is the most conversations one list returns.
*/
const listLimit = 25

/*
listSchema is the schema of list_conversations.
*/
const listSchema = `{"type":"object","properties":{` +
	`"status":{"type":"string","enum":["open","snoozed","closed","any"],` +
	`"description":"Which conversations by status. Defaults to open."},` +
	`"assignee":{"type":"string","enum":["anyone","me","unassigned"],` +
	`"description":"Whose conversations. Defaults to anyone."},` +
	`"ai":{"type":"string","enum":["any","handed_off","answered_by_ai"],` +
	`"description":"Only those the AI agent handed to the team, or ones it is answering."},` +
	`"limit":{"type":"integer","minimum":1,"maximum":25,"description":"How many, at most 25. Defaults to 15."}}}`

/*
listArgs are list_conversations' filters.
*/
type listArgs struct {
	Status   string `json:"status"`
	Assignee string `json:"assignee"`
	AI       string `json:"ai"`
	Limit    int    `json:"limit"`
}

/*
listTool is the tool that lists conversations by status, owner and the
agent's part in them.
*/
func (e *Engine) listTool() tool {
	return tool{spec: contracts.ToolSpec{
		Name: "list_conversations",
		Description: "List conversations by status, by who holds them and by the AI agent's part, newest " +
			"activity first: open ones, ones waiting on the teammate, unassigned ones, ones the agent " +
			"handed to the team. Use this for questions about what is open or waiting; use " +
			"search_conversations only to find conversations by their words or a customer.",
		Schema: schema(listSchema),
	}, handle: e.listConversations}
}

/*
listConversations lists the conversations the filters ask for.
*/
func (e *Engine) listConversations(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in listArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}

	page, err := e.Services.List.Handle(ctx, conversations.ListConversationsInput{
		Membership: s.m, Filters: filtersOf(in), DefaultSort: true, Limit: limitOf(in.Limit),
	})
	if err != nil {
		return result{}, err
	}

	s.tainted = true

	var b strings.Builder
	for _, l := range page.Conversations {
		c := l.Conversation
		fmt.Fprintf(&b, "#%d %s · %s · %s · AI %s · %s · last from %s · %s\n", c.Number,
			untrusted(deref(c.Subject)), c.Status, assigneeOf(l), c.AiState, untrusted(deref(l.Contact.Name)),
			lastFrom(l), waiting(l))
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

	switch in.AI {
	case "handed_off":
		f.Ai = []string{"handed_off"}
	case "answered_by_ai":
		f.Ai = []string{"active"}
	}

	return f
}

/*
limitOf is the list's size, the default when none is asked.
*/
func limitOf(asked int) int {
	if asked <= 0 {
		return 15
	}

	return min(asked, listLimit)
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

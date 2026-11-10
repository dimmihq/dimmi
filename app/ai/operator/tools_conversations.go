/*
|--------------------------------------------------------------------------
| Conversation Tools
|--------------------------------------------------------------------------
|
| Reading the inbox the way the teammate can: one conversation with its
| messages and notes, a search across conversations and contacts, a
| customer's profile and history, and the workspace's tags and people.
| Anything a customer wrote comes back inside untrusted tags, and reading
| it marks the run, so no change it proposes afterwards applies on its
| own.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
timelineSize is how many of a conversation's latest entries
get_conversation reads.
*/
const timelineSize = 60

/*
numberArgs names a conversation by number; zero means the thread's.
*/
type numberArgs struct {
	Number int64 `json:"number"`
}

/*
numberSchema is the schema of a tool that takes a conversation number.
*/
const numberSchema = `{"type":"object","properties":{"number":{"type":"integer",` +
	`"description":"Conversation number, such as 123 for #123. Leave out for this thread's conversation."}}}`

/*
conversationTools are the tools that read conversations and people.
*/
func (e *Engine) conversationTools() []tool {
	return []tool{
		{spec: contracts.ToolSpec{
			Name: "get_conversation", Schema: schema(numberSchema),
			Description: "Read one conversation: its details, tags, assignee, customer, and its latest " +
				"messages and internal notes, oldest first.",
		}, handle: e.getConversation},
		e.listTool(),
		{spec: contracts.ToolSpec{
			Name: "search_conversations",
			Description: "Search conversations and contacts by words, a customer's name or email, or " +
				"#number. Returns up to 20 conversations, newest first.",
			Schema: schema(`{"type":"object","required":["query"],"properties":{` +
				`"query":{"type":"string","maxLength":256}}}`),
		}, handle: e.searchConversations},
		{spec: contracts.ToolSpec{
			Name: "get_customer", Schema: schema(numberSchema),
			Description: "Read the customer of a conversation: their profile, attributes, company and " +
				"their other recent conversations.",
		}, handle: e.getCustomer},
		{spec: contracts.ToolSpec{
			Name: "list_tags", Schema: schema(`{"type":"object","properties":{}}`),
			Description: "List the workspace's tags.",
		}, handle: e.listTags},
		{spec: contracts.ToolSpec{
			Name: "list_teammates", Schema: schema(`{"type":"object","properties":{}}`),
			Description: "List the workspace's active teammates and teams, with their ids for assigning.",
		}, handle: e.listTeammates},
	}
}

/*
ref names the conversation a tool means: the number given, else the
thread's own.
*/
func ref(s *session, number int64) (conversations.Ref, error) {
	if number > 0 {
		return conversations.Ref{Number: number}, nil
	}
	if s.conversation != nil {
		return conversations.Ref{ID: s.conversation.ID}, nil
	}
	return conversations.Ref{}, fmt.Errorf("%w: give a conversation number", errBadArguments)
}

/*
getConversation reads one conversation and its latest entries.
*/
func (e *Engine) getConversation(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in numberArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	r, err := ref(s, in.Number)
	if err != nil {
		return result{}, err
	}
	loaded, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{Membership: s.m, Ref: r})
	if err != nil {
		return result{}, err
	}
	timeline, err := e.Services.Messages.Handle(ctx, messages.ListMessagesInput{
		Membership: s.m, Ref: conversations.Ref{ID: loaded.Conversation.ID}, Limit: timelineSize,
	})
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	var b strings.Builder
	writeConversation(&b, loaded)
	writeTimeline(&b, timeline)
	n := loaded.Conversation.Number
	return result{text: b.String(), step: Step{Tool: "get_conversation", Summary: fmt.Sprintf("Read #%d", n)}}, nil
}

/*
writeConversation writes a conversation's details.
*/
func writeConversation(b *strings.Builder, l conversations.Loaded) {
	c := l.Conversation
	fmt.Fprintf(b, "Conversation #%d (id %s)\n", c.Number, models.PublicID(models.PrefixConversation, c.ID))
	fmt.Fprintf(b, "Subject: %s\nStatus: %s\nPriority: %s\nChannel: %s\nInbox: %s\nOpened: %s\n",
		untrusted(deref(c.Subject)), c.Status, enums.ConversationPriorityOf(c.Priority), l.Connection.Type,
		l.Inbox.Name, c.CreatedAt.UTC().Format("2006-01-02 15:04"))
	fmt.Fprintf(b, "Assignee: %s\nTeam: %s\nTags: %s\n", assigneeOf(l), teamOf(l), tagNames(l.Tags))
	fmt.Fprintf(b, "Customer: %s <%s> (id %s)\nAI agent: %s\n", untrusted(deref(l.Contact.Name)),
		deref(l.Contact.Email), models.PublicID(models.PrefixContact, l.Contact.ID), c.AiState)
}

/*
writeTimeline writes a conversation's entries, oldest first.
*/
func writeTimeline(b *strings.Builder, t messages.Timeline) {
	if t.Older {
		b.WriteString("\n(Older messages are not shown.)\n")
	}
	b.WriteString("\nTimeline:\n")
	for _, m := range t.Messages {
		if m.DeletedAt != nil {
			continue
		}
		fmt.Fprintf(b, "[%s] %s: %s\n", m.CreatedAt.UTC().Format("2006-01-02 15:04"), authorOf(m, t), entryText(m))
	}
}

/*
authorOf names who wrote a timeline entry.
*/
func authorOf(m models.Message, t messages.Timeline) string {
	kind := map[string]string{"message": "", "note": "internal note by ", "event": "event"}[m.Type]
	switch m.AuthorType {
	case "contact":
		return "Customer " + untrusted(deref(t.Contact.Name))
	case "member":
		name := "a teammate"
		if m.AuthorID != nil {
			if who, ok := t.Members[*m.AuthorID]; ok {
				name = who.User.Name
			}
		}
		if kind == "" {
			return "Teammate " + name
		}
		return kind + name
	case "agent":
		return "AI agent"
	default:
		return strings.TrimSpace(m.AuthorType + " " + kind)
	}
}

/*
entryText is the text of one entry; what a customer wrote is untrusted.
*/
func entryText(m models.Message) string {
	if m.Type == "event" {
		return string(m.Event)
	}
	text := truncate(strings.TrimSpace(deref(m.Body)), 4000)
	if m.AuthorType == "contact" {
		return untrusted(text)
	}
	return text
}

/*
searchConversations searches conversations and contacts.
*/
func (e *Engine) searchConversations(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Query string `json:"query"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	found, err := e.Services.Search.Handle(ctx, conversations.SearchInput{Membership: s.m, Query: in.Query, Limit: 20})
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	var b strings.Builder
	for _, hit := range found.Conversations {
		c := hit.Conversation.Conversation
		fmt.Fprintf(&b, "#%d %s · %s · %s · %s\n", c.Number, untrusted(deref(c.Subject)), c.Status,
			untrusted(deref(hit.Conversation.Contact.Name)), c.CreatedAt.UTC().Format("2006-01-02"))
		if hit.Highlight != "" {
			fmt.Fprintf(&b, "  match: %s\n", untrusted(hit.Highlight))
		}
	}
	if b.Len() == 0 {
		b.WriteString("No conversations found.")
	}
	summary := fmt.Sprintf("Searched for %q: %d found", in.Query, len(found.Conversations))
	return result{text: b.String(), step: Step{Tool: "search_conversations", Summary: summary}}, nil
}

/*
getCustomer reads a conversation's customer and their history.
*/
func (e *Engine) getCustomer(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in numberArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	r, err := ref(s, in.Number)
	if err != nil {
		return result{}, err
	}
	about, err := e.Services.Context.Handle(ctx, conversations.ContextInput{Membership: s.m, Ref: r})
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	c := about.Contact.Contact
	var b strings.Builder
	fmt.Fprintf(&b, "Customer %s (id %s)\nEmail: %s\nPhone: %s\nLocale: %s\nTimezone: %s\nFirst seen: %s\n",
		untrusted(deref(c.Name)), models.PublicID(models.PrefixContact, c.ID), deref(c.Email), deref(c.Phone),
		deref(c.Locale), deref(c.Timezone), dateOf(c.FirstSeenAt))
	fmt.Fprintf(&b, "Attributes: %s\n", untrusted(string(c.CustomAttributes)))
	fmt.Fprintf(&b, "Other conversations (%d):\n", about.PastTotal)
	for _, p := range about.Past {
		fmt.Fprintf(&b, "  #%d %s · %s · %s\n", p.Conversation.Number, untrusted(deref(p.Conversation.Subject)),
			p.Conversation.Status, p.Conversation.CreatedAt.UTC().Format("2006-01-02"))
	}
	summary := "Read the customer " + truncate(deref(c.Name), 60)
	return result{text: b.String(), step: Step{Tool: "get_customer", Summary: summary}}, nil
}

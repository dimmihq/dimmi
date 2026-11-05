/*
|--------------------------------------------------------------------------
| Exported Conversations
|--------------------------------------------------------------------------
|
| What an export says about each conversation: who it was with, who held
| it, where it came in, and for transcripts and data every message in
| order. The workspace's redaction rules run again over subjects and
| text, so what was written before a rule existed leaves masked too.
|
*/

package dataexports

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	actionredaction "github.com/dimmihq/dimmi/app/actions/redaction"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/redaction"
)

/*
messageBatch is how many conversations' messages are read at once.
*/
const messageBatch = 200

/*
exportedConversation is one conversation as an export writes it.
*/
type exportedConversation struct {
	ID            string            `json:"id"`
	Number        int64             `json:"number"`
	Subject       string            `json:"subject"`
	Status        string            `json:"status"`
	Priority      int16             `json:"priority"`
	Inbox         string            `json:"inbox"`
	Channel       string            `json:"channel"`
	ContactName   string            `json:"contact_name"`
	ContactEmail  string            `json:"contact_email"`
	Company       string            `json:"company"`
	Assignee      string            `json:"assignee"`
	Team          string            `json:"team"`
	Tags          []string          `json:"tags"`
	CreatedAt     time.Time         `json:"created_at"`
	LastMessageAt *time.Time        `json:"last_message_at"`
	ClosedAt      *time.Time        `json:"closed_at"`
	MessageCount  int32             `json:"message_count"`
	Messages      []exportedMessage `json:"messages,omitempty"`
}

/*
exportedMessage is one message or note as an export writes it.
*/
type exportedMessage struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

/*
conversationRows shapes the loaded conversations, masking subjects.
*/
func conversationRows(
	ctx context.Context, q *models.Queries, loaded []conversations.Loaded, r redaction.Redactor,
) ([]exportedConversation, error) {
	companies, err := companyNames(ctx, q, loaded)
	if err != nil {
		return nil, err
	}

	rows := make([]exportedConversation, 0, len(loaded))
	for _, l := range loaded {
		rows = append(rows, conversationRow(l, companies, r))
	}

	return rows, nil
}

/*
conversationRow shapes one conversation.
*/
func conversationRow(
	l conversations.Loaded, companies map[ids.UUID]string, r redaction.Redactor,
) exportedConversation {
	c := l.Conversation
	row := exportedConversation{
		ID: models.PublicID(models.PrefixConversation, c.ID), Number: c.Number, Status: c.Status,
		Priority: c.Priority, Inbox: l.Inbox.Name, Channel: l.Connection.Type, ContactName: deref(l.Contact.Name),
		ContactEmail: deref(l.Contact.Email), CreatedAt: c.CreatedAt, LastMessageAt: c.LastMessageAt,
		ClosedAt: c.ClosedAt, MessageCount: c.MessageCount, Tags: make([]string, 0, len(l.Tags)),
	}

	row.Subject, _ = r.Apply(deref(c.Subject))

	if l.Contact.CompanyID != nil {
		row.Company = companies[*l.Contact.CompanyID]
	}

	if l.Assignee != nil {
		row.Assignee = l.Assignee.User.Name
	}

	if l.Team != nil {
		row.Team = l.Team.Name
	}

	for _, tag := range l.Tags {
		row.Tags = append(row.Tags, tag.Name)
	}

	return row
}

/*
companyNames returns the names of the companies of the conversations'
customers.
*/
func companyNames(
	ctx context.Context, q *models.Queries, loaded []conversations.Loaded,
) (map[ids.UUID]string, error) {
	var wanted []ids.UUID

	for _, l := range loaded {
		if l.Contact.CompanyID != nil {
			wanted = append(wanted, *l.Contact.CompanyID)
		}
	}

	names := map[ids.UUID]string{}
	if len(wanted) == 0 {
		return names, nil
	}

	rows, err := q.CompaniesByIDs(ctx, models.CompaniesByIDsParams{
		WorkspaceID: loaded[0].Conversation.WorkspaceID, Ids: wanted,
	})
	if err != nil {
		return nil, fmt.Errorf("find companies: %w", err)
	}

	for _, row := range rows {
		names[row.ID] = row.Name
	}

	return names, nil
}

/*
messageRun reads the messages of the exported conversations: notes too
when asked, their text masked by the workspace's rules.
*/
type messageRun struct {
	q        *models.Queries
	notes    bool
	redactor redaction.Redactor
}

/*
attach reads the conversations' messages in batches.
*/
func (r messageRun) attach(ctx context.Context, rows []exportedConversation, loaded []conversations.Loaded) error {
	for start := 0; start < len(loaded); start += messageBatch {
		end := min(start+messageBatch, len(loaded))
		if err := r.batch(ctx, rows[start:end], loaded[start:end]); err != nil {
			return err
		}
	}

	return nil
}

/*
batch reads the messages of one batch of conversations.
*/
func (r messageRun) batch(ctx context.Context, rows []exportedConversation, loaded []conversations.Loaded) error {
	ws := loaded[0].Conversation.WorkspaceID
	byID := make(map[ids.UUID]int, len(loaded))
	wanted := make([]ids.UUID, 0, len(loaded))

	for i, l := range loaded {
		byID[l.Conversation.ID] = i
		wanted = append(wanted, l.Conversation.ID)
	}

	msgs, err := r.q.ListDataExportMessages(ctx, models.ListDataExportMessagesParams{
		WorkspaceID: ws, ConversationIds: wanted, IncludeNotes: r.notes,
	})
	if err != nil {
		return fmt.Errorf("read messages: %w", err)
	}

	authors, err := memberNames(ctx, r.q, ws, msgs)
	if err != nil {
		return err
	}

	for _, m := range msgs {
		i := byID[m.ConversationID]
		body, _ := r.redactor.Apply(deref(m.Body))
		rows[i].Messages = append(rows[i].Messages, exportedMessage{
			ID: models.PublicID(models.PrefixMessage, m.ID), Type: m.Type,
			Author: authorOf(m, loaded[i].Contact, authors), Body: body, CreatedAt: m.CreatedAt,
		})
	}

	return nil
}

/*
memberNames returns the names of the members who wrote the messages.
*/
func memberNames(
	ctx context.Context, q *models.Queries, ws ids.UUID, msgs []models.Message,
) (map[ids.UUID]string, error) {
	var wanted []ids.UUID

	for _, m := range msgs {
		if m.AuthorType == "member" && m.AuthorID != nil {
			wanted = append(wanted, *m.AuthorID)
		}
	}

	names := map[ids.UUID]string{}
	if len(wanted) == 0 {
		return names, nil
	}

	rows, err := q.MembersByIDs(ctx, models.MembersByIDsParams{WorkspaceID: ws, Ids: wanted})
	if err != nil {
		return nil, fmt.Errorf("find authors: %w", err)
	}

	for _, row := range rows {
		names[row.WorkspaceMember.ID] = row.User.Name
	}

	return names, nil
}

/*
authorOf names who wrote a message: the customer, the teammate, or the
system for automatic messages and teammates no longer known.
*/
func authorOf(m models.Message, contact models.Contact, members map[ids.UUID]string) string {
	if m.AuthorType == "contact" {
		return fallback(deref(contact.Name), "Customer")
	}

	if m.AuthorType == "member" && m.AuthorID != nil {
		return fallback(members[*m.AuthorID], "Teammate")
	}

	return "System"
}

/*
deref returns the text, empty when unset.
*/
func deref(text *string) string {
	if text == nil {
		return ""
	}

	return *text
}

/*
fallback returns the text, or the stand-in when it is empty.
*/
func fallback(text, standIn string) string {
	if text == "" {
		return standIn
	}

	return text
}

/*
writeConversations writes the conversations in the export's format,
reading messages only for formats that hold them.
*/
func writeConversations(
	ctx context.Context, q *models.Queries, loaded []conversations.Loaded, row models.DataExport,
) ([]file, error) {
	r, err := actionredaction.Load(ctx, q, row.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load redaction rules: %w", err)
	}

	rows, err := conversationRows(ctx, q, loaded, r)
	if err != nil {
		return nil, err
	}

	if row.Format == "csv" {
		return conversationSheet(rows)
	}

	run := messageRun{q: q, notes: row.IncludeNotes, redactor: r}
	if err := run.attach(ctx, rows, loaded); err != nil {
		return nil, err
	}

	if row.Format == "txt" {
		return transcripts(rows), nil
	}

	return data("conversations.json", rows)
}

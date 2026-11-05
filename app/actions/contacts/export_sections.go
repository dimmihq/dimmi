/*
|--------------------------------------------------------------------------
| Contact Export Sections
|--------------------------------------------------------------------------
|
| The core of a customer's export, written the way the person reading it
| would want it: their details and addresses as JSON, each conversation
| both as JSON and as a plain transcript, and the files exchanged in it.
| A file the scan found infected is never copied out; the manifest of the
| conversation names it instead.
|
*/

package contacts

import (
	"archive/zip"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/models"
)

/*
exportedContact is the customer's details in the export.
*/
type exportedContact struct {
	ID               string         `json:"id"`
	Kind             string         `json:"kind"`
	Name             *string        `json:"name"`
	Email            *string        `json:"email"`
	Phone            *string        `json:"phone"`
	ExternalID       *string        `json:"external_id"`
	Locale           *string        `json:"locale"`
	Timezone         *string        `json:"timezone"`
	Location         any            `json:"location"`
	CustomAttributes any            `json:"custom_attributes"`
	Notes            *string        `json:"notes,omitempty"`
	FirstSeenAt      *time.Time     `json:"first_seen_at"`
	LastSeenAt       *time.Time     `json:"last_seen_at"`
	CreatedAt        time.Time      `json:"created_at"`
	Identities       []exportedItem `json:"identities"`
}

/*
exportedItem is an identity in the export.
*/
type exportedItem struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

/*
exportedMessage is one message or note of a conversation.
*/
type exportedMessage struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	From      string    `json:"from"`
	Body      *string   `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

/*
exportedConversation is one conversation with its messages and the files
left out of the export.
*/
type exportedConversation struct {
	ID        string            `json:"id"`
	Number    int64             `json:"number"`
	Subject   *string           `json:"subject"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	Messages  []exportedMessage `json:"messages"`
	Omitted   []string          `json:"omitted_files"`
}

/*
exportRun is one build of an export: where it reads from and the zip
it writes into.
*/
type exportRun struct {
	q       *models.Queries
	storage *filesystem.Manager
	w       *zip.Writer
	row     models.ContactExport
}

/*
writeContact adds contact.json.
*/
func (r exportRun) writeContact(ctx context.Context) error {
	q, w, row := r.q, r.w, r.row

	c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: row.WorkspaceID, ID: row.ContactID})
	if err != nil {
		return fmt.Errorf("find contact: %w", err)
	}

	identities, err := q.ListContactIdentities(ctx, models.ListContactIdentitiesParams{
		WorkspaceID: row.WorkspaceID, ContactID: row.ContactID,
	})
	if err != nil {
		return fmt.Errorf("list identities: %w", err)
	}

	out := exportedContact{
		ID: models.PublicID(models.PrefixContact, c.ID), Kind: c.Kind, Name: c.Name, Email: c.Email, Phone: c.Phone,
		ExternalID: c.ExternalID, Locale: c.Locale, Timezone: c.Timezone, Location: raw(c.Location),
		CustomAttributes: raw(c.CustomAttributes), FirstSeenAt: c.FirstSeenAt, LastSeenAt: c.LastSeenAt,
		CreatedAt: c.CreatedAt, Identities: []exportedItem{},
	}

	if row.IncludeNotes {
		out.Notes = c.Notes
	}

	for _, identity := range identities {
		out.Identities = append(out.Identities, exportedItem{Type: identity.Type, Value: identity.Value})
	}

	return writeJSON(w, "contact.json", out)
}

/*
writeConversations adds each conversation's JSON, transcript and files.
*/
func (r exportRun) writeConversations(ctx context.Context) error {
	q, w, row := r.q, r.w, r.row

	conversations, err := q.ListExportConversations(ctx, models.ListExportConversationsParams{
		WorkspaceID: row.WorkspaceID, ContactID: row.ContactID,
	})
	if err != nil {
		return fmt.Errorf("list conversations: %w", err)
	}

	for _, c := range conversations {
		out, err := r.exportConversation(ctx, c)
		if err != nil {
			return err
		}

		if out.Omitted, err = r.writeAttachments(ctx, c); err != nil {
			return err
		}

		folder := fmt.Sprintf("conversations/%d", c.Number)
		if err := writeJSON(w, folder+".json", out); err != nil {
			return err
		}

		if err := writeFile(w, folder+".txt", []byte(transcript(out))); err != nil {
			return err
		}
	}

	return nil
}

/*
exportConversation reads a conversation's messages, and its notes when
the export includes them.
*/
func (r exportRun) exportConversation(ctx context.Context, c models.Conversation) (exportedConversation, error) {
	row := r.row

	messages, err := r.q.ListExportMessages(ctx, models.ListExportMessagesParams{
		WorkspaceID: row.WorkspaceID, ConversationID: c.ID, IncludeNotes: row.IncludeNotes,
	})
	if err != nil {
		return exportedConversation{}, fmt.Errorf("list messages: %w", err)
	}

	out := exportedConversation{
		ID: models.PublicID(models.PrefixConversation, c.ID), Number: c.Number, Subject: c.Subject,
		Status: c.Status, CreatedAt: c.CreatedAt, Messages: []exportedMessage{}, Omitted: []string{},
	}

	for _, m := range messages {
		out.Messages = append(out.Messages, exportedMessage{
			ID: models.PublicID(models.PrefixMessage, m.ID), Type: m.Type, From: from(m), Body: m.Body,
			CreatedAt: m.CreatedAt,
		})
	}

	return out, nil
}

/*
writeAttachments copies the conversation's clean files into the zip and
returns the names of those it left out.
*/
func (r exportRun) writeAttachments(ctx context.Context, c models.Conversation) ([]string, error) {
	files, err := r.q.ListExportAttachments(ctx, models.ListExportAttachmentsParams{
		WorkspaceID: r.row.WorkspaceID, ConversationID: c.ID, IncludeNotes: r.row.IncludeNotes,
	})
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	omitted := []string{}

	for _, file := range files {
		if file.ScanStatus == "infected" {
			omitted = append(omitted, file.Filename)

			continue
		}

		disk, err := r.storage.Disk(file.Disk)
		if err != nil {
			return nil, fmt.Errorf("open disk %s: %w", file.Disk, err)
		}

		contents, err := disk.Get(ctx, file.Path)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}

		name := fmt.Sprintf("attachments/%d/%s-%s", c.Number, file.ID.String()[:8], file.Filename)
		if err := writeFile(r.w, name, contents); err != nil {
			return nil, err
		}
	}

	return omitted, nil
}

/*
from says who wrote a message, as the transcript names them.
*/
func from(m models.Message) string {
	switch m.AuthorType {
	case "contact":
		return "Customer"
	case "member":
		return "Team"
	}

	return "System"
}

/*
transcript writes the conversation as plain text, one message after the
other.
*/
func transcript(c exportedConversation) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Conversation #%d", c.Number)

	if c.Subject != nil {
		fmt.Fprintf(&b, ": %s", *c.Subject)
	}

	b.WriteString("\n")

	for _, m := range c.Messages {
		body := ""
		if m.Body != nil {
			body = *m.Body
		}

		fmt.Fprintf(&b, "\n%s, %s (%s):\n%s\n", m.From, m.CreatedAt.UTC().Format(time.RFC3339), m.Type, body)
	}

	return b.String()
}

/*
raw passes a stored JSON value through as it is, null when empty.
*/
func raw(value []byte) any {
	if len(value) == 0 {
		return nil
	}

	return jsonRaw(value)
}

/*
jsonRaw is stored JSON written into the export unchanged.
*/
type jsonRaw []byte

/*
MarshalJSON writes the stored JSON as it is.
*/
func (r jsonRaw) MarshalJSON() ([]byte, error) { return r, nil }

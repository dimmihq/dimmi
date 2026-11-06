/*
|--------------------------------------------------------------------------
| Export Formats
|--------------------------------------------------------------------------
|
| Each format writes the rows its own way: a readable transcript per
| conversation, one spreadsheet row per conversation or customer, or the
| data as JSON. Several files leave as one zip; a single file leaves as
| it is, so one transcript opens straight away.
|
*/

package dataexports

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
file is one file of an export.
*/
type file struct {
	name string
	data []byte
}

/*
packed is what an export stores: its file name, type and bytes.
*/
type packed struct {
	name        string
	contentType string
	data        []byte
}

/*
contentTypes maps a file's extension to its type.
*/
func contentTypes() map[string]string {
	return map[string]string{
		"txt": "text/plain; charset=utf-8", "csv": "text/csv; charset=utf-8", "json": "application/json",
		"pdf": "application/pdf", "zip": "application/zip",
	}
}

/*
pack returns a single file as it is and several as one zip.
*/
func pack(files []file) (packed, error) {
	if len(files) == 1 {
		ext := files[0].name[strings.LastIndex(files[0].name, ".")+1:]

		return packed{name: files[0].name, contentType: contentTypes()[ext], data: files[0].data}, nil
	}

	var buf bytes.Buffer

	w := zip.NewWriter(&buf)
	for _, f := range files {
		entry, err := w.Create(f.name)
		if err != nil {
			return packed{}, fmt.Errorf("add %s: %w", f.name, err)
		}

		if _, err := entry.Write(f.data); err != nil {
			return packed{}, fmt.Errorf("write %s: %w", f.name, err)
		}
	}

	if err := w.Close(); err != nil {
		return packed{}, fmt.Errorf("close zip: %w", err)
	}

	return packed{name: "export.zip", contentType: contentTypes()["zip"], data: buf.Bytes()}, nil
}

/*
transcripts writes one text file per conversation.
*/
func transcripts(rows []exportedConversation) []file {
	files := make([]file, 0, len(rows))

	for _, c := range rows {
		var b strings.Builder

		fmt.Fprintf(&b, "Conversation #%d: %s\n", c.Number, c.Subject)
		fmt.Fprintf(&b, "Customer: %s\nStatus: %s\nStarted: %s\n", contactLabel(c), c.Status, stamp(c.CreatedAt))

		for _, m := range c.Messages {
			label := m.Author
			if m.Type == "note" {
				label += " (internal note)"
			}

			fmt.Fprintf(&b, "\n%s, %s:\n%s\n", label, stamp(m.CreatedAt), m.Body)
		}

		name := "conversation-" + strconv.FormatInt(c.Number, 10) + ".txt"
		files = append(files, file{name: name, data: []byte(b.String())})
	}

	return files
}

/*
contactLabel names the customer with their address.
*/
func contactLabel(c exportedConversation) string {
	if c.ContactEmail == "" {
		return fallback(c.ContactName, "Unknown")
	}

	return strings.TrimSpace(c.ContactName + " <" + c.ContactEmail + ">")
}

/*
conversationSheet writes one spreadsheet row per conversation.
*/
func conversationSheet(rows []exportedConversation) ([]file, error) {
	records := [][]string{{
		"number", "subject", "status", "contact", "email", "company", "assignee", "team", "inbox", "channel",
		"priority", "tags", "created_at", "last_message_at", "closed_at", "messages",
	}}

	for _, c := range rows {
		records = append(records, []string{
			strconv.FormatInt(c.Number, 10), c.Subject, c.Status, c.ContactName, c.ContactEmail, c.Company,
			c.Assignee, c.Team, c.Inbox, c.Channel, strconv.Itoa(int(c.Priority)), strings.Join(c.Tags, ", "),
			stamp(c.CreatedAt), optionalStamp(c.LastMessageAt), optionalStamp(c.ClosedAt),
			strconv.Itoa(int(c.MessageCount)),
		})
	}

	return sheet("conversations.csv", records)
}

/*
exportedContact is one customer as an export writes it.
*/
type exportedContact struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Email      string          `json:"email"`
	Phone      string          `json:"phone"`
	Kind       string          `json:"kind"`
	ExternalID string          `json:"external_id"`
	Company    string          `json:"company"`
	Tags       []string        `json:"tags"`
	Attributes json.RawMessage `json:"custom_attributes"`
	LastSeenAt *time.Time      `json:"last_seen_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

/*
contactRows shapes the customers.
*/
func contactRows(details []contacts.Detail) []exportedContact {
	rows := make([]exportedContact, 0, len(details))

	for _, d := range details {
		c := d.Contact
		row := exportedContact{
			ID: models.PublicID(models.PrefixContact, c.ID), Name: deref(c.Name), Email: deref(c.Email),
			Phone: deref(c.Phone), Kind: c.Kind, ExternalID: deref(c.ExternalID), Tags: make([]string, 0, len(d.Tags)),
			Attributes: c.CustomAttributes, LastSeenAt: c.LastSeenAt, CreatedAt: c.CreatedAt,
		}

		if d.Company != nil {
			row.Company = d.Company.Name
		}

		for _, tag := range d.Tags {
			row.Tags = append(row.Tags, tag.Name)
		}

		rows = append(rows, row)
	}

	return rows
}

/*
contactSheet writes one spreadsheet row per customer.
*/
func contactSheet(rows []exportedContact) ([]file, error) {
	records := [][]string{{
		"id", "name", "email", "phone", "kind", "external_id", "company", "tags", "custom_attributes",
		"last_seen_at", "created_at",
	}}

	for _, c := range rows {
		records = append(records, []string{
			c.ID, c.Name, c.Email, c.Phone, c.Kind, c.ExternalID, c.Company, strings.Join(c.Tags, ", "),
			string(c.Attributes), optionalStamp(c.LastSeenAt), stamp(c.CreatedAt),
		})
	}

	return sheet("contacts.csv", records)
}

/*
sheet writes the records as a CSV file. A cell a spreadsheet would read
as a formula is quoted with a leading apostrophe, so opening the file
never runs what a customer typed.
*/
func sheet(name string, records [][]string) ([]file, error) {
	var buf bytes.Buffer

	w := csv.NewWriter(&buf)
	for _, record := range records {
		if err := w.Write(defused(record)); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
	}

	w.Flush()

	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("write %s: %w", name, err)
	}

	return []file{{name: name, data: buf.Bytes()}}, nil
}

/*
defused prefixes cells starting like a formula with an apostrophe.
*/
func defused(record []string) []string {
	out := make([]string, len(record))

	for i, cell := range record {
		if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
			cell = "'" + cell
		}

		out[i] = cell
	}

	return out
}

/*
data writes the rows as one JSON file.
*/
func data(name string, rows any) ([]file, error) {
	encoded, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", name, err)
	}

	return []file{{name: name, data: encoded}}, nil
}

/*
stamp writes a time in UTC.
*/
func stamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

/*
optionalStamp writes a time in UTC, empty when unset.
*/
func optionalStamp(t *time.Time) string {
	if t == nil {
		return ""
	}

	return stamp(*t)
}

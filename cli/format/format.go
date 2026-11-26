/*
Package format is dimmi's import format, version 1, as the dimmi CLI
writes it: source-neutral records, one kind at a time, in dependency
order. Every record carries the id it had in the source tool, its
source_id, and names the records it points at by their source ids, so
the server resolves references and a rerun updates instead of
duplicating. Times are RFC 3339. Bodies are Markdown, except a saved
reply's, which is dimmi's Liquid.

The server's own definition lives in app/actions/migrations; these
types mirror it field for field. Fields the server doesn't read yet
are ignored by it, so a translator may send what the source has before
dimmi can keep it.
*/
package format

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

/*
Version is the import format version these records follow.
*/
const Version = 1

/*
Kind names a record kind.
*/
type Kind string

/*
The record kinds, in the order they are sent.
*/
const (
	KindTeammate            Kind = "teammate"
	KindTeam                Kind = "team"
	KindTag                 Kind = "tag"
	KindAttributeDefinition Kind = "attribute_definition"
	KindCompany             Kind = "company"
	KindContact             Kind = "contact"
	KindContactNote         Kind = "contact_note"
	KindTicketType          Kind = "ticket_type"
	KindHelpCenter          Kind = "help_center"
	KindCollection          Kind = "collection"
	KindArticle             Kind = "article"
	KindSnippet             Kind = "snippet"
	KindSavedReply          Kind = "saved_reply"
	KindSchedule            Kind = "schedule"
	KindConversation        Kind = "conversation"
	KindTicket              Kind = "ticket"
	KindWorkflow            Kind = "workflow"
)

/*
Kinds lists every kind in sending order.
*/
func Kinds() []Kind {
	return []Kind{
		KindTeammate, KindTeam, KindTag, KindAttributeDefinition, KindCompany, KindContact, KindContactNote,
		KindTicketType, KindHelpCenter, KindCollection, KindArticle, KindSnippet, KindSavedReply, KindSchedule,
		KindConversation, KindTicket, KindWorkflow,
	}
}

/*
Valid reports whether the kind is known.
*/
func (k Kind) Valid() bool { return slices.Contains(Kinds(), k) }

/*
File is a file records point at: an attachment or an inline image. The
engine uploads it under SourceID before the records that name it, and
never again once dimmi has it. Source is where the source tool serves
it; it is never sent to dimmi.
*/
type File struct {
	SourceID    string `json:"source_id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Source      string `json:"-"`
}

/*
Record is one record ready to send: its kind and source id, the record
itself, and the files it points at.
*/
type Record struct {
	Kind     Kind
	SourceID string
	Data     any
	Files    []*File
}

/*
NewRecord wraps a record of the kind.
*/
func NewRecord(kind Kind, sourceID string, data any, files ...*File) Record {
	return Record{Kind: kind, SourceID: sourceID, Data: data, Files: files}
}

/*
Line is how a record is written in a JSON Lines file, the format
`dimmi migrate import` reads: the kind, the record, and the local
paths or URLs of the files it names.
*/
type Line struct {
	Kind   Kind            `json:"kind"`
	Record json.RawMessage `json:"record"`
	Files  []LineFile      `json:"files,omitempty"`
}

/*
LineFile is a file a line names, by a local path or a URL.
*/
type LineFile struct {
	SourceID    string `json:"source_id"`
	Name        string `json:"name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
}

/*
ParseLine reads one line of an import file.
*/
func ParseLine(raw []byte) (Record, error) {
	var l Line
	if err := json.Unmarshal(raw, &l); err != nil {
		return Record{}, err
	}

	if !l.Kind.Valid() {
		return Record{}, fmt.Errorf("unknown kind %q", l.Kind)
	}

	var base Base
	if err := json.Unmarshal(l.Record, &base); err != nil {
		return Record{}, err
	}

	if base.SourceID == "" {
		return Record{}, fmt.Errorf("a %s record has no source_id", l.Kind)
	}

	rec := Record{Kind: l.Kind, SourceID: base.SourceID, Data: l.Record}

	for _, f := range l.Files {
		src := f.URL
		if src == "" {
			src = "file://" + f.Path
		}

		rec.Files = append(rec.Files, &File{SourceID: f.SourceID, Name: f.Name, ContentType: f.ContentType, Source: src})
	}

	return rec, nil
}

/*
Base is what every record carries: its id in the source tool and when
it last changed there.
*/
type Base struct {
	SourceID        string     `json:"source_id"`
	SourceUpdatedAt *time.Time `json:"source_updated_at,omitempty"`
}

/*
At returns a pointer to t, or nil for the zero time.
*/
func At(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	t = t.UTC()

	return &t
}

/*
Unix turns a Unix time in seconds into a time, nil for 0.
*/
func Unix(sec int64) *time.Time {
	if sec <= 0 {
		return nil
	}

	return At(time.Unix(sec, 0))
}

/*
FileID names a file the same way on every run. Sources often sign their
download links, so the query string changes each time; the path, name
and size don't.
*/
func FileID(source, name string, size int64) string {
	path, _, _ := strings.Cut(source, "?")
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%d", path, name, size))

	return "file_" + hex.EncodeToString(sum[:12])
}

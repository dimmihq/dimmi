/*
|--------------------------------------------------------------------------
| Conversation Entries
|--------------------------------------------------------------------------
|
| Messages, replies and notes are written at their own time, never
| before the conversation's latest, as whoever wrote them, with their
| files and email headers. A customer writing again opens it again.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
at returns when the entry is written: its own time, or the
conversation's latest when it is older, as entries never land before
what is already there.
*/
func (r *replay) at(e Entry) time.Time {
	at := e.CreatedAt.UTC()
	if latest := conversations.Latest(r.conv); at.Before(latest) {
		if !r.clamped {
			r.out.warn("entries", "entry_time_adjusted",
				"Entries older than the conversation's latest were written at its latest time.")
			r.clamped = true
		}

		return latest
	}

	return at
}

/*
writer returns the conversation writer acting as the author at the
time.
*/
func (r *replay) writer(a Author, at time.Time) *conversations.Writer {
	return r.w.Importing(at).As(r.actor(a))
}

/*
actor returns who the timeline says the author is.
*/
func (r *replay) actor(a Author) conversations.Actor {
	switch a.Type {
	case AuthorContact:
		if id, ok := r.s.refs.id(KindContact, a.Ref); ok && id == r.conv.ContactID {
			return conversations.Actor{Type: string(enums.AuthorTypeContact), ID: id, Name: deref(r.contact.Name)}
		}

		if id, ok := r.s.refs.id(KindContact, a.Ref); ok {
			return conversations.Actor{Type: string(enums.AuthorTypeContact), ID: id}
		}
	case AuthorTeammate:
		if id, ok := r.s.refs.id(KindTeammate, a.Ref); ok {
			return conversations.Actor{
				Type: string(enums.AuthorTypeMember), ID: id, Name: r.s.refs.label(KindTeammate, a.Ref),
			}
		}

		return conversations.Actor{Type: string(enums.AuthorTypeSystem), Name: r.s.authorName(&a)}
	case AuthorAIAgent:
		return conversations.Actor{Type: string(enums.AuthorTypeAgent), Name: r.s.authorName(&a)}
	}

	return conversations.Actor{Type: string(enums.AuthorTypeSystem)}
}

/*
deref returns the text, empty for nil.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

/*
entryKeyFor returns the key an entry is kept under in the conversation.
*/
func (r *replay) entryKeyFor(external string, e Entry) *string {
	return new("migration:" + external + ":" + e.SourceID)
}

/*
entryKey returns the key the entry is kept under, once the conversation
is there.
*/
func (r *replay) entryKey(e Entry) *string {
	return r.entryKeyFor(r.external(), e)
}

/*
entryBody returns the text of a message, reply or note.
*/
func entryBody(e Entry) string {
	if e.Redacted && strings.TrimSpace(e.Body) == "" {
		return "[redacted]"
	}

	if strings.TrimSpace(e.Body) == "" {
		return "(attachment)"
	}

	return e.Body
}

/*
entryMetadata returns what the entry keeps about where it came from: that
it was imported, its id there, the label of an author who is no member
here, and whether it was redacted there.
*/
func (r *replay) entryMetadata(e Entry) (json.RawMessage, error) {
	data := map[string]any{"imported": true, "external_id": e.SourceID}
	if name := r.s.authorName(&e.Author); name != "" {
		if _, ok := r.s.member(&e.Author); !ok {
			data["author_name"] = name
		}
	}

	if e.Redacted {
		data["redacted"] = true
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode entry metadata: %w", err)
	}

	return out, nil
}

/*
entry replays one entry.
*/
func (r *replay) entry(ctx context.Context, e Entry) error {
	if e.Type == EntryEvent {
		return r.event(ctx, e)
	}

	key := r.entryKey(e)

	if _, err := r.s.q.FindMessageByDedupeKey(ctx, models.FindMessageByDedupeKeyParams{
		WorkspaceID: r.s.ws(), DedupeKey: key,
	}); err == nil {
		return nil
	}

	at := r.at(e)
	w := r.writer(e.Author, at)

	kind := string(enums.MessageTypeMessage)
	if e.Type == EntryNote {
		kind = string(enums.MessageTypeNote)
	}

	msg, err := conversations.ImportedMessage(conversations.ImportedEntry{
		Kind: kind, Body: entryBody(e), ByMember: e.Type == EntryReply, Key: key,
	}, conversations.Backfill{At: at})
	if err != nil {
		return err
	}

	if msg.Metadata, err = r.entryMetadata(e); err != nil {
		return err
	}

	written, conv, err := w.Append(ctx, r.conv, msg)
	if err != nil {
		return err
	}

	r.conv = conv

	if err := r.extras(ctx, e, written); err != nil {
		return err
	}

	return r.reopen(ctx, w, e)
}

/*
reopen opens the conversation again when the customer writes to it,
unless a teammate changed its status here since the migration did.
*/
func (r *replay) reopen(ctx context.Context, w *conversations.Writer, e Entry) error {
	if e.Type != EntryMessage || r.conv.Status == string(enums.ConversationStatusOpen) || r.conv.Spam ||
		r.conflicts[fieldStatus] {
		return nil
	}

	conv, _, err := w.ChangeStatus(ctx, r.conv, conversations.StatusChange{
		Status: string(enums.ConversationStatusOpen), Reason: conversations.ReasonCustomerMessage,
	})
	if err == nil {
		r.conv = conv
	}

	return r.allowed(err)
}

/*
extras attaches the entry's files and files its email headers.
*/
func (r *replay) extras(ctx context.Context, e Entry, msg models.Message) error {
	s := r.s

	var files []ids.UUID

	for _, public := range e.Attachments {
		row, err := s.file(ctx, public)
		if err != nil {
			r.out.warn("attachments", "file_unavailable", "Files not uploaded to this migration were left out.")

			continue
		}

		files = append(files, row.ID)
	}

	if len(files) > 0 {
		if _, err := s.q.LinkAttachments(ctx, models.LinkAttachmentsParams{
			MessageID: &msg.ID, Now: s.now, WorkspaceID: s.ws(), Ids: files,
		}); err != nil {
			return fmt.Errorf("attach files: %w", err)
		}
	}

	return r.emailRefs(ctx, e, msg)
}

/*
emailRefs files the entry's Message-ID and the ids it referenced under
the conversation, so a customer replying to the old thread lands here.
*/
func (r *replay) emailRefs(ctx context.Context, e Entry, msg models.Message) error {
	if e.Email == nil {
		return nil
	}

	s := r.s

	kind := "outbound"
	if e.Type == EntryMessage {
		kind = "inbound"
	}

	file := func(id, kind string, message *ids.UUID) error {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 998 {
			return nil
		}

		err := s.q.CreateEmailMessageRef(ctx, models.CreateEmailMessageRefParams{
			WorkspaceID: s.ws(), RfcMessageID: id, ConversationID: r.conv.ID, MessageID: message, Kind: kind,
			Now: e.CreatedAt.UTC(),
		})
		if err != nil {
			return fmt.Errorf("file email reference: %w", err)
		}

		return nil
	}

	if err := file(e.Email.MessageID, kind, &msg.ID); err != nil {
		return err
	}

	for _, ref := range append([]string{e.Email.InReplyTo}, e.Email.References...) {
		if err := file(ref, "referenced", nil); err != nil {
			return err
		}
	}

	return nil
}

/*
|--------------------------------------------------------------------------
| Conversations
|--------------------------------------------------------------------------
|
| A conversation is replayed entry by entry, oldest first, each at its
| own time and by whoever wrote it or made it: messages, replies and
| notes with their files and email headers, then assignments, teams,
| statuses with their snooze, priorities, tags and anything else the
| other tool kept, which joins the timeline with its own type and text.
| The state it ended in is then applied, and a closed conversation
| keeps its rating. Nothing is sent and nothing reacts.
|
| A later sync appends the entries that are new, in order, and moves
| the conversation to the state it is in now, except for a field a
| teammate changed here since the migration last wrote it: that field
| stays as the teammate set it and is reported as a conflict.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxEntries is how many entries one conversation record carries.
*/
const maxEntries = 5000

/*
The fields a sync compares with what the migration last wrote.
*/
const (
	fieldStatus   = "status"
	fieldAssignee = "assignee_ref"
	fieldTeam     = "team_ref"
	fieldPriority = "priority"
	fieldTags     = "tag_refs"
)

/*
convState is what a sync of a conversation remembers: the entries
already replayed, and the value of each field the migration last left.
*/
type convState struct {
	Entries []string          `json:"entries"`
	Fields  map[string]string `json:"fields"`
}

func (c *Conversation) validate() error {
	if err := required("contact_ref", c.ContactRef); err != nil {
		return err
	}

	if c.CreatedAt.IsZero() {
		return invalid("created_at", "created_at is required.")
	}

	if err := oneOf("status", c.Status, "open", "closed", "snoozed"); err != nil {
		return err
	}

	if err := oneOf("priority", c.Priority, "none", "low", "medium", "high", "urgent"); err != nil {
		return err
	}

	if c.Rating != nil && (c.Rating.Score < 1 || c.Rating.Score > 5) {
		return invalid("rating.score", "A rating is 1 to 5.")
	}

	if len(c.Entries) > maxEntries {
		return invalid("entries", fmt.Sprintf("A conversation carries at most %d entries.", maxEntries))
	}

	seen := map[string]bool{}

	for i, e := range c.Entries {
		if err := e.check(fmt.Sprintf("entries[%d]", i)); err != nil {
			return err
		}

		if seen[e.SourceID] {
			return invalid(fmt.Sprintf("entries[%d].source_id", i), "Each entry has its own source_id.")
		}

		seen[e.SourceID] = true
	}

	if !slices.ContainsFunc(c.Entries, func(e Entry) bool { return e.Type == EntryMessage || e.Type == EntryReply }) {
		return invalid("entries", "A conversation has at least one message or reply.")
	}

	return nil
}

/*
check refuses an entry that cannot be replayed.
*/
func (e Entry) check(at string) error {
	if strings.TrimSpace(e.SourceID) == "" {
		return invalid(at+".source_id", "Every entry has a source_id.")
	}

	if e.CreatedAt.IsZero() {
		return invalid(at+".created_at", "Every entry has a created_at.")
	}

	if err := oneOf(at+".type", e.Type, EntryMessage, EntryReply, EntryNote, EntryEvent); err != nil {
		return err
	}

	if e.Type == "" {
		return invalid(at+".type", "Every entry has a type.")
	}

	if err := checkAuthor(at+".author", e.Author); err != nil {
		return err
	}

	if e.Type != EntryEvent {
		if strings.TrimSpace(e.Body) == "" && len(e.Attachments) == 0 && !e.Redacted {
			return invalid(at+".body", "A message, reply or note has a body or attachments.")
		}

		return nil
	}

	if e.Event == nil {
		return invalid(at+".event", "An event entry describes its event.")
	}

	return oneOf(at+".event.type", e.Event.Type, EventAssignment, EventTeamAssignment, EventStatus, EventPriority,
		EventTag, EventAttribute, EventCustom)
}

func (c *Conversation) refs() []Ref {
	out := refsOf(KindContact, c.ContactRef)
	out = append(out, refsOf(KindTeammate, c.AssigneeRef)...)
	out = append(out, refsOf(KindTeam, c.TeamRef)...)
	out = append(out, refsOf(KindTag, c.TagRefs...)...)

	for _, e := range c.Entries {
		out = append(out, authorRefs(&e.Author)...)
		if e.Event == nil {
			continue
		}

		if e.Event.AssigneeRef != nil {
			out = append(out, refsOf(KindTeammate, *e.Event.AssigneeRef)...)
		}

		if e.Event.TeamRef != nil {
			out = append(out, refsOf(KindTeam, *e.Event.TeamRef)...)
		}

		out = append(out, refsOf(KindTag, e.Event.AddedTagRefs...)...)
		out = append(out, refsOf(KindTag, e.Event.RemovedTagRef...)...)
	}

	return out
}

func (c *Conversation) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: c.Subject}

	if len(c.ParticipantRefs) > 0 {
		out.unsupported("participant_refs")
	}

	if len(c.Attributes) > 0 {
		out.unsupported("custom_attributes")
	}

	contactID, err := s.refs.must("contact_ref", KindContact, c.ContactRef)
	if err != nil {
		return out, err
	}

	entries := slices.Clone(c.Entries)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CreatedAt.Before(entries[j].CreatedAt) })

	r := &replay{s: s, c: c, out: &out, state: convState{Fields: map[string]string{}}}

	resumed, err := r.resume(ctx)
	if err != nil {
		return out, err
	}

	if resumed {
		return r.finish(ctx, entries)
	}

	if err := r.open(ctx, contactID, entries); err != nil {
		return out, err
	}

	out.created = true

	return r.finish(ctx, entries)
}

/*
replay is one conversation record being written: the conversation as it
now is, its customer, the sync's memory, the fields in conflict, and the
outcome so far.
*/
type replay struct {
	s         *step
	c         *Conversation
	out       *outcome
	conv      models.Conversation
	contact   models.Contact
	w         *conversations.Writer
	state     convState
	conflicts map[string]bool
	clamped   bool
}

/*
resume picks up the conversation this record made before, with what the
migration last left in it and the fields a teammate changed since, and
reports whether it is still there.
*/
func (r *replay) resume(ctx context.Context) (bool, error) {
	s := r.s
	if s.prev == nil || s.prev.RecordID == nil {
		return false, nil
	}

	found, err := s.q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: s.ws(), ID: *s.prev.RecordID})
	if err != nil || found.DeletedAt != nil {
		return false, nil //nolint:nilerr // a conversation gone since is brought in again
	}

	_ = json.Unmarshal(s.prev.State, &r.state)
	if r.state.Fields == nil {
		r.state.Fields = map[string]string{}
	}

	r.conv = found
	if err := r.load(ctx); err != nil {
		return false, err
	}

	r.findConflicts(ctx)

	return true, nil
}

/*
load reads the conversation's customer and makes its writer.
*/
func (r *replay) load(ctx context.Context) error {
	s := r.s

	contact, err := s.q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.ws(), ID: r.conv.ContactID})
	if err != nil {
		return fmt.Errorf("find contact: %w", err)
	}

	r.contact = contact

	services := conversations.Services{Events: s.kit().Events, Clock: s.runner.svc.Clock}

	w, err := conversations.NewWriter(ctx, s.tx, services, s.runner.m)
	if err != nil {
		return err
	}

	r.w = w

	return nil
}

/*
open starts the conversation with its first message or reply.
*/
func (r *replay) open(ctx context.Context, contactID ids.UUID, entries []Entry) error {
	s, c := r.s, r.c

	i := slices.IndexFunc(entries, func(e Entry) bool { return e.Type == EntryMessage || e.Type == EntryReply })
	first := entries[i]

	conn, err := r.connection(ctx)
	if err != nil {
		return err
	}

	in := conversations.CreateConversationInput{
		Membership: s.runner.m, ByContact: first.Type == EntryMessage, ConnectionID: &conn.ID,
		Contact: contacts.Ref{ID: &contactID}, Subject: orNil(c.Subject), Body: entryBody(first),
	}

	imp, err := r.opening(first)
	if err != nil {
		return err
	}

	loaded, _, err := s.kit().Conversations.Import(ctx, in, imp)
	if err != nil {
		return err
	}

	r.conv = loaded.Conversation
	if err := r.load(ctx); err != nil {
		return err
	}

	msg, err := s.q.FindMessageByDedupeKey(ctx, models.FindMessageByDedupeKeyParams{
		WorkspaceID: s.ws(), DedupeKey: imp.OpeningKey,
	})
	if err != nil {
		return fmt.Errorf("find opening message: %w", err)
	}

	if err := r.extras(ctx, first, msg); err != nil {
		return err
	}

	r.state.Entries = append(r.state.Entries, first.SourceID)

	return nil
}

/*
opening returns how the conversation starts: at its own time or its
first entry's, whichever is earlier, kept under the migration's id for
it, with its first entry's key, metadata and author.
*/
func (r *replay) opening(first Entry) (conversations.ConversationImport, error) {
	external := r.s.runner.row.ID.String() + ":" + r.s.rec.base.SourceID

	metadata, err := r.entryMetadata(first)
	if err != nil {
		return conversations.ConversationImport{}, err
	}

	at := first.CreatedAt.UTC()
	if r.c.CreatedAt.Before(at) {
		at = r.c.CreatedAt.UTC()
	}

	imp := conversations.ConversationImport{
		Backfill:   conversations.Backfill{At: at, ExternalID: &external},
		OpeningKey: r.entryKeyFor(external, first), OpeningMetadata: metadata, Source: r.source(external),
	}

	if first.Type == EntryReply {
		actor := r.actor(first.Author)
		imp.Opener = &actor
	}

	return imp, nil
}

/*
source returns what the conversation keeps about where it came from.
*/
func (r *replay) source(external string) json.RawMessage {
	data := map[string]any{"imported": true, "external_id": external, "migration": true}
	if t := r.c.Source.Type; t != "" {
		data["type"] = cut(t, 60)
	}

	if v := r.c.Source.VisitorRef; v != "" {
		data["visitor_ref"] = cut(v, 255)
	}

	return mustJSON(data)
}

/*
connection returns the channel the conversation lands on: one of its
channel's type in the inbox, else the inbox's API channel.
*/
func (r *replay) connection(ctx context.Context) (models.ChannelConnection, error) {
	s := r.s

	inbox, err := s.q.FindDefaultInbox(ctx, s.ws())
	if err != nil {
		return models.ChannelConnection{}, fmt.Errorf("find default inbox: %w", err)
	}

	if t := strings.TrimSpace(r.c.Channel); t != "" && t != "api" {
		conn, err := s.q.FindInboxConnectionOfType(ctx, models.FindInboxConnectionOfTypeParams{
			WorkspaceID: s.ws(), InboxID: inbox.ID, Type: t,
		})
		if err == nil {
			return conn, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return conn, fmt.Errorf("find channel: %w", err)
		}
	}

	conn, err := s.q.FindInboxAPIConnection(ctx, models.FindInboxAPIConnectionParams{
		WorkspaceID: s.ws(), InboxID: inbox.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return conn, exceptions.ErrNoAPIChannel
	}

	if err != nil {
		return conn, fmt.Errorf("find api channel: %w", err)
	}

	return conn, nil
}

/*
finish replays the entries not yet replayed, applies the final state
and the rating, and returns the outcome with the sync's memory.
*/
func (r *replay) finish(ctx context.Context, entries []Entry) (outcome, error) {
	for _, e := range entries {
		if slices.Contains(r.state.Entries, e.SourceID) {
			continue
		}

		if err := r.entry(ctx, e); err != nil {
			return *r.out, err
		}

		r.state.Entries = append(r.state.Entries, e.SourceID)
	}

	if err := r.final(ctx); err != nil {
		return *r.out, err
	}

	if err := r.rating(ctx); err != nil {
		return *r.out, err
	}

	r.out.id = r.conv.ID
	r.out.state = mustJSON(r.state)

	return *r.out, nil
}

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
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/enums"
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

	if s.prev != nil && s.prev.RecordID != nil {
		found, err := s.q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: s.ws(), ID: *s.prev.RecordID})
		if err == nil && found.DeletedAt == nil {
			_ = json.Unmarshal(s.prev.State, &r.state)
			if r.state.Fields == nil {
				r.state.Fields = map[string]string{}
			}

			r.conv = found
			if err := r.load(ctx); err != nil {
				return out, err
			}

			r.findConflicts(ctx)

			return r.finish(ctx, entries)
		}
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
load reads the conversation's customer and makes its writer.
*/
func (r *replay) load(ctx context.Context) error {
	s := r.s

	contact, err := s.q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.ws(), ID: r.conv.ContactID})
	if err != nil {
		return fmt.Errorf("find contact: %w", err)
	}

	r.contact = contact

	w, err := conversations.NewWriter(ctx, s.tx, conversations.Services{Events: s.kit().Events, Clock: s.runner.svc.Clock},
		s.runner.m)
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

	actor := r.actor(first.Author)
	external := s.runner.row.ID.String() + ":" + s.rec.base.SourceID

	in := conversations.CreateConversationInput{
		Membership: s.runner.m, ByContact: first.Type == EntryMessage, ConnectionID: &conn.ID,
		Contact: contacts.Ref{ID: &contactID}, Subject: orNil(c.Subject), Body: entryBody(first),
	}

	metadata, err := r.entryMetadata(first)
	if err != nil {
		return err
	}

	at := first.CreatedAt.UTC()
	if c.CreatedAt.Before(at) {
		at = c.CreatedAt.UTC()
	}

	imp := conversations.ConversationImport{
		Backfill:   conversations.Backfill{At: at, ExternalID: &external},
		OpeningKey: r.entryKeyFor(external, first), OpeningMetadata: metadata, Source: r.source(external),
	}
	if first.Type == EntryReply {
		imp.Opener = &actor
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
	return r.entryKeyFor(r.s.runner.row.ID.String()+":"+r.s.rec.base.SourceID, e)
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

	if e.Type != EntryMessage || r.conv.Status == string(enums.ConversationStatusOpen) || r.conv.Spam {
		return nil
	}

	if r.conflicts[fieldStatus] {
		return nil
	}

	r.conv, _, err = w.ChangeStatus(ctx, r.conv, conversations.StatusChange{
		Status: string(enums.ConversationStatusOpen), Reason: conversations.ReasonCustomerMessage,
	})

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

/*
event replays one event entry.
*/
func (r *replay) event(ctx context.Context, e Entry) error {
	ev := e.Event
	w := r.writer(e.Author, r.at(e))

	switch ev.Type {
	case EventAssignment:
		if r.conflicts[fieldAssignee] {
			return nil
		}

		return r.assign(ctx, w, ev.AssigneeRef, true, e)
	case EventTeamAssignment:
		if r.conflicts[fieldTeam] {
			return nil
		}

		return r.assign(ctx, w, ev.TeamRef, false, e)
	case EventStatus:
		if r.conflicts[fieldStatus] {
			return nil
		}

		return r.status(ctx, w, ev.Status, ev.Until)
	case EventPriority:
		if r.conflicts[fieldPriority] {
			return nil
		}

		return r.priority(ctx, w, ev.Priority)
	case EventTag:
		if r.conflicts[fieldTags] {
			return nil
		}

		return r.tags(ctx, w, ev.AddedTagRefs, ev.RemovedTagRef)
	case EventAttribute:
		r.out.unsupported("entries.event.attribute")

		return nil
	}

	return r.custom(ctx, w, ev.RawType, ev.Text)
}

/*
custom adds an event the other tool kept to the timeline, with its own
type and text.
*/
func (r *replay) custom(ctx context.Context, w *conversations.Writer, rawType, text string) error {
	return w.Record(ctx, r.conv, "conversation.imported_event", map[string]any{
		"raw_type": cut(rawType, 100), "text": cut(text, 2000),
	})
}

/*
assign hands the conversation to the teammate or team the event names,
nobody for an empty one; one who is not active here is kept on the
timeline as text instead.
*/
func (r *replay) assign(ctx context.Context, w *conversations.Writer, ref *string, member bool, e Entry) error {
	to := conversations.Assignment{Member: r.conv.AssigneeMemberID, Team: r.conv.AssigneeTeamID}
	kind := KindTeam

	if member {
		kind = KindTeammate
	}

	var id *ids.UUID

	if ref != nil && *ref != "" {
		found, ok := r.s.refs.id(kind, *ref)
		if !ok {
			return r.custom(ctx, w, "assignment", "Assigned to "+r.nameOf(kind, *ref))
		}

		id = &found
	}

	if member {
		to.Member = id
	} else {
		to.Team = id
	}

	conv, _, err := w.Assign(ctx, r.conv, to)
	if errors.Is(err, exceptions.ErrMemberInactive) {
		return r.custom(ctx, w, "assignment", "Assigned to "+r.nameOf(kind, deref(ref)))
	}

	if err != nil {
		return err
	}

	r.conv = conv
	_ = e

	return nil
}

/*
nameOf returns what the record of the kind was called.
*/
func (r *replay) nameOf(kind, ref string) string {
	if name := r.s.refs.label(kind, ref); name != "" {
		return name
	}

	return "someone no longer here"
}

/*
status moves the conversation to the status, snoozed until the time
given; a move the conversation cannot make from where it is is skipped.
*/
func (r *replay) status(ctx context.Context, w *conversations.Writer, status string, until *time.Time) error {
	if status == "" || status == r.conv.Status {
		return nil
	}

	change := conversations.StatusChange{Status: status}

	if status == string(enums.ConversationStatusSnoozed) {
		if until == nil {
			r.out.warn("status", "snooze_without_until", "A snooze without an end was left out.")

			return nil
		}

		change.Until = until.UTC()
	}

	conv, _, err := w.ChangeStatus(ctx, r.conv, change)
	if err := r.allowed(err); err != nil {
		return err
	}

	if err == nil {
		r.conv = conv
	}

	return nil
}

/*
allowed drops a refusal of a status move, keeping it as a warning.
*/
func (r *replay) allowed(err error) error {
	if errors.Is(err, exceptions.ErrInvalidTransition) || errors.Is(err, exceptions.ErrSnoozeInPast) ||
		errors.Is(err, exceptions.ErrSnoozeTooFar) {
		r.out.warn("status", "status_skipped", "A status change the conversation could not make was left out.")

		return nil
	}

	return err
}

/*
priority sets the conversation's priority.
*/
func (r *replay) priority(ctx context.Context, w *conversations.Writer, p string) error {
	if p == "" {
		return nil
	}

	level := enums.ConversationPriority(p)

	conv, _, err := w.SetAttributes(ctx, r.conv, conversations.Attributes{Priority: &level})
	if err != nil {
		return err
	}

	r.conv = conv

	return nil
}

/*
tags puts the tags on the conversation and takes the others off.
*/
func (r *replay) tags(ctx context.Context, w *conversations.Writer, added, removed []string) error {
	if ids := r.s.refs.ids(KindTag, added); len(ids) > 0 {
		if _, err := w.AddTags(ctx, r.conv, tags.Refs{IDs: ids}, nil); err != nil {
			return err
		}
	}

	if ids := r.s.refs.ids(KindTag, removed); len(ids) > 0 {
		if _, err := w.RemoveTags(ctx, r.conv, ids); err != nil {
			return err
		}
	}

	return nil
}

/*
fields returns the conversation's fields as a sync compares them.
*/
func (r *replay) fields(ctx context.Context) map[string]string {
	out := map[string]string{
		fieldStatus: r.conv.Status, fieldPriority: priorityName(r.conv.Priority),
		fieldAssignee: idText(r.conv.AssigneeMemberID), fieldTeam: idText(r.conv.AssigneeTeamID),
	}

	rows, err := r.s.q.ListConversationTags(ctx, models.ListConversationTagsParams{
		WorkspaceID: r.s.ws(), ConversationIds: []ids.UUID{r.conv.ID},
	})
	if err == nil {
		var list []string
		for _, row := range rows {
			list = append(list, row.Tag.ID.String())
		}

		sort.Strings(list)
		out[fieldTags] = strings.Join(list, ",")
	}

	return out
}

/*
priorityName returns the name of a priority level.
*/
func priorityName(level int16) string {
	for _, p := range []enums.ConversationPriority{
		enums.ConversationPriorityNone, enums.ConversationPriorityLow, enums.ConversationPriorityMedium,
		enums.ConversationPriorityHigh, enums.ConversationPriorityUrgent,
	} {
		if l, ok := p.Level(); ok && l == level {
			return string(p)
		}
	}

	return ""
}

/*
idText returns the id as text, empty for nil.
*/
func idText(id *ids.UUID) string {
	if id == nil {
		return ""
	}

	return id.String()
}

/*
findConflicts notes each field a teammate changed here since the
migration last wrote it.
*/
func (r *replay) findConflicts(ctx context.Context) {
	r.conflicts = map[string]bool{}

	for field, now := range r.fields(ctx) {
		last, ok := r.state.Fields[field]
		if ok && last != now {
			r.conflicts[field] = true
		}
	}
}

/*
final moves the conversation to the state it ended in: its status,
priority, assignee, team and tags, each when sent and not in conflict,
and remembers what the migration left.
*/
func (r *replay) final(ctx context.Context) error {
	c := r.c
	at := conversations.Latest(r.conv)
	w := r.w.Importing(at).AsSystem()

	want := map[string]string{}

	if c.Status != "" {
		want[fieldStatus] = c.Status
	}

	if c.Priority != "" {
		want[fieldPriority] = c.Priority
	}

	if c.AssigneeRef != "" {
		id, _ := r.s.refs.id(KindTeammate, c.AssigneeRef)
		want[fieldAssignee] = idText(nilIfZero(id))
	}

	if c.TeamRef != "" {
		id, _ := r.s.refs.id(KindTeam, c.TeamRef)
		want[fieldTeam] = idText(nilIfZero(id))
	}

	now := r.fields(ctx)

	for field, value := range want {
		if value == now[field] {
			continue
		}

		if r.conflicts[field] {
			r.out.conflicts = append(r.out.conflicts, Conflict{Field: field, Dimmi: now[field], Source: value})

			continue
		}

		if err := r.set(ctx, w, field); err != nil {
			return err
		}
	}

	if len(c.TagRefs) > 0 {
		if r.conflicts[fieldTags] {
			r.out.conflicts = append(r.out.conflicts, Conflict{Field: fieldTags, Dimmi: now[fieldTags], Source: c.TagRefs})
		} else if err := r.tags(ctx, w, c.TagRefs, nil); err != nil {
			return err
		}
	}

	for field, value := range r.fields(ctx) {
		if !r.conflicts[field] {
			r.state.Fields[field] = value
		}
	}

	return nil
}

/*
nilIfZero returns nil for the nil id.
*/
func nilIfZero(id ids.UUID) *ids.UUID {
	if id == ids.Nil() {
		return nil
	}

	return &id
}

/*
set applies one field of the final state.
*/
func (r *replay) set(ctx context.Context, w *conversations.Writer, field string) error {
	c := r.c

	switch field {
	case fieldStatus:
		return r.status(ctx, w, c.Status, c.SnoozedUntil)
	case fieldPriority:
		return r.priority(ctx, w, c.Priority)
	case fieldAssignee:
		ref := c.AssigneeRef
		return r.assign(ctx, w, &ref, true, Entry{})
	case fieldTeam:
		ref := c.TeamRef
		return r.assign(ctx, w, &ref, false, Entry{})
	}

	return nil
}

/*
rating keeps how the customer rated the conversation, once it is
closed.
*/
func (r *replay) rating(ctx context.Context) error {
	rt := r.c.Rating
	if rt == nil {
		return nil
	}

	if r.conv.Status != string(enums.ConversationStatusClosed) || r.conv.ClosedAt == nil {
		r.out.warn("rating", "conversation_not_closed", "A rating is kept only on a closed conversation.")

		return nil
	}

	channel := "widget"
	if r.c.Channel == "email" {
		channel = "email"
	}

	err := r.s.q.MigrateRating(ctx, models.MigrateRatingParams{
		WorkspaceID: r.s.ws(), ConversationID: r.conv.ID, ContactID: r.conv.ContactID,
		MemberID: r.conv.AssigneeMemberID, Channel: channel, ClosedAt: *r.conv.ClosedAt,
		RatedAt: rt.CreatedAt.UTC(), Score: new(int16(rt.Score)), Comment: orNil(cut(rt.Remark, 2000)), //nolint:gosec // 1 to 5
	})
	if err != nil {
		return fmt.Errorf("keep rating: %w", err)
	}

	return nil
}

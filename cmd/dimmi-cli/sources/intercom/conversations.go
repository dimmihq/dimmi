package intercom

import (
	"encoding/json"
	"strings"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
	"github.com/dimmihq/dimmi/framework/dom"
)

/*
channel maps a conversation source's type onto dimmi's channels; the
original type is kept on the conversation's source.
*/
func channel(sourceType string) string {
	switch sourceType {
	case "conversation", "chat", "push", "":
		return "chat"
	case "email":
		return "email"
	case "whatsapp", "facebook", "instagram", "sms":
		return sourceType
	case "phone_call", "phone_switch", "call":
		return "phone"
	}

	return "api"
}

/*
contactTypes are the author types of a customer.
*/
func isContact(t string) bool { return t == "user" || t == "lead" || t == "contact" }

/*
authorOf maps a part's author. Bots and Fin are AI agents, labelled
with the name they showed; a team author is the system.
*/
func authorOf(a author, contacts map[string]bool) format.Author {
	switch {
	case isContact(a.Type):
		return format.Author{Type: format.AuthorContact, Ref: string(a.ID)}
	case a.Type == "bot" || a.FromAIAgent || a.IsAIAnswer:
		label := text(a.Name)
		if a.FromAIAgent || a.IsAIAnswer || label == "" {
			label = firstOf(label, "Fin")
		}

		return format.Author{Type: format.AuthorAIAgent, Label: label}
	case a.Type == "admin" && a.ID != "":
		return format.Author{Type: format.AuthorTeammate, Ref: string(a.ID)}
	}

	if contacts[string(a.ID)] {
		return format.Author{Type: format.AuthorContact, Ref: string(a.ID)}
	}

	return format.Author{Type: format.AuthorSystem, Label: text(a.Name)}
}

/*
files turns attachments into file records and the ids entries name.
*/
func files(list []attachment) ([]*format.File, []string) {
	var out []*format.File
	var ids []string

	for _, a := range list {
		if a.URL == "" {
			continue
		}

		f := &format.File{
			SourceID: format.FileID(a.URL, a.Name, a.Filesize), Name: firstOf(a.Name, "attachment"),
			ContentType: a.ContentType, Size: a.Filesize, Source: a.URL,
		}
		out = append(out, f)
		ids = append(ids, f.SourceID)
	}

	return out, ids
}

/*
Former is a teammate an entry names who GET /admins doesn't list:
someone removed from the Intercom workspace.
*/
type Former struct {
	ID, Name, Email string
}

/*
convTranslation is a conversation's translation in progress.
*/
type convTranslation struct {
	known    map[string]bool
	contacts map[string]bool
	former   map[string]Former
	files    []*format.File
	seen     map[string]bool
}

func (t *convTranslation) addFiles(list []attachment) []string {
	fs, ids := files(list)
	for _, f := range fs {
		if !t.seen[f.SourceID] {
			t.seen[f.SourceID] = true
			t.files = append(t.files, f)
		}
	}

	return ids
}

/*
noteFormer records an admin author missing from GET /admins.
*/
func (t *convTranslation) noteFormer(a author) {
	if a.Type != "admin" || a.ID == "" || a.FromAIAgent || t.known == nil || t.known[string(a.ID)] {
		return
	}

	if _, ok := t.former[string(a.ID)]; !ok {
		t.former[string(a.ID)] = Former{ID: string(a.ID), Name: text(a.Name), Email: text(a.Email)}
	}
}

func refOrNil(id ID) *string {
	s := string(id)
	if s == "0" {
		s = ""
	}

	return &s
}

/*
eventDetails reads the attribute change of a
conversation_attribute_updated_* part.
*/
func eventDetails(raw json.RawMessage) (name string, value any) {
	var d struct {
		Attribute struct {
			Name string `json:"name"`
		} `json:"attribute"`
		Value struct {
			Name  any `json:"name"`
			Value any `json:"value"`
		} `json:"value"`
	}

	if json.Unmarshal(raw, &d) != nil {
		return "", nil
	}

	value = d.Value.Name
	if value == nil {
		value = d.Value.Value
	}

	return d.Attribute.Name, value
}

/*
plain is a part's body as text, for timeline events.
*/
func plain(body string) string { return strings.TrimSpace(dom.ToText(body)) }

/*
entry turns one part. Message, reply and note parts become entries with
their bodies; state changes become events; anything else is kept as a
custom event with its own type, never dropped.
*/
func (t *convTranslation) entry(p part) []format.Entry {
	t.noteFormer(p.Author)

	e := format.Entry{
		SourceID: string(p.ID), CreatedAt: at(p.CreatedAt), Author: authorOf(p.Author, t.contacts), Redacted: p.Redacted,
	}

	if p.EmailMetadata != nil && p.EmailMetadata.MessageID != "" {
		e.Email = &format.EmailRefs{MessageID: p.EmailMetadata.MessageID}
	}

	event := func(ev format.Event) []format.Entry {
		e.Type = format.EntryEvent
		if e.Author.Type == format.AuthorAIAgent && !p.Author.FromAIAgent {
			e.Author = format.Author{Type: format.AuthorSystem, Label: e.Author.Label}
		}

		ev.RawType = p.PartType
		e.Event = &ev

		return []format.Entry{e}
	}

	switch p.PartType {
	case "comment", "quick_reply", "email", "conversation_message":
		e.Type = format.EntryReply
		if e.Author.Type == format.AuthorContact {
			e.Type = format.EntryMessage
		}

		e.Body = toMarkdown(p.Body)
		e.Attachments = t.addFiles(p.Attachments)

		if e.Body == "" && len(e.Attachments) == 0 && !e.Redacted {
			return event(format.Event{Type: format.EventCustom, Text: plain(p.Body)})
		}

		return []format.Entry{e}
	case "note", "note_and_reopen":
		e.Type = format.EntryNote
		e.Body = toMarkdown(p.Body)
		e.Attachments = t.addFiles(p.Attachments)
		out := []format.Entry{e}

		if p.PartType == "note_and_reopen" {
			out = append(out, reopen(p, e.Author))
		}

		return out
	case "assignment", "assign_and_reopen", "default_assignment", "away_mode_assignment", "assign_and_unsnooze":
		out := t.assignment(p, e)
		if p.PartType != "assignment" && p.PartType != "default_assignment" && p.PartType != "away_mode_assignment" {
			out = append(out, reopen(p, e.Author))
		}

		return out
	case "open", "timer_unsnooze", "unsnoozed":
		return event(format.Event{Type: format.EventStatus, Status: "open"})
	case "close", "closed":
		return event(format.Event{Type: format.EventStatus, Status: "closed"})
	case "snoozed", "snooze":
		return event(format.Event{Type: format.EventStatus, Status: "snoozed"})
	case "conversation_attribute_updated_by_admin", "conversation_attribute_updated_by_workflow",
		"conversation_attribute_updated_by_user":
		name, value := eventDetails(p.EventDetails)
		if name != "" {
			return event(format.Event{Type: format.EventAttribute, Attribute: attrKey(name), Value: value})
		}
	case "priority_changed", "priority_updated", "conversation_priority_changed":
		return event(format.Event{Type: format.EventCustom, Text: plain(p.Body)})
	}

	if strings.HasPrefix(p.PartType, "tag") && len(p.Tags) > 0 {
		var ids []string
		for _, tg := range p.Tags {
			ids = append(ids, string(tg.ID))
		}

		return event(format.Event{Type: format.EventTag, AddedTagRefs: ids})
	}

	txt := plain(p.Body)
	if txt == "" && len(p.EventDetails) > 0 && string(p.EventDetails) != "null" {
		txt = eventText(p.EventDetails)
	}

	e.Attachments = t.addFiles(p.Attachments)

	return event(format.Event{Type: format.EventCustom, Text: txt})
}

/*
eventText keeps a readable summary of event details: the names of
workflows, actions and steps, never values the customer typed.
*/
func eventText(raw json.RawMessage) string {
	var d map[string]json.RawMessage
	if json.Unmarshal(raw, &d) != nil {
		return ""
	}

	var parts []string

	for _, key := range []string{"workflow", "action", "event", "attribute"} {
		var named struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Result string `json:"result"`
		}

		if json.Unmarshal(d[key], &named) == nil {
			if s := firstOf(named.Name, named.Type); s != "" {
				parts = append(parts, key+": "+s)
			}

			if named.Result != "" {
				parts = append(parts, "result: "+named.Result)
			}
		}
	}

	return strings.Join(parts, ", ")
}

func reopen(p part, a format.Author) format.Entry {
	return format.Entry{
		SourceID: string(p.ID) + ":open", Type: format.EntryEvent, CreatedAt: at(p.CreatedAt), Author: a,
		Event: &format.Event{Type: format.EventStatus, Status: "open", RawType: p.PartType},
	}
}

/*
assignment turns an assignment into a teammate or team assignment.
Intercom's nobody_admin is unassigned.
*/
func (t *convTranslation) assignment(p part, e format.Entry) []format.Entry {
	e.Type = format.EntryEvent

	if p.AssignedTo == nil {
		e.Event = &format.Event{Type: format.EventCustom, RawType: p.PartType, Text: plain(p.Body)}

		return []format.Entry{e}
	}

	switch p.AssignedTo.Type {
	case "team":
		e.Event = &format.Event{Type: format.EventTeamAssignment, TeamRef: refOrNil(p.AssignedTo.ID), RawType: p.PartType}
	case "nobody_admin":
		e.Event = &format.Event{Type: format.EventAssignment, AssigneeRef: refOrNil(""), RawType: p.PartType}
	default:
		t.noteFormer(author{Type: "admin", ID: p.AssignedTo.ID})
		e.Event = &format.Event{Type: format.EventAssignment, AssigneeRef: refOrNil(p.AssignedTo.ID), RawType: p.PartType}
	}

	if body := toMarkdown(p.Body); body != "" {
		note := e
		note.SourceID = string(p.ID) + ":note"
		note.Type, note.Body, note.Event = format.EntryNote, body, nil
		note.Attachments = t.addFiles(p.Attachments)

		return []format.Entry{e, note}
	}

	return []format.Entry{e}
}

/*
status maps a conversation's state.
*/
func status(state string, open bool) string {
	switch state {
	case "open", "closed", "snoozed":
		return state
	}

	if open {
		return "open"
	}

	return "closed"
}

/*
translateConversation turns a conversation with every part, oldest
first. known lists the teammates GET /admins returned; authors outside
it are returned as former teammates.
*/
func translateConversation(c conversation, parts []part, known map[string]bool) (format.Record, []Former) {
	t := &convTranslation{known: known, contacts: map[string]bool{}, former: map[string]Former{}, seen: map[string]bool{}}

	conv := &format.Conversation{
		Base: base(c.ID, c.UpdatedAt), Channel: channel(c.Source.Type), Subject: text(firstOf(c.Title, c.Source.Subject)),
		CreatedAt: at(c.CreatedAt), Status: status(c.State, false), SnoozedUntil: when(c.SnoozedUntil),
		Priority: priority(c.Priority), Attributes: attrValues(c.CustomAttrs), Entries: []format.Entry{},
		Source: format.ConversationSource{Type: c.Source.Type, URL: c.Source.URL},
	}

	for i, ct := range c.Contacts.Contacts {
		t.contacts[string(ct.ID)] = true

		if i == 0 {
			conv.ContactRef = string(ct.ID)
		} else {
			conv.ParticipantRefs = append(conv.ParticipantRefs, string(ct.ID))
		}
	}

	if conv.ContactRef == "" && isContact(c.Source.Author.Type) {
		conv.ContactRef = string(c.Source.Author.ID)
	}

	if c.Company != nil {
		conv.CompanyRef = string(c.Company.ID)
	}

	if c.AdminAssigneeID != "" && c.AdminAssigneeID != "0" {
		conv.AssigneeRef = string(c.AdminAssigneeID)
		t.noteFormer(author{Type: "admin", ID: c.AdminAssigneeID})
	}

	if c.TeamAssigneeID != "" && c.TeamAssigneeID != "0" {
		conv.TeamRef = string(c.TeamAssigneeID)
	}

	for _, tg := range c.Tags.Tags {
		conv.TagRefs = append(conv.TagRefs, string(tg.ID))
	}

	first := format.Entry{
		SourceID: "source_" + string(firstID(c.Source.ID, c.ID)), CreatedAt: at(c.CreatedAt),
		Author: authorOf(c.Source.Author, t.contacts), Body: toMarkdown(c.Source.Body),
		Attachments: t.addFiles(c.Source.Attachments), Redacted: c.Source.Redacted,
	}
	t.noteFormer(c.Source.Author)

	if c.Source.EmailMetadata != nil && c.Source.EmailMetadata.MessageID != "" {
		first.Email = &format.EmailRefs{MessageID: c.Source.EmailMetadata.MessageID}
	}

	first.Type = format.EntryMessage
	if first.Author.Type != format.AuthorContact {
		first.Type = format.EntryReply
	}

	if first.Body != "" || len(first.Attachments) > 0 || first.Redacted {
		conv.Entries = append(conv.Entries, first)
	}

	for _, p := range parts {
		conv.Entries = append(conv.Entries, t.entry(p)...)
	}

	if c.Rating != nil && c.Rating.Rating > 0 {
		conv.Rating = &format.Rating{Score: c.Rating.Rating, Remark: text(c.Rating.Remark), CreatedAt: at(c.Rating.CreatedAt)}
		if conv.Rating.CreatedAt.IsZero() {
			conv.Rating.CreatedAt = at(c.UpdatedAt)
		}

		if c.Rating.Teammate != nil {
			conv.Rating.TeammateRef = string(c.Rating.Teammate.ID)
		}
	}

	var former []Former
	for _, id := range sortedKeys(t.former) {
		former = append(former, t.former[id])
	}

	return format.NewRecord(format.KindConversation, conv.SourceID, conv, t.files...), former
}

func priority(p string) string {
	switch p {
	case "", "none", "not_priority":
		return ""
	case "priority":
		return "high"
	}

	return p
}

/*
translateTicket turns a ticket with its parts. Intercom returns at most
500; a ticket with more is marked capped.
*/
func translateTicket(tk ticket, known map[string]bool) (format.Record, []Former) {
	t := &convTranslation{known: known, contacts: map[string]bool{}, former: map[string]Former{}, seen: map[string]bool{}}

	title, description := "", ""
	values := map[string]any{}

	for k, v := range tk.Attributes {
		switch k {
		case defaultTitle:
			title, _ = v.(string)
		case defaultDescription:
			description, _ = v.(string)
		default:
			values[attrKey(k)] = v
		}
	}

	if len(values) == 0 {
		values = nil
	}

	rec := &format.Ticket{
		Base: base(tk.ID, tk.UpdatedAt), TicketTypeRef: string(tk.Type.ID), StateRef: string(tk.State.ID),
		Title: firstOf(text(title), text(tk.Type.Name)+" "+tk.TicketID), Description: toMarkdown(description), Values: values,
		CreatedAt: at(tk.CreatedAt), UpdatedAt: when(tk.UpdatedAt), CompanyRef: string(tk.CompanyID),
	}

	for i, ct := range tk.Contacts.Contacts {
		t.contacts[string(ct.ID)] = true

		if i == 0 {
			rec.ContactRef = string(ct.ID)
		}
	}

	if tk.AdminAssigneeID != "" && tk.AdminAssigneeID != "0" {
		rec.AssigneeRef = string(tk.AdminAssigneeID)
		t.noteFormer(author{Type: "admin", ID: tk.AdminAssigneeID})
	}

	if tk.TeamAssigneeID != "" && tk.TeamAssigneeID != "0" {
		rec.TeamRef = string(tk.TeamAssigneeID)
	}

	for _, l := range tk.Linked.Data {
		if l.Type == "conversation" && rec.ConversationRef == "" {
			rec.ConversationRef = string(l.ID)
		}
	}

	for _, tg := range tk.Tags.Tags {
		rec.TagRefs = append(rec.TagRefs, string(tg.ID))
	}

	states := map[string]string{}
	for _, s := range tk.Type.States.Data {
		states[s.Category] = string(s.ID)
	}

	for _, p := range tk.Parts.Parts {
		if p.PartType == "ticket_state_updated_by_admin" || p.TicketState != "" && p.PreviousState != p.TicketState {
			if ref := states[p.TicketState]; ref != "" {
				rec.StateHistory = append(rec.StateHistory, format.StateMove{StateRef: ref, At: at(p.CreatedAt)})
			}
		}

		rec.Entries = append(rec.Entries, t.entry(p)...)
	}

	if len(rec.StateHistory) == 0 && rec.StateRef != "" {
		rec.StateHistory = []format.StateMove{{StateRef: rec.StateRef, At: at(tk.CreatedAt)}}
	}

	rec.EntriesCapped = tk.Parts.TotalCount > len(tk.Parts.Parts) || len(tk.Parts.Parts) >= 500

	var former []Former
	for _, id := range sortedKeys(t.former) {
		former = append(former, t.former[id])
	}

	return format.NewRecord(format.KindTicket, rec.SourceID, rec, t.files...), former
}

/*
translateFormer turns a teammate removed from Intercom, known only from
what their entries say about them.
*/
func translateFormer(f Former) format.Record {
	current := false

	return format.NewRecord(format.KindTeammate, f.ID, &format.Teammate{
		Base: format.Base{SourceID: f.ID}, Email: f.Email, Name: f.Name, Current: &current,
	})
}

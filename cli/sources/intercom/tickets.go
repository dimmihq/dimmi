package intercom

import "github.com/dimmihq/dimmi/cli/format"

/*
translateTicket turns a ticket with its parts. Intercom returns at most
500; a ticket with more is marked capped.
*/
func translateTicket(tk ticket, known map[string]bool) (format.Record, []Former) {
	t := newTranslation(known)

	title, description, values := ticketValues(tk.Attributes)

	rec := &format.Ticket{
		Base: base(tk.ID, tk.UpdatedAt), TicketTypeRef: string(tk.Type.ID), StateRef: string(tk.State.ID),
		Title:       firstOf(text(title), text(tk.Type.Name)+" "+tk.TicketID),
		Description: toMarkdown(description), Values: values,
		CreatedAt: at(tk.CreatedAt), UpdatedAt: when(tk.UpdatedAt), CompanyRef: string(tk.CompanyID),
	}

	t.ticketParties(tk, rec)

	states := map[string]string{}
	for _, s := range tk.Type.States.Data {
		states[s.Category] = string(s.ID)
	}

	for _, p := range tk.Parts.Parts {
		if move, ok := stateMove(p, states); ok {
			rec.StateHistory = append(rec.StateHistory, move)
		}

		rec.Entries = append(rec.Entries, t.entry(p)...)
	}

	if len(rec.StateHistory) == 0 && rec.StateRef != "" {
		rec.StateHistory = []format.StateMove{{StateRef: rec.StateRef, At: at(tk.CreatedAt)}}
	}

	rec.EntriesCapped = tk.Parts.TotalCount > len(tk.Parts.Parts) || len(tk.Parts.Parts) >= 500

	return format.NewRecord(format.KindTicket, rec.SourceID, rec, t.files...), t.formerList()
}

/*
ticketValues splits a ticket's attributes into its title, its
description and the values of its other fields.
*/
func ticketValues(attrs map[string]any) (title, description string, values map[string]any) {
	values = map[string]any{}

	for k, v := range attrs {
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

	return title, description, values
}

/*
ticketParties sets the ticket's contact, assignees, linked conversation
and tags.
*/
func (t *convTranslation) ticketParties(tk ticket, rec *format.Ticket) {
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
}

/*
stateMove reads the state a part moved the ticket to, given the ticket
type's state ids by category.
*/
func stateMove(p part, states map[string]string) (format.StateMove, bool) {
	if p.PartType != "ticket_state_updated_by_admin" && (p.TicketState == "" || p.PreviousState == p.TicketState) {
		return format.StateMove{}, false
	}

	ref := states[p.TicketState]
	if ref == "" {
		return format.StateMove{}, false
	}

	return format.StateMove{StateRef: ref, At: at(p.CreatedAt)}, true
}

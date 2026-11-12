/*
|--------------------------------------------------------------------------
| Intercom Tickets
|--------------------------------------------------------------------------
|
| Tickets are searched page by page, oldest first, then each is read
| whole for its parts: every part that moved the state is a step of its
| history. Parts name the state's category, so a step becomes the state
| of that category the ticket's type uses, the ticket's own state when
| the category is the one it is in now. Requesters and teammates are
| read once each, for their email.
|
*/

package ticketimports

import (
	"context"
	"fmt"
	"strings"
	"time"
)

/*
searchPage is how many tickets a search answers at a time.
*/
const searchPage = 50

/*
intercomTicket is a ticket as Intercom shows it.
*/
type intercomTicket struct {
	ID         string         `json:"id"`
	Attributes map[string]any `json:"ticket_attributes"`
	State      intercomState  `json:"ticket_state"`
	Type       struct {
		ID string `json:"id"`
	} `json:"ticket_type"`
	Contacts struct {
		Contacts []struct {
			ID string `json:"id"`
		} `json:"contacts"`
	} `json:"contacts"`
	Assignee  any   `json:"admin_assignee_id"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
	Linked    struct {
		Data []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"data"`
	} `json:"linked_objects"`
	Parts struct {
		Parts []intercomPart `json:"ticket_parts"`
	} `json:"ticket_parts"`
}

/*
intercomPart is one part of a ticket's history.
*/
type intercomPart struct {
	PartType  string `json:"part_type"`
	State     string `json:"ticket_state"`
	Previous  string `json:"previous_ticket_state"`
	CreatedAt int64  `json:"created_at"`
}

/*
Tickets reads every ticket, oldest first.
*/
func (s *Intercom) Tickets(ctx context.Context, each func(Ticket) error) error {
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return err
	}

	if err := s.loadAdmins(ctx); err != nil {
		return err
	}

	cursor := ""

	for {
		page, next, err := s.search(ctx, cursor)
		if err != nil {
			return err
		}

		for _, found := range page {
			t, err := s.ticket(ctx, catalog, found.ID)
			if err != nil {
				return err
			}

			if err := each(t); err != nil {
				return err
			}
		}

		if next == "" {
			return nil
		}

		cursor = next
	}
}

/*
search reads one page of tickets after the cursor.
*/
func (s *Intercom) search(ctx context.Context, cursor string) ([]intercomTicket, string, error) {
	pagination := map[string]any{"per_page": searchPage}
	if cursor != "" {
		pagination["starting_after"] = cursor
	}

	body := map[string]any{
		"query":      map[string]any{"field": "created_at", "operator": ">", "value": "0"},
		"pagination": pagination,
	}

	var out struct {
		Tickets []intercomTicket `json:"tickets"`
		Pages   struct {
			Next *struct {
				StartingAfter string `json:"starting_after"`
			} `json:"next"`
		} `json:"pages"`
	}
	if err := s.reader().post(ctx, s.url("/tickets/search"), body, &out); err != nil {
		return nil, "", err
	}

	next := ""
	if out.Pages.Next != nil {
		next = out.Pages.Next.StartingAfter
	}

	return out.Tickets, next, nil
}

/*
ticket reads one ticket whole and shapes it.
*/
func (s *Intercom) ticket(ctx context.Context, catalog Catalog, id string) (Ticket, error) {
	var it intercomTicket
	if err := s.reader().get(ctx, s.url("/tickets/"+id), &it); err != nil {
		return Ticket{}, err
	}

	t := Ticket{
		ExternalID: "intercom:" + it.ID, TypeID: it.Type.ID, StateID: it.State.ID,
		Title: text(it.Attributes["_default_title_"]), Description: text(it.Attributes["_default_description_"]),
		CreatedAt: unix(it.CreatedAt), UpdatedAt: unix(it.UpdatedAt), Values: map[string]any{},
		AssigneeEmail: s.admins[text(it.Assignee)],
	}

	for name, v := range it.Attributes {
		if !strings.HasPrefix(name, "_default_") && v != nil {
			t.Values[KeyOf(name)] = v
		}
	}

	if len(it.Contacts.Contacts) > 0 {
		p, err := s.person(ctx, it.Contacts.Contacts[0].ID)
		if err != nil {
			return t, err
		}

		t.RequesterEmail, t.RequesterName = p.email, p.name
	}

	s.links(&t, it)
	t.History = intercomHistory(catalog, it)

	return t, nil
}

/*
links reads the tracker the ticket reports and the conversation it
came from.
*/
func (s *Intercom) links(t *Ticket, it intercomTicket) {
	for _, l := range it.Linked.Data {
		switch {
		case l.Type == "ticket" && t.TrackerID == "":
			t.TrackerID = "intercom:" + l.ID
		case l.Type == "conversation" && t.Conversation == "":
			t.Conversation = l.ID
		}
	}
}

/*
intercomHistory turns the parts that moved the state into steps.
*/
func intercomHistory(catalog Catalog, it intercomTicket) []Move {
	var out []Move

	for _, p := range it.Parts.Parts {
		if p.State == "" || p.State == p.Previous {
			continue
		}

		if len(out) == 0 && p.Previous != "" {
			out = append(out, Move{At: unix(it.CreatedAt), StateID: stateOf(catalog, it, p.Previous)})
		}

		out = append(out, Move{At: unix(p.CreatedAt), StateID: stateOf(catalog, it, p.State)})
	}

	return out
}

/*
stateOf returns the state of the category the ticket's type uses.
*/
func stateOf(catalog Catalog, it intercomTicket, category string) string {
	group := intercomGroup(category)
	if intercomGroup(it.State.Category) == group {
		return it.State.ID
	}

	allowed := map[string]bool{}

	for _, ty := range catalog.Types {
		if ty.ExternalID == it.Type.ID {
			for _, id := range ty.States {
				allowed[id] = true
			}
		}
	}

	for _, st := range catalog.States {
		if st.Group == group && (len(allowed) == 0 || allowed[st.ExternalID]) {
			return st.ExternalID
		}
	}

	return it.State.ID
}

/*
person reads a contact's email and name once.
*/
func (s *Intercom) person(ctx context.Context, id string) (person, error) {
	if p, ok := s.people[id]; ok {
		return p, nil
	}

	var c struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := s.reader().get(ctx, s.url("/contacts/"+id), &c); err != nil {
		return person{}, err
	}

	if s.people == nil {
		s.people = map[string]person{}
	}

	s.people[id] = person{email: c.Email, name: c.Name}

	return s.people[id], nil
}

/*
loadAdmins reads the teammates' emails by id.
*/
func (s *Intercom) loadAdmins(ctx context.Context) error {
	var out struct {
		Admins []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"admins"`
	}
	if err := s.reader().get(ctx, s.url("/admins"), &out); err != nil {
		return err
	}

	s.admins = map[string]string{}
	for _, a := range out.Admins {
		s.admins[a.ID] = a.Email
	}

	return nil
}

/*
text reads a value as text: a string, or a number written out.
*/
func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", x), "0"), ".")
	}

	return fmt.Sprint(v)
}

/*
unix reads seconds since the epoch, zero for none.
*/
func unix(s int64) time.Time {
	if s == 0 {
		return time.Time{}
	}

	return time.Unix(s, 0).UTC()
}

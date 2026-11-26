/*
|--------------------------------------------------------------------------
| Request Pages
|--------------------------------------------------------------------------
|
| The signed-in reader's requests: their list, the form to open one and
| a request's own page with its thread.
|
*/

package helpcenter

import "github.com/dimmihq/dimmi/framework/view"

/*
TicketVars is a reader's request, in a list or on its page.
*/
type TicketVars struct {
	// Its reference, such as T-12.
	Ref string
	// Its subject.
	Title string
	// Its type's name.
	Type string
	// Its type's icon.
	TypeIcon string
	// Where it stands, as customers read it.
	Label string
	// Submitted, in_progress, waiting_customer or resolved.
	Group string
	// When it last changed, such as 2h.
	Updated string
	// The reader opened it, rather than someone at their company.
	Mine bool
	// Its page.
	URL string
	// On its page: its details.
	Description string
	// On its page: Submitted, We're on it and Resolved, with done and current.
	Steps []RequestStep
	// On its page: the fields the reader sees, with label and value.
	Fields []RequestField
	// On its page: the thread.
	Messages []MessageVars
	// On its page: why a reply was refused.
	Error string
	// On its page: where the reply form posts, with body.
	ReplyURL string
}

/*
MessageVars is a message in a thread.
*/
type MessageVars struct {
	// Who sent it.
	Author PersonVars
	// The reader sent it.
	Mine bool
	// Its text.
	Body string
	// When.
	At string
}

/*
TicketTypeVars is a kind of request a reader may open.
*/
type TicketTypeVars struct {
	// Its public id.
	ID string
	// Its name.
	Name string
	// Its icon.
	Icon string
	// What it is for.
	Description string
	// Its form.
	URL string
}

/*
TicketsVars is tickets, on the reader's list of requests.
*/
type TicketsVars struct {
	// Company when the company's are shown, else empty.
	Scope string
	// The company tab shows.
	Company bool
	// My requests and Company requests.
	Tabs []LinkVars
	// The requests.
	Items []TicketVars
	// The types the reader may open.
	Types []TicketTypeVars
	// The form to open one.
	NewURL string
}

/*
TicketFormVars is ticket_form, on the page to open a request.
*/
type TicketFormVars struct {
	// The type picked; id is empty before one is.
	Type TicketTypeVars
	// Every type the reader may open.
	Types []TicketTypeVars
	// The type's fields: key, label, description, data_type, options, required, show_field, show_equals, value.
	Fields []RequestField
	// The subject typed.
	Title string
	// The details typed.
	Description string
	// Why it was refused.
	Error string
	// A key that stops a double post.
	Key string
	// Where the form posts.
	Action string
}

func (c Center) ticket(r Request) TicketVars {
	return TicketVars{
		Ref: r.Ref, Title: r.Title, Type: r.Type, TypeIcon: r.TypeIcon, Label: r.Label, Group: r.Group,
		Updated: r.Updated, Mine: r.Mine, URL: c.url("/tickets/" + r.Ref),
	}
}

func (c Center) ticketTypes(list []RequestType) []TicketTypeVars {
	out := make([]TicketTypeVars, 0, len(list))
	for _, t := range list {
		out = append(out, c.ticketType(t))
	}
	return out
}

func (c Center) ticketType(t RequestType) TicketTypeVars {
	return TicketTypeVars{
		ID: t.ID, Name: t.Name, Icon: t.Icon, Description: t.Description, URL: c.url("/tickets/new/" + t.ID),
	}
}

func messages(list []Message) []MessageVars {
	out := make([]MessageVars, 0, len(list))
	for _, m := range list {
		out = append(out, MessageVars{Author: personVars(m.Author), Mine: m.Mine, Body: m.Body, At: m.At})
	}
	return out
}

/*
RequestsPage lists the signed-in reader's requests.
*/
func RequestsPage(c Center, list Requests) view.Component {
	m := Meta{
		Title: "Your requests | " + c.Product, Section: "requests",
		Trail: []Crumb{{Title: "Requests", Path: "/tickets"}}, NoIndex: true,
	}
	v := TicketsVars{
		Scope: list.Scope, Company: list.Company, Types: c.ticketTypes(list.Types), NewURL: c.url("/tickets/new"),
		Tabs: []LinkVars{
			{Name: "My requests", URL: c.url("/tickets"), Current: list.Scope != "company"},
			{Name: "Company requests", URL: c.url("/tickets?scope=company"), Current: list.Scope == "company"},
		},
	}
	for _, r := range list.Items {
		v.Items = append(v.Items, c.ticket(r))
	}
	return Render(c, "tickets", m, map[string]any{"tickets": v})
}

/*
NewRequestPage asks which kind of request the reader has, then shows
that type's form.
*/
func NewRequestPage(c Center, form RequestForm) view.Component {
	m := Meta{
		Title: "New request | " + c.Product, Section: "requests",
		Trail: []Crumb{
			{Title: "Requests", Path: "/tickets"}, {Title: "New request", Path: "/tickets/new"},
		}, NoIndex: true,
	}
	v := TicketFormVars{
		Type: c.ticketType(form.Type), Types: c.ticketTypes(form.Types), Fields: form.Fields, Title: form.Title,
		Description: form.Description, Error: form.Error, Key: form.Key, Action: c.url("/tickets/new/" + form.Type.ID),
	}
	return Render(c, "ticket_new", m, map[string]any{"ticket_form": v})
}

/*
RequestShowPage shows one request with its thread.
*/
func RequestShowPage(c Center, p RequestPage) view.Component {
	m := Meta{
		Title: p.Title + " | " + c.Product, Section: "requests",
		Trail: []Crumb{{Title: "Requests", Path: "/tickets"}, {Title: p.Ref, Path: "/tickets/" + p.Ref}}, NoIndex: true,
	}
	v := c.ticket(p.Request)
	v.Description, v.Steps, v.Fields = p.Description, p.Steps, p.Fields
	v.Messages, v.Error = messages(p.Messages), p.Error
	v.ReplyURL = c.url("/tickets/" + p.Ref + "/messages")
	return Render(c, "ticket", m, map[string]any{"ticket": v})
}

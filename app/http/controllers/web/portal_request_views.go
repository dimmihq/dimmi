/*
|--------------------------------------------------------------------------
| Portal Request Views
|--------------------------------------------------------------------------
|
| Shapes requests for the portal's pages: types by public id, the form
| fields a reader can fill on a page (files go through the messenger),
| list rows and a request's page with its strip and thread.
|
*/

package web

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
formType reports whether the portal's form can fill the data type:
everything but files, teammates and companies.
*/
func formType(dataType string) bool {
	switch dataType {
	case tickets.TypeFile, tickets.TypeMember, tickets.TypeCompany:
		return false
	}

	return true
}

func requestType(t models.TicketType) helpcenter.RequestType {
	return helpcenter.RequestType{
		ID: models.PublicID(models.PrefixTicketType, t.ID), Name: t.Name, Icon: t.Icon, Description: t.Description,
	}
}

func requestTypes(types []tickets.CustomerType) []helpcenter.RequestType {
	out := make([]helpcenter.RequestType, len(types))
	for i, t := range types {
		out[i] = requestType(t.Type)
	}

	return out
}

/*
pickType finds the type a public id names among those a reader opens.
*/
func pickType(types []tickets.CustomerType, public string) (tickets.CustomerType, bool) {
	for _, t := range types {
		if models.PublicID(models.PrefixTicketType, t.Type.ID) == public {
			return t, true
		}
	}

	return tickets.CustomerType{}, false
}

/*
formFields lists the fields of a type's form with what the reader sent.
*/
func formFields(fields []models.TicketField, values map[string]any) []helpcenter.RequestField {
	out := []helpcenter.RequestField{}

	for _, f := range fields {
		if !f.EditableCustomer || !formType(f.DataType) {
			continue
		}

		field := helpcenter.RequestField{
			Key: f.Key, Label: f.Label, Description: f.Description, DataType: f.DataType, Options: f.Options,
			Required: f.RequiredCreateCustomer, Value: shown(values[f.Key]),
		}

		var sw tickets.ShowWhen
		if len(f.ShowWhen) > 0 && json.Unmarshal(f.ShowWhen, &sw) == nil {
			field.ShowField, field.ShowEquals = sw.Field, strings.Join(sw.Equals, "\n")
		}

		out = append(out, field)
	}

	return out
}

/*
shown writes a field's value as a page shows it.
*/
func shown(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "Yes"
		}

		return "No"
	case []any:
		parts := make([]string, len(t))
		for i, p := range t {
			parts[i] = shown(p)
		}

		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(t)
	}
}

/*
requestRow shapes a request for the reader's list.
*/
func requestRow(t tickets.CustomerTicket, reader ids.UUID, now time.Time) helpcenter.Request {
	return helpcenter.Request{
		Ref: tickets.Reference(t.Ticket.Number), Title: t.Ticket.Title, Type: t.Type.Name, TypeIcon: t.Type.Icon,
		Label: t.State.CustomerLabel, Group: t.State.StateGroup, Updated: ago(t.Ticket.UpdatedAt, now),
		Mine: t.Ticket.ContactID != nil && *t.Ticket.ContactID == reader,
	}
}

/*
requestPage shapes a request's page: its strip, the fields it has a
value for and its thread.
*/
func requestPage(t tickets.PortalThread, req requester, now time.Time) helpcenter.RequestPage {
	values := map[string]any{}
	_ = json.Unmarshal(t.Ticket.Fields, &values)

	out := helpcenter.RequestPage{
		Request: requestRow(t.CustomerTicket, req.who.ContactID, now), Description: t.Ticket.Description,
		Steps: helpcenter.Progress(t.State.StateGroup), Types: requestTypes(req.types),
	}

	for _, f := range t.Fields {
		if v := shown(values[f.Key]); v != "" && formType(f.DataType) {
			out.Fields = append(out.Fields, helpcenter.RequestField{Key: f.Key, Label: f.Label, Value: v})
		}
	}

	team := req.site.center.Product
	for _, m := range t.Messages {
		out.Messages = append(out.Messages, threadMessage(m, req.who.ContactID, team, now))
	}

	return out
}

/*
threadMessage shapes a message of the thread: the reader's own, or the
team's under the product's name.
*/
func threadMessage(m models.Message, reader ids.UUID, team string, now time.Time) helpcenter.Message {
	body := ""
	if m.Body != nil {
		body = *m.Body
	}

	mine := m.AuthorType == string(enums.AuthorTypeContact) && m.AuthorID != nil && *m.AuthorID == reader
	who := helpcenter.Person{Name: "You", Initials: "Y"}

	if !mine {
		who = helpcenter.Person{Name: team, Initials: initial(team)}
	}

	return helpcenter.Message{Author: who, Mine: mine, Body: body, At: ago(m.CreatedAt, now)}
}

func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}

	return "?"
}

/*
ago writes how long ago something happened, the date past a week.
*/
func ago(t, now time.Time) string {
	d := now.Sub(t)

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return helpcenter.ShortDay(t)
	}
}

/*
|--------------------------------------------------------------------------
| Portal Requests
|--------------------------------------------------------------------------
|
| A real help center's requests, for a signed-in reader: their list, and
| their company's when a type shares it, the form of each type, and a
| request's page with its thread and a reply box. Anyone else is sent to
| sign in and brought back. The help center's old conversation pages
| lead here. Every form answers with a redirect once it succeeded, so a
| reload never sends it twice, and a client key the form carries makes
| a double submit one request.
|
*/

package web

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
requester is a signed-in reader as a customer, on the real help center.
*/
type requester struct {
	site  liveSite
	who   tickets.Customer
	types []tickets.CustomerType
}

/*
requester finds the signed-in reader of a real help center, sending
anyone else to sign in and the sample portal to its conversations.
*/
func (c PortalController) requester(w fhttp.ResponseWriter, r *fhttp.Request, path string) (requester, bool) {
	s, live, ok := c.live(w, r)
	if !ok {
		return requester{}, false
	}

	if !live || c.Requests == nil {
		center, found := c.center(w, r)
		if found {
			back(w, center.Base+"/conversations")
		}

		return requester{}, false
	}

	if s.reader == nil {
		c.signInFirst(w, s, path)

		return requester{}, false
	}

	who, err := c.Requests.CustomerFor(r.Context(), s.help.WorkspaceID, *s.reader, true)
	if err != nil {
		c.Errors.Render(w, r, err)

		return requester{}, false
	}

	types, err := c.Requests.CustomerTypes(r.Context(), s.help.WorkspaceID)
	if err != nil {
		c.Errors.Render(w, r, err)

		return requester{}, false
	}

	w.Header().Set("Cache-Control", "private, no-store")

	return requester{site: s, who: who, types: types}, true
}

/*
Tickets lists the reader's requests, or their company's.
*/
func (c PortalController) Tickets(w fhttp.ResponseWriter, r *fhttp.Request) {
	req, ok := c.requester(w, r, "/tickets")
	if !ok {
		return
	}

	scope := tickets.ScopeMine
	if r.URL.Query().Get("scope") == tickets.ScopeCompany {
		scope = tickets.ScopeCompany
	}

	list, err := c.Requests.CustomerTickets(r.Context(), req.who, scope, false)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	out := helpcenter.Requests{
		Scope: scope, Company: tickets.CustomerShares(req.types) && req.who.CompanyID != nil,
		Types: requestTypes(req.types),
	}

	for _, t := range list {
		if scope == tickets.ScopeCompany && !t.Type.CompanySharing {
			continue
		}

		out.Items = append(out.Items, requestRow(t, req.who.ContactID, c.Requests.Clock.Now()))
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.RequestsPage(req.site.center, out, c.styles()))
}

/*
NewTicket shows the types a reader may open, and the form of the one
picked.
*/
func (c PortalController) NewTicket(w fhttp.ResponseWriter, r *fhttp.Request) {
	path := "/tickets/new"
	if t := r.PathValue("type"); t != "" {
		path += "/" + t
	}

	req, ok := c.requester(w, r, path)
	if !ok {
		return
	}

	form := helpcenter.RequestForm{Types: requestTypes(req.types), Key: ids.New().String()}

	if public := r.PathValue("type"); public != "" {
		ty, found := pickType(req.types, public)
		if !found {
			c.NotFound(w, r)

			return
		}

		form.Type, form.Fields = requestType(ty.Type), formFields(ty.Fields, nil)
	} else if len(req.types) == 1 {
		back(w, req.site.center.Base+"/tickets/new/"+models.PublicID(models.PrefixTicketType, req.types[0].Type.ID))

		return
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.NewRequestPage(req.site.center, form, c.styles()))
}

/*
SubmitTicket opens the reader's request and shows it, or the form again
with why it was refused.
*/
func (c PortalController) SubmitTicket(w fhttp.ResponseWriter, r *fhttp.Request) {
	public := r.PathValue("type")

	req, ok := c.requester(w, r, "/tickets/new/"+public)
	if !ok {
		return
	}

	ty, found := pickType(req.types, public)
	if !found {
		c.NotFound(w, r)

		return
	}

	if err := r.ParseForm(); err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	values := formValues(ty.Fields, r.PostForm)
	key := "portal:" + models.PublicID(models.PrefixContact, req.who.ContactID) + ":" + r.PostFormValue("key")

	made, err := c.Requests.PortalCreate(r.Context(), req.who, tickets.PortalInput{
		CustomerInput: tickets.CustomerInput{
			TypeID: ty.Type.ID, Title: r.PostFormValue("title"), Description: r.PostFormValue("description"),
			Fields: values, ExternalID: &key,
		},
		Messenger: helpcenters.SettingsOf(req.site.help).MessengerID,
	})

	if why, refused := refusal(err, ty.Fields); refused {
		form := helpcenter.RequestForm{
			Types: requestTypes(req.types), Type: requestType(ty.Type), Fields: formFields(ty.Fields, values),
			Title: r.PostFormValue("title"), Description: r.PostFormValue("description"), Error: why,
			Key: r.PostFormValue("key"),
		}
		c.respond(w, r, fhttp.StatusUnprocessableEntity, helpcenter.NewRequestPage(req.site.center, form, c.styles()))

		return
	}

	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	back(w, req.site.center.Base+"/tickets/"+tickets.Reference(made.Ticket.Number))
}

/*
Ticket shows one of the reader's requests with its thread.
*/
func (c PortalController) Ticket(w fhttp.ResponseWriter, r *fhttp.Request) {
	number, valid := tickets.ParseNumber(r.PathValue("ticket"))

	req, ok := c.requester(w, r, "/tickets/"+r.PathValue("ticket"))
	if !ok {
		return
	}

	if !valid {
		c.NotFound(w, r)

		return
	}

	shown, err := c.Requests.PortalShow(r.Context(), req.who, tickets.Ref{Number: number})
	if errors.Is(err, tickets.ErrTicketNotFound) {
		c.NotFound(w, r)

		return
	}

	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	page := requestPage(shown, req, c.Requests.Clock.Now())
	if r.URL.Query().Has("empty") {
		page.Error = "Write something before sending."
	}

	c.respond(w, r, fhttp.StatusOK, helpcenter.RequestShowPage(req.site.center, page, c.styles()))
}

/*
ReplyTicket writes the reader's reply and returns to the request.
*/
func (c PortalController) ReplyTicket(w fhttp.ResponseWriter, r *fhttp.Request) {
	number, valid := tickets.ParseNumber(r.PathValue("ticket"))
	ref := tickets.Reference(number)

	req, ok := c.requester(w, r, "/tickets/"+ref)
	if !ok {
		return
	}

	if !valid {
		c.NotFound(w, r)

		return
	}

	err := c.Requests.PortalReply(r.Context(), req.who, tickets.Ref{Number: number}, r.FormValue("body"))

	switch {
	case errors.Is(err, tickets.ErrTicketNotFound):
		c.NotFound(w, r)
	case errors.Is(err, tickets.ErrReplyInvalid):
		back(w, req.site.center.Base+"/tickets/"+ref+"?empty")
	case err != nil:
		c.Errors.Render(w, r, err)
	default:
		back(w, req.site.center.Base+"/tickets/"+ref)
	}
}

/*
refusal says why a form was refused in the reader's words, and false
when it was not refused for something they can fix.
*/
func refusal(err error, fields []models.TicketField) (string, bool) {
	var f *fault.Error
	if err == nil || !errors.As(err, &f) || f.Kind() != fault.Invalid {
		return "", false
	}

	if errors.Is(err, tickets.ErrMissingFields) {
		missing, _ := f.Meta()["missing"].([]string)
		var labels strings.Builder

		for i, key := range missing {
			if i > 0 {
				labels.WriteString(", ")
			}

			labels.WriteString(labelOf(fields, key))
		}

		return "Please fill in " + labels.String() + ".", true
	}

	return f.Message(), true
}

/*
labelOf names a field by its label.
*/
func labelOf(fields []models.TicketField, key string) string {
	for _, f := range fields {
		if f.Key == key {
			return f.Label
		}
	}

	return key
}

/*
formValues reads the values the form sent for the fields the reader
may fill, typed as each field holds them; empty ones are left out.
*/
func formValues(fields []models.TicketField, form url.Values) map[string]any {
	out := map[string]any{}

	for _, f := range fields {
		if !f.EditableCustomer {
			continue
		}

		sent := form["field."+f.Key]
		if len(sent) == 0 || sent[0] == "" {
			continue
		}

		switch f.DataType {
		case tickets.TypeMultipleChoice:
			list := make([]any, len(sent))
			for i, s := range sent {
				list[i] = s
			}

			out[f.Key] = list
		case tickets.TypeNumber, tickets.TypeDecimal, tickets.TypeCurrency:
			if n, err := strconv.ParseFloat(sent[0], 64); err == nil {
				out[f.Key] = n
			} else {
				out[f.Key] = sent[0]
			}
		case tickets.TypeBoolean:
			out[f.Key] = sent[0] == "true"
		default:
			out[f.Key] = sent[0]
		}
	}

	return out
}

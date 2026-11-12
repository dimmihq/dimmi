/*
|--------------------------------------------------------------------------
| Zendesk
|--------------------------------------------------------------------------
|
| Zendesk's ticket forms become customer types with the custom fields
| each form shows; system fields such as subject, status and priority
| map to the ticket itself. Custom statuses become states, grouped by
| their category: new is submitted, open in progress, pending waits on
| the customer, on-hold waits on another team, solved is resolved. The
| agent label is the name and the end-user label what customers read.
| Problem tickets become trackers of a Problem type every status
| applies to.
|
*/

package ticketimports

import (
	"context"
	"encoding/base64"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
ProblemType is the external id of the tracker type problems become,
and DefaultForm the type of tickets in an account without forms.
*/
const (
	ProblemType = "zendesk:problem"
	DefaultForm = "zendesk:default"
)

/*
Zendesk reads tickets from a Zendesk account with an API token. BaseURL
is https://<subdomain>.zendesk.com.
*/
type Zendesk struct {
	HTTP    *httpclient.Client
	BaseURL string
	Email   string
	Token   string

	statuses []zendeskStatus
	fields   map[int64]zendeskField
	forms    []zendeskForm
}

/*
zendeskStatus is a custom status.
*/
type zendeskStatus struct {
	ID       int64  `json:"id"`
	Category string `json:"status_category"`
	Agent    string `json:"agent_label"`
	EndUser  string `json:"end_user_label"`
	Active   bool   `json:"active"`
	Default  bool   `json:"default"`
}

/*
zendeskField is a ticket field.
*/
type zendeskField struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Active    bool   `json:"active"`
	Removable bool   `json:"removable"`
	Portal    bool   `json:"visible_in_portal"`
	Options   []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"custom_field_options"`
}

/*
zendeskForm is a ticket form and the fields it shows.
*/
type zendeskForm struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Active   bool    `json:"active"`
	Default  bool    `json:"default"`
	FieldIDs []int64 `json:"ticket_field_ids"`
}

/*
Name returns zendesk.
*/
func (s *Zendesk) Name() string { return "zendesk" }

/*
reader returns the authenticated reader.
*/
func (s *Zendesk) reader() reader {
	auth := base64.StdEncoding.EncodeToString([]byte(s.Email + "/token:" + s.Token))

	return reader{http: s.HTTP, headers: map[string]string{"Authorization": "Basic " + auth}}
}

/*
url returns the API address of the path.
*/
func (s *Zendesk) url(path string) string {
	return strings.TrimRight(s.BaseURL, "/") + "/api/v2" + path
}

/*
load reads the statuses, fields and forms once.
*/
func (s *Zendesk) load(ctx context.Context) error {
	if s.fields != nil {
		return nil
	}

	var statuses struct {
		Statuses []zendeskStatus `json:"custom_statuses"`
	}
	if err := s.reader().get(ctx, s.url("/custom_statuses.json"), &statuses); err != nil {
		return err
	}

	var fields struct {
		Fields []zendeskField `json:"ticket_fields"`
	}
	if err := s.reader().get(ctx, s.url("/ticket_fields.json"), &fields); err != nil {
		return err
	}

	var forms struct {
		Forms []zendeskForm `json:"ticket_forms"`
	}
	if err := s.reader().get(ctx, s.url("/ticket_forms.json"), &forms); err != nil {
		return err
	}

	s.statuses, s.forms, s.fields = statuses.Statuses, forms.Forms, map[int64]zendeskField{}
	for _, f := range fields.Fields {
		s.fields[f.ID] = f
	}

	return nil
}

/*
Catalog reads the custom statuses and the forms with their fields.
*/
func (s *Zendesk) Catalog(ctx context.Context) (Catalog, error) {
	if err := s.load(ctx); err != nil {
		return Catalog{}, err
	}

	var out Catalog

	for _, st := range s.statuses {
		if st.Active {
			out.States = append(out.States, State{
				ExternalID: id(st.ID), Group: zendeskGroup(st.Category), Name: st.Agent, CustomerLabel: st.EndUser,
			})
		}
	}

	all := make([]string, 0, len(out.States))
	for _, st := range out.States {
		all = append(all, st.ExternalID)
	}

	for _, f := range s.forms {
		if f.Active {
			out.Types = append(out.Types, Type{
				ExternalID: "zendesk:form:" + id(f.ID), Kind: "customer", Name: f.Name, States: all,
				Fields: s.formFields(f),
			})
		}
	}

	if len(out.Types) == 0 {
		out.Types = append(out.Types, Type{
			ExternalID: DefaultForm, Kind: "customer", Name: "Zendesk ticket",
			States: all, Fields: s.formFields(zendeskForm{}),
		})
	}

	out.Types = append(out.Types, Type{
		ExternalID: ProblemType, Kind: "tracker", Name: "Problem", States: all,
		Description: "A problem many Zendesk incidents reported.",
	})

	return out, nil
}

/*
formFields returns the custom fields the form shows, every active one
for no form.
*/
func (s *Zendesk) formFields(f zendeskForm) []Field {
	ids := f.FieldIDs
	if f.ID == 0 {
		for fid := range s.fields {
			ids = append(ids, fid)
		}

		slices.Sort(ids)
	}

	var out []Field

	for _, fid := range ids {
		zf, ok := s.fields[fid]
		if !ok || !zf.Active || !zf.Removable {
			continue
		}

		out = append(out, zendeskFieldOf(zf))
	}

	return out
}

/*
zendeskFieldOf reads a custom field.
*/
func zendeskFieldOf(zf zendeskField) Field {
	f := Field{ExternalID: id(zf.ID), Key: KeyOf(zf.Title), Label: zf.Title, Customer: zf.Portal}

	switch zf.Type {
	case "textarea":
		f.DataType = "long_text"
	case "tagger", "multiselect":
		f.DataType = "single_choice"
		if zf.Type == "multiselect" {
			f.DataType = "multiple_choice"
		}

		for _, o := range zf.Options {
			f.Options = append(f.Options, o.Name)
		}
	case "integer":
		f.DataType = "number"
	case "decimal":
		f.DataType = "decimal"
	case "date":
		f.DataType = "date"
	case "checkbox":
		f.DataType = "boolean"
	default:
		f.DataType = "short_text"
	}

	return f
}

/*
zendeskGroup maps a status category to its group.
*/
func zendeskGroup(category string) string {
	switch category {
	case "open":
		return "in_progress"
	case "pending":
		return "waiting_customer"
	case "hold":
		return "waiting_team"
	case "solved", "closed":
		return "resolved"
	}

	return "submitted"
}

/*
id writes a Zendesk id.
*/
func id(n int64) string {
	return strconv.FormatInt(n, 10)
}

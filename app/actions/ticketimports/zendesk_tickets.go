/*
|--------------------------------------------------------------------------
| Zendesk Tickets
|--------------------------------------------------------------------------
|
| Tickets come from the incremental export, oldest first, with their
| requesters and assignees side-loaded. Custom field values become
| field values: a dropdown's tag becomes its option's name. Each
| ticket's audits are its history: a change of custom status, or of
| status in an account without them, is one step. An incident that
| names a problem reports that problem's tracker.
|
*/

package ticketimports

import (
	"context"
	"net/url"
	"time"
)

/*
zendeskTicket is a ticket as the export shows it.
*/
type zendeskTicket struct {
	ID           int64     `json:"id"`
	Type         string    `json:"type"`
	Subject      string    `json:"subject"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	CustomStatus int64     `json:"custom_status_id"`
	Priority     string    `json:"priority"`
	RequesterID  int64     `json:"requester_id"`
	AssigneeID   int64     `json:"assignee_id"`
	FormID       int64     `json:"ticket_form_id"`
	ProblemID    int64     `json:"problem_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Fields       []struct {
		ID    int64 `json:"id"`
		Value any   `json:"value"`
	} `json:"custom_fields"`
}

/*
zendeskUser is a side-loaded user.
*/
type zendeskUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

/*
zendeskExport is one page of the incremental export.
*/
type zendeskExport struct {
	Tickets []zendeskTicket `json:"tickets"`
	Users   []zendeskUser   `json:"users"`
	After   string          `json:"after_cursor"`
	End     bool            `json:"end_of_stream"`
}

/*
Tickets reads every ticket, oldest first.
*/
func (s *Zendesk) Tickets(ctx context.Context, each func(Ticket) error) error {
	if err := s.load(ctx); err != nil {
		return err
	}

	address := s.url("/incremental/tickets/cursor.json?start_time=0&include=users")

	for address != "" {
		var err error
		if address, err = s.page(ctx, address, each); err != nil {
			return err
		}
	}

	return nil
}

/*
page hands one page of the export to the function and returns the next
page's address, empty at the end.
*/
func (s *Zendesk) page(ctx context.Context, address string, each func(Ticket) error) (string, error) {
	var page zendeskExport
	if err := s.reader().get(ctx, address, &page); err != nil {
		return "", err
	}

	users := map[int64]zendeskUser{}
	for _, u := range page.Users {
		users[u.ID] = u
	}

	for _, zt := range page.Tickets {
		t, err := s.ticket(ctx, zt, users)
		if err != nil {
			return "", err
		}

		if err := each(t); err != nil {
			return "", err
		}
	}

	if page.End || page.After == "" {
		return "", nil
	}

	return s.url("/incremental/tickets/cursor.json?include=users&cursor=" + url.QueryEscape(page.After)), nil
}

/*
ticket shapes one exported ticket with its history.
*/
func (s *Zendesk) ticket(ctx context.Context, zt zendeskTicket, users map[int64]zendeskUser) (Ticket, error) {
	t := Ticket{
		ExternalID: "zendesk:" + id(zt.ID), TypeID: s.typeOf(zt), StateID: s.stateOf(zt.CustomStatus, zt.Status),
		Title: zt.Subject, Description: zt.Description, Priority: zendeskPriority(zt.Priority),
		RequesterEmail: users[zt.RequesterID].Email, RequesterName: users[zt.RequesterID].Name,
		AssigneeEmail: users[zt.AssigneeID].Email, CreatedAt: zt.CreatedAt, UpdatedAt: zt.UpdatedAt,
		Values: s.values(zt),
	}

	if t.Title == "" {
		t.Title = "Zendesk ticket #" + id(zt.ID)
	}

	if zt.Type == "incident" && zt.ProblemID != 0 {
		t.TrackerID = "zendesk:" + id(zt.ProblemID)
	}

	history, err := s.history(ctx, zt.ID)
	if err != nil {
		return t, err
	}

	t.History = history

	return t, nil
}

/*
typeOf returns the type a ticket becomes: Problem for a problem, else
its form's.
*/
func (s *Zendesk) typeOf(zt zendeskTicket) string {
	if zt.Type == "problem" {
		return ProblemType
	}

	var fallback string

	for _, f := range s.forms {
		if !f.Active {
			continue
		}

		if f.ID == zt.FormID {
			return "zendesk:form:" + id(f.ID)
		}

		if fallback == "" || f.Default {
			fallback = "zendesk:form:" + id(f.ID)
		}
	}

	if fallback == "" {
		return DefaultForm
	}

	return fallback
}

/*
stateOf returns the custom status, or the default status of the
category for a ticket without one.
*/
func (s *Zendesk) stateOf(custom int64, status string) string {
	for _, st := range s.statuses {
		if st.ID == custom && custom != 0 {
			return id(st.ID)
		}
	}

	group, first := zendeskGroup(status), ""

	for _, st := range s.statuses {
		if !st.Active || zendeskGroup(st.Category) != group {
			continue
		}

		if st.Default {
			return id(st.ID)
		}

		if first == "" {
			first = id(st.ID)
		}
	}

	return first
}

/*
values reads the custom field values, a dropdown's tags as option
names.
*/
func (s *Zendesk) values(zt zendeskTicket) map[string]any {
	out := map[string]any{}

	for _, cf := range zt.Fields {
		zf, ok := s.fields[cf.ID]
		if !ok || cf.Value == nil || !zf.Removable {
			continue
		}

		out[KeyOf(zf.Title)] = optionNames(zf, cf.Value)
	}

	return out
}

/*
optionNames turns a dropdown's tags into the names of its options.
*/
func optionNames(zf zendeskField, v any) any {
	name := func(tag any) any {
		for _, o := range zf.Options {
			if o.Value == tag {
				return o.Name
			}
		}

		return tag
	}

	switch x := v.(type) {
	case []any:
		out := make([]any, 0, len(x))
		for _, tag := range x {
			out = append(out, name(tag))
		}

		return out
	case string:
		if zf.Type == "tagger" {
			return name(x)
		}
	}

	return v
}

/*
zendeskPriority maps a priority, normal being medium.
*/
func zendeskPriority(p string) string {
	switch p {
	case "low", "high", "urgent":
		return p
	case "normal":
		return "medium"
	}

	return ""
}

/*
|--------------------------------------------------------------------------
| Zendesk History
|--------------------------------------------------------------------------
|
| A ticket's audits list every change with when it happened. The audit
| that created the ticket says the status it started in; each later
| change of custom status, or of status when no custom status changed
| with it, is a step into that state.
|
*/

package ticketimports

import (
	"context"
	"strconv"
	"time"
)

/*
zendeskAudit is one audit of a ticket.
*/
type zendeskAudit struct {
	CreatedAt time.Time `json:"created_at"`
	Events    []struct {
		Type  string `json:"type"`
		Field string `json:"field_name"`
		Value any    `json:"value"`
	} `json:"events"`
}

/*
history reads the ticket's audits as steps, page by page.
*/
func (s *Zendesk) history(ctx context.Context, ticket int64) ([]Move, error) {
	var out []Move

	address := s.url("/tickets/" + id(ticket) + "/audits.json")

	for address != "" {
		var page struct {
			Audits []zendeskAudit `json:"audits"`
			Next   *string        `json:"next_page"`
		}
		if err := s.reader().get(ctx, address, &page); err != nil {
			return nil, err
		}

		for _, a := range page.Audits {
			if st := s.auditState(a); st != "" && (len(out) == 0 || out[len(out)-1].StateID != st) {
				out = append(out, Move{At: a.CreatedAt, StateID: st})
			}
		}

		address = ""
		if page.Next != nil {
			address = *page.Next
		}
	}

	return out, nil
}

/*
auditState returns the state an audit moved the ticket into, empty for
none.
*/
func (s *Zendesk) auditState(a zendeskAudit) string {
	status, custom := "", int64(0)

	for _, e := range a.Events {
		if e.Type != "Change" && e.Type != "Create" {
			continue
		}

		switch e.Field {
		case "status":
			status = text(e.Value)
		case "custom_status_id":
			custom, _ = strconv.ParseInt(text(e.Value), 10, 64)
		}
	}

	if status == "" && custom == 0 {
		return ""
	}

	return s.stateOf(custom, status)
}

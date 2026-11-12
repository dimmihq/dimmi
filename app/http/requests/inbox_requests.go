/*
|--------------------------------------------------------------------------
| Inbox Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on inboxes. Every field of an update is
| optional and a missing one stays as it is; a null default team clears
| it, and settings are a merge patch on what the inbox has.
|
*/

package requests

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ShowInboxRequest is the input of GET /inboxes/{inbox}.
*/
type ShowInboxRequest struct {
	Inbox string `path:"inbox"`
}

/*
ShowAssignmentStatusRequest is the input of GET
/inboxes/{inbox}/assignment-status: the inbox and the channel type its
pool is judged for.
*/
type ShowAssignmentStatusRequest struct {
	Inbox       string `path:"inbox"`
	ChannelType string `query:"channel_type" validate:"omitempty,oneof=widget email api"`
}

/*
StoreInboxRequest is the input of POST /inboxes.
*/
type StoreInboxRequest struct {
	Body struct {
		Name string `json:"name" validate:"required,max=80"`
	}
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *StoreInboxRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if str.Clean(r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
UpdateInboxRequest is the input of PATCH /inboxes/{inbox}. A null
ticket type stops new conversations becoming tickets.
*/
type UpdateInboxRequest struct {
	Inbox string `path:"inbox"`
	Body  struct {
		Name          *string         `json:"name,omitempty"           validate:"omitempty,max=80"`
		DefaultTeamID *string         `json:"default_team_id,omitempty"`
		Settings      json.RawMessage `json:"settings,omitempty"`
		TicketTypeID  *string         `json:"ticket_type_id,omitempty" maxLength:"64"`
	}
	RawBody []byte
}

/*
Validate refuses a name made only of invisible characters.
*/
func (r *UpdateInboxRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.Name != nil && str.Clean(*r.Body.Name) == "" {
		return violation(ctx, "name", "required", "validation.required")
	}

	return nil
}

/*
DeleteInboxRequest is the input of DELETE /inboxes/{inbox}: the inbox
and, when it holds conversations, the inbox they move to.
*/
type DeleteInboxRequest struct {
	Inbox  string `path:"inbox"`
	MoveTo string `query:"move_to"`
}

/*
|--------------------------------------------------------------------------
| Ticket Type Requests
|--------------------------------------------------------------------------
|
| The inputs of a workspace's ticket types: a new type of a kind, a
| change to one with the states it may use, archiving one, and the order
| of its fields. A null inbox, team, auto-resolve or close-the-loop
| setting clears it.
|
*/

package requests

/*
TicketTypeBody is a type's settings; fields left out stay as they are.
The kind is read only when the type is added.
*/
type TicketTypeBody struct {
	Kind               *string  `json:"kind,omitempty"                      enum:"customer,back_office,tracker,incident"` //nolint:lll // one enum
	Name               *string  `json:"name,omitempty"                      maxLength:"60"`
	Icon               *string  `json:"icon,omitempty"                      maxLength:"40"`
	Description        *string  `json:"description,omitempty"               maxLength:"500"`
	InboxID            *string  `json:"inbox_id,omitempty"                  maxLength:"64"`
	TeamID             *string  `json:"team_id,omitempty"                   maxLength:"64"`
	CustomerVisible    *bool    `json:"customer_visible,omitempty"`
	CompanySharing     *bool    `json:"company_sharing,omitempty"`
	ReopenDays         *int32   `json:"reopen_days,omitempty"               minimum:"0" maximum:"90"`
	AutoResolveDays    *int32   `json:"auto_resolve_days,omitempty"         minimum:"1" maximum:"90"`
	AutoResolveMessage *string  `json:"auto_resolve_message,omitempty"      maxLength:"2000"`
	AIAutoFill         *bool    `json:"ai_auto_fill,omitempty"`
	StateIDs           []string `json:"state_ids,omitempty"                 maxItems:"100"`
	DefaultStateID     *string  `json:"default_state_id,omitempty"          maxLength:"64"`
	DefaultResolvedID  *string  `json:"default_resolved_state_id,omitempty" maxLength:"64"`
}

/*
StoreTicketTypeRequest is the input of POST /ticket-types.
*/
type StoreTicketTypeRequest struct {
	Body TicketTypeBody
}

/*
UpdateTicketTypeRequest is the input of PATCH /ticket-types/{type}.
RawBody tells a null apart from a field left out.
*/
type UpdateTicketTypeRequest struct {
	Type    string `path:"type"`
	Body    TicketTypeBody
	RawBody []byte
}

/*
TicketTypeRequest names a type in the path.
*/
type TicketTypeRequest struct {
	Type string `path:"type"`
}

/*
ReorderTicketFieldsRequest is a type's fields in their new order.
*/
type ReorderTicketFieldsRequest struct {
	Type string `path:"type"`
	Body struct {
		FieldIDs []string `json:"field_ids" maxItems:"200" validate:"required,min=1"`
	}
}

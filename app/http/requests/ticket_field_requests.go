/*
|--------------------------------------------------------------------------
| Ticket Field Requests
|--------------------------------------------------------------------------
|
| The inputs of a type's fields: a new field with its key and data type,
| a change to one, and archiving one. Key and data type never change. A
| null currency, condition or AI hint clears it.
|
*/

package requests

/*
ShowWhenBody shows a field only when another field has a value, one of
equals when it lists any.
*/
type ShowWhenBody struct {
	Field  string   `json:"field"            maxLength:"40" validate:"required"`
	Equals []string `json:"equals,omitempty" maxItems:"50"`
}

/*
TicketFieldBody is a field's settings; fields left out stay as they are.
*/
type TicketFieldBody struct {
	Key                    *string       `json:"key,omitempty"                      maxLength:"40"`
	Label                  *string       `json:"label,omitempty"                    maxLength:"80"`
	Description            *string       `json:"description,omitempty"              maxLength:"300"`
	DataType               *string       `json:"data_type,omitempty"                enum:"short_text,long_text,number,decimal,currency,boolean,date,single_choice,multiple_choice,file,member,company"` //nolint:lll // one enum
	Options                []string      `json:"options,omitempty"                  maxItems:"200"`
	Currency               *string       `json:"currency,omitempty"                 maxLength:"3"`
	RequiredCreate         *bool         `json:"required_create,omitempty"`
	RequiredCreateCustomer *bool         `json:"required_create_customer,omitempty"`
	RequiredResolve        *bool         `json:"required_resolve,omitempty"`
	VisibleCustomer        *bool         `json:"visible_customer,omitempty"`
	EditableCustomer       *bool         `json:"editable_customer,omitempty"`
	ShowWhen               *ShowWhenBody `json:"show_when,omitempty"`
	AIHint                 *string       `json:"ai_hint,omitempty"                  maxLength:"500"`
	AIAuto                 *bool         `json:"ai_auto,omitempty"`
}

/*
StoreTicketFieldRequest is the input of POST /ticket-types/{type}/fields.
*/
type StoreTicketFieldRequest struct {
	Type string `path:"type"`
	Body TicketFieldBody
}

/*
UpdateTicketFieldRequest is the input of PATCH /ticket-fields/{field}.
RawBody tells a null apart from a field left out.
*/
type UpdateTicketFieldRequest struct {
	Field   string `path:"field"`
	Body    TicketFieldBody
	RawBody []byte
}

/*
TicketFieldRequest names a field in the path.
*/
type TicketFieldRequest struct {
	Field string `path:"field"`
}

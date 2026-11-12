/*
|--------------------------------------------------------------------------
| Widget Ticket Requests
|--------------------------------------------------------------------------
|
| The inputs of a visitor's requests: the form they submit, the list of
| their requests, theirs or their company's, and one request. A form
| sent twice with the same client id makes one request.
|
*/

package requests

/*
StoreWidgetTicketRequest is the input of POST /widget/v1/tickets.
Fields holds the values of the fields the customer may fill, a file
field holding attachment ids from the visitor's uploads.
*/
type StoreWidgetTicketRequest struct {
	Body struct {
		_             struct{}        `additionalProperties:"true"`
		ClientID      string          `json:"client_id"                maxLength:"64"   validate:"required"`
		TypeID        string          `json:"type_id"                  maxLength:"64"   validate:"required"`
		Title         string          `json:"title"                    maxLength:"200"  validate:"required"`
		Description   string          `json:"description,omitempty"    maxLength:"10000"`
		Fields        map[string]any  `json:"fields,omitempty"`
		AttachmentIDs []string        `json:"attachment_ids,omitempty" maxItems:"10"`
		Page          *WidgetPageBody `json:"page,omitempty"`
	}
}

/*
IndexWidgetTicketsRequest lists the visitor's requests: mine, or the
company's when a type shares them and the visitor is verified. Open
keeps only those not resolved.
*/
type IndexWidgetTicketsRequest struct {
	Scope string `query:"scope" enum:"mine,company,"`
	Open  bool   `query:"open"  doc:"Only requests not resolved."`
}

/*
ShowWidgetTicketRequest names one of the visitor's requests, by id or
by reference such as T-482.
*/
type ShowWidgetTicketRequest struct {
	Ticket string `path:"ticket"`
}

/*
|--------------------------------------------------------------------------
| Widget Contact Requests
|--------------------------------------------------------------------------
|
| What a lead writes about themselves. Sizes are bounded here; the action
| decides what each field may hold.
|
*/

package requests

/*
UpdateWidgetContactBody is what a lead writes about themselves.
*/
type UpdateWidgetContactBody struct {
	_                struct{}       `additionalProperties:"true"`
	Name             *string        `json:"name,omitempty"              maxLength:"1000"`
	Email            *string        `json:"email,omitempty"             maxLength:"1000"`
	Phone            *string        `json:"phone,omitempty"             maxLength:"64"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

/*
UpdateWidgetContactRequest is the input of PATCH /widget/v1/contact.
*/
type UpdateWidgetContactRequest struct {
	Body UpdateWidgetContactBody
}

/*
StoreWidgetPingRequest is the input of POST /widget/v1/pings: the page the
visitor is on and context their page passes along.
*/
type StoreWidgetPingRequest struct {
	Body struct {
		_       struct{}        `additionalProperties:"true"`
		Page    *WidgetPageBody `json:"page,omitempty"`
		Context map[string]any  `json:"context,omitempty"`
		Visible *bool           `json:"visible,omitempty"`
	}
}

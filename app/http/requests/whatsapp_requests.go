/*
|--------------------------------------------------------------------------
| WhatsApp Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that connect WhatsApp numbers and send
| templates. A token is a system user's of a Meta app; the app's secret
| comes along only for an app other than the install's own. What Meta's
| popup hands back arrives as it came.
|
*/

package requests

/*
WhatsAppTokenBody is a Meta app's token, the WhatsApp account to read
with it and, for an app other than the install's own, its secret.
*/
type WhatsAppTokenBody struct {
	AccessToken string `json:"access_token" validate:"required,max=1024"`
	WabaID      string `json:"waba_id"      validate:"required,max=64"`
	AppSecret   string `json:"app_secret,omitempty" validate:"omitempty,max=128"`
}

/*
WhatsAppAccountRequest is the input of POST /channels/whatsapp/accounts.
*/
type WhatsAppAccountRequest struct {
	Body WhatsAppTokenBody
}

/*
StoreWhatsAppChannelRequest is the input of POST /channels/whatsapp.
*/
type StoreWhatsAppChannelRequest struct {
	Body struct {
		WhatsAppTokenBody

		PhoneNumberID string  `json:"phone_number_id"    validate:"required,max=64"`
		InboxID       *string `json:"inbox_id,omitempty"`
		Name          string  `json:"name,omitempty"     validate:"omitempty,max=80"`
	}
}

/*
StoreWhatsAppSignupRequest is the input of POST
/channels/whatsapp/signups: what Meta's popup handed back.
*/
type StoreWhatsAppSignupRequest struct {
	Body struct {
		Code          string  `json:"code"                      validate:"required,max=1024"`
		RedirectURI   string  `json:"redirect_uri,omitempty"     validate:"omitempty,max=512"`
		WabaID        string  `json:"waba_id,omitempty"          validate:"omitempty,max=64"`
		PhoneNumberID string  `json:"phone_number_id,omitempty"  validate:"omitempty,max=64"`
		BusinessID    string  `json:"business_id,omitempty" validate:"omitempty,max=64"`
		Coexistence   bool    `json:"coexistence,omitempty"`
		InboxID       *string `json:"inbox_id,omitempty"`
	}
}

/*
|--------------------------------------------------------------------------
| WhatsApp Template Parts
|--------------------------------------------------------------------------
|
| The smaller inputs of a written template: a button, and the customer
| detail one of its variables stands for.
|
*/

package requests

/*
WhatsAppTemplateButtonBody is a button of a written template.
*/
type WhatsAppTemplateButtonBody struct {
	Type        string `json:"type"                   validate:"required,oneof=quick_reply url phone_number"`
	Text        string `json:"text"                   validate:"max=100"`
	URL         string `json:"url,omitempty"          validate:"omitempty,max=2000"`
	PhoneNumber string `json:"phone_number,omitempty" validate:"omitempty,max=32"`
}

/*
WhatsAppBindingBody is the customer detail a variable stands for, such
as contact.first_name, and its fallback.
*/
type WhatsAppBindingBody struct {
	Key      string `json:"key"                validate:"required,max=64"`
	Fallback string `json:"fallback,omitempty" validate:"omitempty,max=64"`
}

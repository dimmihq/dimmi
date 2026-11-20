/*
|--------------------------------------------------------------------------
| WhatsApp Template Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that write a number's templates. A template is
| a text header or none, a body, a footer, buttons, and a sample for each
| variable keyed as "header.1" or "body.2". Its name and language are
| chosen once, when it is created.
|
*/

package requests

/*
WhatsAppTemplateBody is a template's content as a teammate wrote it.
*/
type WhatsAppTemplateBody struct {
	Category string                         `json:"category"           validate:"required,oneof=utility marketing"`
	Header   string                         `json:"header,omitempty"   validate:"omitempty,max=200"`
	Body     string                         `json:"body"               validate:"max=4096"`
	Footer   string                         `json:"footer,omitempty"   validate:"omitempty,max=200"`
	Buttons  []WhatsAppTemplateButtonBody   `json:"buttons,omitempty"  validate:"omitempty,max=10,dive"`
	Samples  map[string]string              `json:"samples,omitempty"  validate:"omitempty,max=40"`
	Bindings map[string]WhatsAppBindingBody `json:"bindings,omitempty" validate:"omitempty,max=40"`
}

/*
StoreWhatsAppTemplateRequest is the input of POST
/channels/whatsapp/{channel}/templates.
*/
type StoreWhatsAppTemplateRequest struct {
	Channel string `path:"channel"`
	Body    struct {
		WhatsAppTemplateBody

		Name     string `json:"name"     validate:"max=512"`
		Language string `json:"language" validate:"required,max=16"`
	}
}

/*
UpdateWhatsAppTemplateRequest is the input of PUT
/channels/whatsapp/{channel}/templates/{template}.
*/
type UpdateWhatsAppTemplateRequest struct {
	Channel  string `path:"channel"`
	Template string `path:"template"`
	Body     WhatsAppTemplateBody
}

/*
WhatsAppTemplateRequest names one template of a number.
*/
type WhatsAppTemplateRequest struct {
	Channel  string `path:"channel"`
	Template string `path:"template"`
}

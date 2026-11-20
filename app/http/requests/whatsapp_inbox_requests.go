/*
|--------------------------------------------------------------------------
| WhatsApp Inbox Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that read a number's templates, where a
| conversation stands on WhatsApp, and send a template as a reply.
|
*/

package requests

/*
WhatsAppTemplatesRequest is the input of GET
/channels/whatsapp/{channel}/templates.
*/
type WhatsAppTemplatesRequest struct {
	Channel string `path:"channel"`
	Refresh bool   `query:"refresh"`
}

/*
WhatsAppConversationRequest is the input of GET
/conversations/{conversation}/whatsapp.
*/
type WhatsAppConversationRequest struct {
	Conversation string `path:"conversation"`
}

/*
WhatsAppTemplateReplyRequest is the input of POST
/conversations/{conversation}/whatsapp-templates.
*/
type WhatsAppTemplateReplyRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Name            string            `json:"name"     validate:"required,max=512"`
		Language        string            `json:"language" validate:"required,max=15"`
		Values          map[string]string `json:"values,omitempty"`
		ClientMessageID *string           `json:"client_message_id,omitempty" validate:"omitempty,min=1,max=64"`
	}
}

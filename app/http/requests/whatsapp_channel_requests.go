/*
|--------------------------------------------------------------------------
| WhatsApp Number Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that name a connected WhatsApp number: moving
| its new conversations to another inbox, and disconnecting it.
|
*/

package requests

/*
WhatsAppChannelRequest is the input of the endpoints that name a
WhatsApp connection.
*/
type WhatsAppChannelRequest struct {
	Channel string `path:"channel"`
}

/*
UpdateWhatsAppChannelRequest is the input of PATCH
/channels/whatsapp/{channel}.
*/
type UpdateWhatsAppChannelRequest struct {
	Channel string `path:"channel"`
	Body    struct {
		InboxID *string `json:"inbox_id,omitempty"`
	}
}

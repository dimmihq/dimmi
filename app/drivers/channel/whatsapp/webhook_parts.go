/*
|--------------------------------------------------------------------------
| Shared Cards And Taps
|--------------------------------------------------------------------------
|
| What a customer's message carries besides text and files: contact
| cards, taps on buttons and list rows, and reactions.
|
*/

package whatsapp

/*
SharedCard is a contact card a customer shared.
*/
type SharedCard struct {
	Name struct {
		FormattedName string `json:"formatted_name"`
	} `json:"name"`
	Phones []struct {
		Phone string `json:"phone"`
	} `json:"phones"`
}

/*
Interactive is a customer's tap on a button or a list row.
*/
type Interactive struct {
	Type        string `json:"type"`
	ButtonReply *Reply `json:"button_reply"`
	ListReply   *Reply `json:"list_reply"`
}

/*
Reply is the button or row tapped.
*/
type Reply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

/*
Button is a tap on a template's quick reply.
*/
type Button struct {
	Payload string `json:"payload"`
	Text    string `json:"text"`
}

/*
Reaction is an emoji put on a message.
*/
type Reaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

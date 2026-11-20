/*
|--------------------------------------------------------------------------
| Signals
|--------------------------------------------------------------------------
|
| Events that are not messages: a reaction, a read receipt, an edit, a
| pressed button, and a change of which app answers the thread.
|
*/

package social

/*
Reaction is an emoji put on, or taken off, a message.
*/
type Reaction struct {
	MessageID string `json:"mid"`
	Action    string `json:"action"`
	Name      string `json:"reaction"`
	Emoji     string `json:"emoji"`
}

/*
Removed reports whether the reaction was taken off.
*/
func (r Reaction) Removed() bool {
	return r.Action == "unreact"
}

/*
Read says the customer read the thread: one message on Instagram, and
everything up to a moment on Messenger.
*/
type Read struct {
	MessageID string   `json:"mid"`
	Watermark flexible `json:"watermark"`
}

/*
Edit is a message whose text the customer changed.
*/
type Edit struct {
	MessageID string `json:"mid"`
	Text      string `json:"text"`
}

/*
Postback is a button the customer pressed, such as a conversation
starter; its title is what they chose.
*/
type Postback struct {
	ID    string `json:"mid"`
	Title string `json:"title"`
}

/*
Control says which app answers the thread from now on.
*/
type Control struct {
	Previous flexible `json:"previous_owner_app_id"`
	Owner    flexible `json:"new_owner_app_id"`
}

/*
|--------------------------------------------------------------------------
| WhatsApp Template Content
|--------------------------------------------------------------------------
|
| A template's parts as the settings editor fills them in: its header,
| body, footer and buttons, and the samples Meta reviewed its variables
| with.
|
*/

package resources

/*
WhatsAppContent is a template's parts as the editor fills them in, with
the samples Meta reviewed its variables with.
*/
type WhatsAppContent struct {
	Header  string            `json:"header"`
	Body    string            `json:"body"`
	Footer  string            `json:"footer"`
	Buttons []WhatsAppButton  `json:"buttons"`
	Samples map[string]string `json:"samples"`
	// Editable says whether dimmi's editor holds every part: a text
	// header or none, and buttons it can send.
	Editable bool `json:"editable"`
}

/*
WhatsAppButton is a template's button.
*/
type WhatsAppButton struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	URL         string `json:"url,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

/*
WhatsAppSignupConfig is what Meta's signup popup needs to open: the Meta
app, its Embedded Signup configuration and the Graph API version, and
whether this server can open the popup at all.
*/
type WhatsAppSignupConfig struct {
	Available    bool   `json:"available"`
	AppID        string `json:"app_id"`
	ConfigID     string `json:"config_id"`
	GraphVersion string `json:"graph_version"`
}

/*
WhatsAppReplyMeta carries the conversation as it is after a template was
sent.
*/
type WhatsAppReplyMeta struct {
	Conversation Conversation `json:"conversation"`
}

/*
|--------------------------------------------------------------------------
| Broadcast Topic Requests
|--------------------------------------------------------------------------
|
| The inputs of broadcast topics and of the counted links in broadcast
| emails. A topic is what people choose to keep or leave on the
| preference page.
|
*/

package requests

/*
BroadcastTopicBody is a topic's editable fields.
*/
type BroadcastTopicBody struct {
	Name        string `json:"name"                  maxLength:"80" minLength:"1"`
	Description string `json:"description,omitempty" maxLength:"300"`
	Consent     string `json:"consent,omitempty"     enum:"opt_in,opt_out,"`
	Archived    bool   `json:"archived,omitempty"`
}

/*
StoreBroadcastTopicRequest is the input of POST /broadcast-topics.
*/
type StoreBroadcastTopicRequest struct {
	Body BroadcastTopicBody
}

/*
UpdateBroadcastTopicRequest is the input of PATCH
/broadcast-topics/{topic}.
*/
type UpdateBroadcastTopicRequest struct {
	Topic string `path:"topic"`
	Body  BroadcastTopicBody
}

/*
BroadcastLinkRequest names a public broadcast link by its token.
*/
type BroadcastLinkRequest struct {
	Token string `path:"token"`
}

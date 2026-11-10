/*
|--------------------------------------------------------------------------
| Broadcast Requests
|--------------------------------------------------------------------------
|
| The inputs of broadcasts, their topics and the public preference page.
| A draft is saved a field at a time: what a request leaves out keeps
| its stored value, so the composer saves only what changed.
|
*/

package requests

/*
BroadcastRequest names a broadcast in the path.
*/
type BroadcastRequest struct {
	Broadcast string `path:"broadcast"`
}

/*
ListBroadcastsRequest narrows the list of broadcasts.
*/
type ListBroadcastsRequest struct {
	Status  string `query:"status"  enum:"draft,scheduled,sending,paused,sent,stopped,"`
	Channel string `query:"channel" enum:"email,chat,post,slack,"`
	Search  string `query:"q"`
	Limit   int    `query:"limit"`
	Offset  int    `query:"offset"`
}

/*
StoreBroadcastRequest is the input of POST /broadcasts.
*/
type StoreBroadcastRequest struct {
	Body struct {
		Name    string `json:"name"    maxLength:"120" minLength:"1"`
		Channel string `json:"channel" enum:"email,chat,post,slack"`
	}
}

/*
UpdateBroadcastRequest is the input of PATCH /broadcasts/{broadcast}.
*/
type UpdateBroadcastRequest struct {
	Broadcast string `path:"broadcast"`
	Body      BroadcastBody
}

/*
TestBroadcastRequest is the input of POST /broadcasts/{broadcast}/tests:
an email address, or a Slack channel by its room id.
*/
type TestBroadcastRequest struct {
	Broadcast string `path:"broadcast"`
	Body      struct {
		Email  string `json:"email,omitempty"   maxLength:"254"`
		RoomID string `json:"room_id,omitempty"`
	}
}

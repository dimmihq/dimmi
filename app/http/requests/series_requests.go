/*
|--------------------------------------------------------------------------
| Series Requests
|--------------------------------------------------------------------------
|
| The inputs of the series API. The definition travels as the same JSON
| the builder and the CLI write; ids inside it are public ids. A save
| carries the lock version it read, so a stale save is refused.
|
*/

package requests

import "encoding/json"

/*
SeriesRequest names a series in the path.
*/
type SeriesRequest struct {
	Series string `path:"series"`
}

/*
ListSeriesRequest narrows the list of series.
*/
type ListSeriesRequest struct {
	Status string `query:"status" enum:"draft,live,paused,archived,"`
	Search string `query:"q"`
	Limit  int    `query:"limit"`
	Offset int    `query:"offset"`
}

/*
StoreSeriesRequest is the input of POST /series.
*/
type StoreSeriesRequest struct {
	Body struct {
		Name    string          `json:"name"           maxLength:"120"        minLength:"1"`
		Subject string          `json:"subject"        enum:"contact,company"`
		Spec    json.RawMessage `json:"spec,omitempty"`
	}
}

/*
SeriesBody is a save of a series: everything the builder holds. Ids are
public ids; an empty string clears one.
*/
type SeriesBody struct {
	LockVersion    int32           `json:"lock_version"`
	Name           string          `json:"name"                       maxLength:"120"   minLength:"1"`
	Description    string          `json:"description,omitempty"      maxLength:"500"`
	Spec           json.RawMessage `json:"spec,omitempty"`
	TopicID        string          `json:"topic_id,omitempty"`
	Service        bool            `json:"service,omitempty"`
	EmailAddressID string          `json:"email_address_id,omitempty"`
	SenderID       string          `json:"sender_id,omitempty"`
	SlackAs        string          `json:"slack_as,omitempty"         enum:"bot,member,"`
	ReplyTeamID    string          `json:"reply_team_id,omitempty"`
	ReplyMemberID  string          `json:"reply_member_id,omitempty"`
	AIReplies      bool            `json:"ai_replies,omitempty"`
}

/*
UpdateSeriesRequest is the input of PATCH /series/{series}.
*/
type UpdateSeriesRequest struct {
	Series string `path:"series"`
	Body   SeriesBody
}

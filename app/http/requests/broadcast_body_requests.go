/*
|--------------------------------------------------------------------------
| Broadcast Body Requests
|--------------------------------------------------------------------------
|
| The fields of a broadcast draft and when it is scheduled, plus the
| page of recipients a report lists. What a request leaves out keeps its
| stored value.
|
*/

package requests

import (
	"encoding/json"
	"time"
)

/*
BroadcastSchedule is when a broadcast goes out: now, at a time, or at a
time in each person's own zone.
*/
type BroadcastSchedule struct {
	Kind string     `json:"kind"         enum:"now,at,local"`
	At   *time.Time `json:"at,omitempty"`
}

/*
BroadcastBody is a draft's editable fields. Ids are public ids; an empty
string clears an id.
*/
type BroadcastBody struct {
	Name             *string            `json:"name,omitempty"               maxLength:"120"`
	AudienceID       *string            `json:"audience_id,omitempty"`
	Rules            json.RawMessage    `json:"rules,omitempty"`
	RoomIDs          []string           `json:"room_ids,omitempty"`
	TopicID          *string            `json:"topic_id,omitempty"`
	Service          *bool              `json:"service,omitempty"`
	Subject          *string            `json:"subject,omitempty"            maxLength:"200"`
	Preview          *string            `json:"preview,omitempty"            maxLength:"200"`
	Body             *string            `json:"body,omitempty"               maxLength:"50000"`
	SenderID         *string            `json:"sender_id,omitempty"`
	EmailAddressID   *string            `json:"email_address_id,omitempty"`
	SlackAs          *string            `json:"slack_as,omitempty"           enum:"bot,member"`
	ReplyTeamID      *string            `json:"reply_team_id,omitempty"`
	ReplyMemberID    *string            `json:"reply_member_id,omitempty"`
	AIReplies        *bool              `json:"ai_replies,omitempty"`
	TrackClicks      *bool              `json:"track_clicks,omitempty"`
	SkipFrequencyCap *bool              `json:"skip_frequency_cap,omitempty"`
	Schedule         *BroadcastSchedule `json:"schedule,omitempty"`
	Goal             json.RawMessage    `json:"goal,omitempty"`
}

/*
BroadcastRecipientsRequest narrows a broadcast's recipient list by what
happened and by account.
*/
type BroadcastRecipientsRequest struct {
	Broadcast string `path:"broadcast"`
	Status    string `query:"status"`
	CompanyID string `query:"company_id"`
	Limit     int    `query:"limit"`
	Offset    int    `query:"offset"`
}

/*
|--------------------------------------------------------------------------
| Messenger And Instagram Conversations
|--------------------------------------------------------------------------
|
| Where a conversation stands on its platform, for the composer, and who
| its customer is there, for the customer panel.
|
*/

package resources

import "time"

/*
SocialConversation is where a conversation stands on Messenger or
Instagram. Phase is open while anyone may reply, human while only a
teammate may, and closed until the customer writes again.
*/
type SocialConversation struct {
	ChannelID  string        `json:"channel_id"`
	Platform   string        `json:"platform"`
	Phase      string        `json:"phase"`
	LastWrote  *time.Time    `json:"last_wrote_at"`
	OpenUntil  *time.Time    `json:"open_until"`
	HumanUntil *time.Time    `json:"human_until"`
	Profile    SocialProfile `json:"profile"`
}

/*
SocialProfile is who the customer is on the platform. Followers,
Verified and Follows are Instagram's.
*/
type SocialProfile struct {
	Name       string `json:"name"`
	Username   string `json:"username"`
	PictureURL string `json:"picture_url"`
	Followers  int64  `json:"followers"`
	Verified   bool   `json:"verified"`
	Follows    bool   `json:"follows"`
}

/*
SocialFinding is one thing a connection's check asked Meta: check is
token, delivery or messages, ok whether it holds, and detail Meta's
words when it does not.
*/
type SocialFinding struct {
	Check  string `json:"check"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

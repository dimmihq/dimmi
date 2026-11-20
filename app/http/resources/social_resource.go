/*
|--------------------------------------------------------------------------
| Messenger And Instagram Resources
|--------------------------------------------------------------------------
|
| What the dashboard reads about the two channels: a connected Facebook
| Page or Instagram account, the accounts a sign-in at Meta shared, and
| where a conversation stands on its platform.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SocialChannel is a connected Facebook Page or Instagram account.
Platform is facebook or instagram. AccountID is the Page's or the
account's id at Meta.
*/
type SocialChannel struct {
	ID         string    `json:"id"`
	Object     string    `json:"object"`
	Platform   string    `json:"platform"`
	Name       string    `json:"name"`
	Username   string    `json:"username"`
	PictureURL string    `json:"picture_url"`
	Status     string    `json:"status"`
	InboxID    string    `json:"inbox_id"`
	AccountID  string    `json:"account_id"`
	LastError  *string   `json:"last_error"`
	CreatedAt  time.Time `json:"created_at"`
}

/*
SocialFacts are a connection's facts as the resource shows them.
*/
type SocialFacts struct {
	AccountID  string
	Username   string
	PictureURL string
}

/*
NewSocialChannel shows a connection.
*/
func NewSocialChannel(conn models.ChannelConnection, f SocialFacts) SocialChannel {
	return SocialChannel{
		ID: models.PublicID(models.PrefixChannelConnection, conn.ID), Object: "social_channel", Platform: conn.Type,
		Name: conn.Name, Username: f.Username, PictureURL: f.PictureURL, Status: conn.Status,
		InboxID: models.PublicID(models.PrefixInbox, conn.InboxID), AccountID: f.AccountID,
		LastError: conn.LastError, CreatedAt: conn.CreatedAt,
	}
}

/*
SocialSignupConfig is what Meta's sign-in popup needs to open: the Meta
app, the platform's login configuration and the Graph API version, and
whether this server can open the popup at all.
*/
type SocialSignupConfig struct {
	Available    bool   `json:"available"`
	AppID        string `json:"app_id"`
	ConfigID     string `json:"config_id"`
	GraphVersion string `json:"graph_version"`
}

/*
SocialGrant is a sign-in at Meta waiting for its account to be picked:
the grant that connects one, and the accounts Meta shared.
*/
type SocialGrant struct {
	Grant    string          `json:"grant"`
	Accounts []SocialAccount `json:"accounts"`
}

/*
SocialAccount is a Page or Instagram account a sign-in shared, and
whether it is connected here already.
*/
type SocialAccount struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	PictureURL string `json:"picture_url"`
	Connected  bool   `json:"connected"`
}

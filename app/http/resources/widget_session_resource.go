/*
|--------------------------------------------------------------------------
| Widget Session Resource
|--------------------------------------------------------------------------
|
| What a visitor's messenger is told about its own session. It carries the
| token only on the answer that created it, the visitor only as far as the
| widget shows them, and the counts on this widget alone. It never carries
| anything about the team.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/widget"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetIdentity says whether the session was verified, and why not.
*/
type WidgetIdentity struct {
	Status   string   `json:"status"`
	Error    *string  `json:"error"`
	Warnings []string `json:"warnings"`
}

/*
Realtime is where the messenger's stream connects and the channel it
receives.
*/
type Realtime struct {
	URL      string `json:"url"`
	Channel  string `json:"channel"`
	TokenURL string `json:"token_url"`
}

/*
WidgetSession is a visitor's session.
*/
type WidgetSession struct {
	ID                  string         `json:"id"`
	Object              string         `json:"object"`
	Token               string         `json:"token,omitempty"`
	Kind                string         `json:"kind"`
	Test                bool           `json:"test"`
	ExpiresAt           time.Time      `json:"expires_at"`
	Contact             WidgetContact  `json:"contact"`
	Identity            WidgetIdentity `json:"identity"`
	UnreadCount         int64          `json:"unread_count"`
	OpenConversationIDs []string       `json:"open_conversation_ids"`
	Realtime            Realtime       `json:"realtime"`
	ConfigVersion       int64          `json:"config_version"`
	ServerTime          time.Time      `json:"server_time"`
}

/*
SessionAnswer is how a session is answered: the address the visitor reached
this install on, the token, shown only when the session was just created,
and the server's time.
*/
type SessionAnswer struct {
	Base  string
	Token string
	Now   time.Time
}

/*
NewWidgetSession returns the session as its messenger sees it.
*/
func NewWidgetSession(v widget.SessionView, cfg models.WidgetSetting, a SessionAnswer) WidgetSession {
	base, token, now := a.Base, a.Token, a.Now
	status := "anonymous"
	if v.Session.Kind == string(enums.WidgetSessionIdentified) {
		status = "verified"
	}

	return WidgetSession{
		ID: models.PublicID(models.PrefixWidgetSession, v.Session.ID), Object: "widget_session", Token: token,
		Kind: v.Session.Kind, Test: v.Session.Test, ExpiresAt: v.Session.ExpiresAt.UTC(),
		Contact:     NewWidgetContact(v.Contact),
		Identity:    WidgetIdentity{Status: status, Warnings: []string{}},
		UnreadCount: v.UnreadCount, OpenConversationIDs: publicIDs(models.PrefixConversation, v.OpenConversationIDs),
		Realtime: Realtime{
			URL: base + "/connection/uni_sse", Channel: appbroadcasting.WidgetContactChannel(v.Contact.ID),
			TokenURL: base + "/widget/v1/realtime-tokens",
		},
		ConfigVersion: cfg.ConfigVersion, ServerTime: now.UTC(),
	}
}

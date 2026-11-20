/*
|--------------------------------------------------------------------------
| Messenger And Instagram Settings
|--------------------------------------------------------------------------
|
| A workspace connects its Facebook Pages and Instagram accounts through
| the install's Meta app, and their direct messages become conversations.
| Meta decides how long a business may answer: anyone, the AI agent
| included, for a day after the customer's last message, and only a
| person for the rest of a week. Both lengths are settings so a test can
| shorten them; an install has no reason to change them.
|
*/

package config

import "time"

/*
SocialChannelConfig configures the Messenger and Instagram channels.

GraphURL is the Graph API host, Meta's own unless a test or a proxy
needs another, and GraphVersion the version every call names.
MaxPayloadBytes bounds one delivery and DedupeWindow is how long a
delivered event is recognized quickly. Window is how long after a
customer's last message anyone may reply, HumanWindow how long a
teammate still may. MaxMediaBytes is the largest file the inbox copies
from a message.
*/
type SocialChannelConfig struct {
	GraphURL        string        `default:"https://graph.facebook.com" koanf:"graph_url"`
	GraphVersion    string        `default:"v27.0"                      koanf:"graph_version"`
	MaxPayloadBytes int64         `default:"3145728"                    koanf:"max_payload_bytes"`
	DedupeWindow    time.Duration `default:"1h"                         koanf:"dedupe_window"`
	Window          time.Duration `default:"24h"                        koanf:"window"`
	HumanWindow     time.Duration `default:"168h"                       koanf:"human_window"`
	MaxMediaBytes   int64         `default:"26214400"                   koanf:"max_media_bytes"`
}

/*
MetaServiceConfig is the install's Meta app as Messenger and Instagram
use it: its id, the secret Meta signs every delivery with, and the token
Meta must echo when it checks the webhook address. An install that set
these for WhatsApp need not repeat them: the same app serves all three.
*/
type MetaServiceConfig struct {
	AppID       string `koanf:"app_id"`
	AppSecret   string `koanf:"app_secret"   secret:"true"`
	VerifyToken string `koanf:"verify_token" secret:"true"`
}

/*
Configured reports whether the Meta app can connect accounts.
*/
func (c MetaServiceConfig) Configured() bool {
	return c.AppID != "" && c.AppSecret != ""
}

/*
SocialServiceConfig is one platform's login on the Meta app: ConfigID
is the Facebook Login for Business configuration the connect popup
opens. Empty means the platform cannot be connected.
*/
type SocialServiceConfig struct {
	ConfigID string `koanf:"config_id"`
}

/*
MetaApp returns the Meta app Messenger and Instagram connect through:
the one set for them, or the one set for WhatsApp.
*/
func (c ServicesConfig) MetaApp() MetaServiceConfig {
	if c.Meta.Configured() {
		return c.Meta
	}

	return MetaServiceConfig{
		AppID: c.WhatsApp.AppID, AppSecret: c.WhatsApp.AppSecret, VerifyToken: c.WhatsApp.VerifyToken,
	}
}

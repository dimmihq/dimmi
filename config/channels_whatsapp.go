/*
|--------------------------------------------------------------------------
| WhatsApp Channel Settings
|--------------------------------------------------------------------------
|
| How the WhatsApp channel behaves on this install. Meta retries a
| delivery for up to seven days and sends bodies of up to 3 MB, so a
| delivery seen within the dedupe window is known again without a
| database read. Every Graph call goes to GraphURL at one pinned
| version, so a new Graph version never changes what the channel sends
| until the install moves to it. Templates are read from Meta again on
| the sync interval, besides every time Meta reports a change.
|
*/

package config

import "time"

/*
WhatsAppChannelConfig configures the WhatsApp channel.

GraphURL is the Graph API host, Meta's own unless a test or a proxy
needs another, and GraphVersion the version every call names.
MaxPayloadBytes bounds one delivery and DedupeWindow is how long a
delivered change is recognized quickly. Window is how long after a
customer's last message free text may still be sent, 24 hours on Meta.
MaxMediaBytes is the largest file the inbox downloads from a message.
*/
type WhatsAppChannelConfig struct {
	GraphURL        string        `default:"https://graph.facebook.com" koanf:"graph_url"`
	GraphVersion    string        `default:"v25.0"                      koanf:"graph_version"`
	MaxPayloadBytes int64         `default:"3145728"                    koanf:"max_payload_bytes"`
	DedupeWindow    time.Duration `default:"1h"                         koanf:"dedupe_window"`
	Window          time.Duration `default:"24h"                        koanf:"window"`
	MaxMediaBytes   int64         `default:"26214400"                   koanf:"max_media_bytes"`
}

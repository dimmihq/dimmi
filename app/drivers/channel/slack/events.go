/*
|--------------------------------------------------------------------------
| Slack Deliveries
|--------------------------------------------------------------------------
|
| What Slack sends, read into plain shapes the rest of the product works
| with: here the Events API envelope around one event, and the
| installations the event is visible to. Only the fields the product
| reads are kept.
|
*/

package slack

import (
	"encoding/json"
	"fmt"
)

/*
The envelope types the Events API sends.
*/
const (
	EnvelopeURLVerification = "url_verification"
	EnvelopeEventCallback   = "event_callback"
)

/*
Envelope is one Events API delivery around an event. Authorizations
are the installations of the app the event is visible to.
*/
type Envelope struct {
	Type               string          `json:"type"`
	Challenge          string          `json:"challenge"`
	TeamID             string          `json:"team_id"`
	APIAppID           string          `json:"api_app_id"`
	EventID            string          `json:"event_id"`
	EventTime          int64           `json:"event_time"`
	EventContext       string          `json:"event_context"`
	IsExtSharedChannel bool            `json:"is_ext_shared_channel"`
	Authorizations     []Authorization `json:"authorizations"`
	Event              json.RawMessage `json:"event"`
}

/*
Authorization is one installation an event is visible to.
*/
type Authorization struct {
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
	IsBot  bool   `json:"is_bot"`
}

/*
ParseEnvelope reads an Events API delivery.
*/
func ParseEnvelope(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return env, fmt.Errorf("read slack envelope: %w", err)
	}

	return env, nil
}

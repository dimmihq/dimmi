/*
|--------------------------------------------------------------------------
| Slack Channel Resource
|--------------------------------------------------------------------------
|
| A Slack connection as the settings page shows it: its status and the
| team it is installed in, and, while its app is being set up, the
| manifest and addresses that set it up. Its tokens never appear.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackChannel is a Slack connection.
*/
type SlackChannel struct {
	ID               string     `json:"id"`
	Object           string     `json:"object"`
	Type             string     `json:"type"`
	Name             string     `json:"name"`
	Status           string     `json:"status"`
	Mode             string     `json:"mode"`
	InboundTransport string     `json:"inbound_transport"`
	ExternalID       *string    `json:"external_id"`
	ExternalName     string     `json:"external_name"`
	Setup            SlackSetup `json:"setup"`
	LastError        *string    `json:"last_error"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

/*
SlackSetup is how the connection's app is set up: the manifest, the
address that creates the app, and the addresses Slack delivers to.
*/
type SlackSetup struct {
	Complete        bool           `json:"complete"`
	Manifest        map[string]any `json:"manifest"`
	CreateAppURL    string         `json:"create_app_url"`
	RequestURL      string         `json:"request_url"`
	InteractionsURL string         `json:"interactions_url"`
}

/*
SlackFacts are what a connection knows of its install, as the resource
shows them.
*/
type SlackFacts struct {
	Mode             string
	InboundTransport string
	TeamName         string
}

/*
NewSlackChannel returns the resource of the connection.
*/
func NewSlackChannel(conn models.ChannelConnection, facts SlackFacts, setup SlackSetup) SlackChannel {
	setup.Complete = conn.Status == "active"

	return SlackChannel{
		ID: models.PublicID(models.PrefixChannelConnection, conn.ID), Object: "channel", Type: conn.Type,
		Name: conn.Name, Status: conn.Status, Mode: facts.Mode, InboundTransport: facts.InboundTransport,
		ExternalID: conn.ExternalID, ExternalName: facts.TeamName, Setup: setup, LastError: conn.LastError,
		CreatedAt: conn.CreatedAt, UpdatedAt: conn.UpdatedAt,
	}
}

/*
|--------------------------------------------------------------------------
| API Channel
|--------------------------------------------------------------------------
|
| Conversations a developer relays through the REST API live on the api
| channel. Nothing is ever pushed anywhere: the developer reads replies
| back through the API or a webhook. The team may start a conversation
| on it at any time, since there is no provider to refuse.
|
*/

package api

import (
	"context"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Type is the connection type the driver serves.
*/
const Type = "api"

/*
Driver is the api channel driver.
*/
type Driver struct{}

/*
Type names the api connection type.
*/
func (Driver) Type() string {
	return Type
}

/*
Capabilities says replies are not sent and the team may start threads.
*/
func (Driver) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	return contracts.ChannelCapabilities{
		Transport: contracts.TransportNone, StartConversation: contracts.StartAlways,
	}
}

/*
Send does nothing: the developer reads the reply through the API.
*/
func (Driver) Send(context.Context, models.ChannelConnection, contracts.OutboundMessage) (contracts.SendResult, error) {
	return contracts.SendResult{}, nil
}

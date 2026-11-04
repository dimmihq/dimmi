/*
|--------------------------------------------------------------------------
| Outbound Delivery
|--------------------------------------------------------------------------
|
| What happens to a reply depends only on what its channel's driver
| says it does. A channel that pushes, such as email, gets the reply
| from a job once the request commits, and the reply waits as pending
| until then; it needs an address for the customer before the reply is
| even written. A live channel shows the reply at once, so it is sent
| on arrival. A channel that sends nothing, such as the API, leaves
| delivery out of it.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Delivery sends replies through the drivers of their channels.
*/
type Delivery struct {
	Channels *channels.Manager
	Queue    *queue.Queue
}

/*
Route is how a reply on a connection travels: the connection, what its
driver can do, and the status the reply starts with.
*/
type Route struct {
	Connection   models.ChannelConnection
	Capabilities contracts.ChannelCapabilities
	Status       string
}

/*
Plan finds the route of a reply to the customer on the connection,
refusing a channel this server cannot send on, and a pushing channel
without an address for the customer.
*/
func (d Delivery) Plan(conn models.ChannelConnection, contact models.Contact) (Route, error) {
	driver, err := d.Channels.Driver(conn.Type)
	if errors.Is(err, channels.ErrUnknownChannel) {
		return Route{}, exceptions.ErrChannelUnsupported
	}

	if err != nil {
		return Route{}, fmt.Errorf("find channel driver: %w", err)
	}

	route := Route{Connection: conn, Capabilities: driver.Capabilities(conn), Status: string(enums.DeliveryStatusNotApplicable)}

	switch route.Capabilities.Transport {
	case contracts.TransportPush:
		if contact.Email == nil {
			return route, exceptions.ErrContactUnreachable
		}

		route.Status = string(enums.DeliveryStatusPending)
	case contracts.TransportRealtime:
		route.Status = string(enums.DeliveryStatusSent)
	case contracts.TransportNone:
	}

	return route, nil
}

/*
Dispatch hands a pending reply to its job, in the transaction that
wrote it, so the job only runs once the reply is committed.
*/
func (d Delivery) Dispatch(ctx context.Context, tx *database.Tx, msg models.Message) error {
	if msg.DeliveryStatus == nil || *msg.DeliveryStatus != string(enums.DeliveryStatusPending) {
		return nil
	}

	err := d.Queue.PushTx(ctx, tx, jobs.SendChannelMessageArgs{
		WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, MessageID: msg.ID,
	})
	if err != nil {
		return fmt.Errorf("queue delivery: %w", err)
	}

	return nil
}

/*
Connection returns the conversation's channel connection.
*/
func Connection(ctx context.Context, q *models.Queries, c models.Conversation) (models.ChannelConnection, error) {
	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: c.WorkspaceID, ID: c.ChannelConnectionID,
	})
	if err != nil {
		return conn, fmt.Errorf("find channel connection: %w", err)
	}

	return conn, nil
}

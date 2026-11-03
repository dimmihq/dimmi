/*
|--------------------------------------------------------------------------
| Fake Channel
|--------------------------------------------------------------------------
|
| Replies on real channels leave the process, so tests send them through
| this driver instead: it pushes like email does, records every message
| it was handed, and fails the way a test scripts, so the inbox's
| handling of each outcome can be watched without a provider.
|
*/

package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Type is the connection type the driver serves.
*/
const Type = "fake"

/*
Driver is the fake channel driver. Failures are the errors the next
sends return, one per send, oldest first.
*/
type Driver struct {
	mu       sync.Mutex
	sent     []contracts.OutboundMessage
	failures []error
}

/*
Type names the fake connection type.
*/
func (d *Driver) Type() string {
	return Type
}

/*
Capabilities says replies are pushed and the team may start threads.
*/
func (d *Driver) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	return contracts.ChannelCapabilities{
		Transport: contracts.TransportPush, StartConversation: contracts.StartAlways,
	}
}

/*
Send records the message, or returns the next scripted failure.
*/
func (d *Driver) Send(
	_ context.Context, _ models.ChannelConnection, msg contracts.OutboundMessage,
) (contracts.SendResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.failures) > 0 {
		err := d.failures[0]
		d.failures = d.failures[1:]

		return contracts.SendResult{}, err
	}

	d.sent = append(d.sent, msg)

	return contracts.SendResult{
		ChannelMessageID: fmt.Sprintf("fake-%d", len(d.sent)), ExternalThreadKey: msg.IdempotencyKey,
	}, nil
}

/*
FailNext makes the next sends fail with the errors, one each.
*/
func (d *Driver) FailNext(errs ...error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.failures = append(d.failures, errs...)
}

/*
Sent returns every message the driver delivered, oldest first.
*/
func (d *Driver) Sent() []contracts.OutboundMessage {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]contracts.OutboundMessage(nil), d.sent...)
}

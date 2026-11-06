/*
|--------------------------------------------------------------------------
| Email Setup Broadcasts
|--------------------------------------------------------------------------
|
| A forwarding test shows its progress live: sent, back, failed or out of
| time. The broadcast names the test, its address and its status and
| nothing more; the page reads the test through the API for the rest.
|
*/

package broadcasting

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailSetup broadcasts the forwarding setup events to the workspace.
*/
type EmailSetup struct {
	Server Publisher
}

/*
testChange names the test a broadcast is about and its new status.
*/
type testChange struct {
	ID           string `json:"id"`
	Object       string `json:"object"`
	EmailAddress string `json:"email_address"`
	Status       string `json:"status"`
}

/*
Listen makes the broadcasts follow every committed test change.
*/
func (b *EmailSetup) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.EmailAddressTestUpdated) error {
		data := testChange{
			ID: models.PublicID(models.PrefixEmailAddressTest, e.TestID), Object: "email_address_test",
			EmailAddress: models.PublicID(models.PrefixEmailAddress, e.AddressID), Status: e.Status,
		}
		channels := []string{WorkspaceChannel(e.Header().WorkspaceID)}

		return (&Broadcaster{Server: b.Server}).publish(ctx, channels, message(e, data))
	})
}

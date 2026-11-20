/*
|--------------------------------------------------------------------------
| WhatsApp Channel Provider
|--------------------------------------------------------------------------
|
| Wires the WhatsApp channel: its driver registered with the inbox, and
| the job that processes Meta's deliveries. Replies' and customers' files
| live on the default disk. The install's own Meta app comes from the
| services configuration; without it numbers connect only through
| workspaces' own Meta apps.
|
*/

package providers

import (
	"fmt"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootWhatsApp builds the WhatsApp channel, registers its driver and job.
*/
func bootWhatsApp(a *app.App) error {
	cfg := a.Config.Channels.WhatsApp
	diskName := a.Application.Config.Filesystems.Default

	disk, err := a.Storage.Disk(diskName)
	if err != nil {
		return fmt.Errorf("whatsapp disk: %w", err)
	}

	places := &channels.WhatsAppPlaces{DB: a.DB, Clock: a.Clock}
	driver := &wadriver.Driver{
		HTTP: a.HTTP, GraphURL: cfg.GraphURL, Version: cfg.GraphVersion, Places: places, Clock: a.Clock,
		FactsOf: channels.WhatsAppFactsOf,
	}

	channel := &channels.WhatsApp{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Cache: a.Cache, Events: a.Events, IDs: a.IDs, Log: a.Log,
		Keys: a.Keys, Driver: driver, Config: cfg, Service: a.Config.Services.WhatsApp,
		AppURL: a.Application.Config.App.URL,
		Receiver: &conversations.ReceiveInboundMessage{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Disk: disk, DiskName: diskName, Queue: a.Queue,
			ReopenWindow: a.Config.Channels.Email.Inbound.ReopenWindow,
		},
	}
	driver.Credentials = channel
	places.Sync = channel.SyncTemplates

	if err := a.Channels.Register(driver); err != nil {
		return err
	}

	a.Actions.WhatsApp = channel

	return queue.Register(a.Queue, &jobs.ProcessWhatsAppEvent{Processor: channel})
}

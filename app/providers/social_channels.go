/*
|--------------------------------------------------------------------------
| Messenger And Instagram Channel Provider
|--------------------------------------------------------------------------
|
| Wires the two channels: one driver and one channel for each platform,
| registered with the inbox, and the job that processes what Meta
| delivers for either.
|
*/

package providers

import (
	"fmt"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootSocial builds the Messenger and Instagram channels and registers
their drivers and job.
*/
func bootSocial(a *app.App) error {
	cfg := a.Config.Channels.Social
	diskName := a.Application.Config.Filesystems.Default

	disk, err := a.Storage.Disk(diskName)
	if err != nil {
		return fmt.Errorf("social disk: %w", err)
	}

	build := func(platform social.Platform) (*channels.Social, error) {
		driver := &social.Driver{
			Platform: platform, HTTP: a.HTTP, GraphURL: cfg.GraphURL, Version: cfg.GraphVersion,
			Places: &channels.SocialPlaces{DB: a.DB, Clock: a.Clock}, Clock: a.Clock,
			FactsOf: channels.SocialFactsOf, Standard: cfg.Window, Human: cfg.HumanWindow,
		}

		login := a.Config.Services.Facebook
		if platform == social.Instagram {
			login = a.Config.Services.Instagram
		}

		channel := &channels.Social{
			Platform: platform, DB: a.DB, Clock: a.Clock, Queue: a.Queue, Cache: a.Cache, IDs: a.IDs, Log: a.Log,
			Keys: a.Keys, Driver: driver, Config: cfg, App: a.Config.Services.MetaApp(), Login: login,
			AppURL: a.Application.Config.App.URL, Pictures: disk,
			Edits: &conversations.InboundEdits{DB: a.DB, Events: a.Events, Clock: a.Clock},
			Receiver: &conversations.ReceiveInboundMessage{
				DB: a.DB, Events: a.Events, Clock: a.Clock, Disk: disk, DiskName: diskName, Queue: a.Queue,
				ReopenWindow: a.Config.Channels.Email.Inbound.ReopenWindow,
			},
		}
		driver.Credentials = channel

		return channel, a.Channels.Register(driver)
	}

	if a.Actions.Facebook, err = build(social.Facebook); err != nil {
		return err
	}

	if a.Actions.Instagram, err = build(social.Instagram); err != nil {
		return err
	}

	return queue.Register(a.Queue, &jobs.ProcessSocialEvent{Processors: map[string]jobs.ChannelEventProcessor{
		string(social.Facebook): a.Actions.Facebook, string(social.Instagram): a.Actions.Instagram,
	}})
}

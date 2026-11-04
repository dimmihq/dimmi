/*
|--------------------------------------------------------------------------
| Boot: Shared State, Jobs and Outside Services
|--------------------------------------------------------------------------
|
| Cache, rate limits, the internal bus and realtime fan-out all live on
| the one Valkey connection, so every replica shares them. The queue only
| pushes until a worker role starts it. Services that reach outside the
| process, such as mail, storage and malware scanning, are built next,
| all sending HTTP through the one guarded client.
|
*/

package foundation

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/malware"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/process"
	"github.com/dimmihq/dimmi/framework/pubsub"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/schedule"
)

/*
leaderQueue is the only queue a scheduler that is not also a worker
listens on. Nothing is pushed to it: the node joins the queue's leader
election to push periodic jobs, and leaves the work to worker nodes.
*/
const leaderQueue = "scheduler"

/*
bootValkeyServices builds the cache, rate limiter, bus, queue, schedule,
events, realtime server and maintenance state.
*/
func (a *Application) bootValkeyServices(ctx context.Context) error {
	var err error

	a.Cache, err = cache.Open(a.Redis, a.Config.Cache)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}

	a.onClose("cache", closeFunc(a.Cache.Close))

	a.Limiter, err = ratelimit.New(a.Redis, a.Config.RateLimit)
	if err != nil {
		return fmt.Errorf("open rate limiter: %w", err)
	}

	a.PubSub = pubsub.New(a.Redis, a.Config.PubSub)

	if err := a.bootQueue(ctx); err != nil {
		return err
	}

	if err := a.bootBroadcasting(ctx); err != nil {
		return err
	}

	store, err := NewMaintenanceStore(log.NewContext(ctx, a.Log), a.DB, a.PubSub, a.Clock)
	if err != nil {
		return err
	}

	a.Maintenance = store
	a.onClose("maintenance", closeFunc(store.Close))

	return nil
}

/*
bootQueue builds the queue, its schedule and the event dispatcher. The
queue logs through the application's logger. A scheduler that is not a
worker listens only on the leader queue, and a worker that is not a
scheduler stays out of leader election, so only scheduler nodes ever
push periodic jobs.
*/
func (a *Application) bootQueue(ctx context.Context) error {
	cfg := a.Config.Queue
	if a.Roles.Has(RoleScheduler) && !a.Roles.Has(RoleWorker) {
		cfg.Queues = map[string]int{leaderQueue: 1}
	}

	cfg.NoLeader = !a.Roles.Has(RoleScheduler)

	q, err := queue.New(log.NewContext(ctx, a.Log), a.Config.Database, cfg)
	if err != nil {
		return fmt.Errorf("open queue: %w", err)
	}

	a.Queue = q
	a.onClose("queue", closeFunc(q.Close))
	a.Schedule = schedule.New(q)
	a.Events = events.New(q)

	return nil
}

/*
bootBroadcasting starts this node's realtime server. Without a resolver
from the application every connection is refused.
*/
func (a *Application) bootBroadcasting(ctx context.Context) error {
	cfg := a.Config.Broadcasting
	cfg.Clock = a.Clock

	cfg.Resolve = a.opts.Broadcast
	if cfg.Resolve == nil {
		cfg.Resolve = func(context.Context, *http.Request, string) (broadcasting.Principal, error) {
			return broadcasting.Principal{}, broadcasting.ErrUnauthenticated
		}
	}

	server, err := broadcasting.Open(log.NewContext(ctx, a.Log), a.Redis, cfg)
	if err != nil {
		return fmt.Errorf("open broadcasting: %w", err)
	}

	a.Broadcast = server
	a.onClose("broadcasting", a.stopBroadcasting())

	return nil
}

/*
stopBroadcasting returns a release that shuts the realtime server down
once, however often it is called: early while stopping the roles, and
again at the end for an application that never started them.
*/
func (a *Application) stopBroadcasting() func(context.Context) error {
	var stopped atomic.Bool

	return func(ctx context.Context) error {
		if !stopped.CompareAndSwap(false, true) {
			return nil
		}

		if err := a.Broadcast.Shutdown(ctx); err != nil {
			return fmt.Errorf("shut down broadcasting: %w", err)
		}

		return nil
	}
}

/*
bootIO builds the translator and the services that reach outside the
process, and registers every built-in driver. Mail and notifications
render in their recipients' locales with the translator.
*/
func (a *Application) bootIO(ctx context.Context) error {
	var err error

	a.HTTP, err = httpclient.New(a.Config.HTTPClient)
	if err != nil {
		return fmt.Errorf("build http client: %w", err)
	}

	a.DNS = dns.New(a.Config.DNS)
	a.Process = process.New()
	a.Storage = filesystem.NewManager(a.Config.Filesystems)
	a.Scanner = malware.NewManager(a.Config.Malware)
	a.Lang, err = i18n.New(
		i18n.Config{Locale: a.Config.App.Locale, Locales: a.Config.App.Locales},
		append(frameworkLanguage(), languageFiles(a.opts.Language))...,
	)
	if err != nil {
		return fmt.Errorf("load translations: %w", err)
	}

	mailConfig := a.Config.Mail
	mailConfig.Translator = a.Lang
	a.Mail = mail.NewManager(mailConfig)
	a.Mailbox = mailbox.NewManager(a.Config.Mailbox)

	notifyConfig := a.Config.Notifications
	notifyConfig.Translator = a.Lang

	a.Notify, err = notifications.New(notifyConfig, a.Queue)
	if err != nil {
		return fmt.Errorf("build notifications: %w", err)
	}

	a.registerDrivers(ctx)

	if err := a.Mail.UseQueue(a.Queue); err != nil {
		return fmt.Errorf("queue mail: %w", err)
	}

	return nil
}

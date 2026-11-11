/*
|--------------------------------------------------------------------------
| Knowledge Sources
|--------------------------------------------------------------------------
|
| The sources a workspace can read knowledge from, each a driver
| registered here at boot, and the engine that syncs and imports them.
| A new source is one driver package and one line in sourceDrivers.
|
*/

package providers

import (
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/knowledgesources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/github"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/intercom"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/notion"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/upload"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/website"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/zendesk"
	appmail "github.com/dimmihq/dimmi/app/mail"
)

/*
sourceDrivers lists the drivers the install offers, in the order the
dashboard's gallery shows them.
*/
func sourceDrivers(a *app.App) []sources.Driver {
	drivers := []sources.Driver{
		siteDriver(a),
		intercom.New(a.HTTP),
		zendesk.New(a.HTTP),
		github.New(a.HTTP),
		notion.New(a.HTTP),
	}

	if disk, err := a.Storage.Default(); err == nil {
		drivers = append(drivers, upload.New(disk))
	}

	return drivers
}

/*
siteDriver is the website source, crawling through Cloudflare when the
install has a Cloudflare token and with the built-in crawler otherwise.
*/
func siteDriver(a *app.App) *website.Driver {
	cfg := a.Config.AI.Knowledge
	d := website.New(website.NewBuiltin(a.HTTP, a.Clock, time.Duration(cfg.CrawlIntervalMS)*time.Millisecond))

	if cfg.CloudflareAPIToken == "" {
		return d
	}

	return d.Through(&website.Cloudflare{
		HTTP: a.HTTP, Clock: a.Clock, Token: cfg.CloudflareAPIToken, AccountID: cfg.CloudflareAccountID,
	})
}

/*
bootKnowledgeSources builds the engine, its drivers registered with
the jobs.
*/
func bootKnowledgeSources(a *app.App) {
	registry := sources.NewRegistry()

	acts := a.Actions
	engine := &knowledgesources.Engine{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Events: a.Events, Keys: a.Keys, Drivers: registry,
		Vectors: a.Vectors, Articles: acts.Articles, Collections: acts.Collections, Images: acts.HelpImages,
		HTTP: a.HTTP, Mail: a.Mail, Links: a.Links, IDs: a.IDs,
		BrowserHourMicros: a.Config.AI.Knowledge.CloudflareBrowserHourMicros,
	}

	if disk, err := a.Storage.Default(); err == nil {
		engine.Uploads = disk
	}

	acts.KnowledgeSources = engine
	appmail.RegisterKnowledgeNotices(a.Mail)
}

/*
registerKnowledgeSourceJobs registers the drivers and the sync and
import workers.
*/
func registerKnowledgeSourceJobs(a *app.App) error {
	e := a.Actions.KnowledgeSources
	if e == nil {
		return nil
	}

	if a.Actions.Integrations != nil {
		e.Connections = a.Actions.Integrations.Tokens
	}

	var errs []error
	for _, d := range sourceDrivers(a) {
		errs = append(errs, e.Drivers.Register(d))
	}

	return errors.Join(
		errors.Join(errs...),
		queue.Register(a.Queue, &knowledgesources.DiscoverWorker{Engine: e}),
		queue.Register(a.Queue, &knowledgesources.FetchWorker{Engine: e}),
		queue.Register(a.Queue, &knowledgesources.FinishWorker{Engine: e}),
		queue.Register(a.Queue, &knowledgesources.ImportWorker{Engine: e}),
		queue.Register(a.Queue, &knowledgesources.ScheduleWorker{Engine: e}),
	)
}

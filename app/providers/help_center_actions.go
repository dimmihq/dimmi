/*
|--------------------------------------------------------------------------
| Help Center Wiring
|--------------------------------------------------------------------------
|
| The actions behind a workspace's public docs and its changelog, and
| the jobs that publish scheduled entries and mail them out.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
bootHelpCenters builds the help center actions.
*/
func bootHelpCenters(a *app.App) {
	acts := a.Actions
	acts.HelpCenters = &helpcenters.HelpCenters{DB: a.DB, Clock: a.Clock}
	acts.Collections = &helpcenters.Collections{DB: a.DB, Clock: a.Clock}
	acts.Articles = helpcenters.NewArticles(a.DB, a.Clock)
	acts.HelpImages = &helpcenters.Images{Disks: a.Storage}
	acts.HelpPublic = helpcenters.NewPublic(a.DB, a.Clock)
	acts.HelpReaders = &helpcenters.Readers{
		DB: a.DB, Clock: a.Clock, Verifier: bootIdentityVerifier(a), Events: a.Events,
		Cache: a.Cache, Limiter: a.Limiter, Mail: a.Mail, Key: a.DerivedKey("help.readers"),
	}
	acts.HelpCenterAddresses = helpcenters.Addresses{
		BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: a.Application.Config.App.URL,
	}
	acts.ChangelogEntries = changelog.NewEntries(a.DB, a.Clock, a.Queue)
	acts.ChangelogSubscribers = &changelog.Subscribers{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Key: a.DerivedKey("changelog.subscribers"),
		Addresses: acts.HelpCenterAddresses,
	}
	acts.ChangelogPublic = &changelog.Public{DB: a.DB, Clock: a.Clock, Key: a.DerivedKey("changelog.reactions")}
}

/*
registerChangelogJobs runs scheduled publishes and mailings.
*/
func registerChangelogJobs(a *app.App) error {
	return errors.Join(
		queue.Register(a.Queue, &changelog.PublishWorker{Entries: a.Actions.ChangelogEntries}),
		queue.Register(a.Queue, &changelog.MailWorker{Subscribers: a.Actions.ChangelogSubscribers}),
	)
}

/*
registerHelpCenters serves help centers on their own subdomains of the
help domain, when the install has one.
*/
func registerHelpCenters(a *app.App, fa *foundation.Application) {
	if a.Config.HelpCenters.BaseDomain == "" {
		return
	}
	fa.Kernel.Use(web.HostedHelpCenters(helpcenters.Addresses{
		BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: fa.Config.App.URL,
	}))
}

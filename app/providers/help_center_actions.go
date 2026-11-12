/*
|--------------------------------------------------------------------------
| Help Center Wiring
|--------------------------------------------------------------------------
|
| The actions behind a workspace's public docs, its changelog and its
| feature requests, and
| the jobs that publish scheduled entries and mail them out.
|
*/

package providers

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/changelog"
	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootHelpCenters builds the help center actions and the knowledge
index their articles feed.
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
	acts.HelpRatings = &helpcenters.Ratings{
		DB: a.DB, Clock: a.Clock, Limiter: a.Limiter, Key: a.DerivedKey("help.ratings"),
	}
	acts.HelpCenterAddresses = helpcenters.Addresses{
		BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: a.Application.Config.App.URL,
	}
	acts.ChangelogEntries = changelog.NewEntries(a.DB, a.Clock, a.Queue)
	acts.ChangelogEntries.Events = a.Events
	acts.ChangelogSubscribers = &changelog.Subscribers{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Key: a.DerivedKey("changelog.subscribers"),
		Addresses: acts.HelpCenterAddresses,
	}
	acts.ChangelogPublic = &changelog.Public{DB: a.DB, Clock: a.Clock, Key: a.DerivedKey("changelog.reactions")}
	acts.Feedback = feedback.NewPosts(a.DB, a.Clock, a.Mail, acts.HelpCenterAddresses)
	acts.Feedback.Events = a.Events
	bootHelpCenterDomains(a)
	bootKnowledge(a)
}

/*
registerChangelogJobs runs scheduled publishes and mailings, and the
embedding jobs of the knowledge index the help centers feed.
*/
func registerChangelogJobs(a *app.App) error {
	return errors.Join(
		queue.Register(a.Queue, &changelog.PublishWorker{Entries: a.Actions.ChangelogEntries}),
		queue.Register(a.Queue, &changelog.MailWorker{Subscribers: a.Actions.ChangelogSubscribers}),
		registerKnowledgeJobs(a),
		queue.Register(a.Queue, &jobs.CheckHelpCenterDomain{Domains: a.Actions.HelpDomains}),
		queue.Register(a.Queue, &jobs.ScheduleHelpCenterDomainChecks{DB: a.DB, Queue: a.Queue, Clock: a.Clock}),
		queue.Register(a.Queue, &jobs.PruneHelpAnalytics{Analytics: a.Actions.HelpAnalytics}),
	)
}

/*
registerHelpCenters serves help centers on their own subdomains of the
help domain, when the install has one, and on workspaces' verified
custom domains.
*/
func registerHelpCenters(a *app.App, fa *foundation.Application) {
	custom := func(ctx context.Context, host string) string {
		if a.Actions == nil || a.Actions.HelpDomainHosts == nil {
			return ""
		}
		return a.Actions.HelpDomainHosts.Subdomain(ctx, host)
	}
	fa.Kernel.Use(web.HostedHelpCenters(helpcenters.Addresses{
		BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: fa.Config.App.URL,
	}, custom))
}

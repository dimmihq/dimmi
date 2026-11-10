/*
|--------------------------------------------------------------------------
| Integrations
|--------------------------------------------------------------------------
|
| The tools a workspace connects, wired before the channels and features
| that use them: the registry of providers with the scopes each feature
| needs, the install and grant flows, the token service, the sweep that
| checks every installation's health, and the listener that takes a
| departed teammate's grants back.
|
*/

package providers

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/integrations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootIntegrations builds the integrations' actions, listener and job.
*/
func bootIntegrations(a *app.App) error {
	registry := oauth.NewRegistry()
	oauth.RegisterSlack(registry, a.HTTP)

	s := &integrations.Services{
		DB: a.DB, Keys: a.Keys, Clock: a.Clock, Events: a.Events, OAuth: a.OAuthClient, Cache: a.Cache,
		Registry: registry, Audit: a.Actions.RecordAuditLog, Config: a.Config.Integrations,
	}
	grants := &integrations.CompleteMemberGrant{Services: s}

	acts := &app.IntegrationActions{
		Registry:          registry,
		Tokens:            &integrations.Tokens{Services: s},
		List:              &integrations.ListInstallations{Services: s},
		StartInstallation: &integrations.StartInstallation{Services: s},
		CompleteCallback: &integrations.CompleteCallback{
			Services: s, Resolve: a.Actions.ResolveWorkspace, Grants: grants,
		},
		CreateManual:      &integrations.CreateManualInstallation{Services: s},
		StartMemberGrant:  &integrations.StartMemberGrant{Services: s},
		RevokeMemberGrant: &integrations.RevokeMemberGrant{Services: s},
		Disconnect:        &integrations.DisconnectInstallation{Services: s},
		ApplyRevocation:   &integrations.ApplyProviderRevocation{Services: s},
		CheckHealth:       &integrations.CheckInstallationHealth{Services: s},
		RevokeDeparted:    &integrations.RevokeDepartedMember{Services: s},
	}
	a.Actions.Integrations = acts

	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.MemberDeactivated) error {
		return acts.RevokeDeparted.Handle(context.WithoutCancel(ctx), e.WorkspaceID, e.MemberID)
	})

	return queue.Register(a.Queue, &jobs.CheckIntegrationHealth{Check: acts.CheckHealth})
}

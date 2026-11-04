/*
|--------------------------------------------------------------------------
| Email Sending Broadcasts
|--------------------------------------------------------------------------
|
| A sending domain's page refreshes itself when a check lands or the
| domain changes state, so nobody reloads while waiting for DNS. The
| broadcast names the domain and nothing more: the page reads the domain
| again through the API, which decides what the viewer may see.
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
EmailSending broadcasts the email sending events to the workspace.
*/
type EmailSending struct {
	Server Publisher
}

/*
domainChange names the domain a broadcast is about, and its new state
when it changed.
*/
type domainChange struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason,omitempty"`
}

/*
Listen makes the broadcasts follow every committed domain change.
*/
func (b *EmailSending) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.EmailDomainStatusChanged) error {
		return b.domain(ctx, e, domainChange{
			ID: models.PublicID(models.PrefixEmailDomain, e.DomainID), From: e.From, To: e.To, Reason: e.Reason,
		})
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.EmailDomainRecordsUpdated) error {
		return b.domain(ctx, e, domainChange{ID: models.PublicID(models.PrefixEmailDomain, e.DomainID)})
	})
}

/*
domain publishes a domain change on the workspace's channel.
*/
func (b *EmailSending) domain(ctx context.Context, e appevents.Named, data domainChange) error {
	data.Object = "email_domain"
	channels := []string{WorkspaceChannel(e.Header().WorkspaceID)}

	return (&Broadcaster{Server: b.Server}).publish(ctx, channels, message(e, data))
}

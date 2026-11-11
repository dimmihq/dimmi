/*
|--------------------------------------------------------------------------
| Outbound Webhooks
|--------------------------------------------------------------------------
|
| A workspace's owners and admins register endpoints; every event an
| endpoint subscribes to is snapshotted once, stored as the exact bytes
| that get signed, and posted until the endpoint answers 2xx or a day
| of retries runs out. Reading endpoints and their log takes
| webhooks:read, anything that changes them or sends takes
| webhooks:write. An endpoint is the workspace's, not the member's who
| made it, so it outlives them.
|
*/

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
APIVersion is the payload version every endpoint is pinned to today,
the same date the REST API's version header uses.
*/
const APIVersion = "2026-11-01"

/*
Webhooks manages endpoints and delivers events to them.

Snapshots renders the objects payloads carry. DNS resolves endpoint
hosts when they are added. Entitlements caps the
endpoints of a workspace, Retention may hold log rows past their
retention, and Limiter paces requests to one endpoint and test sends.
AppURL is this server's own address, never a valid endpoint.
*/
type Webhooks struct {
	DB           *database.DB
	Clock        clock.Clock
	Queue        *queue.Queue
	Keys         *encryption.Keys
	HTTP         *httpclient.Client
	DNS          dns.Resolver
	Mail         *mail.Manager
	Links        auth.Links
	Audit        *audit.RecordAuditLog
	Entitlements contracts.Entitlements
	Retention    contracts.RetentionGuard
	Limiter      *ratelimit.Limiter
	Snapshots    Snapshots
	Config       config.WebhooksConfig
	Edition      config.Edition
	AppURL       string
}

/*
Snapshot is an object as a payload carries it. Object is the API's
shape of it, nil when it no longer exists. ContactID is the contact it
is about, InboxID the inbox of its conversation, and Note marks an
internal note. Skip marks an object that never leaves as a webhook,
such as a system entry on a timeline.
*/
type Snapshot struct {
	Object    any
	ContactID *ids.UUID
	InboxID   *ids.UUID
	Note      bool
	Skip      bool
}

/*
Snapshots renders the objects webhook payloads carry, in the API's
shapes.
*/
type Snapshots interface {
	/*
		Snapshot renders the object of the kind, such as conversation,
		message, contact or tag.
	*/
	Snapshot(ctx context.Context, workspaceID ids.UUID, kind string, id ids.UUID) (Snapshot, error)
}

/*
requireRead checks the caller may see endpoints and their log.
*/
func requireRead(m authz.Membership) error {
	if !m.Can("webhooks:read") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
requireWrite checks the caller may change endpoints and send to them.
*/
func requireWrite(m authz.Membership) error {
	if !m.Can("webhooks:write") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
inWorkspace runs fn in a transaction of the workspace.
*/
func (w *Webhooks) inWorkspace(
	ctx context.Context, ws ids.UUID, fn func(ctx context.Context, tx *database.Tx, q *models.Queries) error,
) error {
	return w.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, tx, models.New(tx))
	})
}

/*
findEndpoint returns one endpoint, not found for an id of another
workspace.
*/
func findEndpoint(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.WebhookEndpoint, error) {
	e, err := q.FindWebhookEndpoint(ctx, models.FindWebhookEndpointParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return e, exceptions.ErrNotFound
	}

	if err != nil {
		return e, fmt.Errorf("find webhook endpoint: %w", err)
	}

	return e, nil
}

/*
Host returns the host of an endpoint's URL, all that logs and mails
show of it: paths and queries can hold the receiver's tokens.
*/
func Host(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return u.Host
}

/*
EndpointPublicID returns an endpoint's public id.
*/
func EndpointPublicID(id ids.UUID) string {
	return models.PublicID(models.PrefixWebhookEndpoint, id)
}

/*
|--------------------------------------------------------------------------
| Integration Services
|--------------------------------------------------------------------------
|
| What every integration action shares: the database, the data keys that
| seal tokens, the account connection flow, the registry of providers
| and their scopes, and the audit log. Tokens are sealed bound to the
| row that holds them, so a sealed value copied to another row does not
| open. Nothing here ever returns a token to a caller other than the
| token service.
|
*/

package integrations

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
ManagePermission is what connecting, upgrading and disconnecting an
integration takes.
*/
const ManagePermission = "integrations:manage"

/*
DefaultReturnTo is where the browser lands after a connection when the
caller named no page.
*/
const DefaultReturnTo = "/d/settings/integrations"

/*
The statuses of an installation and of a member grant.
*/
const (
	StatusActive       = "active"
	StatusError        = "error"
	StatusDisconnected = "disconnected"
	StatusRevoked      = "revoked"
)

/*
The sources of an installation: an OAuth install, or tokens pasted from
a custom app.
*/
const (
	SourceOAuth  = "oauth"
	SourceManual = "manual"
)

/*
The audit events of integrations.
*/
const (
	AuditConnected     = "integration.connected"
	AuditScopesChanged = "integration.scopes_changed"
	AuditDisconnected  = "integration.disconnected"
	AuditGrantCreated  = "integration.member_grant_created"
	AuditGrantReplaced = "integration.member_grant_replaced"
	AuditGrantRevoked  = "integration.member_grant_revoked"
)

/*
Services holds what the integration actions share. Config is the
integrations section of the configuration.
*/
type Services struct {
	DB       *database.DB
	Keys     *encryption.Keys
	Clock    clock.Clock
	Events   *events.Dispatcher
	OAuth    *oauthclient.Manager
	Cache    *cache.Store
	Registry *oauth.Registry
	Audit    *audit.RecordAuditLog
	Config   appconfig.IntegrationsConfig
}

/*
Enabled reports whether this install offers the provider at all.
*/
func (s *Services) Enabled(provider string) bool {
	_, registered := s.Registry.Provider(provider)

	return registered && slices.Contains(s.Config.Enabled, provider)
}

/*
Installable reports whether the provider can be installed over OAuth,
which needs its app's credentials.
*/
func (s *Services) Installable(provider string) bool {
	return s.Enabled(provider) && s.OAuth.Has(provider)
}

/*
provider returns the enabled provider of the name.
*/
func (s *Services) provider(name string) (oauth.Provider, error) {
	p, ok := s.Registry.Provider(name)
	if !ok || !s.Enabled(name) {
		return oauth.Provider{}, exceptions.ErrIntegrationUnavailable
	}

	return p, nil
}

/*
inWorkspace runs the work in one transaction of the workspace.
*/
func (s *Services) inWorkspace(ctx context.Context, workspaceID ids.UUID, fn work) error {
	scoped := tenancy.With(ctx, workspaceID.String())

	return s.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx), tx)
	})
}

/*
work is what runs in a workspace's transaction, with the transaction's
context, its queries and the transaction itself for dispatching events.
*/
type work func(ctx context.Context, q *models.Queries, tx *database.Tx) error

/*
base is the header of an event about the workspace, raised by the
member, or by the system for ids.Nil().
*/
func (s *Services) base(workspaceID, memberID ids.UUID) appevents.Base {
	actor := appevents.Actor{Type: "system", ID: ids.Nil()}
	if memberID != ids.Nil() {
		actor = appevents.Actor{Type: "member", ID: memberID}
	}

	return appevents.Base{ID: ids.New(), WorkspaceID: workspaceID, OccurredAt: s.Clock.Now(), Actor: actor}
}

/*
note is an audit entry about an installation: who acted, ids.Nil() for
the system or the provider, the event and what changed.
*/
type note struct {
	actor   ids.UUID
	event   string
	changes map[string]any
}

/*
record writes an audit entry about the installation.
*/
func (s *Services) record(ctx context.Context, row models.IntegrationInstallation, n note) error {
	_, err := s.Audit.Handle(ctx, audit.Entry{
		WorkspaceID: row.WorkspaceID, Event: n.event, ActorID: n.actor, Changes: n.changes,
		Subject: audit.Subject{Type: "integration", ID: row.ID, Label: row.Name},
	})

	return err
}

/*
revokeQuietly gives the token back to the provider when it has one,
best effort: a provider that cannot be reached keeps a token that no
longer opens anything here.
*/
func (s *Services) revokeQuietly(ctx context.Context, provider, token string) {
	if token == "" || !s.OAuth.Has(provider) {
		return
	}

	_ = s.OAuth.Revoke(context.WithoutCancel(ctx), provider, token)
}

/*
requireManage checks the member may manage integrations.
*/
func requireManage(m authz.Membership) error {
	if !m.Can(ManagePermission) {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
ReturnTo checks a page to come back to: a path on this site, never
another origin. Empty means the integrations settings.
*/
func ReturnTo(raw string) (string, error) {
	if raw == "" {
		return DefaultReturnTo, nil
	}

	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, `\`) ||
		u.Scheme != "" || u.Host != "" || u.User != nil {
		return "", exceptions.ErrIntegrationReturnTo
	}

	return raw, nil
}

/*
Landing returns the page with the query values added.
*/
func Landing(returnTo string, values url.Values) string {
	u, err := url.Parse(returnTo)
	if err != nil {
		u = &url.URL{Path: DefaultReturnTo}
	}

	query := u.Query()
	maps.Copy(query, values)

	u.RawQuery = query.Encode()

	return u.String()
}

/*
findInstallation returns a live installation of the workspace, not found
otherwise.
*/
func findInstallation(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID,
) (models.IntegrationInstallation, error) {
	row, err := q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
		WorkspaceID: workspaceID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find integration installation: %w", err)
	}

	return row, nil
}

/*
expiry returns a token's expiry, nil when it has none.
*/
func expiry(t *oauthclient.Token) *time.Time {
	if t == nil || t.Expiry.IsZero() {
		return nil
	}

	at := t.Expiry

	return &at
}

/*
value returns a token's secret, empty for none.
*/
func value(t *oauthclient.Token) string {
	if t == nil {
		return ""
	}

	return t.Value
}

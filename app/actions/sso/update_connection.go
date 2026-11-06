/*
|--------------------------------------------------------------------------
| Update Connection
|--------------------------------------------------------------------------
|
| A connection only signs people in once it was tested, a verified domain
| routes to it and the install is served over HTTPS. Changing how it
| talks to the identity provider sends it back to draft, unenforced, so a
| typo is caught by the next test instead of locking everyone out.
| Enforcing it takes proof that owners can still get in: the caller
| signed in through it, or someone tested it within a day and the caller
| confirms it. Whenever which connections are active or enforced changes,
| every workspace of the organization learns it in the same transaction.
|
*/

package sso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
testedWindow is how recent a test must be to enforce a connection the
caller did not sign in through.
*/
const testedWindow = 24 * time.Hour

/*
ConnectionChange is what an update changes; a nil field keeps its value.
DefaultCustomRole points at nil to clear the custom role new members
start with.
ConfirmOwnersCanSignIn is the caller's word that owners can sign in
through a connection tested within the day.
*/
type ConnectionChange struct {
	Name                   *string
	Config                 *Config
	ClientSecret           *string
	Status                 *string
	Enforced               *bool
	JITEnabled             *bool
	DefaultRole            *string
	DefaultTeamIDs         *[]ids.UUID
	DefaultCustomRole      **ids.UUID
	GroupRoleMap           *[]GroupRole
	ConfirmOwnersCanSignIn bool
}

/*
UpdateConnectionInput is who changes a connection, through their
membership, the connection and the change.
*/
type UpdateConnectionInput struct {
	Principal    *appauth.Principal
	Membership   authz.Membership
	ConnectionID ids.UUID
	Change       ConnectionChange
}

/*
UpdateConnection changes a connection of the organization of the
workspace in play.

Rules:
  - The caller needs sso:manage.
  - Connections of other organizations are not found.
  - A change to the protocol settings or the secret sends a connection
    that is not disabled back to draft, unenforced and untested.
  - Active needs a test, a verified domain routed to it and HTTPS;
    disabled comes from active and stops enforcing; nothing else may be
    set by hand.
  - Enforcing needs an active connection, and a caller who signed in
    through it or confirms a test from within the last day.

Events: [appevents.SSOConnectionChanged] for each kind of change.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidTransition], [exceptions.ErrNoVerifiedDomain],
[exceptions.ErrHTTPSRequired], [exceptions.ErrConnectionNotActive],
[exceptions.ErrEnforceUnproven], [exceptions.ErrInvalidConnectionConfig],
or a wrapped error.
*/
type UpdateConnection struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Keys   encryption.KeyManager
	Links  appauth.Links
}

/*
editing is a connection being changed: the transaction, the locked row
and its decoded settings, and the request.
*/
type editing struct {
	q    *models.Queries
	tx   *database.Tx
	row  models.SSOConnection
	conn Connection
	in   UpdateConnectionInput
}

/*
Handle applies the change and returns the connection as it now is.
*/
func (a *UpdateConnection) Handle(ctx context.Context, in UpdateConnectionInput) (Connection, error) {
	if !in.Membership.Can(managePermission) {
		return Connection{}, exceptions.ErrForbidden
	}

	var out Connection

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := lockConnection(ctx, q, in.Membership.Workspace.OrganizationID, in.ConnectionID)
		if err != nil {
			return err
		}

		conn, err := Decode(row)
		if err != nil {
			return err
		}

		out, err = a.apply(ctx, editing{q: q, tx: tx, row: row, conn: conn, in: in})

		return err
	})

	return out, err
}

/*
apply works out the connection's next state, writes it and announces
what changed.
*/
func (a *UpdateConnection) apply(ctx context.Context, e editing) (Connection, error) {
	next, err := a.next(ctx, e)
	if err != nil {
		return Connection{}, err
	}

	if err := a.transition(ctx, e, &next); err != nil {
		return Connection{}, err
	}

	updated, err := e.q.UpdateSSOConnection(ctx, next)
	if err != nil {
		return Connection{}, fmt.Errorf("update connection: %w", err)
	}

	if updated.Status != e.row.Status || updated.Enforced != e.row.Enforced {
		if err := Mirror(ctx, e.q, updated.OrganizationID, a.Clock.Now()); err != nil {
			return Connection{}, err
		}
	}

	if err := a.announce(ctx, e, updated); err != nil {
		return Connection{}, err
	}

	return Decode(updated)
}

/*
next returns the row with the plain fields changed and, when the
protocol settings or the secret changed, back in draft.
*/
func (a *UpdateConnection) next(ctx context.Context, e editing) (models.UpdateSSOConnectionParams, error) {
	row, c := e.row, e.in.Change
	next := models.UpdateSSOConnectionParams{
		OrganizationID: row.OrganizationID, ID: row.ID, Name: row.Name, Status: row.Status, Config: row.Config,
		SecretEncrypted: row.SecretEncrypted, JitEnabled: row.JitEnabled, DefaultRole: row.DefaultRole,
		DefaultTeamIds: row.DefaultTeamIds, GroupRoleMap: row.GroupRoleMap, Enforced: row.Enforced,
		DefaultCustomRoleID: row.DefaultCustomRoleID,
		TestedAt:            row.TestedAt, Now: a.Clock.Now(),
	}

	if c.Name != nil {
		next.Name = str.Clean(*c.Name)
	}

	if c.JITEnabled != nil {
		next.JitEnabled = *c.JITEnabled
	}

	if c.DefaultRole != nil {
		next.DefaultRole = *c.DefaultRole
	}

	if c.DefaultCustomRole != nil {
		if err := a.ownRole(ctx, e, *c.DefaultCustomRole); err != nil {
			return next, err
		}

		next.DefaultCustomRoleID = *c.DefaultCustomRole
	}

	if c.DefaultTeamIDs != nil {
		next.DefaultTeamIds = *c.DefaultTeamIDs
	}

	if c.GroupRoleMap != nil {
		raw, err := json.Marshal(*c.GroupRoleMap)
		if err != nil {
			return next, fmt.Errorf("write group roles: %w", err)
		}

		next.GroupRoleMap = raw
	}

	return a.reconnect(ctx, e, next)
}

/*
ownRole refuses a custom role that is not one of the workspace's.
*/
func (a *UpdateConnection) ownRole(ctx context.Context, e editing, role *ids.UUID) error {
	if role == nil {
		return nil
	}

	_, err := e.q.FindRole(ctx, models.FindRoleParams{WorkspaceID: e.in.Membership.Workspace.ID, ID: *role})
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("find default role: %w", err)
	}

	return nil
}

/*
reconnect applies new protocol settings or a new secret, sending the
connection back to draft when they differ from what it was tested with.
*/
func (a *UpdateConnection) reconnect(
	ctx context.Context, e editing, next models.UpdateSSOConnectionParams,
) (models.UpdateSSOConnectionParams, error) {
	c, cfg := e.in.Change, e.conn.Config
	changed := false

	if c.Config != nil {
		incoming := *c.Config
		incoming.SPCertificate = cfg.SPCertificate
		changed = !sameProtocolSettings(cfg, incoming)
		cfg = incoming

		raw, err := json.Marshal(cfg)
		if err != nil {
			return next, fmt.Errorf("write connection config: %w", err)
		}

		next.Config = raw
	}

	if c.ClientSecret != nil && *c.ClientSecret != "" {
		secret, err := OpenSecret(ctx, a.Keys, e.row)
		if err != nil {
			return next, err
		}

		secret.ClientSecret, changed = *c.ClientSecret, true

		if next.SecretEncrypted, err = seal(ctx, a.Keys, e.row, secret); err != nil {
			return next, err
		}
	}

	if err := checkConfig(e.row.Protocol, cfg, secretOf(next.SecretEncrypted)); err != nil {
		return next, err
	}

	if changed {
		next.Enforced, next.TestedAt = false, nil
		if next.Status != StatusDisabled {
			next.Status = StatusDraft
		}
	}

	return next, nil
}

/*
transition applies the status and enforcement asked for, refusing what
the connection's state does not allow.
*/
func (a *UpdateConnection) transition(ctx context.Context, e editing, next *models.UpdateSSOConnectionParams) error {
	c := e.in.Change

	if c.Status != nil && *c.Status != next.Status {
		if err := a.status(ctx, e, next, *c.Status); err != nil {
			return err
		}
	}

	if c.Enforced == nil || *c.Enforced == next.Enforced {
		return nil
	}

	if !*c.Enforced {
		next.Enforced = false

		return nil
	}

	if next.Status != StatusActive {
		return exceptions.ErrConnectionNotActive
	}

	recent := next.TestedAt != nil && a.Clock.Now().Sub(*next.TestedAt) <= testedWindow
	proven := e.in.Principal.SSOConnectionID == e.row.ID || (recent && c.ConfirmOwnersCanSignIn)
	if !proven {
		return exceptions.ErrEnforceUnproven
	}

	next.Enforced = true

	return nil
}

/*
status moves the connection to the status asked for.
*/
func (a *UpdateConnection) status(
	ctx context.Context, e editing, next *models.UpdateSSOConnectionParams, to string,
) error {
	switch {
	case to == StatusDisabled && next.Status == StatusActive:
		next.Status, next.Enforced = StatusDisabled, false

		return nil
	case to == StatusActive && (next.Status == StatusTested || next.Status == StatusDisabled) && next.TestedAt != nil:
		if err := a.activatable(ctx, e); err != nil {
			return err
		}

		next.Status = StatusActive

		return nil
	default:
		return exceptions.ErrInvalidTransition.OnField("status").With("from", next.Status).With("to", to)
	}
}

/*
activatable refuses to activate a connection on a plain HTTP install or
without a verified domain routed to it.
*/
func (a *UpdateConnection) activatable(ctx context.Context, e editing) error {
	if !a.Links.Secure() {
		return exceptions.ErrHTTPSRequired
	}

	count, err := e.q.CountVerifiedDomainsForConnection(ctx, models.CountVerifiedDomainsForConnectionParams{
		OrganizationID: e.row.OrganizationID, SsoConnectionID: &e.row.ID,
	})
	if err != nil {
		return fmt.Errorf("count routed domains: %w", err)
	}

	if count == 0 {
		return exceptions.ErrNoVerifiedDomain
	}

	return nil
}

/*
announce dispatches what the update changed: the settings, then the
status, then enforcement.
*/
func (a *UpdateConnection) announce(ctx context.Context, e editing, updated models.SSOConnection) error {
	fields := fieldChanges(e.row, updated)
	if len(fields) == 0 {
		return nil
	}

	changes := []string{}

	if updated.Status != e.row.Status && updated.Status == StatusActive {
		changes = append(changes, "activated")
	}

	if updated.Enforced != e.row.Enforced {
		changes = append(changes, map[bool]string{true: "enforced", false: "unenforced"}[updated.Enforced])
	}

	if len(changes) == 0 {
		changes = append(changes, "updated")
	}

	for _, change := range changes {
		err := a.Events.Dispatch(ctx, e.tx, appevents.SSOConnectionChanged{
			WorkspaceID: e.in.Membership.Workspace.ID, ConnectionID: updated.ID, Name: updated.Name, Change: change,
			Changes: fields, ChangedBy: e.in.Membership.Member.ID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

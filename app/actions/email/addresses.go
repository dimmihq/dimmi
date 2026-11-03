/*
|--------------------------------------------------------------------------
| Email Addresses
|--------------------------------------------------------------------------
|
| A workspace receives email through forwarding addresses on the
| install's inbound domain: its support address forwards there, and
| replies go back out from it. Each address is one email channel
| connection in an inbox, created together in one transaction.
|
| A new workspace gets its first address the moment it exists, named
| after its slug, so there is something to forward to before anyone
| opens the settings. A forwarding address is never handed out twice,
| deleted addresses included, so a taken one gets a number after it.
| An install without an inbound domain gives out no addresses at all.
|
*/

package email

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
Bounds of address creation: how many numbered forwarding addresses are
tried, and the longest display name.
*/
const (
	maxForwardingAttempts = 50
	maxDisplayNameLength  = 80
	defaultInboxName      = "Support"
)

/*
Unique constraints a new address can run into.
*/
const (
	forwardingTakenConnection = "channel_connections_type_external_id_key"
	forwardingTakenAddress    = "email_addresses_forwarding_address_key"
	publicAddressTaken        = "email_addresses_workspace_id_address_key"
)

/*
newAddress is an address about to be created: its public address and
name, the inbox it feeds, the local part its forwarding address starts
from, the forwarding address being tried, and whether it becomes the
workspace's default.
*/
type newAddress struct {
	workspaceID ids.UUID
	inboxID     ids.UUID
	address     string
	displayName string
	local       string
	forwarding  string
	isDefault   bool
}

/*
EnsureDefaultEmailAddress gives a new workspace its first forwarding
address, named after its slug, in a first inbox when the workspace has
none. A workspace that has a default address keeps it, and an install
without an inbound domain is left alone.
*/
type EnsureDefaultEmailAddress struct {
	Clock  clock.Clock
	Config appconfig.EmailConfig
}

/*
Name names the defaults the step creates.
*/
func (a *EnsureDefaultEmailAddress) Name() string {
	return "default_email_address"
}

/*
Initialize creates the default address when the workspace lacks one.
*/
func (a *EnsureDefaultEmailAddress) Initialize(ctx context.Context, tx *database.Tx, ws models.Workspace) error {
	if !a.Config.Receives() {
		return nil
	}

	q := models.New(tx)

	_, err := q.FindDefaultEmailAddress(ctx, ws.ID)
	if err == nil {
		return nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find default email address: %w", err)
	}

	c := creator{clock: a.Clock, config: a.Config}

	inboxID, err := c.firstInbox(ctx, q, ws.ID)
	if err != nil {
		return err
	}

	local := inbound.DefaultLocal(ws.Slug)
	_, err = c.create(ctx, tx, newAddress{
		workspaceID: ws.ID, inboxID: inboxID, address: local + "@" + a.Config.InboundDomain,
		displayName: displayName(ws.Name), local: local, isDefault: true,
	})

	return err
}

/*
CreateEmailAddressInput is who asks and the address to connect: the
public address customers write to, how replies are signed, and the
inbox it feeds, the workspace's first when nil.
*/
type CreateEmailAddressInput struct {
	Membership  authz.Membership
	Address     string
	DisplayName string
	InboxID     *ids.UUID
}

/*
CreateEmailAddress connects a public address the workspace forwards to
a new forwarding address.

Rules:
  - The caller needs email:manage.
  - The install must receive mail.
  - The address must be valid and not on the inbound domain itself.
  - The first address of a workspace becomes its default.

Errors: [exceptions.ErrForbidden], [exceptions.ErrInboundNotConfigured],
[exceptions.ErrAddressInUse], [exceptions.ErrNotFound] for an unknown
inbox, a validation fault, or a wrapped database error.
*/
type CreateEmailAddress struct {
	DB     *database.DB
	Clock  clock.Clock
	Config appconfig.EmailConfig
}

/*
Handle creates the address and its connection.
*/
func (a *CreateEmailAddress) Handle(ctx context.Context, in CreateEmailAddressInput) (models.EmailAddress, error) {
	if !in.Membership.Can("email:manage") {
		return models.EmailAddress{}, exceptions.ErrForbidden
	}

	if !a.Config.Receives() {
		return models.EmailAddress{}, exceptions.ErrInboundNotConfigured
	}

	address, err := publicAddress(in.Address, a.Config.InboundDomain)
	if err != nil {
		return models.EmailAddress{}, err
	}

	ws := in.Membership.Workspace

	var created models.EmailAddress

	err = a.DB.Transaction(tenancy.With(ctx, ws.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c := creator{clock: a.Clock, config: a.Config}

		inboxID, err := c.chooseInbox(ctx, q, ws.ID, in.InboxID)
		if err != nil {
			return err
		}

		_, err = q.FindDefaultEmailAddress(ctx, ws.ID)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find default email address: %w", err)
		}

		created, err = c.create(ctx, tx, newAddress{
			workspaceID: ws.ID, inboxID: inboxID, address: address,
			displayName: displayName(cmp.Or(strings.TrimSpace(in.DisplayName), ws.Name)),
			local:       inbound.LabeledLocal(ws.Slug, address), isDefault: errors.Is(err, database.ErrNotFound),
		})

		return err
	})

	return created, err
}

/*
creator creates addresses with the clock and the email configuration.
*/
type creator struct {
	clock  clock.Clock
	config appconfig.EmailConfig
}

/*
create stores the connection and the address, trying numbered
forwarding addresses until one is free.
*/
func (c creator) create(ctx context.Context, tx *database.Tx, in newAddress) (models.EmailAddress, error) {
	for n := 1; n <= maxForwardingAttempts; n++ {
		in.forwarding = inbound.WithSuffix(in.local, n) + "@" + c.config.InboundDomain

		created, err := c.insert(ctx, tx, in)
		if database.IsUniqueViolation(err, forwardingTakenConnection) ||
			database.IsUniqueViolation(err, forwardingTakenAddress) {
			continue
		}

		if database.IsUniqueViolation(err, publicAddressTaken) {
			return models.EmailAddress{}, exceptions.ErrAddressInUse
		}

		return created, err
	}

	return models.EmailAddress{}, fmt.Errorf("create email address: no free forwarding address for %s", in.local)
}

/*
insert stores the connection and the address in a savepoint, so a taken
forwarding address leaves the transaction usable.
*/
func (c creator) insert(ctx context.Context, tx *database.Tx, in newAddress) (models.EmailAddress, error) {
	var created models.EmailAddress

	settings, err := json.Marshal(models.DefaultEmailAddressSettings())
	if err != nil {
		return created, fmt.Errorf("encode email address settings: %w", err)
	}

	err = tx.Transaction(ctx, func(ctx context.Context, sp *database.Tx) error {
		q := models.New(sp)

		conn, err := q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: in.workspaceID, InboxID: in.inboxID, Type: string(enums.ChannelTypeEmail),
			Name: in.address, Status: "active", ExternalID: &in.forwarding, Config: json.RawMessage(`{}`),
			Now: c.clock.Now(),
		})
		if err != nil {
			return err
		}

		created, err = q.CreateEmailAddress(ctx, models.CreateEmailAddressParams{
			WorkspaceID: in.workspaceID, ChannelConnectionID: conn.ID, Address: in.address,
			DisplayName: in.displayName, ForwardingAddress: in.forwarding, IsDefault: in.isDefault,
			Settings: settings, Now: c.clock.Now(),
		})

		return err
	})

	return created, err
}

/*
firstInbox returns the workspace's oldest live inbox, creating one when
it has none yet.
*/
func (c creator) firstInbox(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (ids.UUID, error) {
	inboxes, err := q.ListInboxes(ctx, workspaceID)
	if err != nil {
		return ids.Nil(), fmt.Errorf("list inboxes: %w", err)
	}

	if len(inboxes) > 0 {
		return inboxes[0].ID, nil
	}

	inbox, err := q.CreateInbox(ctx, models.CreateInboxParams{
		WorkspaceID: workspaceID, Name: defaultInboxName, Settings: json.RawMessage(`{}`), Now: c.clock.Now(),
	})
	if err != nil {
		return ids.Nil(), fmt.Errorf("create inbox: %w", err)
	}

	return inbox.ID, nil
}

/*
chooseInbox returns the inbox the caller named, which must be live in
the workspace, or the first inbox.
*/
func (c creator) chooseInbox(
	ctx context.Context, q *models.Queries, workspaceID ids.UUID, named *ids.UUID,
) (ids.UUID, error) {
	if named == nil {
		return c.firstInbox(ctx, q, workspaceID)
	}

	inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: workspaceID, ID: *named})
	if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
		return ids.Nil(), exceptions.ErrNotFound
	}

	if err != nil {
		return ids.Nil(), fmt.Errorf("find inbox: %w", err)
	}

	return inbox.ID, nil
}

/*
publicAddress validates and lowercases a public address. An address on
the inbound domain would forward to itself.
*/
func publicAddress(address, inboundDomain string) (string, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(address))
	if err != nil || parsed.Name != "" {
		return "", exceptions.ErrInvalidEmailAddress
	}

	email := strings.ToLower(parsed.Email)
	if _, domain, _ := strings.Cut(email, "@"); domain == inboundDomain {
		return "", exceptions.ErrInvalidEmailAddress
	}

	return email, nil
}

/*
displayName returns a display name that fits: trimmed, at most 80
characters, never empty.
*/
func displayName(name string) string {
	name = inbound.DisplayName(name)
	if runes := []rune(name); len(runes) > maxDisplayNameLength {
		name = strings.TrimSpace(string(runes[:maxDisplayNameLength]))
	}

	if name == "" {
		return "Support"
	}

	return name
}

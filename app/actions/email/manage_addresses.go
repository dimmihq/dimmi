/*
|--------------------------------------------------------------------------
| Managing Email Addresses
|--------------------------------------------------------------------------
|
| Everyone who may read email settings sees the workspace's addresses,
| how to set forwarding up and whether mail arrived yet. Changing how an
| address signs and handles mail, or disconnecting it, takes
| email:manage. A disconnected address stops receiving: its forwarding
| address is never handed out again, and mail still forwarded to it is
| recorded so an admin can see the forward was not removed.
|
| The workspace always has one default address, which new mail leaves
| from when nothing else decides. Making another address the default
| moves it; the default is never just turned off.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListEmailAddresses lists the workspace's live addresses, the default
first.

Rules:
  - The caller needs email:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListEmailAddresses struct {
	DB *database.DB
}

/*
Handle returns the addresses.
*/
func (a *ListEmailAddresses) Handle(ctx context.Context, m authz.Membership) ([]models.ListEmailAddressesRow, error) {
	if !m.Can("email:read") {
		return nil, exceptions.ErrForbidden
	}

	var rows []models.ListEmailAddressesRow

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListEmailAddresses(ctx, m.Workspace.ID)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list email addresses: %w", err)
	}

	return rows, nil
}

/*
ShowEmailAddress returns one live address of the workspace.

Rules:
  - The caller needs email:read.
  - Addresses of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowEmailAddress struct {
	DB *database.DB
}

/*
Handle returns the address.
*/
func (a *ShowEmailAddress) Handle(
	ctx context.Context, m authz.Membership, id ids.UUID,
) (models.FindEmailAddressRow, error) {
	if !m.Can("email:read") {
		return models.FindEmailAddressRow{}, exceptions.ErrForbidden
	}

	var row models.FindEmailAddressRow

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		row, err = findAddress(ctx, models.New(tx), m.Workspace.ID, id)

		return err
	})

	return row, err
}

/*
UpdateEmailAddressInput is who asks, the address, and what changes; a
nil field stays as it is. Style holds the look of replies, its brand
color and footer.
*/
type UpdateEmailAddressInput struct {
	Membership       authz.Membership
	ID               ids.UUID
	DisplayName      *string
	ReplyToMode      *enums.ReplyToMode
	FromNameStyle    *enums.FromNameStyle
	QuotePrevious    *bool
	BulkMailHandling *enums.BulkMailHandling
	IsDefault        *bool
	Style            ReplyStyleInput
}

/*
UpdateEmailAddress changes how an address signs and handles mail.

Rules:
  - The caller needs email:manage.
  - Making the address the default leaves the previous one an
    ordinary address; the default itself cannot be turned off.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDefaultAddressRequired], or a wrapped database error.
*/
type UpdateEmailAddress struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle applies the changes and returns the address as it now is.
*/
func (a *UpdateEmailAddress) Handle(
	ctx context.Context, in UpdateEmailAddressInput,
) (models.FindEmailAddressRow, error) {
	if !in.Membership.Can("email:manage") {
		return models.FindEmailAddressRow{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace

	var row models.FindEmailAddressRow

	err := a.DB.Transaction(tenancy.With(ctx, ws.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := findAddress(ctx, q, ws.ID, in.ID)
		if err != nil {
			return err
		}

		params := updateParams(current.EmailAddress, in)
		params.Now = a.Clock.Now()
		if _, err := q.UpdateEmailAddress(ctx, params); err != nil {
			return fmt.Errorf("update email address: %w", err)
		}

		if err := a.makeDefault(ctx, q, current.EmailAddress, in.IsDefault); err != nil {
			return err
		}

		row, err = findAddress(ctx, q, ws.ID, in.ID)

		return err
	})

	return row, err
}

/*
updateParams merges the changes into the address's current values.
*/
func updateParams(current models.EmailAddress, in UpdateEmailAddressInput) models.UpdateEmailAddressParams {
	settings := models.ReadEmailAddressSettings(current.Settings)
	if in.QuotePrevious != nil {
		settings.QuotePrevious = *in.QuotePrevious
	}

	if in.BulkMailHandling != nil {
		settings.BulkMailHandling = string(*in.BulkMailHandling)
	}

	settings.Template = pick(in.Style.Template, settings.Template)
	settings.BrandColor = strings.ToLower(pick(in.Style.BrandColor, settings.BrandColor))
	settings.FooterMarkdown = strings.TrimSpace(pick(in.Style.FooterMarkdown, settings.FooterMarkdown))

	params := models.UpdateEmailAddressParams{
		DisplayName: current.DisplayName, ReplyToMode: current.ReplyToMode, FromNameStyle: current.FromNameStyle,
		Settings: jsonOf(settings), WorkspaceID: current.WorkspaceID, ID: current.ID,
	}

	if in.DisplayName != nil {
		params.DisplayName = displayName(*in.DisplayName)
	}

	if in.ReplyToMode != nil {
		params.ReplyToMode = string(*in.ReplyToMode)
	}

	if in.FromNameStyle != nil {
		params.FromNameStyle = string(*in.FromNameStyle)
	}

	return params
}

/*
makeDefault makes the address the workspace's default when asked to,
and refuses to leave the workspace without one.
*/
func (a *UpdateEmailAddress) makeDefault(
	ctx context.Context, q *models.Queries, current models.EmailAddress, makeIt *bool,
) error {
	if makeIt == nil || *makeIt == current.IsDefault {
		return nil
	}

	if !*makeIt {
		return exceptions.ErrDefaultAddressRequired
	}

	now := a.Clock.Now()
	if err := q.ClearDefaultEmailAddress(ctx, models.ClearDefaultEmailAddressParams{
		WorkspaceID: current.WorkspaceID, Now: now,
	}); err != nil {
		return fmt.Errorf("clear default email address: %w", err)
	}

	err := q.SetDefaultEmailAddress(ctx, models.SetDefaultEmailAddressParams{
		WorkspaceID: current.WorkspaceID, ID: current.ID, Now: now,
	})
	if err != nil {
		return fmt.Errorf("set default email address: %w", err)
	}

	return nil
}

/*
pick returns the change when there is one, else the current value.
*/
func pick(change *string, current string) string {
	if change == nil {
		return current
	}

	return *change
}

/*
DeleteEmailAddress disconnects an address and its connection.

Rules:
  - The caller needs email:manage.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteEmailAddress struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle disconnects the address.
*/
func (a *DeleteEmailAddress) Handle(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("email:manage") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findAddress(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		err := q.DeleteEmailAddress(ctx, models.DeleteEmailAddressParams{
			Now: a.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("delete email address: %w", err)
		}

		return nil
	})
}

/*
findAddress returns a live address of the workspace, or
[exceptions.ErrNotFound].
*/
func findAddress(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.FindEmailAddressRow, error) {
	row, err := q.FindEmailAddress(ctx, models.FindEmailAddressParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find email address: %w", err)
	}

	return row, nil
}

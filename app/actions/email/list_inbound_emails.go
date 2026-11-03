/*
|--------------------------------------------------------------------------
| Inbound Activity
|--------------------------------------------------------------------------
|
| Everything that reached an address, newest first, ignored mail
| included, with what became of it. It is how an admin checks a forward
| works and finds out why an email never showed up in the inbox, so it
| is for those allowed to read raw mail.
|
*/

package email

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxPage is the most emails one page holds.
*/
const maxPage = 100

/*
ListInboundEmailsInput is who asks, the address, an optional status to
narrow to, and where the page starts: after the email received at
Before with id BeforeID.
*/
type ListInboundEmailsInput struct {
	Membership authz.Membership
	AddressID  ids.UUID
	Status     *string
	Before     *time.Time
	BeforeID   *ids.UUID
	Limit      int
}

/*
InboundEmailPage is one page of received emails, and whether more
follow.
*/
type InboundEmailPage struct {
	Emails []models.InboundEmail
	More   bool
}

/*
ListInboundEmails lists what reached one address.

Rules:
  - The caller needs email:read_raw.
  - Addresses of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListInboundEmails struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListInboundEmails) Handle(ctx context.Context, in ListInboundEmailsInput) (InboundEmailPage, error) {
	if !in.Membership.Can("email:read_raw") {
		return InboundEmailPage{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID

	var rows []models.InboundEmail

	err := within(tenancy.With(ctx, ws.String()), a.DB, func(ctx context.Context, q *models.Queries) error {
		if _, err := findAddress(ctx, q, ws, in.AddressID); err != nil {
			return err
		}

		var err error

		rows, err = q.ListAddressInboundEmails(ctx, models.ListAddressInboundEmailsParams{
			WorkspaceID: ws, EmailAddressID: &in.AddressID, Status: in.Status, BeforeID: in.BeforeID,
			BeforeReceivedAt: in.Before, RowLimit: int64(min(in.Limit, maxPage) + 1),
		})
		if err != nil {
			return fmt.Errorf("list inbound emails: %w", err)
		}

		return nil
	})
	if err != nil {
		return InboundEmailPage{}, err
	}

	if len(rows) > in.Limit {
		return InboundEmailPage{Emails: rows[:in.Limit], More: true}, nil
	}

	return InboundEmailPage{Emails: rows}, nil
}

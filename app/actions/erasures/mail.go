/*
|--------------------------------------------------------------------------
| Erasing A Customer's Mail Records
|--------------------------------------------------------------------------
|
| Email leaves records beside the messages it became: the stored source
| and delivery record of what the customer sent, the delivery record of
| what was sent to them, and the suppressions on their addresses. This
| eraser deletes the received mail and its files, forgets who the sent
| mail went to and what it was about, and deletes the suppressions.
|
| A suppression made because the customer unsubscribed or complained is
| the one thing kept, as a hash of the address in place of the address:
| erasing someone must not start mailing them again. Running the eraser
| again finds nothing left.
|
*/

package erasures

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/retention"
	"github.com/dimmihq/dimmi/app/models"
)

/*
emailIdentity is the type of an identity that is an email address.
*/
const emailIdentity = "email"

/*
MailEraser removes an erased customer's received mail, anonymises the
mail sent to them and deletes or hashes their suppressions.
*/
type MailEraser struct {
	DB      *database.DB
	Clock   clock.Clock
	Storage retention.Disks
	Keys    encryption.KeyManager
}

/*
Name names what the eraser removes.
*/
func (e *MailEraser) Name() string {
	return "mail_records"
}

/*
Erase deletes the stored source of the contact's mail, then removes or
anonymises the records in one transaction.
*/
func (e *MailEraser) Erase(ctx context.Context, workspaceID, contactID ids.UUID) error {
	scoped := tenancy.With(ctx, workspaceID.String())

	var (
		emails []string
		raws   []retention.RawEmail
	)

	err := e.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error

		if emails, err = e.addresses(ctx, models.New(tx), workspaceID, contactID); err != nil {
			return err
		}

		raws, err = erasedRawEmails(ctx, models.New(tx), erasedMail{workspaceID, contactID, emails})

		return err
	})
	if err != nil {
		return err
	}

	if err := retention.DeleteRawEmails(scoped, e.DB, e.Storage, raws); err != nil {
		return err
	}

	return e.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return e.records(ctx, models.New(tx), erasedMail{workspaceID, contactID, emails})
	})
}

/*
addresses returns every email address the contact was known by, in
lower case: the one on their profile and those among the identities
their erasure set aside.
*/
func (e *MailEraser) addresses(
	ctx context.Context, q *models.Queries, workspaceID, contactID ids.UUID,
) ([]string, error) {
	emails := []string{}

	profile, err := q.FindErasedContactEmail(ctx, models.FindErasedContactEmailParams{
		WorkspaceID: workspaceID, ID: contactID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("find contact email: %w", err)
	}

	emails = addAddress(emails, profile)

	row, err := q.FindOpenContactErasure(ctx, models.FindOpenContactErasureParams{
		WorkspaceID: workspaceID, ContactID: contactID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return emails, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find erasure: %w", err)
	}

	stash, err := openStash(ctx, e.Keys, row)
	if err != nil {
		return nil, err
	}

	for _, identity := range stash {
		if identity.Type == emailIdentity {
			emails = addAddress(emails, identity.Value)
		}
	}

	return emails, nil
}

/*
addAddress adds the address in lower case, unless it is empty or there
already.
*/
func addAddress(emails []string, address string) []string {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" || slices.Contains(emails, address) {
		return emails
	}

	return append(emails, address)
}

/*
erasedMail names whose mail is erased: the workspace, the contact and
every address the contact was known by.
*/
type erasedMail struct {
	workspaceID ids.UUID
	contactID   ids.UUID
	emails      []string
}

/*
erasedRawEmails returns the stored source of the mail the contact sent.
*/
func erasedRawEmails(ctx context.Context, q *models.Queries, who erasedMail) ([]retention.RawEmail, error) {
	rows, err := q.ListErasedRawEmails(ctx, models.ListErasedRawEmailsParams{
		WorkspaceID: who.workspaceID, ContactID: who.contactID, Emails: who.emails,
	})
	if err != nil {
		return nil, fmt.Errorf("list raw email: %w", err)
	}

	raws := make([]retention.RawEmail, 0, len(rows))
	for _, row := range rows {
		raws = append(raws, retention.RawEmail{ID: row.ID, Disk: row.RawDisk, Path: *row.RawPath})
	}

	return raws, nil
}

/*
records deletes the received mail's records, forgets the recipients and
subjects of the sent mail, and hashes or deletes the suppressions.
*/
func (e *MailEraser) records(ctx context.Context, q *models.Queries, who erasedMail) error {
	now := e.Clock.Now()
	workspaceID, contactID, emails := who.workspaceID, who.contactID, who.emails

	if _, err := q.DeleteErasedInboundEmails(ctx, models.DeleteErasedInboundEmailsParams{
		WorkspaceID: workspaceID, ContactID: contactID, Emails: emails,
	}); err != nil {
		return fmt.Errorf("delete received email: %w", err)
	}

	if _, err := q.ScrubErasedOutboundEmails(ctx, models.ScrubErasedOutboundEmailsParams{
		Now: now, WorkspaceID: workspaceID, ContactID: contactID, Emails: emails,
	}); err != nil {
		return fmt.Errorf("scrub sent email: %w", err)
	}

	if len(emails) == 0 {
		return nil
	}

	if _, err := q.StripErasedOutboundCopies(ctx, models.StripErasedOutboundCopiesParams{
		Now: now, WorkspaceID: workspaceID, Emails: emails,
	}); err != nil {
		return fmt.Errorf("strip copied addresses: %w", err)
	}

	if _, err := q.HashErasedSuppressions(ctx, models.HashErasedSuppressionsParams{
		Now: now, WorkspaceID: workspaceID, Emails: emails,
	}); err != nil {
		return fmt.Errorf("hash suppressions: %w", err)
	}

	if _, err := q.DeleteErasedSuppressions(ctx, models.DeleteErasedSuppressionsParams{
		WorkspaceID: workspaceID, Emails: emails,
	}); err != nil {
		return fmt.Errorf("delete suppressions: %w", err)
	}

	return nil
}

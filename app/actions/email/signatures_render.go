/*
|--------------------------------------------------------------------------
| Signing A Reply
|--------------------------------------------------------------------------
|
| Which signature a reply carries, and how one looks before it is saved.
| A teammate's reply carries their own signature for the address, else
| their own for every address, else the address's. A reply nobody wrote
| by hand, from the agent or the system, carries only the address's: a
| teammate's name never signs words they did not write. A reply may ask
| for no signature at all.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RenderEmailSignature shows how a signature reads in a reply from the
caller, before it is saved.

Rules:
  - The caller needs email_signatures:manage_own or manage_any.
  - Placeholders are filled in as for the caller's own reply from the
    address, or from no address in particular when none is named.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrUnknownSignaturePlaceholder], or a wrapped database error.
*/
type RenderEmailSignature struct {
	DB *database.DB
}

/*
Handle returns the signature's HTML and text.
*/
func (a *RenderEmailSignature) Handle(
	ctx context.Context, m authz.Membership, body string, address *ids.UUID,
) (inbound.Signature, error) {
	if !m.Can("email_signatures:manage_own") && !m.Can("email_signatures:manage_any") {
		return inbound.Signature{}, exceptions.ErrForbidden
	}

	if unknown := inbound.UnknownSignaturePlaceholders(body); len(unknown) > 0 {
		return inbound.Signature{}, exceptions.ErrUnknownSignaturePlaceholder.With("placeholders", unknown)
	}

	values := inbound.SignatureValues{CompanyName: m.Workspace.Name}

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		member, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
			WorkspaceID: m.Workspace.ID, ID: m.Member.ID,
		})
		if err != nil {
			return err
		}

		values.MemberName = member.User.Name
		if address == nil {
			return nil
		}

		row, err := q.FindEmailAddress(ctx, models.FindEmailAddressParams{WorkspaceID: m.Workspace.ID, ID: *address})
		values.AddressName = row.EmailAddress.DisplayName

		return notFound(err)
	})
	if err != nil {
		return inbound.Signature{}, fmt.Errorf("render email signature: %w", err)
	}

	return renderSignature(body, values)
}

/*
signature returns the signature the reply carries, empty when it
carries none.
*/
func (a *SendEmailMessage) signature(
	ctx context.Context, q *models.Queries, out *outbound,
) (inbound.Signature, error) {
	options, err := readOptions(out.msg.Options)
	if err != nil {
		return inbound.Signature{}, err
	}

	if options.Signature != nil && !*options.Signature {
		return inbound.Signature{}, nil
	}

	params := models.ResolveEmailSignatureParams{WorkspaceID: out.msg.WorkspaceID, AddressID: &out.address.ID}
	values := inbound.SignatureValues{AddressName: out.address.DisplayName, CompanyName: out.workspace.Name}

	if out.msg.Author.Type == enums.AuthorTypeMember {
		params.MemberID = &out.msg.Author.ID
		values.MemberName = out.msg.Author.Name
	}

	row, err := q.ResolveEmailSignature(ctx, params)
	if errors.Is(err, database.ErrNotFound) {
		return inbound.Signature{}, nil
	}

	if err != nil {
		return inbound.Signature{}, fmt.Errorf("resolve email signature: %w", err)
	}

	return renderSignature(row.Body, values)
}

/*
renderSignature renders a signature, wrapping a failure to render it.
*/
func renderSignature(body string, values inbound.SignatureValues) (inbound.Signature, error) {
	signature, err := inbound.RenderSignature(body, values)
	if err != nil {
		return inbound.Signature{}, fmt.Errorf("render signature: %w", err)
	}

	return signature, nil
}

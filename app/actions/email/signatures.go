/*
|--------------------------------------------------------------------------
| Email Signatures
|--------------------------------------------------------------------------
|
| A teammate keeps their own signature, for every address or for one,
| and the team keeps one per address for replies from it. Teammates
| change their own with email_signatures:manage_own; anyone else's, and
| the team's, take email_signatures:manage_any. Everyone who may keep a
| signature sees the team's, so they know what replies carry when they
| have none of their own.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Signature kinds: a teammate's own, or an address's.
*/
const (
	SignatureKindPersonal = "personal"
	SignatureKindAddress  = "address"
)

/*
signatureScopeKey is the unique index of one signature per teammate and
address.
*/
const signatureScopeKey = "email_signatures_scope_key"

/*
SignatureFilter narrows a list to one teammate, one address or one kind.
*/
type SignatureFilter struct {
	MemberID  *ids.UUID
	AddressID *ids.UUID
	Kind      *string
}

/*
ListEmailSignatures lists the signatures the caller may see.

Rules:
  - The caller needs email_signatures:manage_own or manage_any.
  - With manage_own only, the caller sees their own and the team's.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListEmailSignatures struct {
	DB *database.DB
}

/*
Handle returns the signatures.
*/
func (a *ListEmailSignatures) Handle(
	ctx context.Context, m authz.Membership, f SignatureFilter,
) ([]models.EmailSignature, error) {
	if !m.Can("email_signatures:manage_own") && !m.Can("email_signatures:manage_any") {
		return nil, exceptions.ErrForbidden
	}

	var rows []models.EmailSignature

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListEmailSignatures(ctx, models.ListEmailSignaturesParams{
			WorkspaceID: m.Workspace.ID, MemberID: f.MemberID, AddressID: f.AddressID, Kind: f.Kind,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list email signatures: %w", err)
	}

	return slices.DeleteFunc(rows, func(s models.EmailSignature) bool {
		return s.WorkspaceMemberID != nil && !maySign(m, s.WorkspaceMemberID)
	}), nil
}

/*
CreateEmailSignatureInput is who asks and the signature to keep. Kind is
personal or address; a personal signature is the caller's unless
MemberID names another teammate, and is for every address unless
AddressID names one. Enabled defaults to true.
*/
type CreateEmailSignatureInput struct {
	Membership authz.Membership
	Kind       string
	MemberID   *ids.UUID
	AddressID  *ids.UUID
	Body       string
	Enabled    *bool
}

/*
CreateEmailSignature keeps a new signature.

Rules:
  - The caller's own takes manage_own; another teammate's or the
    team's takes manage_any.
  - An address signature names an address of the workspace.
  - Every placeholder must be one a reply can fill.
  - One signature per teammate and address.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrSignatureAddressRequired],
[exceptions.ErrUnknownSignaturePlaceholder],
[exceptions.ErrSignatureExists], or a wrapped database error.
*/
type CreateEmailSignature struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle keeps the signature and returns it.
*/
func (a *CreateEmailSignature) Handle(
	ctx context.Context, in CreateEmailSignatureInput,
) (models.EmailSignature, error) {
	m, member := in.Membership, in.MemberID
	if in.Kind == SignatureKindAddress {
		member = nil
	} else if member == nil {
		member = &m.Member.ID
	}

	if err := checkSignature(m, member, in.AddressID, in.Body); err != nil {
		return models.EmailSignature{}, err
	}

	var row models.EmailSignature

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		params := models.CreateEmailSignatureParams{
			WorkspaceID: m.Workspace.ID, MemberID: member, AddressID: in.AddressID, Body: in.Body,
			Enabled: in.Enabled == nil || *in.Enabled, Now: a.Clock.Now(),
		}

		if err := signatureScopeExists(ctx, q, params); err != nil {
			return err
		}

		var err error

		row, err = q.CreateEmailSignature(ctx, params)

		return err
	})
	if database.IsUniqueViolation(err, signatureScopeKey) {
		return models.EmailSignature{}, exceptions.ErrSignatureExists
	}

	if err != nil {
		return models.EmailSignature{}, fmt.Errorf("create email signature: %w", err)
	}

	return row, nil
}

/*
maySign reports whether the caller may change the signature of the
teammate, or the team's when member is nil.
*/
func maySign(m authz.Membership, member *ids.UUID) bool {
	if m.Can("email_signatures:manage_any") {
		return true
	}

	return member != nil && *member == m.Member.ID && m.Can("email_signatures:manage_own")
}

/*
checkSignature refuses a signature the caller may not keep, an address
signature without an address, and unknown placeholders.
*/
func checkSignature(m authz.Membership, member, address *ids.UUID, body string) error {
	if !maySign(m, member) {
		return exceptions.ErrForbidden
	}

	if member == nil && address == nil {
		return exceptions.ErrSignatureAddressRequired
	}

	if unknown := inbound.UnknownSignaturePlaceholders(body); len(unknown) > 0 {
		return exceptions.ErrUnknownSignaturePlaceholder.With("placeholders", unknown)
	}

	return nil
}

/*
signatureScopeExists checks that the teammate and address a new
signature names are the workspace's.
*/
func signatureScopeExists(ctx context.Context, q *models.Queries, s models.CreateEmailSignatureParams) error {
	if s.MemberID != nil {
		_, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: s.WorkspaceID, ID: *s.MemberID,
		})
		if err != nil {
			return notFound(err)
		}
	}

	if s.AddressID != nil {
		_, err := q.FindEmailAddress(ctx, models.FindEmailAddressParams{
			WorkspaceID: s.WorkspaceID, ID: *s.AddressID,
		})
		if err != nil {
			return notFound(err)
		}
	}

	return nil
}

/*
ownSignature returns a signature of the workspace the caller may change.
*/
func ownSignature(
	ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID,
) (models.EmailSignature, error) {
	row, err := q.FindEmailSignature(ctx, models.FindEmailSignatureParams{WorkspaceID: m.Workspace.ID, ID: id})
	if err != nil {
		return row, notFound(err)
	}

	if !maySign(m, row.WorkspaceMemberID) {
		return row, exceptions.ErrForbidden
	}

	return row, nil
}

/*
notFound turns a missing row into the not found error.
*/
func notFound(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	return err
}

/*
|--------------------------------------------------------------------------
| Email Signature Controller
|--------------------------------------------------------------------------
|
| The signatures of the workspace in play: listing them, keeping a new
| one, changing or removing one, and previewing one as a reply shows it.
| A list may be narrowed to one teammate, "me" for the caller, to one
| address, or to one kind. Ids of other workspaces' signatures, teammates
| and addresses are simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailSignaturesOutput is the signatures the caller may see.
*/
type EmailSignaturesOutput struct {
	Body fhttp.Collection[resources.EmailSignature]
}

/*
EmailSignatureOutput is one signature.
*/
type EmailSignatureOutput struct {
	Body fhttp.Resource[resources.EmailSignature]
}

/*
EmailSignatureRenderingOutput is a signature as a reply shows it.
*/
type EmailSignatureRenderingOutput struct {
	Body fhttp.Resource[resources.EmailSignatureRendering]
}

/*
EmailSignatureController serves the workspace's signatures.
*/
type EmailSignatureController struct {
	List   *email.ListEmailSignatures
	Create *email.CreateEmailSignature
	Change *email.UpdateEmailSignature
	Remove *email.DeleteEmailSignature
	Render *email.RenderEmailSignature
	IDs    *ids.Registry
}

/*
EmailSignatureListing is what the signature list accepts: filters on
the member, the address and the kind of signature.
*/
func EmailSignatureListing() pagination.Allow {
	return pagination.Allow{Filters: []string{"member", "address", "kind"}}
}

/*
Index lists the signatures, narrowed by filter[member], filter[address]
and filter[kind].
*/
func (c *EmailSignatureController) Index(ctx context.Context, _ *requests.NoInput) (*EmailSignaturesOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), EmailSignatureListing())
	if err != nil {
		return nil, err
	}

	filter, err := c.filter(ctx, query.Filters)
	if err != nil {
		return nil, err
	}

	rows, err := c.List.Handle(ctx, membership(ctx), filter)
	if err != nil {
		return nil, err
	}

	items := resources.NewEmailSignatures(rows)

	return &EmailSignaturesOutput{Body: fhttp.NewCollection(pagination.Page[resources.EmailSignature]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Store keeps a new signature.
*/
func (c *EmailSignatureController) Store(
	ctx context.Context, in *requests.StoreEmailSignatureRequest,
) (*EmailSignatureOutput, error) {
	b := in.Body
	input := email.CreateEmailSignatureInput{
		Membership: membership(ctx), Kind: b.Kind, Body: b.Body, Enabled: b.Enabled,
	}

	var err error
	if input.MemberID, err = c.decode(models.PrefixMember, b.MemberID); err != nil {
		return nil, err
	}

	if input.AddressID, err = c.decode(models.PrefixEmailAddress, b.EmailAddressID); err != nil {
		return nil, err
	}

	row, err := c.Create.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &EmailSignatureOutput{Body: fhttp.NewResource(resources.NewEmailSignature(row))}, nil
}

/*
Update changes a signature's text or whether it is used.
*/
func (c *EmailSignatureController) Update(
	ctx context.Context, in *requests.UpdateEmailSignatureRequest,
) (*EmailSignatureOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixEmailSignature), in.Signature)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	row, err := c.Change.Handle(ctx, email.UpdateEmailSignatureInput{
		Membership: membership(ctx), ID: id, Body: in.Body.Body, Enabled: in.Body.Enabled,
	})
	if err != nil {
		return nil, err
	}

	return &EmailSignatureOutput{Body: fhttp.NewResource(resources.NewEmailSignature(row))}, nil
}

/*
Destroy removes a signature.
*/
func (c *EmailSignatureController) Destroy(
	ctx context.Context, in *requests.EmailSignatureRequest,
) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixEmailSignature), in.Signature)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if err := c.Remove.Handle(ctx, membership(ctx), id); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
StoreRendering previews a signature as the caller's reply shows it.
*/
func (c *EmailSignatureController) StoreRendering(
	ctx context.Context, in *requests.StoreEmailSignatureRenderingRequest,
) (*EmailSignatureRenderingOutput, error) {
	address, err := c.decode(models.PrefixEmailAddress, in.Body.EmailAddressID)
	if err != nil {
		return nil, err
	}

	signature, err := c.Render.Handle(ctx, membership(ctx), in.Body.Body, address)
	if err != nil {
		return nil, err
	}

	return &EmailSignatureRenderingOutput{
		Body: fhttp.NewResource(resources.NewEmailSignatureRendering(signature)),
	}, nil
}

/*
filter reads the list filters; an id that is not one is not found.
*/
func (c *EmailSignatureController) filter(
	ctx context.Context, filters map[string][]string,
) (email.SignatureFilter, error) {
	var (
		out email.SignatureFilter
		err error
	)

	if kind := first(filters["kind"]); kind != nil {
		if *kind != email.SignatureKindPersonal && *kind != email.SignatureKindAddress {
			return out, pagination.ErrInvalidFilter
		}

		out.Kind = kind
	}

	if member := first(filters["member"]); member != nil && *member == "me" {
		m := membership(ctx)
		out.MemberID = &m.Member.ID
	} else if out.MemberID, err = c.decode(models.PrefixMember, member); err != nil {
		return out, err
	}

	out.AddressID, err = c.decode(models.PrefixEmailAddress, first(filters["address"]))

	return out, err
}

/*
decode reads an optional public id; one that is not an id of the kind
is not found.
*/
func (c *EmailSignatureController) decode(prefix models.Prefix, public *string) (*ids.UUID, error) {
	if public == nil {
		return nil, nil
	}

	id, err := c.IDs.Decode(string(prefix), *public)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return &id, nil
}

/*
first returns the first value of a filter, or nil without one.
*/
func first(values []string) *string {
	if len(values) == 0 {
		return nil
	}

	return &values[0]
}

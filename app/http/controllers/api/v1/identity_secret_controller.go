/*
|--------------------------------------------------------------------------
| Identity Secret Controller
|--------------------------------------------------------------------------
|
| The keys a workspace's server signs visitors' identity tokens with:
| listed, created, rotated, renamed, retired and revoked. Material goes
| out once, in the answer that made it, and never again.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IdentitySecretController serves the identity secrets.
*/
type IdentitySecretController struct {
	Secrets *identity.Secrets
	IDs     *ids.Registry
}

/*
IdentitySecretsOutput is the secret list.
*/
type IdentitySecretsOutput struct {
	Body fhttp.Collection[resources.IdentitySecret]
}

/*
IdentitySecretOutput is one secret.
*/
type IdentitySecretOutput struct {
	Body fhttp.Resource[resources.IdentitySecret]
}

/*
CreatedIdentitySecretOutput is a new secret with its material, once.
*/
type CreatedIdentitySecretOutput struct {
	Body struct {
		Data resources.CreatedIdentitySecret     `json:"data"`
		Meta resources.CreatedIdentitySecretMeta `json:"meta"`
	}
}

/*
Index lists the secrets.
*/
func (c *IdentitySecretController) Index(ctx context.Context, _ *requests.NoInput) (*IdentitySecretsOutput, error) {
	rows, err := c.Secrets.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.IdentitySecret]{Items: resources.NewIdentitySecrets(rows), Limit: 100}
	return &IdentitySecretsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one secret.
*/
func (c *IdentitySecretController) Show(ctx context.Context, in *requests.IdentitySecretRequest,
) (*IdentitySecretOutput, error) {
	id, err := c.id(in.Secret)
	if err != nil {
		return nil, err
	}
	row, err := c.Secrets.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &IdentitySecretOutput{Body: fhttp.NewResource(resources.NewIdentitySecret(row))}, nil
}

/*
Store creates a secret.
*/
func (c *IdentitySecretController) Store(ctx context.Context, in *requests.StoreIdentitySecretRequest,
) (*CreatedIdentitySecretOutput, error) {
	created, err := c.Secrets.Create(ctx, identity.NewSecret{
		Membership: membership(ctx), Name: in.Body.Name, Algorithm: in.Body.Algorithm, PublicKey: in.Body.PublicKey,
		JWKSURL: in.Body.JWKSURL, Issuer: in.Body.Issuer, Audience: in.Body.Audience,
	})
	if err != nil {
		return nil, err
	}
	return createdOutput(created), nil
}

/*
Rotate makes a new secret and retires the old one after the grace
period.
*/
func (c *IdentitySecretController) Rotate(ctx context.Context, in *requests.RotateIdentitySecretRequest,
) (*CreatedIdentitySecretOutput, error) {
	id, err := c.id(in.Secret)
	if err != nil {
		return nil, err
	}
	created, err := c.Secrets.Rotate(ctx, membership(ctx), id, in.Body.PublicKey)
	if err != nil {
		return nil, err
	}
	return createdOutput(created), nil
}

/*
Update renames, retires or revokes a secret.
*/
func (c *IdentitySecretController) Update(ctx context.Context, in *requests.UpdateIdentitySecretRequest,
) (*IdentitySecretOutput, error) {
	id, err := c.id(in.Secret)
	if err != nil {
		return nil, err
	}
	b := in.Body
	row, err := c.Secrets.Update(ctx, identity.Change{
		Membership: membership(ctx), ID: id, Name: b.Name, Status: b.Status, RetiresAt: b.RetiresAt,
		Reason: b.Reason, KeepSessions: b.RevokeSessions != nil && !*b.RevokeSessions,
	})
	if err != nil {
		return nil, err
	}
	return &IdentitySecretOutput{Body: fhttp.NewResource(resources.NewIdentitySecret(row))}, nil
}

/*
createdOutput shapes a new secret with its material and where to keep
it.
*/
func createdOutput(created identity.Created) *CreatedIdentitySecretOutput {
	out := &CreatedIdentitySecretOutput{}
	out.Body.Data = resources.CreatedIdentitySecret{
		IdentitySecret: resources.NewIdentitySecret(created.Secret), Secret: created.Value,
	}
	out.Body.Meta = resources.CreatedIdentitySecretMeta{
		SecretShownOnce: created.Value != "", EnvVar: "DIMMI_IDENTITY_SECRET", KidEnvVar: "DIMMI_IDENTITY_KID",
	}
	return out
}

/*
id decodes a secret's public id; an unknown one is not found.
*/
func (c *IdentitySecretController) id(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixIdentitySecret), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}

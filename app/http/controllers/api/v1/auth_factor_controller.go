/*
|--------------------------------------------------------------------------
| Auth Factor Controller
|--------------------------------------------------------------------------
|
| The caller's own auth factors: GET lists them, POST adds one or makes
| new recovery codes, PATCH activates a pending one with its first code
| or renames or prefers one, DELETE removes one, and DELETE on the
| collection turns two-factor authentication off. Answers that carry a
| secret shown once are never cached.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/factors"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
noStore keeps an answer carrying a secret out of every cache.
*/
const noStore = "no-store"

/*
AuthFactorsOutput is the list of the caller's factors.
*/
type AuthFactorsOutput struct {
	Body fhttp.Collection[resources.AuthFactor]
}

/*
AuthFactorOutput is one factor, with Cache-Control set when the answer
carries a secret shown once.
*/
type AuthFactorOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         fhttp.Resource[resources.AuthFactor]
}

/*
AuthFactorController serves the caller's factor collection.
*/
type AuthFactorController struct {
	List      *factors.ListAuthFactors
	Add       *factors.AddAuthFactor
	Activate  *factors.ActivateAuthFactor
	Change    *factors.UpdateAuthFactor
	Remove    *factors.RemoveAuthFactor
	RemoveAll *factors.RemoveAllAuthFactors
	IDs       *ids.Registry
	Cookies   appauth.Cookies
}

/*
Index lists the caller's factors.
*/
func (c *AuthFactorController) Index(ctx context.Context, _ *requests.NoInput) (*AuthFactorsOutput, error) {
	held, err := c.List.Handle(ctx, caller(ctx).User.ID)
	if err != nil {
		return nil, err
	}

	items := resources.NewAuthFactors(held)
	page := pagination.Page[resources.AuthFactor]{Items: items, Limit: len(items)}

	return &AuthFactorsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store adds a pending factor with its setup, or new recovery codes.
*/
func (c *AuthFactorController) Store(
	ctx context.Context, in *requests.StoreAuthFactorRequest,
) (*AuthFactorOutput, error) {
	made, err := c.Add.Handle(ctx, factors.AddAuthFactorInput{
		User: caller(ctx).User, Type: in.Body.Type, Name: in.Body.Name, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &AuthFactorOutput{
		CacheControl: noStore, Body: fhttp.NewResource(resources.NewMadeAuthFactor(made.Factor, made.Setup)),
	}, nil
}

/*
Update activates the factor with a code, or renames or prefers it.
*/
func (c *AuthFactorController) Update(
	ctx context.Context, in *requests.UpdateAuthFactorRequest,
) (*AuthFactorOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAuthFactor), in.Factor)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if in.Activates() {
		return c.activate(ctx, in, id)
	}

	f, err := c.Change.Handle(ctx, factors.UpdateAuthFactorInput{
		User: caller(ctx).User, FactorID: id, Name: in.Body.Name,
		Preferred: in.Body.Preferred != nil && *in.Body.Preferred, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &AuthFactorOutput{Body: fhttp.NewResource(resources.NewAuthFactor(f))}, nil
}

/*
activate turns the pending factor active, answering with the recovery
codes made when it turned two-factor authentication on.
*/
func (c *AuthFactorController) activate(
	ctx context.Context, in *requests.UpdateAuthFactorRequest, id ids.UUID,
) (*AuthFactorOutput, error) {
	if in.Body.Name != nil || in.Body.Preferred != nil {
		return nil, exceptions.ErrCodeProhibited
	}

	activated, err := c.Activate.Handle(ctx, factors.ActivateAuthFactorInput{
		User: caller(ctx).User, FactorID: id, Code: *in.Body.Code, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	out := &AuthFactorOutput{}
	resource := resources.NewAuthFactor(activated.Factor)

	if set := activated.RecoveryCodes; set != nil && set.Factor.ID != ids.Nil() {
		codes := resources.NewMadeAuthFactor(set.Factor, set.Setup)
		resource.RecoveryCodes = &codes
		out.CacheControl = noStore
	}

	out.Body = fhttp.NewResource(resource)

	return out, nil
}

/*
Destroy removes one of the caller's factors.
*/
func (c *AuthFactorController) Destroy(
	ctx context.Context, in *requests.DestroyAuthFactorRequest,
) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixAuthFactor), in.Factor)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	p := caller(ctx)

	err = c.Remove.Handle(ctx, factors.RemoveAuthFactorInput{
		User: p.User, FactorID: id, Client: clientOf(ctx, c.Cookies), KeepGrant: keptGrant(p),
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
DestroyMany removes every factor of the caller, turning two-factor
authentication off.
*/
func (c *AuthFactorController) DestroyMany(ctx context.Context, _ *requests.NoInput) (*EmptyOutput, error) {
	p := caller(ctx)

	_, err := c.RemoveAll.Handle(ctx, factors.RemoveAllAuthFactorsInput{
		UserID: p.User.ID, Client: clientOf(ctx, c.Cookies), KeepGrant: keptGrant(p),
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
keptGrant returns the first-party app grant the caller signed in with,
which turning two-factor authentication off keeps, or ids.Nil() for a
browser.
*/
func keptGrant(p *appauth.Principal) ids.UUID {
	if p.Kind == appauth.KindApp {
		return p.CredentialID
	}

	return ids.Nil()
}

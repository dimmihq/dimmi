/*
|--------------------------------------------------------------------------
| Personal Access Token Controller
|--------------------------------------------------------------------------
|
| The caller's tokens in the workspace in play: GET /tokens lists them, a
| page at a time, POST /tokens mints one and answers with its secret
| once, GET and PATCH /tokens/{token} show and rename one, and DELETE
| revokes it. filter[member] lets an admin list one member's tokens, or
| everyone's with "all". The answer carrying a secret is never cached.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tokens"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
everyMember is the filter[member] value that lists every member's
tokens.
*/
const everyMember = "all"

/*
TokensOutput is a page of tokens.
*/
type TokensOutput struct {
	Body fhttp.Collection[resources.PersonalAccessToken]
}

/*
TokenOutput is one token, with Cache-Control set when the answer carries
its secret.
*/
type TokenOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         fhttp.Resource[resources.PersonalAccessToken]
}

/*
PersonalAccessTokenController serves the tokens of the workspace in
play.
*/
type PersonalAccessTokenController struct {
	Create  *tokens.CreatePersonalAccessToken
	List    *tokens.ListPersonalAccessTokens
	Show    *tokens.ShowPersonalAccessToken
	Rename  *tokens.RenamePersonalAccessToken
	Revoke  *tokens.RevokePersonalAccessToken
	IDs     *ids.Registry
	Cookies appauth.Cookies
	Cursors *pagination.Cursors
}

/*
tokenPage is where a page of tokens starts, carried in its cursor.
*/
type tokenPage struct {
	Offset int `json:"o"`
}

/*
TokenListing is what the token list accepts: a filter on the member
whose tokens to list.
*/
func TokenListing() pagination.Allow {
	return pagination.Allow{Filters: []string{"member"}}
}

/*
Index lists a page of tokens, newest first: the caller's own, or with
filter[member] one member's or everyone's.
*/
func (c *PersonalAccessTokenController) Index(ctx context.Context, _ *requests.NoInput) (*TokensOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), TokenListing())
	if err != nil {
		return nil, err
	}

	in, err := c.listInput(ctx, query)
	if err != nil {
		return nil, err
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.PersonalAccessToken]{
		Items: resources.NewPersonalAccessTokens(page.Tokens), Limit: in.Limit,
	}
	if page.More {
		if out.NextCursor, err = c.Cursors.Encode(tokenPage{Offset: in.Offset + in.Limit}); err != nil {
			return nil, err
		}
	}

	return &TokensOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Store mints a token and answers with its secret, this once.
*/
func (c *PersonalAccessTokenController) Store(
	ctx context.Context, in *requests.StoreTokenRequest,
) (*TokenOutput, error) {
	made, err := c.Create.Handle(ctx, tokens.CreatePersonalAccessTokenInput{
		User: caller(ctx).User, Membership: membership(ctx), Name: in.Body.Name, Abilities: in.Body.Abilities,
		ExpiresAt: in.Body.ExpiresAt, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &TokenOutput{
		CacheControl: noStore, Body: fhttp.NewResource(resources.NewMintedPersonalAccessToken(made)),
	}, nil
}

/*
ShowToken answers with one token.
*/
func (c *PersonalAccessTokenController) ShowToken(
	ctx context.Context, in *requests.TokenRequest,
) (*TokenOutput, error) {
	id, err := c.tokenID(in.Token)
	if err != nil {
		return nil, err
	}

	tok, err := c.Show.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &TokenOutput{Body: fhttp.NewResource(resources.NewPersonalAccessToken(tok))}, nil
}

/*
Update renames a token and answers with it as it now is.
*/
func (c *PersonalAccessTokenController) Update(
	ctx context.Context, in *requests.UpdateTokenRequest,
) (*TokenOutput, error) {
	id, err := c.tokenID(in.Token)
	if err != nil {
		return nil, err
	}

	tok, err := c.Rename.Handle(ctx, tokens.RenamePersonalAccessTokenInput{
		Membership: membership(ctx), TokenID: id, Name: in.Body.Name,
	})
	if err != nil {
		return nil, err
	}

	return &TokenOutput{Body: fhttp.NewResource(resources.NewPersonalAccessToken(tok))}, nil
}

/*
Destroy revokes a token.
*/
func (c *PersonalAccessTokenController) Destroy(ctx context.Context, in *requests.TokenRequest) (*EmptyOutput, error) {
	id, err := c.tokenID(in.Token)
	if err != nil {
		return nil, err
	}

	err = c.Revoke.Handle(ctx, tokens.RevokePersonalAccessTokenInput{
		Membership: membership(ctx), TokenID: id, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
tokenID decodes a token's public id; one that does not decode is not
found.
*/
func (c *PersonalAccessTokenController) tokenID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixPersonalToken), public)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

/*
listInput turns the list query into the action's input.
*/
func (c *PersonalAccessTokenController) listInput(
	ctx context.Context, query pagination.Query,
) (tokens.ListPersonalAccessTokensInput, error) {
	in := tokens.ListPersonalAccessTokensInput{Membership: membership(ctx), Limit: query.Limit}

	for _, member := range query.Filters["member"] {
		if member == everyMember {
			in.Everyone = true

			continue
		}

		id, err := c.IDs.Decode(string(models.PrefixMember), member)
		if err != nil {
			return in, pagination.ErrInvalidFilter
		}

		in.Member = id
	}

	if query.Cursor != "" {
		var start tokenPage
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return in, err
		}

		in.Offset = start.Offset
	}

	return in, nil
}

/*
TokenAbilitiesOutput is the list of ability groups a token may be given.
*/
type TokenAbilitiesOutput struct {
	Body fhttp.Collection[resources.TokenAbility]
}

/*
TokenAbilityController serves the ability groups a member may give a
token.
*/
type TokenAbilityController struct {
	List *tokens.ListTokenAbilities
}

/*
Index lists every ability group, marked allowed when the caller's role
holds a permission of it.
*/
func (c *TokenAbilityController) Index(ctx context.Context, _ *requests.NoInput) (*TokenAbilitiesOutput, error) {
	items := resources.NewTokenAbilities(c.List.Handle(membership(ctx)))

	return &TokenAbilitiesOutput{
		Body: fhttp.NewCollection(pagination.Page[resources.TokenAbility]{Items: items, Limit: len(items)}),
	}, nil
}

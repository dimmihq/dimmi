/*
|--------------------------------------------------------------------------
| Member Sign-In Help Controller
|--------------------------------------------------------------------------
|
| POST /members/{member}/two-factor-resets mails a teammate who lost
| their second factor a link that turns it off, and DELETE
| /members/{member}/sign-in-lock lifts the lock failed guesses set on
| their account. PATCH /two-factor-resets/{reset} is the teammate using
| the link, from any device, signed in or not: the token is the proof,
| with the password when the account has one. Ids of other workspaces'
| members are simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/members"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TwoFactorResetOutput answers with a reset link.
*/
type TwoFactorResetOutput struct {
	Body fhttp.Resource[resources.TwoFactorReset]
}

/*
MemberSignInHelpController serves an admin helping a teammate back in,
and the teammate finishing it.
*/
type MemberSignInHelpController struct {
	Start    *members.StartTwoFactorReset
	Complete *members.CompleteTwoFactorReset
	Clear    *members.ClearSignInLock
	IDs      *ids.Registry
	Cookies  appauth.Cookies
}

/*
StoreReset mails the member the link that turns their second factor off.
*/
func (c *MemberSignInHelpController) StoreReset(
	ctx context.Context, in *requests.MemberSignInHelpRequest,
) (*TwoFactorResetOutput, error) {
	id, err := c.memberID(in.Member)
	if err != nil {
		return nil, err
	}

	reset, err := c.Start.Handle(ctx, members.StartTwoFactorResetInput{
		Membership: membership(ctx), MemberID: id, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &TwoFactorResetOutput{Body: fhttp.NewResource(resources.NewTwoFactorReset(reset, "pending"))}, nil
}

/*
UpdateReset turns the second factor off with the link's token.
*/
func (c *MemberSignInHelpController) UpdateReset(
	ctx context.Context, in *requests.UpdateTwoFactorResetRequest,
) (*TwoFactorResetOutput, error) {
	reset, err := c.Complete.Handle(ctx, members.CompleteTwoFactorResetInput{
		ID: in.Reset, Token: in.Body.Token, Password: in.Body.Password, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &TwoFactorResetOutput{Body: fhttp.NewResource(resources.NewTwoFactorReset(reset, "completed"))}, nil
}

/*
DestroyLock lifts the sign-in lock of the member's account.
*/
func (c *MemberSignInHelpController) DestroyLock(
	ctx context.Context, in *requests.MemberSignInHelpRequest,
) (*EmptyOutput, error) {
	id, err := c.memberID(in.Member)
	if err != nil {
		return nil, err
	}

	err = c.Clear.Handle(ctx, members.ClearSignInLockInput{Membership: membership(ctx), MemberID: id})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
memberID returns the member a path names.
*/
func (c *MemberSignInHelpController) memberID(named string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixMember), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

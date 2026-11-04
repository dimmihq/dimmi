/*
|--------------------------------------------------------------------------
| Account Controller
|--------------------------------------------------------------------------
|
| A person's own account, by browser or app: GET and PATCH /user read and
| change the profile, PATCH /user/password sets a password, and DELETE
| /user deletes the account. They need no workspace, so a person shut out
| of one can still manage themselves.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
UserOutput is the caller's own profile.
*/
type UserOutput struct {
	Body fhttp.Resource[resources.User]
}

/*
PasswordChangeOutput is the outcome of a new password.
*/
type PasswordChangeOutput struct {
	Body fhttp.Resource[resources.PasswordChange]
}

/*
AccountController serves the caller's own account.
*/
type AccountController struct {
	UpdateProfile  *users.UpdateProfile
	UpdatePassword *users.UpdatePassword
	DeleteAccount  *users.DeleteAccount
	Cookies        appauth.Cookies
}

/*
Show answers with the caller's profile.
*/
func (c *AccountController) Show(ctx context.Context, _ *requests.NoInput) (*UserOutput, error) {
	return &UserOutput{Body: fhttp.NewResource(resources.NewUser(caller(ctx).User))}, nil
}

/*
Update changes the profile and answers with it as it now is.
*/
func (c *AccountController) Update(ctx context.Context, in *requests.UpdateUserRequest) (*UserOutput, error) {
	nulls := requests.Nulls(in.RawBody)

	user, err := c.UpdateProfile.Handle(ctx, users.UpdateProfileInput{
		UserID: caller(ctx).ID(), Name: in.Body.Name, Locale: in.Body.Locale, ClearLocale: nulls["locale"],
		Timezone: in.Body.Timezone, ClearTimezone: nulls["timezone"], Preferences: in.PreferenceChange(),
	})
	if err != nil {
		return nil, err
	}

	return &UserOutput{Body: fhttp.NewResource(resources.NewUser(user))}, nil
}

/*
ChangePassword sets a new password and says how many tokens still work.
*/
func (c *AccountController) ChangePassword(
	ctx context.Context, in *requests.UpdateUserPasswordRequest,
) (*PasswordChangeOutput, error) {
	remaining, err := c.UpdatePassword.Handle(ctx, users.UpdatePasswordInput{
		Principal: caller(ctx), CurrentPassword: in.Body.CurrentPassword, Password: in.Body.Password,
		Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &PasswordChangeOutput{Body: fhttp.NewResource(resources.PasswordChange{
		Object: "password_change", TokensRemaining: remaining,
	})}, nil
}

/*
Destroy deletes the account.
*/
func (c *AccountController) Destroy(ctx context.Context, in *requests.DestroyUserRequest) (*EmptyOutput, error) {
	err := c.DeleteAccount.Handle(ctx, users.DeleteAccountInput{
		Principal: caller(ctx), ConfirmEmail: in.Body.ConfirmEmail,
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

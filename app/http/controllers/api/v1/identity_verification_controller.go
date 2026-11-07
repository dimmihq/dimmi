/*
|--------------------------------------------------------------------------
| Identity Verification Controller
|--------------------------------------------------------------------------
|
| A workspace's visitor sign-in beyond its keys: how strict it is, how
| sign-ins went lately, a checker that explains a pasted token, and
| signing one customer out of every messenger at once.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IdentityVerificationController serves sign-in settings, counts and
checks.
*/
type IdentityVerificationController struct {
	Setup   *identity.Setup
	Stats   *identity.Stats
	Checker *identity.Checker
	Secrets *identity.Secrets
	IDs     *ids.Registry
}

/*
IdentitySettingsOutput is the settings.
*/
type IdentitySettingsOutput struct {
	Body fhttp.Resource[resources.IdentitySettings]
}

/*
IdentityStatsOutput is the counts.
*/
type IdentityStatsOutput struct {
	Body fhttp.Resource[resources.IdentityStats]
}

/*
IdentityValidationOutput is a token check.
*/
type IdentityValidationOutput struct {
	Body fhttp.Resource[resources.IdentityValidation]
}

/*
SignedOutOutput says how many sessions ended.
*/
type SignedOutOutput struct {
	Body struct {
		SessionsRevoked int `json:"sessions_revoked"`
	}
}

/*
ShowSettings answers the settings.
*/
func (c *IdentityVerificationController) ShowSettings(ctx context.Context, _ *requests.NoInput,
) (*IdentitySettingsOutput, error) {
	s, err := c.Setup.Show(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}
	return &IdentitySettingsOutput{Body: fhttp.NewResource(resources.NewIdentitySettings(s))}, nil
}

/*
UpdateSettings changes the settings.
*/
func (c *IdentityVerificationController) UpdateSettings(ctx context.Context,
	in *requests.UpdateIdentitySettingsRequest,
) (*IdentitySettingsOutput, error) {
	s, err := c.Setup.Update(ctx, identity.SetupChange{
		Membership: membership(ctx), Mode: in.Body.Mode, Force: in.Body.Force,
		ClientWritable: in.Body.ClientWritableAttributes,
	})
	if err != nil {
		return nil, err
	}
	return &IdentitySettingsOutput{Body: fhttp.NewResource(resources.NewIdentitySettings(s))}, nil
}

/*
ShowStats answers the counts of the last days.
*/
func (c *IdentityVerificationController) ShowStats(ctx context.Context, in *requests.IdentityStatsRequest,
) (*IdentityStatsOutput, error) {
	days := in.Days
	if days == 0 {
		days = identity.GuardDays
	}
	summary, err := c.Stats.Summarize(ctx, membership(ctx), days)
	if err != nil {
		return nil, err
	}
	return &IdentityStatsOutput{Body: fhttp.NewResource(resources.NewIdentityStats(days, summary))}, nil
}

/*
Check explains a pasted token.
*/
func (c *IdentityVerificationController) Check(ctx context.Context, in *requests.CheckIdentityTokenRequest,
) (*IdentityValidationOutput, error) {
	v, err := c.Checker.Check(ctx, identity.CheckInput{
		Membership: membership(ctx), Token: in.Body.Token, AppID: in.Body.AppID,
	})
	if err != nil {
		return nil, err
	}
	return &IdentityValidationOutput{Body: fhttp.NewResource(resources.NewIdentityValidation(v))}, nil
}

/*
SignOut ends every messenger session of a contact.
*/
func (c *IdentityVerificationController) SignOut(ctx context.Context, in *requests.ContactWidgetSessionsRequest,
) (*SignedOutOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	n, err := c.Secrets.SignOutContact(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	out := &SignedOutOutput{}
	out.Body.SessionsRevoked = n
	return out, nil
}

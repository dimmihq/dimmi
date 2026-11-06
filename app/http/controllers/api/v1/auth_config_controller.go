/*
|--------------------------------------------------------------------------
| Auth Config Controller
|--------------------------------------------------------------------------
|
| Anyone may ask how to sign in on this server, before signing in. The
| answer holds no secret: only what a sign-in screen needs to show.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/config"
)

/*
AuthConfigOutput is the sign-in options of the server.
*/
type AuthConfigOutput struct {
	Body fhttp.Resource[resources.AuthConfig]
}

/*
AuthConfigController serves GET /auth/config. Factors are the second-factor
types the install enables.
*/
type AuthConfigController struct {
	Auth      config.AuthConfig
	Factors   []string
	Providers providers.ProviderResolver
	Server    Server
	SSO       appauth.OrganizationLoginPolicy
}

/*
Show answers with the sign-in options.
*/
func (c *AuthConfigController) Show(ctx context.Context, _ *requests.NoInput) (*AuthConfigOutput, error) {
	sso, err := c.SSO.Available(ctx)
	if err != nil {
		return nil, err
	}

	return &AuthConfigOutput{Body: fhttp.NewResource(resources.AuthConfig{
		SSOAvailable:      sso,
		Object:            "auth_config",
		Edition:           c.Server.Edition,
		ServerVersion:     c.Server.Version,
		APIVersion:        c.Server.APIVersion,
		Region:            c.Server.Region,
		SignupMode:        c.Auth.Signup.Mode,
		PasswordLogin:     true,
		MagicLink:         c.Auth.MagicLink.Enabled,
		Captcha:           resources.NewCaptcha(c.Auth.Captcha),
		MailConfigured:    c.Server.MailConfigured,
		PasswordMinLength: c.Auth.Passwords.MinLength,
		Providers:         resources.NewProviders(ctx, c.Providers),
		Factors:           resources.NewFactors(appauth.OfferedFactors(c.Factors)),
		Capabilities:      appauth.Capabilities(c.Auth),
		MinClientVersions: map[string]*string{"ios": nil, "android": nil, "cli": nil},
		Locales:           resources.NewLocales(ctx, i18n.From(ctx)),
	})}, nil
}

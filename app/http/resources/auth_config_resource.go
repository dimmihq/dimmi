/*
|--------------------------------------------------------------------------
| Auth Config Resource
|--------------------------------------------------------------------------
|
| What an app learns before anyone signs in: how people may sign in on
| this server, which version of the API it speaks, and what it can do.
| An app built for a newer server hides what this one lacks instead of
| calling endpoints that are not there. It also lists the languages the
| server is served in, each by its own name, so a picker reads well to
| someone who does not know the current one.
|
*/

package resources

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
)

/*
AuthConfig is the sign-in options and capabilities of the server.
MinClientVersions maps each app platform to the oldest version this
server accepts, null when any version is. Locales lists the languages
people may pick, the default first.
*/
type AuthConfig struct {
	Object            string             `json:"object"`
	Edition           string             `json:"edition"`
	ServerVersion     string             `json:"server_version"`
	APIVersion        string             `json:"api_version"`
	Region            string             `json:"region"`
	SetupRequired     bool               `json:"setup_required"`
	SignupMode        string             `json:"signup_mode"`
	PasswordLogin     bool               `json:"password_login"`
	MagicLink         bool               `json:"magic_link"`
	Captcha           *string            `json:"captcha"`
	MailConfigured    bool               `json:"mail_configured"`
	PasswordMinLength int                `json:"password_min_length"`
	Providers         []Provider         `json:"providers"`
	Factors           []Factor           `json:"factors"`
	Capabilities      []string           `json:"capabilities"`
	MinClientVersions map[string]*string `json:"min_client_versions"`
	Locales           []Locale           `json:"locales"`
}

/*
Locale is a language people may pick: its BCP 47 code and its name
written in that language, such as "Deutsch" for de.
*/
type Locale struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

/*
NewLocales lists the languages the translator serves, each named in its
own language.
*/
func NewLocales(ctx context.Context, t *i18n.Translator) []Locale {
	codes := t.Locales()
	out := make([]Locale, 0, len(codes))

	for _, code := range codes {
		out = append(out, Locale{Code: code, Name: t.T(i18n.WithLocale(ctx, code), "meta.name", nil)})
	}

	return out
}

/*
Provider is a sign-in provider and where its sign-in starts.
*/
type Provider struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	RedirectURL string `json:"redirect_url"`
}

/*
Factor is a second-factor type people may enroll.
*/
type Factor struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

/*
NewProviders shapes the offered sign-in providers.
*/
func NewProviders(ctx context.Context, resolver providers.ProviderResolver) []Provider {
	offered := resolver.Offered(ctx)

	out := make([]Provider, 0, len(offered))
	for _, p := range offered {
		out = append(out, Provider{
			Key: p.Key(), Name: p.Display().Name, Protocol: p.Protocol(), RedirectURL: "/auth/" + p.Key() + "/redirect",
		})
	}

	return out
}

/*
NewFactors shapes the enrollable factor types.
*/
func NewFactors(factors []appauth.Factor) []Factor {
	out := make([]Factor, 0, len(factors))
	for _, f := range factors {
		out = append(out, Factor{Type: f.Type, Name: f.Name})
	}

	return out
}

/*
|--------------------------------------------------------------------------
| Integration Registry
|--------------------------------------------------------------------------
|
| Every tool dimmi connects to is a provider here, and every feature that
| needs a provider declares the scopes it uses at boot. An install asks
| for the union of the scopes of the features it is for, so one consent
| covers them all, and an installation missing a scope a feature needs is
| shown as such instead of failing in silence. Scopes are never guessed:
| a feature that did not register asks for nothing.
|
| Bot scopes act as the app; user scopes act as one teammate and are only
| asked for when a teammate connects their own account.
|
*/

package oauth

import (
	"context"
	"errors"
	"slices"

	"github.com/dimmihq/dimmi/framework/oauthclient"
)

/*
ErrAuthFailed is what a health check returns when the provider refuses
the installation's token, so the installation no longer works.
*/
var ErrAuthFailed = errors.New("integrations: the provider refused the token")

/*
Scopes are what a feature needs from a provider: Bot as the app, User as
a teammate.
*/
type Scopes struct {
	Bot  []string
	User []string
}

/*
Feature is a part of the product that uses a provider, with the scopes
it registered.
*/
type Feature struct {
	Name   string
	Scopes Scopes
}

/*
Health is what a provider says about a working token: the account it
belongs to, its display name and the app it was issued to, the scopes it
carries when the provider tells, and facts worth keeping in the
installation's metadata.
*/
type Health struct {
	Account     string
	AccountName string
	App         string
	Scopes      []string
	Metadata    map[string]any
}

/*
Provider is one tool. Name is its registry value and Label how people
see it. Check asks the provider whether an app token still works and
returns ErrAuthFailed when it does not. Metadata picks the facts worth
keeping from a fresh grant, and Validate refuses a grant the product
cannot use, such as an install it does not support. Hint adds
parameters to a teammate's authorization URL that keep them on the
installation's account. Identify names the account of a grant whose
token response does not, by asking the provider with the token.
AuthParams are added to every authorization URL. Separate providers
connect each account through its own consent, so a workspace can hold
several, such as two help desks, instead of reusing the one it has.
*/
type Provider struct {
	Name       string
	Label      string
	Check      func(ctx context.Context, token string) (Health, error)
	Metadata   func(grant oauthclient.Grant) map[string]any
	Validate   func(grant oauthclient.Grant) error
	Hint       func(account string) map[string]string
	Identify   func(ctx context.Context, token string) (Health, error)
	AuthParams map[string]string
	Separate   bool
}

/*
Registry holds the providers and the scopes each feature registered. It
is filled at boot and only read afterwards.
*/
type Registry struct {
	providers map[string]Provider
	order     []string
	features  map[string][]Feature
}

/*
NewRegistry returns an empty registry.
*/
func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}, features: map[string][]Feature{}}
}

/*
Register adds the provider, replacing one of the same name.
*/
func (r *Registry) Register(p Provider) {
	if _, ok := r.providers[p.Name]; !ok {
		r.order = append(r.order, p.Name)
	}

	r.providers[p.Name] = p
}

/*
RegisterScopes records the scopes a feature needs from the provider.
Registering the same feature again adds to what it already asked for.
*/
func (r *Registry) RegisterScopes(provider, feature string, s Scopes) {
	features := r.features[provider]

	i := slices.IndexFunc(features, func(f Feature) bool { return f.Name == feature })
	if i < 0 {
		r.features[provider] = append(features, Feature{Name: feature, Scopes: normalized(s)})

		return
	}

	features[i].Scopes = normalized(Scopes{
		Bot: slices.Concat(features[i].Scopes.Bot, s.Bot), User: slices.Concat(features[i].Scopes.User, s.User),
	})
}

/*
Provider returns the named provider.
*/
func (r *Registry) Provider(name string) (Provider, bool) {
	p, ok := r.providers[name]

	return p, ok
}

/*
Providers returns every provider, in the order they registered.
*/
func (r *Registry) Providers() []Provider {
	out := make([]Provider, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.providers[name])
	}

	return out
}

/*
Features returns the features that use the provider, in the order they
registered.
*/
func (r *Registry) Features(provider string) []Feature {
	return slices.Clone(r.features[provider])
}

/*
HasFeature reports whether the feature registered scopes for the
provider.
*/
func (r *Registry) HasFeature(provider, feature string) bool {
	return slices.ContainsFunc(r.features[provider], func(f Feature) bool { return f.Name == feature })
}

/*
FeatureNames returns the names of the features of the provider, or of
only those listed when any are.
*/
func (r *Registry) FeatureNames(provider string, only []string) []string {
	var names []string

	for _, f := range r.features[provider] {
		if len(only) == 0 || slices.Contains(only, f.Name) {
			names = append(names, f.Name)
		}
	}

	return names
}

/*
BotScopes returns the union of the bot scopes the features need, every
feature of the provider when none are named, sorted.
*/
func (r *Registry) BotScopes(provider string, features []string) []string {
	var all []string

	for _, f := range r.features[provider] {
		if len(features) == 0 || slices.Contains(features, f.Name) {
			all = append(all, f.Scopes.Bot...)
		}
	}

	return unique(all)
}

/*
UserScopes returns the union of the user scopes every feature of the
provider needs, sorted.
*/
func (r *Registry) UserScopes(provider string) []string {
	var all []string
	for _, f := range r.features[provider] {
		all = append(all, f.Scopes.User...)
	}

	return unique(all)
}

/*
Missing returns the scopes needed that were not granted, sorted.
*/
func Missing(needed, granted []string) []string {
	var out []string

	for _, scope := range unique(needed) {
		if !slices.Contains(granted, scope) {
			out = append(out, scope)
		}
	}

	return out
}

func normalized(s Scopes) Scopes {
	return Scopes{Bot: unique(s.Bot), User: unique(s.User)}
}

func unique(scopes []string) []string {
	out := slices.Clone(scopes)
	slices.Sort(out)

	return slices.Compact(out)
}

/*
|--------------------------------------------------------------------------
| Provider Registry
|--------------------------------------------------------------------------
|
| Drivers register once at boot, each under its own key. A registered
| driver is only offered once the install configures it, so a button is
| never shown for a provider that cannot work, and its routes answer
| as if it did not exist.
|
*/

package providers

import (
	"context"
	"fmt"
	"slices"
)

/*
Registry holds the registered drivers, in registration order.
*/
type Registry struct {
	drivers []IdentityProvider
}

/*
NewRegistry returns an empty registry.
*/
func NewRegistry() *Registry {
	return &Registry{}
}

/*
Register adds the drivers. A key registered twice is an error, which
stops the boot.
*/
func (r *Registry) Register(drivers ...IdentityProvider) error {
	for _, d := range drivers {
		if slices.Contains(r.Keys(), d.Key()) {
			return fmt.Errorf("register identity provider %q: key already registered", d.Key())
		}

		r.drivers = append(r.drivers, d)
	}

	return nil
}

/*
Keys returns the key of every registered driver.
*/
func (r *Registry) Keys() []string {
	keys := make([]string, 0, len(r.drivers))
	for _, d := range r.drivers {
		keys = append(keys, d.Key())
	}

	return keys
}

/*
Static resolves providers from the registry, offering the drivers the
configuration turned on.
*/
type Static struct {
	Registry   *Registry
	Configured func(key string) bool
}

var _ ProviderResolver = Static{}

/*
Find returns the configured driver with the key.
*/
func (s Static) Find(_ context.Context, key string) (IdentityProvider, error) {
	for _, d := range s.Registry.drivers {
		if d.Key() == key && s.Configured(key) {
			return d, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, key)
}

/*
Offered returns the configured drivers in registration order.
*/
func (s Static) Offered(context.Context) []IdentityProvider {
	return slices.DeleteFunc(slices.Clone(s.Registry.drivers), func(d IdentityProvider) bool {
		return !s.Configured(d.Key())
	})
}

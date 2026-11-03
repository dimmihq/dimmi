/*
|--------------------------------------------------------------------------
| Factor Registry
|--------------------------------------------------------------------------
|
| Drivers register once at boot and the configuration chooses which are
| enabled. A type the configuration names without a driver stops the
| boot, since a typo there would otherwise switch a factor off silently.
|
| A disabled driver's factors stay on their users: two-factor
| authentication never turns itself off. They are only left out of
| sign-in, and users may still see and remove them.
|
*/

package factors

import (
	"fmt"
	"slices"
)

/*
Registry holds the registered drivers and the enabled types.
*/
type Registry struct {
	drivers []Factor
	enabled []string
}

/*
NewRegistry returns the registry of the drivers with the types enabled,
or ErrUnknownFactor for an enabled type no driver registers.
*/
func NewRegistry(enabled []string, drivers ...Factor) (*Registry, error) {
	r := &Registry{drivers: drivers}

	for _, kind := range enabled {
		if _, ok := r.Known(kind); !ok {
			return nil, fmt.Errorf("%w: %q in auth.factors.enabled", ErrUnknownFactor, kind)
		}

		if !slices.Contains(r.enabled, kind) {
			r.enabled = append(r.enabled, kind)
		}
	}

	return r, nil
}

/*
Known returns the registered driver of the type, enabled or not.
*/
func (r *Registry) Known(kind string) (Factor, bool) {
	for _, d := range r.drivers {
		if d.Type() == kind {
			return d, true
		}
	}

	return nil, false
}

/*
Find returns the enabled driver of the type.
*/
func (r *Registry) Find(kind string) (Factor, bool) {
	if !slices.Contains(r.enabled, kind) {
		return nil, false
	}

	return r.Known(kind)
}

/*
Enabled returns the enabled drivers, in the order the configuration
lists them.
*/
func (r *Registry) Enabled() []Factor {
	out := make([]Factor, 0, len(r.enabled))
	for _, kind := range r.enabled {
		d, _ := r.Known(kind)
		out = append(out, d)
	}

	return out
}

/*
EnablesTwoFactor reports whether a factor of the type turns two-factor
authentication on, enabled or not, so a disabled authenticator still
keeps its user protected.
*/
func (r *Registry) EnablesTwoFactor(kind string) bool {
	d, ok := r.Known(kind)

	return ok && EnablesTwoFactor(d)
}

/*
|--------------------------------------------------------------------------
| Source Registry
|--------------------------------------------------------------------------
|
| The drivers an install offers, registered at boot and only read
| afterwards. A source's type is checked against it when created, and
| the dashboard's gallery lists what it holds, in registration order.
|
*/

package sources

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

/*
The ways registering a driver fails: a type that is not lowercase with
underscores, one already taken, or a description missing what the
engine relies on.
*/
var (
	ErrInvalidType = errors.New("sources: type must be lowercase letters, digits and underscores")
	ErrDuplicate   = errors.New("sources: type already registered")
	ErrIncomplete  = errors.New("sources: description needs a name, a schema, modes and targets")
)

/*
typePattern is a valid source type, the same rule the table checks.
*/
var typePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)

/*
Registry holds the drivers by type.
*/
type Registry struct {
	drivers map[string]Driver
	order   []string
}

/*
NewRegistry returns an empty registry.
*/
func NewRegistry() *Registry {
	return &Registry{drivers: map[string]Driver{}}
}

/*
Register adds a driver under the type its description names.
*/
func (r *Registry) Register(d Driver) error {
	desc := d.Describe()
	if !typePattern.MatchString(desc.Type) {
		return fmt.Errorf("register %q: %w", desc.Type, ErrInvalidType)
	}

	if _, taken := r.drivers[desc.Type]; taken {
		return fmt.Errorf("register %q: %w", desc.Type, ErrDuplicate)
	}

	if desc.Name == "" || len(desc.Schema) == 0 || len(desc.Modes) == 0 || len(desc.Targets) == 0 {
		return fmt.Errorf("register %q: %w", desc.Type, ErrIncomplete)
	}

	r.drivers[desc.Type] = d
	r.order = append(r.order, desc.Type)

	return nil
}

/*
Driver returns the driver of the type.
*/
func (r *Registry) Driver(kind string) (Driver, bool) {
	d, ok := r.drivers[kind]

	return d, ok
}

/*
Descriptions lists every driver's description in registration order.
*/
func (r *Registry) Descriptions() []Description {
	out := make([]Description, 0, len(r.order))
	for _, kind := range r.order {
		out = append(out, r.drivers[kind].Describe())
	}

	return out
}

/*
Supports reports whether the description allows the mode.
*/
func (d Description) Supports(mode Mode) bool {
	return slices.Contains(d.Modes, mode)
}

/*
Feeds reports whether the description allows the target.
*/
func (d Description) Feeds(target Target) bool {
	return slices.Contains(d.Targets, target)
}

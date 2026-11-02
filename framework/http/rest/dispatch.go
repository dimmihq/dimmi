/*
|--------------------------------------------------------------------------
| Field Dispatch
|--------------------------------------------------------------------------
|
| State changes are fields on the resource, so one PATCH may carry plain
| edits next to a status change. Each state field goes to its own
| handler, which checks the transition and fires its own events, along
| with the side parameters that only make sense with it, such as when a
| pause ends. Everything else goes to the plain update. All of it runs
| in one transaction: the request succeeds or nothing changes.
|
*/

package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
Changes are the fields of a PATCH body, still encoded.
*/
type Changes = map[string]json.RawMessage

/*
FieldHandler applies changes to a resource.
*/
type FieldHandler[R any] func(ctx context.Context, resource R, changes Changes) error

/*
Transactor runs a function in a transaction, as the database pool does.
*/
type Transactor interface {
	/*
		Transaction commits when the function returns nil and rolls back
		otherwise.
	*/
	Transaction(ctx context.Context, fn func(ctx context.Context, tx *database.Tx) error) error
}

/*
Dispatcher routes the fields of an update.

Plain receives every field no other handler claims; without it such
fields are refused. Fields maps a state field to its handler. Params
lists, per state field, the side parameters that travel with it; they
reach the field's handler only when the field itself is present. DB
runs the whole update in one transaction; without it the handlers run
on their own.
*/
type Dispatcher[R any] struct {
	Plain  FieldHandler[R]
	Fields map[string]FieldHandler[R]
	Params map[string][]string
	DB     Transactor
}

/*
Update applies the changes: the plain fields first, then each state
field in name order.
*/
func (d Dispatcher[R]) Update(ctx context.Context, resource R, changes Changes) error {
	plain, fields := d.split(changes)
	if len(plain) > 0 && d.Plain == nil {
		return unknownFields(plain)
	}

	apply := func(ctx context.Context) error {
		if len(plain) > 0 {
			if err := d.Plain(ctx, resource, plain); err != nil {
				return err
			}
		}

		for _, name := range slices.Sorted(maps.Keys(fields)) {
			if err := d.Fields[name](ctx, resource, fields[name]); err != nil {
				return fmt.Errorf("change %s: %w", name, err)
			}
		}

		return nil
	}

	if d.DB == nil {
		return apply(ctx)
	}

	return d.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error { return apply(ctx) })
}

/*
split separates the plain fields from each present state field and its
side parameters.
*/
func (d Dispatcher[R]) split(changes Changes) (Changes, map[string]Changes) {
	plain := maps.Clone(changes)
	fields := map[string]Changes{}

	for name := range d.Fields {
		value, ok := changes[name]
		if !ok {
			continue
		}

		own := Changes{name: value}
		delete(plain, name)

		for _, param := range d.Params[name] {
			if v, ok := changes[param]; ok {
				own[param] = v
				delete(plain, param)
			}
		}

		fields[name] = own
	}

	return plain, fields
}

func unknownFields(plain Changes) error {
	invalid := &validation.Error{}
	for _, name := range slices.Sorted(maps.Keys(plain)) {
		invalid.Violations = append(invalid.Violations, validation.Violation{
			Field: name, Rule: "prohibited", Message: "The " + name + " field cannot be changed.",
		})
	}

	return invalid
}

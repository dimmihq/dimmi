/*
|--------------------------------------------------------------------------
| Applying One Record
|--------------------------------------------------------------------------
|
| How a record of any kind is read, checked and applied: its step holds
| the transaction and the records it named, and its outcome what it
| made, with warnings and conflicts.
|
*/

package migrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
step is one record being applied: its transaction, the references it
named, its earlier ledger row and the server's time.
*/
type step struct {
	runner *runner
	tx     *database.Tx
	q      *models.Queries
	refs   resolved
	prev   *models.MigrationRecord
	rec    record
	now    time.Time
	ledger *ticketLedger
}

/*
ws returns the workspace the step writes in.
*/
func (s *step) ws() ids.UUID {
	return s.runner.row.WorkspaceID
}

/*
outcome is what applying a record made: the row it became, whether the
migration created it, a label for records that name it, what a later
sync needs to remember, and the warnings and conflicts.
*/
type outcome struct {
	id        ids.UUID
	created   bool
	label     string
	state     json.RawMessage
	warnings  []Notice
	conflicts []Conflict
}

/*
warn adds a warning to the outcome.
*/
func (o *outcome) warn(field, code, message string) {
	o.warnings = append(o.warnings, Notice{Field: field, Code: code, Message: message})
}

/*
unsupported warns that a field is not applied yet.
*/
func (o *outcome) unsupported(field string) {
	o.warn(field, "not_supported_yet", "dimmi does not apply this field yet; it was left out.")
}

/*
item is a decoded record of any kind: the records it names, and how it
is applied.
*/
type item interface {
	refs() []Ref
	apply(ctx context.Context, s *step) (outcome, error)
}

/*
kindSpec reads a record of one kind.
*/
type kindSpec struct {
	decode func(raw json.RawMessage) (item, Base, error)
}

/*
validator is a record that checks itself once decoded.
*/
type validator interface {
	validate() error
}

/*
decoder returns the reader of records of the type: decoded, its source
id checked, then checked itself.
*/
func decoder[T any, P interface {
	*T
	item
	validator
	base() Base
}]() kindSpec {
	return kindSpec{decode: func(raw json.RawMessage) (item, Base, error) {
		var v T

		p := P(&v)

		d := json.NewDecoder(bytes.NewReader(raw))
		if err := d.Decode(p); err != nil {
			var peek Base
			_ = json.Unmarshal(raw, &peek)

			return nil, peek, invalid("", "The record could not be read: "+jsonProblem(err))
		}

		b := p.base()
		b.SourceID = strings.TrimSpace(b.SourceID)

		if b.SourceID == "" || len(b.SourceID) > 255 {
			return nil, b, invalid("source_id", "Every record has a source_id of 1 to 255 characters.")
		}

		if err := p.validate(); err != nil {
			return nil, b, err
		}

		return p, b, nil
	}}
}

/*
jsonProblem says what was wrong with the JSON.
*/
func jsonProblem(err error) string {
	if te, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return fmt.Sprintf("%s has the wrong type.", te.Field)
	}

	return "it is not valid JSON."
}

/*
invalid returns a record's refusal of the field.
*/
func invalid(field, message string) error {
	f := fault.New(fault.Invalid, "migration_record_invalid", message)
	if field != "" {
		f = f.OnField(field)
	}

	return f
}

/*
required refuses an empty text field.
*/
func required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalid(field, field+" is required.")
	}

	return nil
}

/*
oneOf refuses a value outside the choices, empty allowed.
*/
func oneOf(field, value string, choices ...string) error {
	if value == "" || slices.Contains(choices, value) {
		return nil
	}

	return invalid(field, field+" is one of "+strings.Join(choices, ", ")+".")
}

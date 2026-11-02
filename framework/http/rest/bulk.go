/*
|--------------------------------------------------------------------------
| Bulk Changes
|--------------------------------------------------------------------------
|
| A bulk change is a PATCH on the collection carrying the ids and the
| changes, the same changes a PATCH on one member accepts, so bulk stays
| a strict subset of the single update. Each id is changed on its own and
| reported on its own: some may succeed while others are skipped or fail,
| and the response says which, never hiding a failure. Only a fault is a
| failure of one id; anything else is a bug or an outage and stops the
| whole run.
|
*/

package rest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
MaxBulkIDs is the most ids one bulk request may name.
*/
const MaxBulkIDs = 100

/*
The outcomes of one id in a bulk change.
*/
const (
	StatusUpdated = "updated"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
)

/*
ErrSkip is returned by a per-id function when the id needs no change,
such as closing an order that is already closed.
*/
var ErrSkip = errors.New("rest: skipped")

/*
BulkRequest is the body of a PATCH on a collection: the ids, or a filter
where the resource allows one, and the changes.

Filter is parsed and checked for shape here but never applied: only the
resource knows the fields it filters by and how to query them. A route
whose resource allows changes by filter handles a request with Filter
itself, running the change in its own query and answering a BulkResult
with only Meta; Bulk applies ids.
*/
type BulkRequest[C any] struct {
	IDs     []string       `json:"ids,omitempty"`
	Filter  map[string]any `json:"filter,omitempty"`
	Changes C              `json:"changes"`
}

/*
BulkItem is the outcome for one id. Error is set when it failed.
*/
type BulkItem struct {
	ID     string              `json:"id"`
	Status string              `json:"status"`
	Error  *http.ResponseError `json:"error,omitempty"`
}

/*
BulkMeta counts the outcomes.
*/
type BulkMeta struct {
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

/*
BulkResult is the response to a bulk change. A change by filter reports
only the counts.
*/
type BulkResult struct {
	Data []BulkItem `json:"data,omitempty"`
	Meta BulkMeta   `json:"meta"`
}

/*
Validate checks the shape of the request: ids or a filter but not both,
and at most MaxBulkIDs ids, none empty.
*/
func (req BulkRequest[C]) Validate() error {
	switch {
	case len(req.IDs) == 0 && len(req.Filter) == 0:
		return invalid("ids", "required", "", "The ids field is required when there is no filter.")
	case len(req.IDs) > 0 && len(req.Filter) > 0:
		return invalid("filter", "excluded_with", "ids", "The filter field cannot be combined with ids.")
	case len(req.IDs) > MaxBulkIDs:
		return invalid("ids", "max", strconv.Itoa(MaxBulkIDs),
			fmt.Sprintf("The ids field must not have more than %d items.", MaxBulkIDs))
	case slices.Contains(req.IDs, ""):
		return invalid("ids", "required", "", "The ids field must not contain empty ids.")
	default:
		return nil
	}
}

/*
Bulk applies the changes to each id of the request in order, once per
distinct id. A fault from perID marks the id failed with the fault as
its error, and ErrSkip marks it skipped. Any other error stops the run
and is returned with the outcomes so far. A request by filter reaching
Bulk is refused on ids, since the route resolves filters before it.
*/
func Bulk[C any](
	ctx context.Context, req BulkRequest[C], perID func(ctx context.Context, id string, changes C) error,
) (BulkResult, error) {
	if err := req.Validate(); err != nil {
		return BulkResult{}, err
	}

	if len(req.IDs) == 0 {
		return BulkResult{}, invalid("ids", "required", "", "The ids field is required.")
	}

	var result BulkResult

	seen := map[string]bool{}
	for _, id := range req.IDs {
		if seen[id] {
			continue
		}

		seen[id] = true

		item, err := outcome(ctx, id, perID(ctx, id, req.Changes))
		if err != nil {
			return result, fmt.Errorf("bulk change %s: %w", id, err)
		}

		result.add(item)
	}

	return result, nil
}

func (r *BulkResult) add(item BulkItem) {
	r.Data = append(r.Data, item)

	switch item.Status {
	case StatusUpdated:
		r.Meta.Updated++
	case StatusSkipped:
		r.Meta.Skipped++
	default:
		r.Meta.Failed++
	}
}

/*
outcome classifies the error of one id, returning it back when it is
not the id's own failure.
*/
func outcome(ctx context.Context, id string, err error) (BulkItem, error) {
	if err == nil {
		return BulkItem{ID: id, Status: StatusUpdated}, nil
	}

	if errors.Is(err, ErrSkip) {
		return BulkItem{ID: id, Status: StatusSkipped}, nil
	}

	_, isValidation := errors.AsType[*validation.Error](err)
	if fault.KindOf(err) == fault.Internal && !isValidation {
		return BulkItem{}, err
	}

	return BulkItem{ID: id, Status: StatusFailed, Error: http.Errors{}.Resolve(ctx, err)}, nil
}

/*
invalid builds a validation error for one field of the request.
*/
func invalid(field, rule, param, message string) error {
	return &validation.Error{Violations: []validation.Violation{
		{Field: field, Rule: rule, Param: param, Message: message},
	}}
}

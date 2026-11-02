package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http/rest"
	"github.com/dimmihq/dimmi/framework/validation"
)

type statusChange struct {
	Status string `json:"status"`
}

/*
closeOne closes tasks by id: "open" ones change, "closed" ones
are skipped, "locked" ones fail with a fault, "invalid" ones fail
validation and "broken" ones hit a bug.
*/
func closeOne(_ context.Context, id string, _ statusChange) error {
	switch id {
	case "closed":
		return rest.ErrSkip
	case "locked":
		return fault.New(fault.Conflict, "invalid_transition", "Cannot close.").With("from", "locked")
	case "invalid":
		return &validation.Error{Violations: []validation.Violation{{Field: "status", Message: "Bad."}}}
	case "broken":
		return errors.New("disk on fire")
	default:
		return nil
	}
}

func fields(t *testing.T, err error) map[string][]string {
	t.Helper()

	invalid, ok := errors.AsType[*validation.Error](err)
	require.True(t, ok, "expected a validation error, got %v", err)

	return invalid.Messages()
}

func TestBulk(t *testing.T) {
	t.Parallel()

	t.Run("reports each id and counts the outcomes", func(t *testing.T) {
		t.Parallel()
		req := rest.BulkRequest[statusChange]{
			IDs:     []string{"open", "closed", "locked", "open", "invalid"},
			Changes: statusChange{Status: "closed"},
		}

		result, err := rest.Bulk(t.Context(), req, closeOne)
		require.NoError(t, err)

		body, err := json.Marshal(result)
		require.NoError(t, err)
		assert.JSONEq(t, `{
			"data": [
				{"id": "open", "status": "updated"},
				{"id": "closed", "status": "skipped"},
				{"id": "locked", "status": "failed",
				 "error": {"message": "Cannot close.", "code": "invalid_transition", "from": "locked"}},
				{"id": "invalid", "status": "failed",
				 "error": {"message": "Bad.", "code": "validation_failed", "errors": {"status": ["Bad."]}}}
			],
			"meta": {"updated": 1, "skipped": 1, "failed": 2}
		}`, string(body))
	})

	t.Run("stops at errors that are not faults", func(t *testing.T) {
		t.Parallel()
		req := rest.BulkRequest[statusChange]{IDs: []string{"open", "broken", "open2"}}

		result, err := rest.Bulk(t.Context(), req, closeOne)
		require.ErrorContains(t, err, "bulk change broken: disk on fire")
		assert.Equal(t, 1, result.Meta.Updated)
	})

	t.Run("needs ids", func(t *testing.T) {
		t.Parallel()
		byFilter := rest.BulkRequest[statusChange]{Filter: map[string]any{"status": "open"}}
		_, err := rest.Bulk(t.Context(), byFilter, closeOne)

		assert.Contains(t, fields(t, err), "ids")
	})

	t.Run("refuses malformed requests", func(t *testing.T) {
		t.Parallel()
		many := make([]string, rest.MaxBulkIDs+1)
		for i := range many {
			many[i] = strconv.Itoa(i)
		}

		cases := map[string]rest.BulkRequest[statusChange]{
			"ids":    {},
			"filter": {IDs: []string{"a"}, Filter: map[string]any{"status": "open"}},
		}
		for field, req := range cases {
			assert.Contains(t, fields(t, req.Validate()), field)
		}

		assert.Equal(t, []string{"The ids field must not have more than 100 items."},
			fields(t, rest.BulkRequest[statusChange]{IDs: many}.Validate())["ids"])
		assert.Contains(t, fields(t, rest.BulkRequest[statusChange]{IDs: []string{"a", ""}}.Validate()), "ids")
	})

	t.Run("accepts a filter alone", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, rest.BulkRequest[statusChange]{Filter: map[string]any{"status": "open"}}.Validate())
	})
}

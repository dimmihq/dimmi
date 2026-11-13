package tickets_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
typeStates reads a type's states in its order.
*/
func (d desk) typeStates(t *testing.T, typeID string) []string {
	t.Helper()

	res := d.Owner.GetJSON("/api/v1/ticket-types/" + typeID).AssertOK()

	var out []string
	for i := 0; res.String(path("data.state_ids", i)) != ""; i++ {
		out = append(out, res.String(path("data.state_ids", i)))
	}

	return out
}

func TestTicketTypeStates(t *testing.T) {
	t.Parallel()

	t.Run("keeps each type's own order of its states", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		task, bug := d.types["Back-office task"], d.types["Bug report"]
		before := d.typeStates(t, bug)
		reversed := slices.Clone(d.typeStates(t, task))
		slices.Reverse(reversed)

		d.Owner.PatchJSON("/api/v1/ticket-types/"+task, testkit.JSON{"state_ids": reversed}).AssertOK()

		assert.Equal(t, reversed, d.typeStates(t, task))
		assert.Equal(t, before, d.typeStates(t, bug))
	})

	t.Run("moves the tickets of a state taken off the type to the state named", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		task := d.types["Back-office task"]
		id := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{"type_id": task, "title": "Refund 8812"}).
			AssertCreated().String("data.id")
		from := d.Owner.GetJSON("/api/v1/tickets/" + id).AssertOK().String("data.state_id")
		d.Owner.GetJSON("/api/v1/ticket-types/"+task).AssertOK().
			AssertJSONPath("data.tickets_by_state."+from, float64(1))

		kept := slices.DeleteFunc(d.typeStates(t, task), func(s string) bool { return s == from })
		require.NotEmpty(t, kept)

		d.Owner.PatchJSON("/api/v1/ticket-types/"+task, testkit.JSON{"state_ids": kept}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "tickets_need_state")
		d.Owner.PatchJSON("/api/v1/ticket-types/"+task, testkit.JSON{
			"state_ids": kept, "move_tickets": []testkit.JSON{{"from_state_id": from, "to_state_id": kept[0]}},
		}).AssertOK()

		d.Owner.GetJSON("/api/v1/tickets/"+id).AssertOK().AssertJSONPath("data.state_id", kept[0])
	})
}

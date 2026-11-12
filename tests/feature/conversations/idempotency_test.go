package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestIdempotentCreates(t *testing.T) {
	t.Parallel()

	t.Run("runs a note once per key and keeps keys apart per caller", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "Ana", "Refund", "Please refund me.")
		path := "/api/v1/conversations/" + id + "/notes"
		body := testkit.JSON{"body": "Checking with billing."}

		first := d.owner.WithHeader("Idempotency-Key", "k1").PostJSON(path, body).AssertCreated().String("data.id")
		again := d.owner.WithHeader("Idempotency-Key", "k1").PostJSON(path, body).AssertCreated().
			AssertHeader("Idempotent-Replayed", "true").String("data.id")
		assert.Equal(t, first, again)

		admin, _ := d.join(t, enums.RoleAdmin)
		other := admin.WithHeader("Idempotency-Key", "k1").PostJSON(path, body).AssertCreated().String("data.id")
		assert.NotEqual(t, first, other)
	})
}

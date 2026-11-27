package migrations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestRepliesByTeammatesWithoutMembers(t *testing.T) {
	t.Parallel()
	d := open(t)

	d.send(t, "teammate", testkit.JSON{"source_id": "adm-9", "email": "gone@example.com", "name": "Grace"}).
		AssertJSONPath("data.results.0.status", "unsupported")
	d.send(t, "contact", contact("c-1", "ada@example.com"))

	d.send(t, "conversation", conversation(message("p1", 0, "Hello?"), reply("p2", 1, "Sent it.", "adm-9"))).
		AssertJSONPath("data.results.0.status", "created")
	d.send(t, "conversation", conversation(
		message("p1", 0, "Hello?"), reply("p2", 1, "Sent it.", "adm-9"), reply("p3", 2, "Anything else?", "adm-9"),
	)).AssertJSONPath("data.results.0.status", "updated")

	replies := `type = 'message' AND author_type = 'system' AND delivery_status = 'delivered'
		AND metadata->>'author_name' = 'Grace (imported)'`
	assert.Equal(t, 2, d.rows(t, "messages", replies), "a reply by a teammate with no member is not a reply")
	assert.Equal(t, 0, d.rows(t, "messages", `type = 'note'`))
}

package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestSeenReceipts(t *testing.T) {
	t.Parallel()

	t.Run("shows when the customer saw a teammate's reply and never on their own messages", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Seen", "Hello")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi Ann"}).AssertCreated()

		entries(d, path).AssertJSONPath("data.1.read_at", nil)

		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE messages SET read_at = $1", d.tc.Clock.Now())
		require.NoError(t, err)

		res := entries(d, path)
		res.AssertJSONPath("data.0.read_at", nil)
		require.NotEmpty(t, res.String("data.1.read_at"))
	})

	t.Run("gives the contact's time zone so times can be read in it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "bea", "Zone", "Hello")
		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE contacts SET timezone = 'Europe/Berlin'")
		require.NoError(t, err)

		d.owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().
			AssertJSONPath("data.contact.timezone", "Europe/Berlin")
	})
}

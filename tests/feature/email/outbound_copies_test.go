package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestRepliesReachEveryoneOnTheThread(t *testing.T) {
	t.Parallel()

	t.Run("copies the people the customer copied when a teammate replies", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer, boss := unique("customer"), unique("boss")

		deliver(tc, email{
			from: "Ann <" + customer + ">", to: address.ForwardingAddress, cc: "Boss <" + boss + ">",
			subject: "Invoice 88", id: "copies-1@customer.test", body: "Please fix the invoice.",
		}.raw(), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, convs[0].ID)
		tc.ActingAsOwner().PostJSON(path+"/replies", testkit.JSON{"body": "Fixed now."}).AssertCreated()
		tc.Drain()

		require.Len(t, received(t, customer), 1)
		copies := copied(t, boss)
		require.Len(t, copies, 1)
		assert.Contains(t, parsed(t, copies[0]).Cc[0].Email, boss)
	})
}

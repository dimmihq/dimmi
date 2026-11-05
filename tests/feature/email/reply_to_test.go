package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestRepliesGoWhereTheCustomerAsked(t *testing.T) {
	t.Parallel()

	t.Run("answers the reply-to address and keeps the sender as the contact", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		from, asked := unique("desk"), unique("home")

		deliver(tc, email{
			from: "Bob Stone <" + from + ">", to: address.ForwardingAddress, subject: "Refund", id: "rt-1." + from,
			headers: []string{"Reply-To: Bob <" + asked + ">"}, body: "Please answer my home address.",
		}.raw(), to(address.ForwardingAddress))

		convs := conversations(tc)
		require.Len(t, convs, 1)
		tc.Table("contacts").AssertHas(map[string]any{"email": from})
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, convs[0].ID)
		tc.ActingAsOwner().PostJSON(path+"/replies", testkit.JSON{"body": "Done."}).AssertCreated()
		tc.Drain()

		assert.Len(t, received(t, asked), 1)
		assert.Empty(t, received(t, from))
	})
}

package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestReplyTextAsTheCustomerReadsIt(t *testing.T) {
	t.Parallel()

	t.Run("keeps the teammate's lines, links and numbers in both parts of the email", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("lines")
		conversation := startConversation(tc, address, customer)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conversation.ID)

		body := "Hi,\nThanks for waiting.\nAna\n\nRead [the guide](https://help.example.test/reset).\n\n3. Open settings"
		tc.ActingAsOwner().PostJSON(path+"/replies", testkit.JSON{"body": body}).AssertCreated()
		tc.Drain()

		mails := received(t, customer)
		require.Len(t, mails, 1)
		got := parsed(t, mails[0])
		text := strings.ReplaceAll(got.Text, "\r\n", "\n")

		assert.Contains(t, text, "Hi,\nThanks for waiting.\nAna")
		assert.Contains(t, text, "the guide (https://help.example.test/reset)")
		assert.Contains(t, text, "3. Open settings")
		assert.Contains(t, got.HTML, "Hi,<br")
	})
}

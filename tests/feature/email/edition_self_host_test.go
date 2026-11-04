//go:build !cloud

package email_test

import (
	"testing"

	"github.com/dimmihq/dimmi/tests"
)

func TestSelfHostedEmail(t *testing.T) {
	t.Parallel()

	t.Run("hides the source and gives out no addresses on an install that receives no mail", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().WithHeader("Authorization", "Bearer "+testSecret).
			Send("POST", "/webhooks/test/inbound", []byte("Subject: x\r\n\r\nx")).AssertNotFound().
			AssertJSONPath("code", "inbound_driver_disabled")

		owner := tc.ActingAsOwner()
		owner.GetJSON("/api/v1/email-addresses").AssertOK().AssertJSONPath("data.#", 0.0)
		owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "support@acme.test"}).
			AssertStatus(422).AssertJSONPath("code", "inbound_not_configured")
	})
}

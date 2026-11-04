//go:build cloud

package email_test

import (
	"testing"

	"github.com/dimmihq/dimmi/tests"
)

func TestCloudEmail(t *testing.T) {
	t.Parallel()

	t.Run("receives on the cloud inbound domain and keeps the test source closed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().GetJSON("/api/v1/email-addresses").AssertOK().
			AssertJSONPath("data.0.forwarding_address", tc.Workspace.Workspace.Slug+"@inbound.trydimmi.com")
		tc.Client().WithHeader("Authorization", "Bearer "+testSecret).
			Send("POST", "/webhooks/test/inbound", []byte("Subject: x\r\n\r\nx")).AssertNotFound()
	})
}

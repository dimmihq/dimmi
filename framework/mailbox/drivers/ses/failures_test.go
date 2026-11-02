package ses_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/ses"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
	"github.com/dimmihq/dimmi/framework/manager"
)

func TestFailures(t *testing.T) {
	t.Parallel()

	rejected := map[string]func(receipt map[string]any){
		"rejects a receipt without an s3 object": func(receipt map[string]any) {
			receipt["action"] = map[string]string{"type": "SNS"}
		},
		"rejects a receipt with an unreadable timestamp": func(receipt map[string]any) {
			receipt["timestamp"] = "yesterday"
		},
	}

	for name, change := range rejected {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			notification := received(drivertest.NewDelivery())
			receipt, ok := notification["receipt"].(map[string]any)
			require.True(t, ok)
			change(receipt)

			_, err := f.source.Receive(t.Context(), f.signer.notification(t, notification))

			require.ErrorIs(t, err, mailbox.ErrRejected)
		})
	}

	t.Run("rejects a notification that is not json", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		_, err := f.source.Receive(t.Context(), f.signer.post(t, map[string]string{
			"Type": "Notification", "MessageId": "sns-3", "TopicArn": topic, "Message": "Successfully validated",
			"Timestamp": "2026-11-02T10:04:01.000Z",
		}))

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("hands over nothing for other notifications", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		env, err := f.source.Receive(t.Context(), f.signer.notification(t, map[string]any{
			"notificationType": "AMAZON_SES_SETUP_NOTIFICATION",
		}))

		require.NoError(t, err)
		assert.Nil(t, env)
	})

	t.Run("reports a raw message missing from the disk", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		_, err := f.source.Receive(t.Context(), f.signer.notification(t, received(drivertest.NewDelivery())))

		require.ErrorIs(t, err, filesystem.ErrNotFound)
	})

	t.Run("rejects a topic it was not configured for", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		body := f.signer.post(t, map[string]string{
			"Type": "Notification", "MessageId": "sns-4", "TopicArn": "arn:aws:sns:us-east-1:1:other",
			"Message": "{}", "Timestamp": "2026-11-02T10:04:01.000Z",
		})

		_, err := f.source.Receive(t.Context(), body)

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("passes on a certificate fetch that may work later", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)
		httpclienttest.New(client).PreventStrayRequests()
		source := ses.New(client, f.disk, ses.Options{TopicARNs: []string{topic}, MaxMessageBytes: 4096})

		_, err = source.Receive(t.Context(), f.signer.notification(t, received(drivertest.NewDelivery())))

		require.ErrorIs(t, err, httpclienttest.ErrStrayRequest)
		require.NotErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("rejects a certificate sns does not serve", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)
		httpclienttest.New(client).Stub("GET "+certURL, httpclient.Response{Status: http.StatusNotFound})
		source := ses.New(client, f.disk, ses.Options{TopicARNs: []string{topic}})

		_, err = source.Receive(t.Context(), f.signer.notification(t, received(drivertest.NewDelivery())))

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})
}

func TestFactory(t *testing.T) {
	t.Parallel()

	disks := filesystem.NewManager(filesystem.Config{Disks: map[string]config.Section{
		"ses_inbound": {"driver": "local", "root": t.TempDir()},
	}})
	disks.Extend("local", local.Factory)

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := ses.Factory(nil, disks)("ses", config.Section{"max_message_bytes": []int{1}})

		require.Error(t, err)
	})

	t.Run("reports a disk that is not configured", func(t *testing.T) {
		t.Parallel()

		_, err := ses.Factory(nil, disks)("ses", config.Section{"disk": "missing"})

		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})
}

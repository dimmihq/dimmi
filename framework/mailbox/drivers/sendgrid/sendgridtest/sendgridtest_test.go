package sendgridtest_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/sendgrid"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/sendgrid/sendgridtest"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
)

func TestSigner(t *testing.T) {
	t.Parallel()

	t.Run("signs posts the source accepts", func(t *testing.T) {
		t.Parallel()
		signer := sendgridtest.NewSigner(t)

		source, err := sendgrid.New(clock.NewFake(time.Unix(1_800_000_000, 0)), sendgrid.Options{
			PublicKey: signer.PublicKey(), MaxWebhookBytes: 8192,
		})
		require.NoError(t, err)

		d := drivertest.NewDelivery()
		post := signer.Sign(t, "1800000000", map[string]string{
			"email": string(d.Raw), "envelope": `{"to":["` + d.Recipients[0] + `"],"from":"` + d.EnvelopeFrom + `"}`,
		})

		env, err := source.Receive(t.Context(), drivertest.Post{
			Header: http.Header{
				"Content-Type":           {post.ContentType},
				sendgrid.HeaderTimestamp: {post.Timestamp},
				sendgrid.HeaderSignature: {post.Signature},
			},
			Body: post.Body,
		}.Delivery())

		require.NoError(t, err)
		assert.Equal(t, d.EnvelopeFrom, env.EnvelopeFrom)
	})
}

package email_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestInboundMetrics(t *testing.T) {
	t.Parallel()

	t.Run("counts mail by source and outcome without naming anyone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_TELEMETRY__METRICS=prometheus",
			"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN="+inboundDomain,
			"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=postmark",
			"DIMMI_MAILBOX__SOURCES__POSTMARK__DRIVER=postmark",
			"DIMMI_MAILBOX__SOURCES__POSTMARK__USERNAME=hook",
			"DIMMI_MAILBOX__SOURCES__POSTMARK__PASSWORD=secret",
		))
		raw, address := inboundRaw(tc)
		client := tc.Client().WithHeader("Authorization", basicAuth("secret"))

		client.Send("POST", "/webhooks/postmark/inbound", postmarkPost(t, raw, address)).AssertOK()
		client.Send("POST", "/webhooks/postmark/inbound", postmarkPost(t, raw, "ghost@"+inboundDomain)).AssertOK()
		tc.Drain()

		body := string(testkit.NewClient(t, tc.App.Telemetry.MetricsHandler()).Send("GET", "/", nil).AssertOK().Body)

		processed := `dimmi_email_inbound_total{classification="normal",source="postmark",status="processed"`
		assert.Contains(t, body, processed)
		assert.Contains(t, body, `dimmi_email_inbound_unroutable_total{source="postmark"`)
		assert.Contains(t, body, `dimmi_email_inbound_latency_seconds_bucket{source="postmark"`)
		assert.Contains(t, body, `dimmi_email_inbound_bytes_bucket{source="postmark"`)
		assert.NotContains(t, body, address)
		assert.NotContains(t, body, "Refund")
	})
}

/*
logBuffer collects log lines written from any goroutine.
*/
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestInboundLogs(t *testing.T) {
	t.Parallel()

	t.Run("never logs who wrote, to whom, or what", func(t *testing.T) {
		t.Parallel()
		logs := &logBuffer{}
		tc := tests.New(t, tests.WithLogs(logs), tests.WithEnv(
			"DIMMI_LOG__LEVEL=debug", "DIMMI_LOG__DRIVER=json",
			"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN="+inboundDomain,
			"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=postmark",
			"DIMMI_MAILBOX__SOURCES__POSTMARK__DRIVER=postmark",
			"DIMMI_MAILBOX__SOURCES__POSTMARK__USERNAME=hook",
			"DIMMI_MAILBOX__SOURCES__POSTMARK__PASSWORD=secret",
		))
		raw, address := inboundRaw(tc)
		client := tc.Client().WithHeader("Authorization", basicAuth("secret"))

		client.Send("POST", "/webhooks/postmark/inbound", postmarkPost(t, raw, address)).AssertOK()
		client.Send("POST", "/webhooks/postmark/inbound", []byte(`{"MessageID":"x","OriginalRecipient":"`+address+`"}`))
		tc.Client().WithHeader("Authorization", basicAuth("wrong")).
			Send("POST", "/webhooks/postmark/inbound", postmarkPost(t, raw, address))
		tc.Drain()

		written := logs.String()
		assert.NotEmpty(t, written)
		for _, secret := range []string{address, "bob@customer.test", "Refund for order 1042", "Charged twice"} {
			assert.NotContains(t, written, secret)
		}
	})
}

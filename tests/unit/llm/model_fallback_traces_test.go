package llm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/failover"
	"github.com/dimmihq/dimmi/app/support/llmretry"
	"github.com/dimmihq/dimmi/tests"
)

/*
A model call that only answered after a retry, a spare or another host
still shows how: the tracker counts a span as failed when its status is
set and is not "ok" (apps/server/src/services/span.rs), so a call saved
by a fallback is failed and marked recovered, with one issue per cause,
while a call saved by a retry carries retry.count and stays green.
*/
func TestModelFallbackTraces(t *testing.T) {
	t.Parallel()

	tracked := func(t *testing.T) (*telemetry.Telemetry, *tests.Tracker) {
		t.Helper()
		tr := tests.NewTracker(t)
		tel, err := telemetry.New(t.Context(), telemetry.Config{
			ServiceName: "dimmi", Tracing: "none", Metrics: "none", SentryDSN: tr.DSN(),
		})
		require.NoError(t, err)
		return tel, tr
	}

	cases := []struct {
		name     string
		call     func(t *testing.T, tel *telemetry.Telemetry) error
		status   string
		kind     string
		fallback string
		retries  float64
	}{
		{
			name: "a retry that answered",
			call: func(t *testing.T, tel *telemetry.Telemetry) error {
				t.Helper()
				inner := &scripted{errs: []error{failure(contracts.LLMErrorRateLimited, 429)}}
				ctx, chat := tel.StartAI(t.Context(), telemetry.OpChat, "chat m")
				defer chat.End()
				_, err := llmretry.Wrap(inner, llmretry.Options{Backoff: noWait}).Chat(ctx, contracts.ChatRequest{})
				return err
			},
			retries: 1,
		},
		{
			name: "an overload the spare answered",
			call: func(t *testing.T, tel *telemetry.Telemetry) error {
				t.Helper()
				busy := failure(contracts.LLMErrorOverloaded, 529)
				inner := &scripted{errs: []error{busy, busy, busy}}
				ctx, chat := tel.StartAI(t.Context(), telemetry.OpChat, "chat m")
				defer chat.End()
				_, err := llmretry.Wrap(inner, llmretry.Options{Backoff: noWait, Fallback: &scripted{}}).
					Chat(ctx, contracts.ChatRequest{})
				return err
			},
			status: "internal_error", kind: "model_overloaded", fallback: "spare_model", retries: 2,
		},
		{
			name: "a host with no credit, the next host answered",
			call: func(t *testing.T, tel *telemetry.Telemetry) error {
				t.Helper()
				list, fake := openWeightHosts(t)
				noCredit := jsonResponse(fhttp.StatusPaymentRequired, apiError("insufficient_quota", "no credit"))
				fake.Stub(basetenURL, noCredit)
				fake.Stub(engyURL, completion("from engy"))
				ctx, chat := tel.StartAI(t.Context(), telemetry.OpChat, "chat m")
				defer chat.End()
				_, err := failover.New(failover.Options{Hosts: list}).Chat(ctx, request("glm-5.3-flash"))
				return err
			},
			status: "internal_error", kind: "model_host_quota", fallback: "next_host",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tel, tr := tracked(t)

			require.NoError(t, tc.call(t, tel), "the fallback answered")

			spans := tr.Spans(t, tel)
			require.Len(t, spans, 1)
			chat := spans[0]
			assert.Equal(t, tc.status, chat.Status)
			assert.Equal(t, tc.kind, chat.Attr("error.type"))
			assert.Equal(t, tc.fallback, chat.Attr("fallback"))
			if tc.retries > 0 {
				assert.InDelta(t, tc.retries, chat.Data["retry.count"], 0)
			}
			issues := tr.Issues(t, tel)
			if tc.kind == "" {
				assert.Empty(t, issues, "a retry that answered is no issue")
				return
			}
			require.Len(t, issues, 1)
			assert.Equal(t, []string{"recovered", tc.kind}, issues[0].Fingerprint)
			assert.Equal(t, chat.TraceID, issues[0].Contexts.Trace.TraceID)
		})
	}
}

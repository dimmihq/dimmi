package ai_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
	"github.com/dimmihq/dimmi/app/drivers/decision/typesafe"
	"github.com/dimmihq/dimmi/tests"
)

/*
deciderContract is what every decider, real or fake, must do: answer
each question it was asked, by name and type.
*/
func deciderContract(t *testing.T, d contracts.Decider) {
	t.Helper()
	res, err := d.Decide(t.Context(), contracts.DecisionRequest{
		State: "<customer_message>I was charged twice</customer_message>",
		Questions: map[string]contracts.DecisionQuestion{
			"intent":  {Type: contracts.DecisionChoice, Choices: map[string]string{"question": "", "spam": ""}},
			"billing": {Type: contracts.DecisionNoul, Instructions: "Is this about billing?"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, contracts.DecisionChoice, res.Answers["intent"].Type)
	assert.Equal(t, contracts.DecisionNoul, res.Answers["billing"].Type)
}

/*
TestDeciders talks to the decision model the way the pipeline does.
*/
func TestDeciders(t *testing.T) {
	t.Parallel()

	answer := httpclient.Response{Status: 200, Body: []byte(`{"model":"jev-1.13.0","answers":{` +
		`"intent":{"type":"choice","choice":"question","confidence":0.91,` +
		`"probabilities":{"question":0.91,"spam":0.09}},` +
		`"billing":{"type":"noul","noul":0.97}},"usage":{"input_tokens":120,"output_tokens":12}}`)}

	newDriver := func(t *testing.T) (*typesafe.Driver, *httpclienttest.Fake) {
		t.Helper()
		tc := tests.New(t)
		http := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		d, err := typesafe.New(tc.App.HTTP, tc.App.Clock, typesafe.Config{
			APIKey: "ts-test", BaseURL: "https://api.typesafe.test/v1", Model: "jev-latest", Timeout: time.Second,
		})
		require.NoError(t, err)
		return d, http
	}

	t.Run("the typesafe driver keeps the contract", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub("POST https://api.typesafe.test/v1/systemone", answer)
		deciderContract(t, d)
	})

	t.Run("the fake keeps the contract", func(t *testing.T) {
		t.Parallel()
		d, err := decisionfake.New(string(foundation.EnvTesting))
		require.NoError(t, err)
		deciderContract(t, d)
	})

	t.Run("the fake refuses to exist outside testing", func(t *testing.T) {
		t.Parallel()
		_, err := decisionfake.New(string(foundation.EnvProduction))
		assert.ErrorIs(t, err, decisionfake.ErrNotTesting)
	})

	t.Run("sends the questions and reads the answers", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub("POST https://api.typesafe.test/v1/systemone", answer)

		res, err := d.Decide(t.Context(), contracts.DecisionRequest{
			State: "state",
			Questions: map[string]contracts.DecisionQuestion{
				"intent":  {Type: contracts.DecisionChoice, Choices: map[string]string{"question": "Asks."}},
				"billing": {Type: contracts.DecisionNoul, Instructions: "Billing?", True: "yes", False: "no"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "jev-1.13.0", res.Model)
		assert.Equal(t, "question", res.Answers["intent"].Choice)
		assert.InDelta(t, 0.91, res.Answers["intent"].Confidence, 0.001)
		assert.InDelta(t, 0.97, res.Answers["billing"].Noul, 0.001)
		assert.Equal(t, int64(120), res.Usage.InputTokens)

		http.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			var body map[string]any
			_ = json.Unmarshal(req.Body, &body)
			questions, _ := body["questions"].(map[string]any)
			billing, _ := questions["billing"].(map[string]any)
			criteria, _ := billing["criteria"].(map[string]any)
			return req.Header.Get("Authorization") == "Bearer ts-test" && body["model"] == "jev-latest" &&
				body["state"] == "state" && criteria["true"] == "yes"
		})
	})

	t.Run("reports a rejected request", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub("POST https://api.typesafe.test/v1/systemone",
			httpclient.Response{Status: 422, Body: []byte(`{"detail":[{"msg":"bad"}]}`)})
		_, err := d.Decide(t.Context(), contracts.DecisionRequest{State: "x"})
		assert.ErrorIs(t, err, typesafe.ErrRejected)
	})

	t.Run("reports an outage", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub("POST https://api.typesafe.test/v1/systemone", httpclient.Response{Status: 503})
		_, err := d.Decide(t.Context(), contracts.DecisionRequest{State: "x"})
		assert.ErrorIs(t, err, typesafe.ErrUnavailable)
	})

	t.Run("needs an api key", func(t *testing.T) {
		t.Parallel()
		_, err := typesafe.New(nil, nil, typesafe.Config{})
		assert.ErrorIs(t, err, typesafe.ErrNotConfigured)
	})
}

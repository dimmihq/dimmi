package ai_test

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/decision/clef"
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

/*
TestClefDecider talks to Clef on Workers AI the way the pipeline does.
*/
func TestClefDecider(t *testing.T) {
	t.Parallel()

	const address = "POST https://cf.test/client/v4/accounts/acct-1/ai/run/@cf/cloudflare/clef"
	answers := `{"model":"clef","answers":{` +
		`"intent":{"type":"choice","choice":"question","confidence":0.91,` +
		`"probabilities":{"question":0.91,"spam":0.09}},` +
		`"billing":{"type":"noul","noul":0.97}},"usage":{"input_tokens":120,"output_tokens":0}}`
	wrapped := httpclient.Response{Status: 200, Body: []byte(`{"result":` + answers + `,"success":true,"errors":[]}`)}

	newDriver := func(t *testing.T) (*clef.Driver, *httpclienttest.Fake) {
		t.Helper()
		tc := tests.New(t)
		http := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		d, err := clef.New(tc.App.HTTP, tc.App.Clock, clef.Config{
			AccountID: "acct-1", APIToken: "cf-test", BaseURL: "https://cf.test/client/v4/", Timeout: time.Second,
		})
		require.NoError(t, err)
		return d, http
	}
	ask := contracts.DecisionRequest{
		State: "state",
		Questions: map[string]contracts.DecisionQuestion{
			"intent":  {Type: contracts.DecisionChoice, Choices: map[string]string{"question": "Asks.", "spam": ""}},
			"billing": {Type: contracts.DecisionNoul, Instructions: "Billing?", True: "yes", False: "no"},
		},
	}

	t.Run("keeps the contract", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, wrapped)
		deciderContract(t, d)
	})

	t.Run("sends the questions and reads the wrapped answers", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, wrapped)

		res, err := d.Decide(t.Context(), ask)
		require.NoError(t, err)
		assert.Equal(t, "clef", res.Model)
		assert.Equal(t, "question", res.Answers["intent"].Choice)
		assert.InDelta(t, 0.91, res.Answers["intent"].Probabilities["question"], 0.001)
		assert.InDelta(t, 0.97, res.Answers["billing"].Noul, 0.001)
		assert.Equal(t, int64(120), res.Usage.InputTokens)

		http.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			var body map[string]any
			_ = json.Unmarshal(req.Body, &body)
			questions, _ := body["questions"].(map[string]any)
			billing, _ := questions["billing"].(map[string]any)
			criteria, _ := billing["criteria"].(map[string]any)
			intent, _ := questions["intent"].(map[string]any)
			return req.Header.Get("Authorization") == "Bearer cf-test" && body["model"] == "clef" &&
				body["state"] == "state" && criteria["true"] == "yes" &&
				billing["instructions"] == "Billing?" && intent["instructions"] != ""
		})
	})

	t.Run("reads an answer that comes without the envelope", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 200, Body: []byte(answers)})
		res, err := d.Decide(t.Context(), ask)
		require.NoError(t, err)
		assert.InDelta(t, 0.97, res.Answers["billing"].Noul, 0.001)
	})

	t.Run("asks the model the account names", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		http := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		d, err := clef.New(tc.App.HTTP, tc.App.Clock, clef.Config{
			AccountID: "acct-1", APIToken: "cf-test", BaseURL: "https://cf.test/client/v4", Model: "clef-flash",
		})
		require.NoError(t, err)
		http.Stub("POST https://cf.test/client/v4/accounts/acct-1/ai/run/@cf/cloudflare/clef-flash", wrapped)
		_, err = d.Decide(t.Context(), ask)
		require.NoError(t, err)
	})

	t.Run("fails when a question comes back unanswered", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 200, Body: []byte(
			`{"result":{"model":"clef","answers":{"billing":{"type":"noul","noul":0.97}}},"success":true}`)})
		_, err := d.Decide(t.Context(), ask)
		assert.ErrorIs(t, err, clef.ErrUnavailable)
	})

	t.Run("fails on a probability out of range", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 200, Body: []byte(
			`{"result":{"answers":{"billing":{"type":"noul","noul":1.7}}},"success":true}`)})
		_, err := d.Decide(t.Context(), contracts.DecisionRequest{
			State: "x",
			Questions: map[string]contracts.DecisionQuestion{
				"billing": {Type: contracts.DecisionNoul, Instructions: "Billing?"},
			},
		})
		assert.ErrorIs(t, err, clef.ErrUnavailable)
	})

	t.Run("reports a rejected request with the api's reason", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 400, Body: []byte(
			`{"result":null,"success":false,"errors":[{"code":5006,"message":"bad input"}]}`)})
		_, err := d.Decide(t.Context(), ask)
		require.ErrorIs(t, err, clef.ErrRejected)
		assert.Contains(t, err.Error(), "5006 bad input")
	})

	t.Run("reports a refused token", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 401, Body: []byte(`{"success":false,"errors":[]}`)})
		_, err := d.Decide(t.Context(), ask)
		assert.ErrorIs(t, err, clef.ErrRejected)
	})

	t.Run("reports a rate limit and an outage as unavailable", func(t *testing.T) {
		t.Parallel()
		for _, status := range []int{429, 500, 503} {
			d, http := newDriver(t)
			http.Stub(address, httpclient.Response{Status: status, Body: []byte(`{"success":false}`)})
			_, err := d.Decide(t.Context(), ask)
			assert.ErrorIs(t, err, clef.ErrUnavailable, "status %d", status)
		}
	})

	t.Run("reports a failure sent with a 200", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 200, Body: []byte(
			`{"result":null,"success":false,"errors":[{"code":3040,"message":"capacity"}]}`)})
		_, err := d.Decide(t.Context(), ask)
		assert.ErrorIs(t, err, clef.ErrUnavailable)
	})

	t.Run("reports an unreadable answer", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Stub(address, httpclient.Response{Status: 200, Body: []byte(`<html>`)})
		_, err := d.Decide(t.Context(), ask)
		assert.ErrorIs(t, err, clef.ErrUnavailable)
	})

	many := func(count int) contracts.DecisionRequest {
		req := contracts.DecisionRequest{State: "x", Questions: map[string]contracts.DecisionQuestion{}}
		for i := range count {
			req.Questions[fmt.Sprintf("q%03d", i)] = contracts.DecisionQuestion{Type: contracts.DecisionNoul}
		}
		return req
	}
	echo := func(broken string) nethttp.Handler {
		return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, req *nethttp.Request) {
			var body struct {
				Questions map[string]any `json:"questions"`
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			if _, hit := body.Questions[broken]; hit {
				w.WriteHeader(nethttp.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"success":false}`))
				return
			}
			out := map[string]any{}
			for name := range body.Questions {
				out[name] = map[string]any{"type": "noul", "noul": 0.9}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{
				"model": "clef", "answers": out, "usage": map[string]int{"input_tokens": len(out)},
			}})
		})
	}

	t.Run("asks a long list of questions as several requests and joins the answers", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Serve(address, echo(""))
		res, err := d.Decide(t.Context(), many(2*clef.MaxQuestions+10))
		require.NoError(t, err)
		assert.Len(t, res.Answers, 2*clef.MaxQuestions+10)
		assert.InDelta(t, 0.9, res.Answers["q137"].Noul, 0.001)
		assert.Equal(t, int64(2*clef.MaxQuestions+10), res.Usage.InputTokens)

		sizes := []int{}
		for _, sent := range http.Sent() {
			var body struct {
				Questions map[string]any `json:"questions"`
			}
			_ = json.Unmarshal(sent.Body, &body)
			sizes = append(sizes, len(body.Questions))
		}
		slices.Sort(sizes)
		assert.Equal(t, []int{10, clef.MaxQuestions, clef.MaxQuestions}, sizes)
	})

	t.Run("fails the whole call when one of its requests fails", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Serve(address, echo("q100"))
		_, err := d.Decide(t.Context(), many(2*clef.MaxQuestions+10))
		assert.ErrorIs(t, err, clef.ErrUnavailable)
	})

	t.Run("sends exactly the limit as one request", func(t *testing.T) {
		t.Parallel()
		d, http := newDriver(t)
		http.Serve(address, echo(""))
		res, err := d.Decide(t.Context(), many(clef.MaxQuestions))
		require.NoError(t, err)
		assert.Len(t, res.Answers, clef.MaxQuestions)
		assert.Len(t, http.Sent(), 1)
	})

	t.Run("needs an account id and a token", func(t *testing.T) {
		t.Parallel()
		_, err := clef.New(nil, nil, clef.Config{AccountID: "acct-1"})
		require.ErrorIs(t, err, clef.ErrNotConfigured)
		_, err = clef.New(nil, nil, clef.Config{APIToken: "cf-test"})
		assert.ErrorIs(t, err, clef.ErrNotConfigured)
	})
}

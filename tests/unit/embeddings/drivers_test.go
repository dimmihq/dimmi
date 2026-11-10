package embeddings_test

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/actions/knowledge"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/dimmi"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/fake"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openai"
	"github.com/dimmihq/dimmi/app/drivers/embeddings/openaicompat"
)

/*
fakeHTTP returns a client a fresh fake answers, refusing strays.
*/
func fakeHTTP(t *testing.T) (*httpclient.Client, *httpclienttest.Fake) {
	t.Helper()
	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	return client, httpclienttest.New(client).PreventStrayRequests()
}

/*
vectorsBody is an embeddings response holding the vectors.
*/
func vectorsBody(vectors ...[]float32) httpclient.Response {
	type item struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	}
	data := make([]item, len(vectors))
	for i, v := range vectors {
		data[len(vectors)-1-i] = item{Object: "embedding", Index: i, Embedding: v}
	}
	body, _ := json.Marshal(map[string]any{
		"object": "list", "data": data, "model": "Qwen/Qwen3-Embedding-8B",
		"usage": map[string]int{"prompt_tokens": 12, "total_tokens": 12},
	})
	return httpclient.Response{
		Status: fhttp.StatusOK, Header: fhttp.Header{"Content-Type": {"application/json"}}, Body: body,
	}
}

/*
ramp is a vector of n values 1..n.
*/
func ramp(n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(i + 1)
	}
	return v
}

/*
norm is a vector's length.
*/
func norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

func TestEmbeddingDrivers(t *testing.T) {
	t.Parallel()

	builtin := func(hc *httpclient.Client) *openaicompat.Driver {
		return dimmi.New(dimmi.Options{
			BaseURL: "https://api.deepinfra.com/v1/openai", APIKey: "di-test", Model: "Qwen/Qwen3-Embedding-8B",
			Dims: 1024, HTTP: hc,
		})
	}

	t.Run("sends the deepinfra request shape with the qwen prefix on queries only", func(t *testing.T) {
		t.Parallel()
		hc, f := fakeHTTP(t)
		f.Stub("POST https://api.deepinfra.com/v1/openai/embeddings", vectorsBody(ramp(1024)))
		drv := builtin(hc)
		_, err := drv.Embed(t.Context(), contracts.EmbedRequest{
			Model: "Qwen/Qwen3-Embedding-8B", Inputs: []string{"Is it prorated?"}, Dimensions: 1024,
			InputType: contracts.EmbedInputQuery,
		})
		require.NoError(t, err)
		_, err = drv.Embed(t.Context(), contracts.EmbedRequest{
			Model: "Qwen/Qwen3-Embedding-8B", Inputs: []string{"Plans can be changed any time."}, Dimensions: 1024,
			InputType: contracts.EmbedInputDocument,
		})
		require.NoError(t, err)

		sent := f.Sent()
		require.Len(t, sent, 2)
		assert.Equal(t, "Bearer di-test", sent[0].Header.Get("Authorization"))
		var query, doc map[string]any
		require.NoError(t, json.Unmarshal(sent[0].Body, &query))
		require.NoError(t, json.Unmarshal(sent[1].Body, &doc))
		assert.Equal(t, "Qwen/Qwen3-Embedding-8B", query["model"])
		assert.Equal(t, "float", query["encoding_format"])
		assert.EqualValues(t, 1024, query["dimensions"])
		assert.Equal(t, []any{dimmi.QueryPrefix + "Is it prorated?"}, query["input"])
		assert.True(t, strings.HasPrefix(dimmi.QueryPrefix,
			"Instruct: Given a customer support question, retrieve help articles that answer it\nQuery: "))
		assert.Equal(t, []any{"Plans can be changed any time."}, doc["input"])
	})

	t.Run("truncates a native vector to the asked size and renormalizes it", func(t *testing.T) {
		t.Parallel()
		hc, f := fakeHTTP(t)
		f.Stub("POST https://api.deepinfra.com/v1/openai/embeddings", vectorsBody(ramp(4096), ramp(4096)))
		resp, err := builtin(hc).Embed(t.Context(), contracts.EmbedRequest{
			Model: "Qwen/Qwen3-Embedding-8B", Inputs: []string{"a", "b"}, Dimensions: 1024,
		})
		require.NoError(t, err)
		require.Len(t, resp.Vectors, 2)
		assert.Equal(t, 1024, resp.Dims)
		assert.Len(t, resp.Vectors[0], 1024)
		assert.InDelta(t, 1.0, norm(resp.Vectors[0]), 1e-5)
		assert.InDelta(t, 1/norm(ramp(1024)), resp.Vectors[0][0], 1e-6)
		assert.Equal(t, contracts.EmbedUsage{InputTokens: 12, Reported: true}, resp.Usage)
	})

	t.Run("orders vectors by input index", func(t *testing.T) {
		t.Parallel()
		hc, f := fakeHTTP(t)
		f.Stub("POST https://api.deepinfra.com/v1/openai/embeddings",
			vectorsBody([]float32{1, 0, 0}, []float32{0, 1, 0}))
		resp, err := builtin(hc).Embed(t.Context(), contracts.EmbedRequest{
			Model: "m", Inputs: []string{"a", "b"}, Dimensions: 3,
		})
		require.NoError(t, err)
		assert.Equal(t, []float32{1, 0, 0}, resp.Vectors[0])
		assert.Equal(t, []float32{0, 1, 0}, resp.Vectors[1])
	})

	t.Run("refuses a fixed-size model answering at another size", func(t *testing.T) {
		t.Parallel()
		hc, f := fakeHTTP(t)
		f.Stub("POST https://ollama.example.com/v1/embeddings", vectorsBody(ramp(768)))
		drv := openaicompat.New(openaicompat.Options{
			Provider: "openai_compatible", BaseURL: "https://ollama.example.com/v1", HTTP: hc,
		})
		_, err := drv.Embed(t.Context(), contracts.EmbedRequest{
			Model: "nomic-embed-text", Inputs: []string{"a"}, Dimensions: 1024,
		})
		var llmErr *contracts.LLMError
		require.ErrorAs(t, err, &llmErr)
		assert.Equal(t, contracts.LLMErrorInvalidRequest, llmErr.Kind)
		var body map[string]any
		require.NoError(t, json.Unmarshal(f.Sent()[0].Body, &body))
		assert.NotContains(t, body, "dimensions")
	})

	t.Run("maps provider failures to error kinds", func(t *testing.T) {
		t.Parallel()
		for status, kind := range map[int]contracts.LLMErrorKind{
			401: contracts.LLMErrorAuth, 429: contracts.LLMErrorRateLimited, 500: contracts.LLMErrorServer,
			400: contracts.LLMErrorInvalidRequest,
		} {
			hc, f := fakeHTTP(t)
			f.Stub("POST https://api.openai.com/v1/embeddings", httpclient.Response{
				Status: status, Body: []byte(`{"error":{"message":"nope","code":"x"}}`),
			})
			_, err := openai.New(openai.Options{APIKey: "sk", HTTP: hc}).Embed(t.Context(),
				contracts.EmbedRequest{Model: "text-embedding-3-small", Inputs: []string{"a"}, Dimensions: 1024})
			var llmErr *contracts.LLMError
			require.ErrorAs(t, err, &llmErr, fmt.Sprint(status))
			assert.Equal(t, kind, llmErr.Kind, fmt.Sprint(status))
		}
	})

	t.Run("refuses empty and oversized batches", func(t *testing.T) {
		t.Parallel()
		hc, _ := fakeHTTP(t)
		_, err := builtin(hc).Embed(t.Context(), contracts.EmbedRequest{Model: "m"})
		require.Error(t, err)
		_, err = builtin(hc).Embed(t.Context(), contracts.EmbedRequest{Model: "m", Inputs: make([]string, 65)})
		require.Error(t, err)
	})

	t.Run("lets the fake script texts that mean the same thing", func(t *testing.T) {
		t.Parallel()
		f := fake.New("dimmi", 8).Script("prorated", fake.Axis(0, 8)).Script("refund", fake.Axis(1, 8))
		resp, err := f.Embed(t.Context(), contracts.EmbedRequest{
			Inputs: []string{"Is it prorated?", "upgrades are PRORATED", "refunds", "unrelated"},
		})
		require.NoError(t, err)
		assert.Equal(t, resp.Vectors[0], resp.Vectors[1])
		assert.NotEqual(t, resp.Vectors[0], resp.Vectors[2])
		again, err := f.Embed(t.Context(), contracts.EmbedRequest{Inputs: []string{"unrelated"}})
		require.NoError(t, err)
		assert.Equal(t, resp.Vectors[3], again.Vectors[0])
		assert.Len(t, f.Sent(), 2)
	})

	t.Run("reads model keys", func(t *testing.T) {
		t.Parallel()
		k, err := knowledge.ParseModelKey("dimmi:Qwen/Qwen3-Embedding-8B@1024")
		require.NoError(t, err)
		assert.Equal(t, knowledge.ModelKey{Provider: "dimmi", Model: "Qwen/Qwen3-Embedding-8B", Dims: 1024}, k)
		assert.Equal(t, "dimmi:Qwen/Qwen3-Embedding-8B@1024", k.String())
		for _, bad := range []string{"", "dimmi", "dimmi:model", "dimmi:model@x", ":m@1", "dimmi:@3"} {
			_, err := knowledge.ParseModelKey(bad)
			assert.ErrorIs(t, err, knowledge.ErrModelKey, bad)
		}
	})
}

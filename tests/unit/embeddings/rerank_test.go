package embeddings_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
	rerank "github.com/dimmihq/dimmi/app/drivers/rerank/voyage"
)

func TestVoyageReranker(t *testing.T) {
	t.Parallel()

	t.Run("sends the question and passages and returns them best first", func(t *testing.T) {
		t.Parallel()
		hc, f := fakeHTTP(t)
		f.Stub("POST https://api.voyageai.com/v1/rerank", httpclient.Response{Status: 200, Body: []byte(
			`{"data":[{"index":1,"relevance_score":0.9},{"index":0,"relevance_score":0.2}],` +
				`"model":"rerank-2.5-lite","usage":{"total_tokens":31}}`)})
		drv := rerank.New(rerank.Options{APIKey: "pa-test", Model: "rerank-2.5-lite", HTTP: hc})
		res, err := drv.Rerank(t.Context(), contracts.RerankRequest{
			Query: "how long do refunds take", Documents: []string{"Shipping zones", "Refunds take five days"},
		})
		require.NoError(t, err)
		assert.Equal(t, []contracts.RerankResult{{Index: 1, Score: 0.9}, {Index: 0, Score: 0.2}}, res.Results)
		assert.Equal(t, contracts.EmbedUsage{InputTokens: 31, Reported: true}, res.Usage)

		sent := f.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, "Bearer pa-test", sent[0].Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, json.Unmarshal(sent[0].Body, &body))
		assert.Equal(t, "rerank-2.5-lite", body["model"])
		assert.Equal(t, "how long do refunds take", body["query"])
		assert.Equal(t, []any{"Shipping zones", "Refunds take five days"}, body["documents"])
		assert.Equal(t, true, body["truncation"])
	})

	t.Run("refuses an answer that names a passage twice or one never sent", func(t *testing.T) {
		t.Parallel()
		for _, data := range []string{
			`[{"index":0,"relevance_score":1},{"index":0,"relevance_score":1}]`,
			`[{"index":5,"relevance_score":1}]`, `[]`,
		} {
			hc, f := fakeHTTP(t)
			f.Stub("POST https://api.voyageai.com/v1/rerank", httpclient.Response{
				Status: 200, Body: []byte(`{"data":` + data + `}`),
			})
			_, err := rerank.New(rerank.Options{HTTP: hc}).Rerank(t.Context(), contracts.RerankRequest{
				Query: "q", Documents: []string{"a", "b"},
			})
			var llmErr *contracts.LLMError
			require.ErrorAs(t, err, &llmErr, data)
			assert.Equal(t, contracts.LLMErrorServer, llmErr.Kind, data)
		}
	})

	t.Run("maps a refused key and refuses an empty request", func(t *testing.T) {
		t.Parallel()
		hc, f := fakeHTTP(t)
		f.Stub("POST https://api.voyageai.com/v1/rerank", httpclient.Response{
			Status: 401, Body: []byte(`{"detail":"Provided API key is invalid."}`),
		})
		drv := rerank.New(rerank.Options{HTTP: hc})
		_, err := drv.Rerank(t.Context(), contracts.RerankRequest{Query: "q", Documents: []string{"a"}})
		var llmErr *contracts.LLMError
		require.ErrorAs(t, err, &llmErr)
		assert.Equal(t, contracts.LLMErrorAuth, llmErr.Kind)
		_, err = drv.Rerank(t.Context(), contracts.RerankRequest{Query: "q"})
		require.Error(t, err)
	})
}

package web_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/tests"
)

func TestLLMsText(t *testing.T) {
	t.Parallel()

	t.Run("serves the SDKs' llms.txt as cached plain text", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().Send(fhttp.MethodGet, "/llms.txt", nil).AssertOK().
			AssertHeader("Content-Type", "text/plain; charset=utf-8").
			AssertHeader("Cache-Control", "public, max-age=3600")
		assert.True(t, strings.HasPrefix(string(res.Body), "# dimmi"))
	})
}

package middleware_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
)

func request(t *testing.T, method, target, body string) *http.Request {
	t.Helper()

	return httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	return rec
}

/*
echo answers 200 with the request body, or "ok" when there is none.
*/
func echo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Errors{}.Render(w, r, err)

			return
		}

		if len(body) == 0 {
			body = []byte("ok")
		}

		_, _ = w.Write(body)
	})
}

func code(t *testing.T, body io.Reader) string {
	t.Helper()

	var out struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.NewDecoder(body).Decode(&out))

	return out.Code
}

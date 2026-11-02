package testkit_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
app is a small handler covering the responses the assertions read.
*/
func app() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /items", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Total", "1")
		items := []testkit.JSON{{"id": 1, "name": "a"}}
		fhttp.WriteJSON(w, http.StatusOK, testkit.JSON{"data": testkit.JSON{"items": items}})
	})

	mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			body = []byte("null")
		}

		caller, _ := fhttp.Principal[string](r.Context())
		theme := ""

		if cookie, err := r.Cookie("theme"); err == nil {
			theme = cookie.Value
		}

		fhttp.WriteJSON(w, http.StatusCreated, testkit.JSON{
			"body": json.RawMessage(body), "caller": caller, "type": r.Header.Get("Content-Type"),
			"accept": r.Header.Get("Accept"), "tag": r.Header.Get("X-Tag"), "theme": theme,
		})
	})

	mux.HandleFunc("PATCH /items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		fhttp.WriteJSON(w, http.StatusUnprocessableEntity, testkit.JSON{
			"code":   "validation_failed",
			"errors": testkit.JSON{"name": []string{"bad"}, "items.0.name": []string{"bad"}},
		})
	})

	mux.HandleFunc("PUT /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, _ = w.Write(body)
	})

	mux.HandleFunc("DELETE /items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name: "deleted", Value: "yes", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /status/{code}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("code") {
		case "401":
			w.WriteHeader(http.StatusUnauthorized)
		case "403":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	return mux
}

/*
theme returns the cookie the app echoes back.
*/
func theme(value string) *http.Cookie {
	return &http.Cookie{Name: "theme", Value: value, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

func TestClient(t *testing.T) {
	t.Parallel()

	client := testkit.NewClient(t, app())

	t.Run("reads a JSON response", func(t *testing.T) {
		t.Parallel()

		res := client.GetJSON("/items").
			AssertOK().
			AssertHeader("X-Total", "1").
			AssertJSONPath("data.items.0", testkit.JSON{"id": 1, "name": "a"}).
			AssertJSONPath("data.items.#", 1).
			AssertJSONMissing("data.items.1")

		var decoded struct {
			Data struct {
				Items []struct {
					Name string `json:"name"`
				} `json:"items"`
			} `json:"data"`
		}
		res.Decode(&decoded)

		assert.Equal(t, "a", decoded.Data.Items[0].Name)
		assert.Equal(t, "a", res.String("data.items.0.name"))
	})

	t.Run("sends the body, caller, headers and cookies", func(t *testing.T) {
		t.Parallel()

		client.ActingAs("ada").
			WithHeader("X-Tag", "blue").
			WithCookie(theme("dark")).
			PostJSON("/items", testkit.JSON{"name": "b"}).
			AssertCreated().
			AssertJSONPath("body", testkit.JSON{"name": "b"}).
			AssertJSONPath("caller", "ada").
			AssertJSONPath("type", "application/json").
			AssertJSONPath("accept", "application/json").
			AssertJSONPath("tag", "blue").
			AssertJSONPath("theme", "dark")
	})

	t.Run("leaves the client it was derived from unchanged", func(t *testing.T) {
		t.Parallel()

		client.ActingAs("ada").WithHeader("X-Tag", "blue")

		client.WithCookie(theme("light")).
			PostJSON("/items", nil).
			AssertJSONPath("caller", "").
			AssertJSONPath("tag", "").
			AssertJSONPath("type", "")
	})

	t.Run("reads a cookie the response set", func(t *testing.T) {
		t.Parallel()

		res := client.Delete("/items/1")

		assert.Equal(t, "yes", res.Cookie("deleted").Value)
		assert.Equal(t, "/", res.Cookie("deleted").Path)
	})

	t.Run("sends raw bodies as they are", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `{"a":1}`, string(client.PutJSON("/items/1", []byte(`{"a":1}`)).AssertOK().Body))
		assert.Equal(t, "plain", string(client.PutJSON("/items/1", "plain").Body))
	})

	t.Run("makes requests as the caller through the impersonator", func(t *testing.T) {
		t.Parallel()

		viaHeader := func(r *http.Request, principal any) *http.Request {
			name, _ := principal.(string)
			r.Header.Set("X-Tag", name)

			return r
		}

		client.ImpersonateWith(viaHeader).ActingAs("grace").
			WithCookie(theme("dark")).
			PostJSON("/items", nil).
			AssertJSONPath("tag", "grace").
			AssertJSONPath("caller", "")
	})

	t.Run("checks statuses", func(t *testing.T) {
		t.Parallel()

		client.Delete("/items/1").AssertNoContent()
		client.GetJSON("/status/401").AssertUnauthorized()
		client.GetJSON("/status/403").AssertForbidden()
		client.GetJSON("/status/404").AssertNotFound()
	})

	t.Run("checks validation errors by field", func(t *testing.T) {
		t.Parallel()

		client.PatchJSON("/items/1", testkit.JSON{}).AssertValidationError("name", "items.0.name")
	})
}

func TestClientFailures(t *testing.T) {
	t.Parallel()

	t.Run("reports every failed assertion", func(t *testing.T) {
		t.Parallel()

		rec := record(t, func(tb *recorder) {
			testkit.NewClient(tb, app()).GetJSON("/items").
				AssertCreated().
				AssertHeader("X-Total", "2").
				AssertJSONPath("data.items.0.name", "z").
				AssertJSONPath("data.missing", "z").
				AssertJSONPath("data.items.0.name", func() {}).
				AssertJSONMissing("data.items.0").
				AssertValidationError("name")
		})

		failures := rec.Failures()
		require.Len(t, failures, 8)
		assert.Contains(t, failures[3], `no JSON value at "data.missing"`)
		assert.Contains(t, failures[4], "encode expected value")
		assert.Contains(t, failures[5], `unexpected JSON value at "data.items.0"`)
		assert.Contains(t, failures[7], `no validation error for "name"`)
	})

	t.Run("reports a body that is not JSON", func(t *testing.T) {
		t.Parallel()

		rec := record(t, func(tb *recorder) {
			testkit.NewClient(tb, app()).PutJSON("/items/1", "plain").AssertJSONMissing("a")
		})

		assert.Equal(t, []string{"response body is not JSON: plain"}, rec.Failures())
	})

	t.Run("stops on a body it cannot encode or decode", func(t *testing.T) {
		t.Parallel()

		encode := record(t, func(tb *recorder) {
			testkit.NewClient(tb, app()).PostJSON("/items", func() {})
		})
		decode := record(t, func(tb *recorder) {
			var v []int

			testkit.NewClient(tb, app()).GetJSON("/items").Decode(&v)
		})

		assert.Contains(t, encode.Failures()[0], "encode POST /items body")
		assert.Contains(t, decode.Failures()[0], "decode response body")
	})
}

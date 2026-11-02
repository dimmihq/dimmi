package view_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/view"
)

var errBroken = errors.New("broken template")

func page(body string) view.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, body)

		return err
	})
}

/*
broken writes part of a page and then fails, as a template does when a
value it needs is missing.
*/
func broken() view.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, _ = io.WriteString(w, "<html><body>")

		return errBroken
	})
}

/*
failingWriter accepts headers but refuses the body, as a connection the
client closed does.
*/
type failingWriter struct {
	*httptest.ResponseRecorder
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestHTML(t *testing.T) {
	t.Parallel()

	t.Run("renders the component to a string", func(t *testing.T) {
		t.Parallel()
		got, err := view.HTML(t.Context(), page("<p>Hello</p>"))
		require.NoError(t, err)
		assert.Equal(t, "<p>Hello</p>", got)
	})

	t.Run("returns the render error", func(t *testing.T) {
		t.Parallel()
		_, err := view.HTML(t.Context(), broken())
		require.ErrorIs(t, err, errBroken)
	})
}

func TestText(t *testing.T) {
	t.Parallel()

	t.Run("converts the html to plain text and keeps link addresses", func(t *testing.T) {
		t.Parallel()
		got, err := view.Text(t.Context(), page(
			`<h1>Welcome</h1><p>Hi &amp; hello.</p><p><a href="https://example.com/x">Sign in</a></p>`,
		))
		require.NoError(t, err)
		assert.Contains(t, got, "Welcome")
		assert.Contains(t, got, "Hi & hello.")
		assert.Contains(t, got, "Sign in <https://example.com/x>")
		assert.NotContains(t, got, "\r\n")
	})

	t.Run("returns the render error", func(t *testing.T) {
		t.Parallel()
		_, err := view.Text(t.Context(), broken())
		require.ErrorIs(t, err, errBroken)
	})
}

func TestRespond(t *testing.T) {
	t.Parallel()

	t.Run("sends the page with the status and html content type", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

		require.NoError(t, view.Respond(w, r, http.StatusCreated, page("<p>Done</p>")))

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, "11", w.Header().Get("Content-Length"))
		assert.Equal(t, "<p>Done</p>", w.Body.String())
	})

	t.Run("renders with the request context", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		r = r.WithContext(view.WithNonce(r.Context(), "abc"))
		c := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			_, err := io.WriteString(w, view.Nonce(ctx))

			return err
		})

		require.NoError(t, view.Respond(w, r, http.StatusOK, c))

		assert.Equal(t, "abc", w.Body.String())
	})

	t.Run("writes nothing when rendering fails", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

		err := view.Respond(w, r, http.StatusOK, broken())

		require.ErrorIs(t, err, errBroken)
		assert.False(t, w.Flushed)
		assert.Empty(t, w.Body.String())
		assert.Empty(t, w.Header().Get("Content-Type"))
	})

	t.Run("returns the write error", func(t *testing.T) {
		t.Parallel()
		w := failingWriter{httptest.NewRecorder()}
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

		err := view.Respond(w, r, http.StatusOK, page("<p>Done</p>"))

		require.ErrorIs(t, err, io.ErrClosedPipe)
	})
}

func TestNonce(t *testing.T) {
	t.Parallel()

	t.Run("is empty without one in the context", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, view.Nonce(t.Context()))
	})

	t.Run("returns the nonce put in the context", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "n0nce", view.Nonce(view.WithNonce(t.Context(), "n0nce")))
	})
}

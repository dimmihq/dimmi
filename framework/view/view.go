/*
|--------------------------------------------------------------------------
| View Rendering
|--------------------------------------------------------------------------
|
| Pages and mail bodies are typed components compiled from templates. A
| page is rendered in full before a single byte is sent, so a component
| that fails halfway never leaves the browser with half a page and a
| success status. The same components give mail its plain text part.
|
*/

package view

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/k3a/html2text"
)

/*
Component is a renderable piece of HTML, as generated from a template
file or written by hand with templ.ComponentFunc.
*/
type Component = templ.Component

/*
HTML renders the component to a string.
*/
func HTML(ctx context.Context, c Component) (string, error) {
	var b strings.Builder
	if err := c.Render(ctx, &b); err != nil {
		return "", fmt.Errorf("render component: %w", err)
	}

	return b.String(), nil
}

/*
Text renders the component and converts the HTML to plain text for the
text part of a mail. Links keep their text with the address after it in
angle brackets, so every link stays usable in a text-only client.
*/
func Text(ctx context.Context, c Component) (string, error) {
	page, err := HTML(ctx, c)
	if err != nil {
		return "", err
	}

	return html2text.HTML2TextWithOptions(
		page,
		html2text.WithUnixLineBreaks(),
		html2text.WithLinksInnerText(),
		html2text.WithListSupport(),
	), nil
}

/*
Respond renders the component with the request's context and sends it
with the status. Nothing is written when rendering fails: the error is
returned so the caller can send an error page instead.
*/
func Respond(w http.ResponseWriter, r *http.Request, status int, c Component) error {
	var page bytes.Buffer
	if err := c.Render(r.Context(), &page); err != nil {
		return fmt.Errorf("render page: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(page.Len()))
	w.WriteHeader(status)

	if _, err := page.WriteTo(w); err != nil {
		return fmt.Errorf("write page: %w", err)
	}

	return nil
}

/*
WithNonce returns a copy of the context carrying the content security
policy nonce for the response, which components put on their inline
scripts and styles.
*/
func WithNonce(ctx context.Context, nonce string) context.Context {
	return templ.WithNonce(ctx, nonce)
}

/*
Nonce returns the content security policy nonce in the context, or ""
when there is none.
*/
func Nonce(ctx context.Context) string {
	return templ.GetNonce(ctx)
}

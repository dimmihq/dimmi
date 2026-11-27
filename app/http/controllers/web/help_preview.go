/*
|--------------------------------------------------------------------------
| Theme Preview Pages
|--------------------------------------------------------------------------
|
| A theme's author previews it with the help center's real pages. The
| page at a path is built here exactly as a signed-out reader gets it,
| through the same routes, languages and redirects, then caught before
| it renders: the preview receives its template's name and variables
| and renders them with the theme being written.
|
*/

package web

import (
	"context"
	"io"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/liquid"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
Preview is a page built for a preview: its status, where it redirects
when it does, and the template and variables it would have rendered,
empty when it is not a themed page.
*/
type Preview struct {
	Status   int
	Location string
	Template string
	Globals  map[string]any
}

/*
PreviewPage builds the page at the path of the help center with the
subdomain, served by the routes, and catches it.
*/
func PreviewPage(ctx context.Context, routes fhttp.Handler, subdomain, path string) (Preview, error) {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		path = "/" + strings.TrimLeft(path, "/")
	}
	ctx, capture := helpcenter.WithCapture(ctx)
	target := "/hc/" + subdomain + path
	if path == "/" || strings.HasPrefix(path, "/?") {
		target = "/hc/" + subdomain + path[1:]
	}
	u, err := url.Parse(target)
	if err != nil {
		return Preview{}, err
	}
	r := (&fhttp.Request{
		Method: fhttp.MethodGet, URL: u, RequestURI: target, Header: fhttp.Header{"Accept": {"text/html"}},
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Host: u.Host,
	}).WithContext(ctx)
	w := &caught{header: fhttp.Header{}}
	stripLocale(routes).ServeHTTP(w, r) //nolint:contextcheck // the request carries the context
	if capture.Globals != nil {
		capture.Globals["form_token"], capture.Globals["form_token_field"] = "", liquid.HTML("")
	}
	return Preview{
		Status: w.status, Location: w.header.Get("Location"), Template: capture.Name, Globals: capture.Globals,
	}, nil
}

/*
caught keeps what a page answered, without its body.
*/
type caught struct {
	header fhttp.Header
	status int
}

/*
Header returns the headers the page set.
*/
func (c *caught) Header() fhttp.Header { return c.header }

/*
Write drops the page's body: a caught page has none worth keeping.
*/
func (c *caught) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = fhttp.StatusOK
	}
	return io.Discard.Write(p)
}

/*
WriteHeader keeps the page's status.
*/
func (c *caught) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

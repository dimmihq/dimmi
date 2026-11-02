/*
|--------------------------------------------------------------------------
| URL Generation
|--------------------------------------------------------------------------
|
| Links are built from route names, never typed out, so moving a route
| changes every link to it at once and a link to a route that does not
| exist fails loudly instead of sending a user to a 404. Parameters fill
| the path's placeholders, escaped, and whatever is left becomes the
| query string.
|
*/

package routing

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Errors URL generation returns.

ErrUnknownName means no route has the name. ErrMissingParameter means a
placeholder in the route's path has no value. ErrNoSigningKey means a
signed URL was asked for without a key.
*/
var (
	ErrUnknownName      = errors.New("routing: unknown route name")
	ErrMissingParameter = errors.New("routing: missing route parameter")
	ErrNoSigningKey     = errors.New("routing: no signing key")
)

/*
URL builds absolute links to named routes and signs them.
*/
type URL struct {
	routes *Router
	base   string
	key    []byte
	clock  clock.Clock
}

/*
NewURL returns a generator for the router's routes. The base URL, such
as "https://app.example.com", is put in front of every path; the key
signs URLs; the clock decides when signed URLs expire.
*/
func NewURL(routes *Router, baseURL string, key []byte, clk clock.Clock) *URL {
	return &URL{routes: routes, base: strings.TrimSuffix(baseURL, "/"), key: key, clock: clk}
}

/*
Route returns the absolute URL of the named route.
*/
func (u *URL) Route(name string, params map[string]string) (string, error) {
	path, query, err := u.fill(name, params)
	if err != nil {
		return "", err
	}

	return u.base + withQuery(path, query), nil
}

/*
MustRoute is Route for links whose name and parameters are fixed in the
code, where a failure is a programming error.
*/
func (u *URL) MustRoute(name string, params map[string]string) string {
	link, err := u.Route(name, params)
	if err != nil {
		panic(err)
	}

	return link
}

/*
fill puts the parameters into the route's placeholders and returns the
path and the parameters left over for the query string.
*/
func (u *URL) fill(name string, params map[string]string) (string, url.Values, error) {
	route, ok := u.routes.table.names[name]
	if !ok {
		return "", nil, fmt.Errorf("route %q: %w", name, ErrUnknownName)
	}

	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}

	segments := strings.Split(route.path, "/")
	for i, segment := range segments {
		placeholder, ok := placeholderName(segment)
		if !ok {
			continue
		}

		value := query.Get(placeholder)
		if value == "" {
			return "", nil, fmt.Errorf("route %q parameter %q: %w", name, placeholder, ErrMissingParameter)
		}

		segments[i] = url.PathEscape(value)
		query.Del(placeholder)
	}

	return strings.Join(segments, "/"), query, nil
}

/*
placeholderName returns the parameter name of a path segment such as
"{id}" or "{id:[0-9]+}".
*/
func placeholderName(segment string) (string, bool) {
	inner, ok := strings.CutPrefix(segment, "{")
	if !ok {
		return "", false
	}

	inner, ok = strings.CutSuffix(inner, "}")
	if !ok {
		return "", false
	}

	name, _, _ := strings.Cut(inner, ":")

	return name, true
}

func withQuery(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}

	return path + "?" + query.Encode()
}

func (u *URL) now() time.Time {
	return u.clock.Now()
}

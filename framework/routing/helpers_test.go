package routing_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/validation"
)

type showInput struct {
	ID string `path:"id"`
}

type item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type itemOutput struct {
	Body http.Resource[item]
}

type createInput struct {
	Body struct {
		Name  string `json:"name"  validate:"max=5"`
		Email string `json:"email" required:"false"`
	}
}

/*
guardedInput refuses every caller whose name is "intruder" and checks
that an email, when given, differs from the name.
*/
type guardedInput struct {
	Name string `query:"name"`
	Body struct {
		Email string `json:"email,omitempty" validate:"omitempty,email"`
	}
}

func (in *guardedInput) Authorize(context.Context) error {
	if in.Name == "intruder" {
		return fault.New(fault.Forbidden, "not_allowed", "You may not do this.")
	}

	return nil
}

func (in *guardedInput) Validate(context.Context, *validation.Factory) error {
	if in.Body.Email == in.Name+"@example.com" {
		return &validation.Error{Violations: []validation.Violation{
			{Field: "email", Rule: "different", Message: "The email must differ from the name."},
		}}
	}

	return nil
}

func newRouter(t *testing.T, kernel *http.Kernel) *routing.Router {
	t.Helper()

	factory, err := validation.New()
	require.NoError(t, err)

	return routing.New(routing.Options{Middleware: kernel, Validator: factory})
}

func request(t *testing.T, method, target, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

func serve(t *testing.T, r *routing.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	h, err := r.Handler()
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

/*
lookup walks a decoded JSON document along the keys.
*/
func lookup(doc any, keys ...string) any {
	for _, key := range keys {
		object, ok := doc.(map[string]any)
		if !ok {
			return nil
		}

		doc = object[key]
	}

	return doc
}

func decode(t *testing.T, body io.Reader) map[string]any {
	t.Helper()

	var out map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&out))

	return out
}

/*
tag returns a middleware that appends its name to the X-Trace response
header, so tests can see which middleware ran and in which order.
*/
func tag(name string) http.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Trace", name)
			next.ServeHTTP(w, r)
		})
	}
}

func tagFactory(params ...string) (http.Middleware, error) {
	return tag(strings.Join(append([]string{"tag"}, params...), ":")), nil
}

func nothing() http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
}

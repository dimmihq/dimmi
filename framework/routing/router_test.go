package routing_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

func TestTypedRoutes(t *testing.T) {
	t.Parallel()

	r := newRouter(t, http.NewKernel())
	r.API("/api/v1", "Test API", "1.0.0", func(g *routing.Router) {
		routing.Get(g, "/items/{id}", "items.show", func(_ context.Context, in *showInput) (*itemOutput, error) {
			switch in.ID {
			case "missing":
				return nil, fault.New(fault.NotFound, "item_not_found", "No such item.").With("id", in.ID)
			case "broken":
				return nil, errors.New("disk on fire")
			}

			return &itemOutput{Body: http.NewResource(item{ID: in.ID, Name: "Lamp"})}, nil
		})
		routing.Post(g, "/items", "items.store", func(_ context.Context, in *createInput) (*itemOutput, error) {
			return &itemOutput{Body: http.NewResource(item{ID: "new", Name: in.Body.Name})}, nil
		})
		routing.Patch(g, "/items/{id}", "items.update", func(context.Context, *guardedInput) (*struct{}, error) {
			return nil, nil
		})
		accept := func(context.Context, *showInput) (*struct{}, error) { return nil, nil }
		routing.Put(g, "/items/{id}/content", "items.content.update", accept).Status(http.StatusAccepted)
		routing.Delete(g, "/items/{id}", "items.destroy", func(context.Context, *showInput) (*struct{}, error) {
			return nil, nil
		})
	})

	t.Run("answers with the resource envelope", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/v1/items/it_1", ""))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"data":{"id":"it_1","name":"Lamp"}}`, rec.Body.String())
	})

	t.Run("creates with 201", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodPost, "/api/v1/items", `{"name":"Desk"}`))

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.JSONEq(t, `{"data":{"id":"new","name":"Desk"}}`, rec.Body.String())
	})

	t.Run("renders a fault with its details", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/v1/items/missing", ""))

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"message":"No such item.","code":"item_not_found","id":"missing"}`, rec.Body.String())
	})

	t.Run("hides unknown errors behind a plain 500", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/v1/items/broken", ""))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.JSONEq(t, `{"message":"Server Error.","code":"internal_error"}`, rec.Body.String())
	})

	t.Run("checks body tags with the validator", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodPost, "/api/v1/items", `{"name":"Wardrobe"}`))

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		body := decode(t, rec.Body)
		assert.Equal(t, "validation_failed", body["code"])
		assert.Contains(t, body["errors"], "name")
	})

	t.Run("reports schema errors in the same envelope", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodPost, "/api/v1/items", `{"email":"a@example.com"}`))

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		body := decode(t, rec.Body)
		assert.Equal(t, "validation_failed", body["code"])
		assert.Contains(t, body["errors"], "name")
	})

	t.Run("answers malformed bodies with 400", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodPost, "/api/v1/items", `{"name":`))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `{"message":"Bad Request.","code":"bad_request"}`, rec.Body.String())
	})

	t.Run("authorizes before validating", func(t *testing.T) {
		t.Parallel()
		req := request(t, http.MethodPatch, "/api/v1/items/it_1?name=intruder", `{"email":"nope"}`)
		rec := serve(t, r, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "not_allowed", decode(t, rec.Body)["code"])
	})

	t.Run("runs the input's own rules after its tags", func(t *testing.T) {
		t.Parallel()
		req := request(t, http.MethodPatch, "/api/v1/items/it_1?name=ann", `{"email":"ann@example.com"}`)
		rec := serve(t, r, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, decode(t, rec.Body)["errors"], "email")
	})

	t.Run("answers empty outputs with no content", func(t *testing.T) {
		t.Parallel()

		patch := request(t, http.MethodPatch, "/api/v1/items/it_1?name=ann", `{}`)
		assert.Equal(t, http.StatusNoContent, serve(t, r, patch).Code)
		assert.Equal(t, http.StatusNoContent, serve(t, r, request(t, http.MethodDelete, "/api/v1/items/it_1", "")).Code)
	})

	t.Run("uses the status set on the route", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodPut, "/api/v1/items/it_1/content", ""))

		assert.Equal(t, http.StatusAccepted, rec.Code)
	})
}

func TestHandle(t *testing.T) {
	t.Parallel()

	r := routing.New(routing.Options{})
	page := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		route, ok := routing.CurrentRoute(req.Context())
		require.True(t, ok)
		stored, ok := http.RequestFrom(req.Context())
		require.True(t, ok)
		_, _ = w.Write([]byte(route.Name() + " " + stored.PathValue("slug")))
	})
	r.Handle(http.MethodGet, "/pages/{slug}", "pages.show", page)

	t.Run("serves plain handlers with the route in context", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/pages/about", ""))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "pages.show about", rec.Body.String())
	})

	t.Run("answers unknown paths with a JSON 404", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/nowhere", ""))

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"message":"Not Found.","code":"not_found"}`, rec.Body.String())
	})

	t.Run("answers other methods with 405 and the allowed ones", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodPost, "/pages/about", ""))

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		assert.Equal(t, []string{http.MethodGet}, rec.Header().Values("Allow"))
		assert.Equal(t, "method_not_allowed", decode(t, rec.Body)["code"])
	})

	t.Run("uses the configured not found handler", func(t *testing.T) {
		t.Parallel()
		custom := routing.New(routing.Options{NotFound: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})})

		assert.Equal(t, http.StatusAccepted, serve(t, custom, request(t, http.MethodGet, "/nowhere", "")).Code)
	})
}

func TestMiddleware(t *testing.T) {
	t.Parallel()

	kernel := http.NewKernel()
	kernel.Alias("tag", tagFactory)
	kernel.Alias("ability", func(...string) (http.Middleware, error) {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				route, _ := routing.CurrentRoute(r.Context())
				w.Header().Set("X-Abilities", route.Abilities()[0])
				next.ServeHTTP(w, r)
			})
		}, nil
	})
	kernel.Group("web", "tag:web")

	r := newRouter(t, kernel)
	r.Middleware("web")
	r.API("/api", "Test API", "1.0.0", func(g *routing.Router) {
		routing.Get(g, "/items/{id}", "items.show", func(_ context.Context, in *showInput) (*itemOutput, error) {
			return &itemOutput{Body: http.NewResource(item{ID: in.ID})}, nil
		}).Middleware("tag:route", "ability").Use(tag("raw-route")).Ability("items:read")
		g.Middleware("tag:api")
		g.Use(tag("raw-api"))
	})

	t.Run("runs group middleware outermost first, then the route's", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/items/it_1", ""))

		require.Equal(t, http.StatusOK, rec.Code)
		want := []string{"tag:web", "tag:api", "raw-api", "tag:route", "raw-route"}
		assert.Equal(t, want, rec.Header().Values("X-Trace"))
	})

	t.Run("lets middleware read the matched route", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/items/it_1", ""))

		assert.Equal(t, "items:read", rec.Header().Get("X-Abilities"))
	})

	t.Run("serves the document without route middleware", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/openapi.json", ""))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Values("X-Trace"))
	})
}

func TestHandler(t *testing.T) {
	t.Parallel()

	noop := func(context.Context, *showInput) (*struct{}, error) { return nil, nil }

	t.Run("refuses unknown middleware names", func(t *testing.T) {
		t.Parallel()
		r := newRouter(t, http.NewKernel())
		r.Handle(http.MethodGet, "/", "home", nothing()).Middleware("missing")

		_, err := r.Handler()
		require.ErrorIs(t, err, http.ErrUnknownMiddleware)
	})

	t.Run("refuses middleware names without a resolver", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		r.Group("/admin", func(g *routing.Router) {
			g.Middleware("auth")
			g.Handle(http.MethodGet, "/", "admin", nothing())
		})

		_, err := r.Handler()
		require.ErrorIs(t, err, http.ErrUnknownMiddleware)
	})

	t.Run("refuses duplicate names", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		r.Handle(http.MethodGet, "/a", "same", nothing())
		r.Handle(http.MethodGet, "/b", "same", nothing())

		_, err := r.Handler()
		require.ErrorIs(t, err, routing.ErrDuplicateName)
	})

	t.Run("refuses typed routes outside an api", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		routing.Get(r, "/items/{id}", "items.show", noop)

		_, err := r.Handler()
		require.ErrorIs(t, err, routing.ErrNoAPI)
	})

	t.Run("refuses routes the api layer cannot serve", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		r.Handle(http.MethodGet, "no-slash", "broken", nothing())

		_, err := r.Handler()
		require.ErrorIs(t, err, routing.ErrInvalidRoute)
	})

	t.Run("builds the table once", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		first, err := r.Handler()
		require.NoError(t, err)
		r.Handle(http.MethodGet, "/late", "late", nothing())

		second, err := r.Handler()
		require.NoError(t, err)
		assert.Same(t, first, second)
	})
}

func TestOpenAPI(t *testing.T) {
	t.Parallel()

	declare := func() *routing.Router {
		r := newRouter(t, http.NewKernel())
		r.Group("/api", func(api *routing.Router) {
			api.API("/v1", "Test API", "1.0.0", func(g *routing.Router) {
				routing.Get(g, "/items/{id}", "items.show", func(context.Context, *showInput) (*itemOutput, error) {
					return nil, nil
				}).Summary("Show an item").Tags("items").Ability("items:read")
				routing.Get(g, "", "root.show", func(context.Context, *struct{}) (*struct{}, error) {
					return nil, nil
				})
			})
		})

		return r
	}

	t.Run("documents operations relative to the prefix", func(t *testing.T) {
		t.Parallel()
		doc, err := declare().OpenAPI("/api/v1")
		require.NoError(t, err)

		var spec any
		require.NoError(t, json.Unmarshal(doc, &spec))

		assert.Equal(t, []any{map[string]any{"url": "/api/v1"}}, lookup(spec, "servers"))
		show := lookup(spec, "paths", "/items/{id}", "get")
		assert.Equal(t, "items.show", lookup(show, "operationId"))
		assert.Equal(t, "Show an item", lookup(show, "summary"))
		assert.Equal(t, []any{"items:read"}, lookup(show, "x-abilities"))
		assert.NotNil(t, lookup(spec, "paths", "/"))
		assert.NotNil(t, lookup(spec, "components", "schemas", "ResponseError"))
	})

	t.Run("produces the same bytes every time", func(t *testing.T) {
		t.Parallel()
		first, err := declare().OpenAPI("/api/v1")
		require.NoError(t, err)
		second, err := declare().OpenAPI("/api/v1")
		require.NoError(t, err)

		assert.Equal(t, first, second)
	})

	t.Run("refuses an unknown prefix", func(t *testing.T) {
		t.Parallel()
		_, err := declare().OpenAPI("/partner/v1")

		require.ErrorIs(t, err, routing.ErrUnknownAPI)
	})

	t.Run("returns the build error", func(t *testing.T) {
		t.Parallel()
		r := routing.New(routing.Options{})
		r.Handle(http.MethodGet, "/", "home", nothing()).Middleware("missing")

		_, err := r.OpenAPI("/api")
		require.ErrorIs(t, err, http.ErrUnknownMiddleware)
	})
}

func TestRoutes(t *testing.T) {
	t.Parallel()

	r := routing.New(routing.Options{})
	r.Middleware("web")
	r.Group("/b", func(g *routing.Router) {
		g.Middleware("auth")
		g.Handle(http.MethodPost, "/", "b.store", nothing()).Middleware("throttle:api")
		g.Handle(http.MethodGet, "/", "b.index", nothing())
	})
	r.Handle(http.MethodGet, "/a", "a.index", nothing())

	t.Run("lists routes by path and method", func(t *testing.T) {
		t.Parallel()
		var got []string
		for _, route := range r.Routes() {
			got = append(got, route.Method()+" "+route.Path()+" "+route.Name())
		}

		assert.Equal(t, []string{"GET /a a.index", "GET /b/ b.index", "POST /b/ b.store"}, got)
	})

	t.Run("lists middleware names outermost first", func(t *testing.T) {
		t.Parallel()
		store := r.Routes()[2]

		assert.Equal(t, []string{"web", "auth", "throttle:api"}, store.MiddlewareNames())
		assert.Empty(t, store.Abilities())
	})
}

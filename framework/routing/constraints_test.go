package routing_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/routing"
)

type address struct {
	City string `json:"city" validate:"required,max=40"`
}

type ruledInput struct {
	Status string `query:"status" validate:"omitempty,oneof=open closed"`
	Limit  int    `query:"limit"  validate:"min=1,max=50"`
	Body   struct {
		Name     string            `json:"name"               validate:"required,min=2,max=80"`
		Email    string            `json:"email,omitempty"    validate:"omitempty,email"`
		Site     string            `json:"site,omitempty"     validate:"omitempty,url"`
		Priority int               `json:"priority,omitempty" validate:"omitempty,oneof=1 2 3"`
		Tags     []string          `json:"tags,omitempty"     validate:"max=10,dive,max=30"`
		Code     string            `json:"code,omitempty"     validate:"omitempty,len=6"`
		Home     *address          `json:"home,omitempty"`
		Others   []address         `json:"others,omitempty"`
		Extra    string            `json:"-"`
		Labels   map[string]string `json:"labels,omitempty" validate:"max=5"`
		Note     string            `json:"note,omitempty"`
	}
}

func TestConstraints(t *testing.T) {
	t.Parallel()

	doc := func(t *testing.T) any {
		t.Helper()
		r := newRouter(t, http.NewKernel())
		r.Group("/api", func(api *routing.Router) {
			api.API("/v1", "Test API", "1.0.0", func(g *routing.Router) {
				routing.Post(g, "/things", "things.store", func(context.Context, *ruledInput) (*struct{}, error) {
					return nil, nil
				})
				g.Handle(http.MethodGet, "/plain", "plain.show", nothing())
				routing.Get(g, "/listed", "listed.index", func(context.Context, *struct {
					Limit int `query:"limit"`
				},
				) (*struct{}, error) {
					return nil, nil
				}).Lists(pagination.Allow{
					Filters: []string{"status"}, Sorts: []string{"created_at"}, Includes: []string{"tags"},
				}).Query(routing.QueryParam{
					Name: "format", Description: "File type.", Values: []string{"csv", "json"},
				})
			})
		})
		raw, err := r.OpenAPI("/api/v1")
		require.NoError(t, err)
		var spec any
		require.NoError(t, json.Unmarshal(raw, &spec))

		return spec
	}

	t.Run("states the body's rules in its schema", func(t *testing.T) {
		t.Parallel()
		spec := doc(t)
		ref, _ := lookup(spec, "paths", "/things", "post", "requestBody", "content", "application/json", "schema",
			"$ref").(string)
		body := lookup(spec, "components", "schemas", ref[len("#/components/schemas/"):])
		props := lookup(body, "properties")

		assert.Contains(t, lookup(body, "required"), "name")
		assert.InDelta(t, 2, lookup(props, "name", "minLength"), 0)
		assert.InDelta(t, 80, lookup(props, "name", "maxLength"), 0)
		assert.Equal(t, "email", lookup(props, "email", "format"))
		assert.Equal(t, "uri", lookup(props, "site", "format"))
		assert.Equal(t, []any{1.0, 2.0, 3.0}, lookup(props, "priority", "enum"))
		assert.InDelta(t, 10, lookup(props, "tags", "maxItems"), 0)
		assert.InDelta(t, 30, lookup(props, "tags", "items", "maxLength"), 0)
		assert.InDelta(t, 6, lookup(props, "code", "maxLength"), 0)
		assert.InDelta(t, 5, lookup(props, "labels", "maxProperties"), 0)
	})

	t.Run("follows nested types once each", func(t *testing.T) {
		t.Parallel()
		spec := doc(t)
		home := lookup(spec, "components", "schemas", "Address")

		assert.Contains(t, lookup(home, "required"), "city")
		assert.InDelta(t, 40, lookup(home, "properties", "city", "maxLength"), 0)
	})

	t.Run("states the parameters' rules", func(t *testing.T) {
		t.Parallel()
		spec := doc(t)
		params, _ := lookup(spec, "paths", "/things", "post", "parameters").([]any)

		byName := map[string]any{}
		for _, p := range params {
			name, _ := lookup(p, "name").(string)
			byName[name] = p
		}

		assert.Equal(t, []any{"open", "closed"}, lookup(byName["status"], "schema", "enum"))
		assert.InDelta(t, 1, lookup(byName["limit"], "schema", "minimum"), 0)
		assert.InDelta(t, 50, lookup(byName["limit"], "schema", "maximum"), 0)
	})

	t.Run("documents the list and query parameters a handler reads itself", func(t *testing.T) {
		t.Parallel()
		spec := doc(t)
		params, _ := lookup(spec, "paths", "/listed", "get", "parameters").([]any)

		byName := map[string]any{}
		for _, p := range params {
			name, _ := lookup(p, "name").(string)
			byName[name] = p
		}

		assert.Contains(t, byName, "filter[status]")
		assert.Equal(t, []any{"created_at", "-created_at"}, lookup(byName["sort"], "schema", "enum"))
		assert.Contains(t, lookup(byName["include"], "description"), "tags")
		assert.Contains(t, byName, "cursor")
		assert.Contains(t, byName, "q")
		assert.Equal(t, []any{"csv", "json"}, lookup(byName["format"], "schema", "enum"))
		assert.Equal(t, "integer", lookup(byName["limit"], "schema", "type"))
	})
}

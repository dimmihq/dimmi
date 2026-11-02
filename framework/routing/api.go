/*
|--------------------------------------------------------------------------
| API Documents
|--------------------------------------------------------------------------
|
| Each API group is one OpenAPI 3.1 document, generated at runtime from
| the Go types of its routes, so the document can never drift from the
| code. Errors raised while binding or checking a request, before any
| handler runs, are rewritten into the same envelope handlers produce, so
| a client parses one error shape whatever went wrong.
|
*/

package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	humavalidation "github.com/danielgtaylor/huma/v2/validation"
	"github.com/go-chi/chi/v5"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ErrUnknownAPI is returned for a prefix no API group was declared with.
*/
var ErrUnknownAPI = errors.New("routing: unknown api")

/*
apiSpec is one declared API group and, once built, its document.
*/
type apiSpec struct {
	prefix  string
	title   string
	version string
	api     huma.API
}

/*
adapter mounts the API layer's operations on the shared router under the
API's prefix, each behind its route's middleware.
*/
type adapter struct {
	mux    chi.Router
	prefix string
	routes map[string]*Route
}

/*
Handle registers an operation. Operations without a declared route, such
as the document itself, are served without middleware.
*/
func (a *adapter) Handle(op *huma.Operation, h func(huma.Context)) {
	var chain []http.Middleware
	if route, ok := a.routes[op.OperationID]; ok {
		chain = route.chain
	}

	a.mux.With(chain...).MethodFunc(op.Method, a.prefix+op.Path, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		h(humachi.NewContext(op, r, w))
	})
}

/*
ServeHTTP serves the shared router.
*/
func (a *adapter) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	a.mux.ServeHTTP(w, r)
}

/*
OpenAPI builds the route table if needed and returns the JSON document
of the API declared at the prefix.
*/
func (r *Router) OpenAPI(prefix string) ([]byte, error) {
	if _, err := r.Handler(); err != nil {
		return nil, err
	}

	for _, spec := range r.table.apis {
		if spec.prefix != prefix {
			continue
		}

		doc, err := json.Marshal(spec.api.OpenAPI())
		if err != nil {
			return nil, fmt.Errorf("encode openapi %s: %w", prefix, err)
		}

		return doc, nil
	}

	return nil, fmt.Errorf("openapi %s: %w", prefix, ErrUnknownAPI)
}

/*
newAPI creates the document for one API group: JSON only, served at
/openapi.json under the prefix, without the API layer's schema links in
response bodies, which would break the response envelope.
*/
func newAPI(spec *apiSpec, a *adapter) huma.API {
	schemas := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	formats := map[string]huma.Format{"application/json": huma.DefaultJSONFormat, "json": huma.DefaultJSONFormat}

	return huma.NewAPI(huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI:    "3.1.0",
			Info:       &huma.Info{Title: spec.title, Version: spec.version},
			Servers:    []*huma.Server{{URL: spec.prefix}},
			Components: &huma.Components{Schemas: schemas},
		},
		OpenAPIPath:   "/openapi",
		Formats:       formats,
		DefaultFormat: "application/json",
		Transformers:  []huma.Transformer{transformError},
	}, a)
}

func errorResponse(api huma.API) *huma.Response {
	schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[http.ResponseError](), true, "Error")

	return &huma.Response{
		Description: "Error",
		Content:     map[string]*huma.MediaType{"application/json": {Schema: schema}},
	}
}

/*
transformError rewrites the API layer's own errors, raised while binding
and checking input, into the envelope.
*/
func transformError(ctx huma.Context, _ string, v any) (any, error) {
	model, ok := v.(*huma.ErrorModel)
	if !ok {
		return v, nil
	}

	ctx.SetHeader("Content-Type", "application/json")

	if model.Status != stdhttp.StatusUnprocessableEntity || len(model.Errors) == 0 {
		return http.StatusError(model.Status), nil
	}

	invalid := &validation.Error{}
	for _, detail := range model.Errors {
		invalid.Violations = append(invalid.Violations, violation(ctx.Context(), detail))
	}

	return http.Errors{}.Resolve(ctx.Context(), invalid), nil
}

/*
violation turns one schema error into a field violation. A missing
required property is reported at the property itself, the way a person
reads it, rather than at the object holding it, with the message the
translator has for "validation.required" in the locale of the context.
*/
func violation(ctx context.Context, detail *huma.ErrorDetail) validation.Violation {
	before, after, _ := strings.Cut(humavalidation.MsgExpectedRequiredProperty, "%s")

	property, hasBefore := strings.CutPrefix(detail.Message, before)
	property, hasAfter := strings.CutSuffix(property, after)

	if !hasBefore || !hasAfter {
		return validation.Violation{Field: field(detail.Location), Rule: "invalid", Message: detail.Message}
	}

	name := strings.TrimPrefix(field(detail.Location)+"."+property, ".")
	message := "The " + name + " field is required."

	if t := i18n.From(ctx); t != nil {
		if localized, ok := t.Lookup(ctx, "validation.required", i18n.Args{"field": name}); ok {
			message = localized
		}
	}

	return validation.Violation{Field: name, Rule: "required", Message: message}
}

/*
field turns a location such as "body.items[3].tags" into the dotted
path clients address fields by, "items.3.tags". Locations outside the
body keep their source, as in "query.limit", and the body itself is "".
*/
func field(location string) string {
	path := strings.NewReplacer("[", ".", "]", "").Replace(location)
	if path == "body" {
		return ""
	}

	if rest, ok := strings.CutPrefix(path, "body."); ok {
		return rest
	}

	return path
}

/*
check runs the input's own hooks and tag rules: authorization first, so
a caller who may not act learns nothing about the input, then the body's
tags, then the input's extra rules.
*/
func check(ctx context.Context, in any, v *validation.Factory) error {
	if a, ok := in.(http.Authorizer); ok {
		if err := a.Authorize(ctx); err != nil {
			return err
		}
	}

	if v != nil {
		if err := v.Struct(ctx, in); err != nil {
			return bodyFields(err)
		}
	}

	if c, ok := in.(http.Validatable); ok {
		return c.Validate(ctx, v)
	}

	return nil
}

/*
bodyFields names the body's fields as the client sent them, "name"
rather than "Body.name"; fields outside the body keep their path.
*/
func bodyFields(err error) error {
	invalid, ok := errors.AsType[*validation.Error](err)
	if !ok {
		return err
	}

	for i, v := range invalid.Violations {
		invalid.Violations[i].Field = strings.TrimPrefix(v.Field, "Body.")
	}

	return invalid
}

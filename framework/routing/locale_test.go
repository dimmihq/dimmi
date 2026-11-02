package routing_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
inFrench returns the request with a context speaking French through a
translator that knows a few messages.
*/
func inFrench(t *testing.T, req *http.Request) *http.Request {
	t.Helper()

	fr := fstest.MapFS{
		"fr/errors.json":     {Data: []byte(`{"not_found": "Introuvable.", "method_not_allowed": "Méthode refusée."}`)},
		"fr/validation.json": {Data: []byte(`{"required": "Le champ {{.field}} est obligatoire."}`)},
	}
	translator, err := i18n.New(i18n.Config{Locale: "en"}, validation.Messages(), http.Messages(), fr)
	require.NoError(t, err)

	return req.WithContext(i18n.WithLocale(i18n.WithTranslator(req.Context(), translator), "fr"))
}

func TestLocalizedErrors(t *testing.T) {
	t.Parallel()

	r := newRouter(t, http.NewKernel())
	r.API("/api/v1", "Test API", "1.0.0", func(g *routing.Router) {
		routing.Post(g, "/items", "items.store", func(_ context.Context, in *createInput) (*itemOutput, error) {
			return &itemOutput{Body: http.NewResource(item{ID: "new", Name: in.Body.Name})}, nil
		})
	})

	t.Run("answers unknown paths and refused methods in the locale of the request", func(t *testing.T) {
		t.Parallel()

		missing := serve(t, r, inFrench(t, request(t, http.MethodGet, "/nowhere", "")))
		refused := serve(t, r, inFrench(t, request(t, http.MethodDelete, "/api/v1/items", "")))

		assert.JSONEq(t, `{"message":"Introuvable.","code":"not_found"}`, missing.Body.String())
		assert.Equal(t, http.StatusMethodNotAllowed, refused.Code)
		assert.JSONEq(t, `{"message":"Méthode refusée.","code":"method_not_allowed"}`, refused.Body.String())
	})

	t.Run("names a missing property in the locale of the request", func(t *testing.T) {
		t.Parallel()

		rec := serve(t, r, inFrench(t, request(t, http.MethodPost, "/api/v1/items", `{"email":"a@example.com"}`)))

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Equal(t, []any{"Le champ name est obligatoire."}, lookup(decode(t, rec.Body), "errors", "name"))
	})

	t.Run("keeps the english message without a translator", func(t *testing.T) {
		t.Parallel()

		rec := serve(t, r, request(t, http.MethodPost, "/api/v1/items", `{"email":"a@example.com"}`))

		assert.Equal(t, []any{"The name field is required."}, lookup(decode(t, rec.Body), "errors", "name"))
	})
}

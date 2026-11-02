package http_test

import (
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
translator holds the framework's English error and validation messages
and a few French ones, as an application with a fr lang directory has.
*/
func translator(t *testing.T) *i18n.Translator {
	t.Helper()

	fr := fstest.MapFS{
		"fr/errors.json": {Data: []byte(`{
			"not_found": "Introuvable.",
			"too_many_requests": "Trop de requêtes. Réessayez dans {{.retry_after}} secondes.",
			"internal_error": "Erreur du serveur."
		}`)},
		"fr/validation.json": {Data: []byte(`{
			"summary": {"one": "{{.message}} (et {{.count}} autre erreur)",
			            "other": "{{.message}} (et {{.count}} autres erreurs)"}
		}`)},
	}

	translator, err := i18n.New(i18n.Config{Locale: "en"}, http.Messages(), validation.Messages(), fr)
	require.NoError(t, err)

	return translator
}

/*
renderIn renders the error for a request in the locale, with the
translator in the context as the locale middleware leaves it.
*/
func renderIn(t *testing.T, locale string, err error) *httptest.ResponseRecorder {
	t.Helper()

	ctx := i18n.WithLocale(i18n.WithTranslator(t.Context(), translator(t)), locale)
	rec := httptest.NewRecorder()
	http.Errors{}.Render(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", stdhttp.NoBody), err)

	return rec
}

func TestLocalizedErrors(t *testing.T) {
	t.Parallel()

	t.Run("localizes the message by code and keeps code and details", func(t *testing.T) {
		t.Parallel()
		err := fault.New(fault.RateLimited, "too_many_requests", "Too many requests.").With("retry_after", 30)

		en := renderIn(t, "en", err)
		fr := renderIn(t, "fr", err)

		assert.Equal(t, http.StatusTooManyRequests, fr.Code)
		assert.JSONEq(t, `{"message":"Too many requests.","code":"too_many_requests","retry_after":30}`,
			en.Body.String())
		assert.JSONEq(t,
			`{"message":"Trop de requêtes. Réessayez dans 30 secondes.","code":"too_many_requests","retry_after":30}`,
			fr.Body.String())
	})

	t.Run("lists the localized message under the field a fault blames", func(t *testing.T) {
		t.Parallel()
		err := fault.New(fault.NotFound, "not_found", "Not Found.").OnField("email")

		rec := renderIn(t, "fr", err)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"message":"Introuvable.","code":"not_found","errors":{"email":["Introuvable."]}}`,
			rec.Body.String())
	})

	t.Run("localizes plain status errors and hidden failures", func(t *testing.T) {
		t.Parallel()

		assert.JSONEq(t, `{"message":"Introuvable.","code":"not_found"}`,
			renderIn(t, "fr", http.StatusError(http.StatusNotFound)).Body.String())
		assert.JSONEq(t, `{"message":"Erreur du serveur.","code":"internal_error"}`,
			renderIn(t, "fr", errors.New("disk on fire")).Body.String())
		assert.JSONEq(t, `{"message":"The request body is too large.","code":"payload_too_large"}`,
			renderIn(t, "fr", &stdhttp.MaxBytesError{Limit: 1}).Body.String())
	})

	t.Run("keeps the original message for a code no locale has", func(t *testing.T) {
		t.Parallel()
		err := fault.New(fault.Conflict, "seat_taken", "That seat is taken.")

		assert.JSONEq(t, `{"message":"That seat is taken.","code":"seat_taken"}`, renderIn(t, "fr", err).Body.String())
	})

	t.Run("does not change a shared response error", func(t *testing.T) {
		t.Parallel()
		shared := http.StatusError(http.StatusNotFound)

		renderIn(t, "fr", shared)

		assert.Equal(t, "Not Found.", shared.Message)
	})

	t.Run("summarizes validation errors in the locale and keeps field keys", func(t *testing.T) {
		t.Parallel()
		err := &validation.Error{Violations: []validation.Violation{
			{Field: "email", Rule: "required", Message: "Le champ email est obligatoire."},
			{Field: "name", Rule: "required", Message: "Le champ name est obligatoire."},
		}}

		rec := renderIn(t, "fr", err)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.JSONEq(t, `{"message":"Le champ email est obligatoire. (et 1 autre erreur)","code":"validation_failed",`+
			`"errors":{"email":["Le champ email est obligatoire."],"name":["Le champ name est obligatoire."]}}`,
			rec.Body.String())
	})

	t.Run("keeps the summary of a response that lists fields", func(t *testing.T) {
		t.Parallel()
		res := &http.ResponseError{
			Status: http.StatusUnprocessableEntity, Message: "Bad.", Code: "validation_failed",
			Errors: map[string][]string{"name": {"Bad."}},
		}

		assert.Contains(t, renderIn(t, "fr", res).Body.String(), `"message":"Bad."`)
	})

	t.Run("keeps the english summary when no locale has one", func(t *testing.T) {
		t.Parallel()
		bare, err := i18n.New(i18n.Config{Locale: "en"})
		require.NoError(t, err)
		ctx := i18n.WithTranslator(t.Context(), bare)
		invalid := &validation.Error{Violations: []validation.Violation{
			{Field: "a", Message: "A."}, {Field: "b", Message: "B."},
		}}

		res := http.Errors{}.Resolve(ctx, invalid)

		assert.Equal(t, "A. (and 1 more error)", res.Message)
	})

	t.Run("ships an english message for every built-in code", func(t *testing.T) {
		t.Parallel()
		keys, err := i18n.Keys(http.Messages())
		require.NoError(t, err)

		for _, code := range []string{
			http.CodeValidationFailed, http.CodeInternal, http.CodePayloadTooLarge, "unauthenticated",
			"missing_ability", "not_found", "too_many_requests", "maintenance", "idempotency_key_in_use",
		} {
			assert.Contains(t, keys["en"], "errors."+code)
		}
	})
}

package http_test

import (
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/validation"
)

func render(t *testing.T, errs http.Errors, err error) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	errs.Render(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", stdhttp.NoBody), err)

	return rec
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("maps fault kinds to statuses and writes details beside the code", func(t *testing.T) {
		t.Parallel()
		err := fault.New(fault.Forbidden, "missing_ability", "Missing ability.").With("required_ability", "x:write")
		rec := render(t, http.Errors{}, fmt.Errorf("update: %w", err))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"message":"Missing ability.","code":"missing_ability","required_ability":"x:write"}`,
			rec.Body.String())
	})

	t.Run("keeps message and code over details of the same name", func(t *testing.T) {
		t.Parallel()
		err := fault.New(fault.Conflict, "stale", "Stale.").With("code", "other").With("message", "other")

		assert.JSONEq(t, `{"message":"Stale.","code":"stale"}`, render(t, http.Errors{}, err).Body.String())
	})

	t.Run("renders validation errors as 422 with the fields", func(t *testing.T) {
		t.Parallel()
		err := &validation.Error{Violations: []validation.Violation{
			{Field: "email", Rule: "required", Message: "The email field is required."},
		}}
		rec := render(t, http.Errors{}, err)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.JSONEq(t, `{"message":"The email field is required.","code":"validation_failed",`+
			`"errors":{"email":["The email field is required."]}}`, rec.Body.String())
	})

	t.Run("answers malformed list queries with 400", func(t *testing.T) {
		t.Parallel()
		rec := render(t, http.Errors{}, pagination.ErrInvalidCursor)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("lets the application override statuses by code", func(t *testing.T) {
		t.Parallel()
		errs := http.Errors{Statuses: map[string]int{"tenant_required": http.StatusBadRequest}}
		rec := render(t, errs, fault.New(fault.Invalid, "tenant_required", "Pick a tenant."))

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("answers oversized bodies with 413", func(t *testing.T) {
		t.Parallel()
		rec := render(t, http.Errors{}, fmt.Errorf("read body: %w", &stdhttp.MaxBytesError{Limit: 10}))

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		assert.JSONEq(t, `{"message":"Request Entity Too Large.","code":"payload_too_large"}`, rec.Body.String())
	})

	t.Run("passes response errors through", func(t *testing.T) {
		t.Parallel()
		rec := render(t, http.Errors{}, http.StatusError(http.StatusGone))

		assert.Equal(t, http.StatusGone, rec.Code)
		assert.JSONEq(t, `{"message":"Gone.","code":"gone"}`, rec.Body.String())
	})

	t.Run("reports unknown errors and hides them", func(t *testing.T) {
		t.Parallel()
		var reported error
		errs := http.Errors{Report: func(_ context.Context, err error) { reported = err }}
		rec := render(t, errs, errors.New("disk on fire"))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.JSONEq(t, `{"message":"Server Error.","code":"internal_error"}`, rec.Body.String())
		assert.EqualError(t, reported, "disk on fire")
	})

	t.Run("names response errors in logs", func(t *testing.T) {
		t.Parallel()
		err := http.StatusError(http.StatusNotFound)

		assert.Equal(t, "not_found: Not Found.", err.Error())
		assert.Equal(t, http.StatusNotFound, err.GetStatus())
	})
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	t.Run("writes the value with a status", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		http.WriteJSON(rec, http.StatusAccepted, map[string]int{"n": 1})

		assert.Equal(t, http.StatusAccepted, rec.Code)
		assert.Equal(t, "{\"n\":1}\n", rec.Body.String())
	})

	t.Run("answers 500 for values that cannot be encoded", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		http.WriteJSON(rec, http.StatusOK, make(chan int))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Empty(t, rec.Body.String())
	})
}

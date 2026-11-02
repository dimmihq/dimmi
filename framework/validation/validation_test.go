package validation_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/validation"
)

type address struct {
	City string `json:"city" validate:"required"`
}

type invite struct {
	Name      string            `json:"name"       validate:"required,max=5"`
	Emails    []string          `json:"emails"     validate:"required,dive,email"`
	Address   address           `json:"address"`
	Labels    map[string]string `json:"labels"     validate:"dive,keys,alpha,endkeys,max=3"`
	Handle    *string           `json:"handle"     validate:"omitempty,prefixed=usr"`
	Note      string            `json:"-"          validate:"max=3"`
	Untagged  string            `validate:"max=3"`
	Reference string            `json:"reference"  validate:"omitempty,prefixed=ref"`
}

type prefixed struct{}

func (prefixed) Name() string { return "prefixed" }

func (prefixed) Message() string { return "The {field} field must start with {param}_." }

func (prefixed) Passes(_ context.Context, value any, param string) bool {
	s, ok := value.(string)

	return ok && strings.HasPrefix(s, param+"_")
}

type unnamed struct{}

func (unnamed) Name() string { return "" }

func (unnamed) Message() string { return "" }

func (unnamed) Passes(context.Context, any, string) bool { return true }

func newFactory(t *testing.T) *validation.Factory {
	t.Helper()

	factory, err := validation.New()
	require.NoError(t, err)
	require.NoError(t, factory.Register(prefixed{}))

	return factory
}

func validInvite() invite {
	return invite{Name: "ana", Emails: []string{"ana@example.com"}, Address: address{City: "Lagos"}}
}

func violations(t *testing.T, err error) *validation.Error {
	t.Helper()

	var failed *validation.Error
	require.ErrorAs(t, err, &failed)

	return failed
}

func TestFactory(t *testing.T) {
	t.Parallel()

	t.Run("passes valid input", func(t *testing.T) {
		t.Parallel()
		input := validInvite()

		require.NoError(t, newFactory(t).Struct(t.Context(), &input))
	})

	t.Run("names fields by their json paths", func(t *testing.T) {
		t.Parallel()
		input := invite{
			Emails:   []string{"ana@example.com", "nope"},
			Labels:   map[string]string{"x": "long value"},
			Note:     "long",
			Untagged: "long",
		}

		failed := violations(t, newFactory(t).Struct(t.Context(), input))

		fields := make([]string, 0, len(failed.Violations))
		for _, v := range failed.Violations {
			fields = append(fields, v.Field)
		}
		assert.ElementsMatch(t, []string{"name", "emails.1", "address.city", "labels.x", "Note", "Untagged"}, fields)
	})

	t.Run("reports rule codes, parameters and english messages", func(t *testing.T) {
		t.Parallel()
		input := validInvite()
		input.Name = "too long"
		input.Emails = []string{"nope"}

		failed := violations(t, newFactory(t).Struct(t.Context(), input))

		assert.Equal(t, []validation.Violation{
			{Field: "name", Rule: "max", Param: "5", Message: "The name field must not be longer than 5 characters."},
			{Field: "emails.0", Rule: "email", Message: "The emails.0 field must be a valid email address."},
		}, failed.Violations)
	})

	t.Run("summarizes and groups messages for a response", func(t *testing.T) {
		t.Parallel()
		input := invite{Emails: []string{"a", "b"}, Address: address{City: "Lagos"}}

		failed := violations(t, newFactory(t).Struct(t.Context(), input))

		assert.Equal(t, "The name field is required. (and 2 more errors)", failed.Message())
		assert.Equal(t, map[string][]string{
			"name":     {"The name field is required."},
			"emails.0": {"The emails.0 field must be a valid email address."},
			"emails.1": {"The emails.1 field must be a valid email address."},
		}, failed.Messages())
		assert.Equal(t, "validation: name: The name field is required.; "+
			"emails.0: The emails.0 field must be a valid email address.; "+
			"emails.1: The emails.1 field must be a valid email address.", failed.Error())
	})

	t.Run("applies registered rules with their own messages", func(t *testing.T) {
		t.Parallel()
		input := validInvite()
		handle := "usr_ana"
		input.Handle = &handle
		input.Reference = "abc"

		failed := violations(t, newFactory(t).Struct(t.Context(), input))

		assert.Equal(t, []validation.Violation{{
			Field: "reference", Rule: "prefixed", Param: "ref", Message: "The reference field must start with ref_.",
		}}, failed.Violations)
		assert.Equal(t, "The reference field must start with ref_. (and 1 more error)",
			(&validation.Error{Violations: append(failed.Violations, failed.Violations...)}).Message())
	})

	t.Run("refuses a rule without a name", func(t *testing.T) {
		t.Parallel()
		factory, err := validation.New()
		require.NoError(t, err)

		require.ErrorContains(t, factory.Register(unnamed{}), `register rule ""`)
	})

	t.Run("refuses a value that is not a struct", func(t *testing.T) {
		t.Parallel()

		err := newFactory(t).Struct(t.Context(), "text")

		require.ErrorIs(t, err, validation.ErrNotStruct)
	})

	t.Run("summarizes an empty error as nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, (&validation.Error{}).Message())
	})
}

type sizes struct {
	Title  string         `json:"title"  validate:"min=1"`
	Seats  int            `json:"seats"  validate:"max=10"`
	Tags   []string       `json:"tags"   validate:"min=2"`
	Code   string         `json:"code"   validate:"uuid"`
	Starts *time.Time     `json:"starts" validate:"omitempty,gt"`
	Labels map[string]int `json:"labels" validate:"omitempty,max=1"`
}

/*
french returns a translator holding the framework's English messages
and a few French ones, as an application with a fr lang directory has.
*/
func french(t *testing.T) *i18n.Translator {
	t.Helper()

	fr := fstest.MapFS{
		"fr/validation.json": {Data: []byte(`{
			"required": "Le champ {{.field}} est obligatoire.",
			"prefixed": "Le champ {{.field}} doit commencer par {{.param}}_.",
			"min": {"string": {"one": "Le champ {{.field}} doit contenir au moins {{.count}} caractère.",
			                   "other": "Le champ {{.field}} doit contenir au moins {{.count}} caractères."}}
		}`)},
	}

	translator, err := i18n.New(i18n.Config{Locale: "en"}, validation.Messages(), fr)
	require.NoError(t, err)

	return translator
}

func TestMessages(t *testing.T) {
	t.Parallel()

	newFrench := func(t *testing.T) *validation.Factory {
		t.Helper()

		factory, err := validation.New(validation.WithTranslator(french(t)))
		require.NoError(t, err)
		require.NoError(t, factory.Register(prefixed{}))

		return factory
	}

	t.Run("renders messages in the locale of the context with stable rule codes", func(t *testing.T) {
		t.Parallel()
		factory := newFrench(t)
		input := validInvite()
		input.Name = ""
		input.Reference = "abc"

		en := violations(t, factory.Struct(i18n.WithLocale(t.Context(), "en"), input))
		fr := violations(t, factory.Struct(i18n.WithLocale(t.Context(), "fr"), input))

		assert.Equal(t, []validation.Violation{
			{Field: "name", Rule: "required", Message: "The name field is required."},
			{Field: "reference", Rule: "prefixed", Param: "ref", Message: "The reference field must start with ref_."},
		}, en.Violations)
		assert.Equal(t, []validation.Violation{
			{Field: "name", Rule: "required", Message: "Le champ name est obligatoire."},
			{
				Field: "reference", Rule: "prefixed", Param: "ref",
				Message: "Le champ reference doit commencer par ref_.",
			},
		}, fr.Violations)
	})

	t.Run("picks the message for the kind of value and its plural form", func(t *testing.T) {
		t.Parallel()
		factory := newFrench(t)
		input := sizes{Seats: 11, Tags: []string{"a"}, Code: "123e4567-e89b-12d3-a456-426614174000"}

		en := violations(t, factory.Struct(t.Context(), input)).Messages()
		fr := violations(t, factory.Struct(i18n.WithLocale(t.Context(), "fr"), input)).Messages()

		assert.Equal(t, map[string][]string{
			"title": {"The title field must be at least 1 character."},
			"seats": {"The seats field must not be greater than 10."},
			"tags":  {"The tags field must have at least 2 items."},
		}, en)
		assert.Equal(t, []string{"Le champ title doit contenir au moins 1 caractère."}, fr["title"])
		assert.Equal(t, []string{"The seats field must not be greater than 10."}, fr["seats"])
	})

	t.Run("falls back to the generic message for a rule without one", func(t *testing.T) {
		t.Parallel()
		past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		input := sizes{
			Title: "x", Tags: []string{"a", "b"}, Code: "nope", Starts: &past, Labels: map[string]int{"a": 1, "b": 2},
		}

		got := violations(t, newFactory(t).Struct(t.Context(), input)).Messages()

		assert.Equal(t, map[string][]string{
			"code":   {"The code field must be a valid UUID."},
			"starts": {"The starts field is invalid."},
			"labels": {"The labels field must not have more than 1 item."},
		}, got)
	})

	t.Run("ships every message as a parseable english bundle", func(t *testing.T) {
		t.Parallel()

		keys, err := i18n.Keys(validation.Messages())

		require.NoError(t, err)
		assert.Contains(t, keys["en"], "validation.required")
		assert.Contains(t, keys["en"], "validation.min.string")
		assert.Contains(t, keys["en"], "validation.summary")
	})
}

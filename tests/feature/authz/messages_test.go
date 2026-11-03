package authz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/rules"
	"github.com/dimmihq/dimmi/tests"
)

func TestRuleMessages(t *testing.T) {
	t.Parallel()

	tc := tests.New(t)

	t.Run("falls back to the same English the translation files hold", func(t *testing.T) {
		t.Parallel()
		ctx := i18n.WithLocale(t.Context(), "en")

		for name, fallback := range map[string]string{
			"password":   rules.UncommonPassword{}.Message(),
			"registered": rules.Registered{}.Message(),
			"slug":       rules.Slug{}.Message(),
			"timezone":   rules.Timezone{}.Message(),
			"locale":     rules.Locale{}.Message(),
		} {
			translated := tc.App.Lang.T(ctx, "validation."+name, i18n.Args{"field": "{field}"})

			assert.Equal(t, fallback, translated, name)
		}
	})
}

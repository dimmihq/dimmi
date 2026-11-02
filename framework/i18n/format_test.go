package i18n_test

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/i18n"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("writes numbers with the separators of the locale", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "1,234,567.5", i18n.FormatNumber("en", 1234567.5))
		assert.Equal(t, "1.234.567,5", i18n.FormatNumber("de", 1234567.5))
		assert.Equal(t, "1 234 567,5", i18n.FormatNumber("fr", 1234567.5))
		assert.Equal(t, "1,234", i18n.FormatNumber("not a locale", 1234))
	})

	t.Run("writes money from minor units with the currency's decimals", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			locale, currency, want string
			minor                  int64
		}{
			{"en", "USD", "$ 1,234.50", 123450},
			{"de", "EUR", "€ 1.234,50", 123450},
			{"en", "JPY", "¥ 1,234", 1234},
			{"en", "KWD", "KWD 1.234", 1234},
		}

		for _, tc := range cases {
			got, err := i18n.FormatMoney(tc.locale, tc.minor, tc.currency)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		}
	})

	t.Run("refuses a code that is not a currency", func(t *testing.T) {
		t.Parallel()

		_, err := i18n.FormatMoney("en", 100, "ZZZ")

		require.ErrorContains(t, err, `currency "ZZZ"`)
	})

	t.Run("writes dates and times with the layouts of the locale in the location", func(t *testing.T) {
		t.Parallel()
		fr := fstest.MapFS{"fr/formats.json": {Data: []byte(`{"date": "02/01/2006", "time": "15:04"}`)}}
		translator, err := i18n.New(i18n.Config{Locale: "en"}, i18n.Messages(), fr)
		require.NoError(t, err)
		at := time.Date(2026, time.March, 4, 17, 5, 0, 0, time.UTC)
		lagos, err := time.LoadLocation("Africa/Lagos")
		require.NoError(t, err)

		assert.Equal(t, "March 4, 2026", translator.FormatDate("en", at, nil))
		assert.Equal(t, "5:05 PM", translator.FormatTime("en", at, nil))
		assert.Equal(t, "04/03/2026", translator.FormatDate("fr", at, lagos))
		assert.Equal(t, "18:05", translator.FormatTime("fr", at, lagos))
	})

	t.Run("uses english layouts when no locale has any", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2026, time.March, 4, 17, 5, 0, 0, time.UTC)

		assert.Equal(t, "March 4, 2026", newTranslator(t).FormatDate("fr", at, nil))
		assert.Equal(t, "5:05 PM", newTranslator(t).FormatTime("fr", at, nil))
	})
}

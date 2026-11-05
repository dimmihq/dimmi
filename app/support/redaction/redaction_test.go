package redaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/redaction"
)

func TestRedaction(t *testing.T) {
	t.Parallel()

	cards, err := redaction.New(redaction.DefaultSettings(), nil)
	require.NoError(t, err)

	t.Run("masks card numbers of every network and keeps the last four", func(t *testing.T) {
		t.Parallel()

		for in, want := range map[string]string{
			"4242424242424242":    "•••• •••• •••• 4242",
			"4242 4242 4242 4242": "•••• •••• •••• 4242",
			"4242-4242-4242-4242": "•••• •••• •••• 4242",
			"5555555555554444":    "•••• •••• •••• 4444",
			"2223003122003222":    "•••• •••• •••• 3222",
			"378282246310005":     "•••• •••• ••• 0005",
			"6011111111111117":    "•••• •••• •••• 1117",
			"3056930009020004":    "•••• •••• •••• 0004",
			"36227206271667":      "•••• •••• •• 1667",
			"3566002020360505":    "•••• •••• •••• 0505",
			"6200000000000005":    "•••• •••• •••• 0005",
			"4222222222222":       "•••• •••• • 2222",
			"6703444444444449":    "•••• •••• •••• 4449",
			"4000056655665556122": "•••• •••• •••• ••• 6122",
		} {
			out, hits := cards.Apply("card " + in + " thanks")
			assert.Equal(t, "card "+want+" thanks", out, in)
			assert.Equal(t, []redaction.Hit{{Rule: redaction.RuleCardNumbers, Count: 1}}, hits, in)
		}
	})

	t.Run("leaves numbers that fail the check digit or no network issues", func(t *testing.T) {
		t.Parallel()

		for _, in := range []string{
			"4242424242424241",
			"1234567812345670",
			"order 9999999999999995",
			"424242424242",
			"42424242424242424242",
			"call +1 555 0100 2000",
		} {
			out, hits := cards.Apply(in)
			assert.Equal(t, in, out)
			assert.Empty(t, hits)
		}
	})

	t.Run("counts every card in the text", func(t *testing.T) {
		t.Parallel()

		out, hits := cards.Apply("4242424242424242 and 5555555555554444")
		assert.Equal(t, "•••• •••• •••• 4242 and •••• •••• •••• 4444", out)
		assert.Equal(t, 2, redaction.Count(hits))
	})

	t.Run("leaves social security numbers alone by default", func(t *testing.T) {
		t.Parallel()

		out, hits := cards.Apply("ssn 123-45-6789")
		assert.Equal(t, "ssn 123-45-6789", out)
		assert.Empty(t, hits)
	})

	t.Run("masks issued social security numbers when turned on", func(t *testing.T) {
		t.Parallel()

		ssn, err := redaction.New(redaction.Settings{USSSN: true}, nil)
		require.NoError(t, err)

		out, hits := ssn.Apply("123-45-6789 000-12-3456 666-12-3456 900-12-3456 123-00-4567 123-45-0000")
		assert.Equal(t, "•••-••-•••• 000-12-3456 666-12-3456 900-12-3456 123-00-4567 123-45-0000", out)
		assert.Equal(t, []redaction.Hit{{Rule: redaction.RuleUSSSN, Count: 1}}, hits)
	})

	t.Run("applies custom rules after the built-ins in their order", func(t *testing.T) {
		t.Parallel()

		r, err := redaction.New(redaction.DefaultSettings(), []redaction.Rule{
			{Key: "rxr_a", Pattern: `(?i)password:\s*\S+`, Replacement: "password: [hidden]"},
			{Key: "rxr_b", Pattern: `ACCT-\d+`},
		})
		require.NoError(t, err)

		out, hits := r.Apply("Password: hunter2 ACCT-991 card 4242424242424242")
		assert.Equal(t, "password: [hidden] [redacted] card •••• •••• •••• 4242", out)
		assert.Equal(t, []redaction.Hit{
			{Rule: redaction.RuleCardNumbers, Count: 1}, {Rule: "rxr_a", Count: 1}, {Rule: "rxr_b", Count: 1},
		}, hits)
	})

	t.Run("refuses patterns that are empty, too long or broken", func(t *testing.T) {
		t.Parallel()

		for _, pattern := range []string{"", "(", string(make([]byte, redaction.MaxPatternLength+1))} {
			_, err := redaction.New(redaction.Settings{}, []redaction.Rule{{Pattern: pattern}})
			require.ErrorIs(t, err, redaction.ErrInvalidPattern)
		}
	})

	t.Run("applies to several texts and adds the hits up by rule", func(t *testing.T) {
		t.Parallel()

		subject, body := "card 4242424242424242", "also 4242424242424242"

		var html *string

		hits := cards.ApplyAll(&subject, &body, html)
		assert.Equal(t, "card •••• •••• •••• 4242", subject)
		assert.Equal(t, "also •••• •••• •••• 4242", body)
		assert.Equal(t, []redaction.Hit{{Rule: redaction.RuleCardNumbers, Count: 2}}, hits)
	})

	t.Run("does nothing when no rule is on", func(t *testing.T) {
		t.Parallel()

		var none redaction.Redactor

		text := "4242424242424242"
		assert.True(t, none.Empty())
		assert.Nil(t, none.ApplyAll(&text))
		assert.Equal(t, "4242424242424242", text)
	})
}

func FuzzRedaction(f *testing.F) {
	for _, seed := range []string{"4242 4242 4242 4242", "123-45-6789", "", "١٢٣٤٥٦٧٨٩٠١٢٣٤", "4-2-4-2"} {
		f.Add(seed)
	}

	r, err := redaction.New(redaction.Settings{CardNumbers: true, USSSN: true}, []redaction.Rule{
		{Key: "rxr_a", Pattern: `secret\d*`},
	})
	require.NoError(f, err)

	f.Fuzz(func(t *testing.T, text string) {
		out, hits := r.Apply(text)
		if len(hits) == 0 {
			assert.Equal(t, text, out)
		}
	})
}

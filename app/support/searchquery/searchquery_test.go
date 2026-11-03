package searchquery_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/searchquery"
)

func TestParse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, raw, want string
	}{
		{"matches every word and the last as a prefix", "refund stripe", "'refund' & 'stripe':*"},
		{"keeps a phrase together", `"card declined" now`, "('card' <-> 'declined') & 'now':*"},
		{"joins alternatives with or", "refund OR chargeback", "'refund' | 'chargeback':*"},
		{"leaves out excluded words", "refund -invoice", "'refund' & !'invoice'"},
		{"never makes a one letter prefix", "refund a", "'refund' & 'a'"},
		{"splits words on anything but letters and digits", "e-mail", "('e' <-> 'mail')"},
		{"drops quotes and operators from words", `it's & | ! :*`, "('it' <-> 's')"},
		{"treats unknown operators as words", "foo:bar", "('foo' <-> 'bar')"},
		{"reads accents and other scripts", "Rückerstattung 退款", "'rückerstattung' & '退款':*"},
		{"ignores a leading or", "OR refund", "'refund':*"},
		{"has nothing to match in an empty search", "  ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, searchquery.Parse(tc.raw).TSQuery)
		})
	}

	t.Run("jumps to a number or an id", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, int64(1042), *searchquery.Parse("#1042").Number)
		assert.Equal(t, int64(7), *searchquery.Parse(" 7 ").Number)
		assert.Equal(t, "conv_01jbd0k3a8x9t2m4q5r6s7v8w9", searchquery.Parse("CONV_01JBD0K3A8X9T2M4Q5R6S7V8W9").ID)
		assert.Nil(t, searchquery.Parse("1042 refund").Number)
		assert.Nil(t, searchquery.Parse("#99999999999999999999").Number)
	})

	t.Run("reads is filters instead of words", func(t *testing.T) {
		t.Parallel()

		q := searchquery.Parse("is:closed is:snoozed is:unread is:unassigned is:mine is:priority refund is:weird")
		assert.Equal(t, []string{"closed", "snoozed"}, q.Statuses)
		assert.True(t, q.Unread && q.Unassigned && q.Mine && q.Priority)
		assert.Equal(t, "'refund' & ('is' <-> 'weird')", q.TSQuery)
		assert.Equal(t, []string{"refund", "is", "weird"}, q.Words)
	})

	t.Run("reads at most sixteen terms of at most 256 characters", func(t *testing.T) {
		t.Parallel()

		q := searchquery.Parse(strings.Repeat("word ", 40))
		assert.Equal(t, 16, strings.Count(q.TSQuery, "'word'"))
		assert.LessOrEqual(t, len(searchquery.Parse(strings.Repeat("x", 1000)).TSQuery), searchquery.MaxLength+4)
	})
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"refund stripe", `"card declined"`, "-x OR y", "#12", "is:open foo:bar", "'\\:*&|!()",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		q := searchquery.Parse(raw)

		for _, r := range q.TSQuery {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" '&|!()<->:*", r) {
				continue
			}

			t.Fatalf("tsquery %q of %q holds %q", q.TSQuery, raw, r)
		}

		if strings.Count(q.TSQuery, "'")%2 != 0 {
			t.Fatalf("tsquery %q of %q leaves a quote open", q.TSQuery, raw)
		}
	})
}

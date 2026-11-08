package audience_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/audience"
)

func parsed(t *testing.T, raw string) audience.Audience {
	t.Helper()
	a, err := audience.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	return a
}

func warningCodes(warnings []audience.Warning) []string {
	out := []string{}
	for _, w := range warnings {
		out = append(out, w.Code)
	}

	return out
}

func TestLint(t *testing.T) {
	t.Parallel()

	t.Run("warns when two negatives on one field are joined by any", func(t *testing.T) {
		t.Parallel()
		a := parsed(t, `{"rules":{"any":[{"field":"contact.custom.plan","op":"neq","value":"a"},`+
			`{"field":"contact.custom.plan","op":"neq","value":"b"}]}}`)

		w := audience.Lint(a)
		assert.Equal(t, []string{"negative_or"}, warningCodes(w))
		assert.Equal(t, "rules.any", w[0].Path)
	})

	t.Run("warns when one field must equal two values at once", func(t *testing.T) {
		t.Parallel()
		a := parsed(t, `{"rules":{"not":{"all":[{"field":"contact.kind","op":"eq","value":"user"},`+
			`{"field":"contact.kind","op":"eq","value":"lead"}]}}}`)

		assert.Equal(t, []string{"impossible_and"}, warningCodes(audience.Lint(a)))
	})

	t.Run("warns that page values are public and stays quiet otherwise", func(t *testing.T) {
		t.Parallel()
		page := parsed(t, `{"rules":{"all":[{"field":"page.path","op":"starts_with","value":"/a"},`+
			`{"field":"contact.kind","op":"eq","value":"user"},{"field":"contact.kind","op":"eq","value":"user"}]}}`)
		plain := parsed(t, `{"rules":{"field":"contact.kind","op":"eq","value":"user"}}`)

		assert.Equal(t, []string{"page_rule_is_public"}, warningCodes(audience.Lint(page)))
		assert.NotNil(t, audience.Lint(plain))
		assert.Empty(t, audience.Lint(plain))
		assert.Empty(t, audience.Lint(audience.Audience{}))
	})
}

func TestSavedIDs(t *testing.T) {
	t.Parallel()

	t.Run("lists saved audiences once each, in reading order", func(t *testing.T) {
		t.Parallel()
		a := parsed(t, `{"rules":{"any":[{"field":"audience","op":"in","value":["audn_b","audn_a"]},`+
			`{"not":{"field":"audience","op":"not_in","value":["audn_a","audn_c"]}}]}}`)

		assert.Equal(t, []string{"audn_b", "audn_a", "audn_c"}, audience.SavedIDs(a.Rules))
		assert.Empty(t, audience.SavedIDs(nil))
	})

	t.Run("says whether a program names a saved audience", func(t *testing.T) {
		t.Parallel()
		named := parsed(t, `{"rules":{"all":[{"field":"audience","op":"in","value":["audn_a"]}]}}`)
		plain := parsed(t, `{"rules":{"field":"contact.kind","op":"eq","value":"user"}}`)

		p, err := audience.Compile(named, audience.Base(), audience.SavedAudience())
		require.NoError(t, err)
		q, err := audience.Compile(plain, audience.Base(), audience.SavedAudience())
		require.NoError(t, err)

		assert.True(t, p.NeedsSaved())
		assert.False(t, q.NeedsSaved())
		assert.False(t, (*audience.Program)(nil).NeedsSaved())
	})
}

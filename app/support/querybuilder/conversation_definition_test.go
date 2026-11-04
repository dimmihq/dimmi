package querybuilder_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
compiler returns a compiler whose search keeps the text as it is.
*/
func compiler(tb testing.TB) querybuilder.Compiler {
	tb.Helper()

	registry := ids.NewRegistry()
	for _, prefix := range []string{"chn", "inbox", "ctc", "mem", "team", "tag", "cmp"} {
		require.NoError(tb, registry.Register(prefix, prefix))
	}

	return querybuilder.Compiler{
		MemberID: ids.New(), Now: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC), Decode: registry,
		Search: func(text string) (string, error) { return text, nil },
	}
}

/*
condition writes one condition as JSON.
*/
func condition(field, op, value string) string {
	return `{"field":"` + field + `","op":"` + op + `","value":` + value + `}`
}

/*
definition writes a definition of groups, each a join and conditions.
*/
func definition(op string, groups ...string) querybuilder.Definition {
	var d querybuilder.Definition
	_ = json.Unmarshal([]byte(`{"op":"`+op+`","groups":[`+strings.Join(groups, ",")+`]}`), &d)

	return d
}

/*
group writes a group as JSON.
*/
func group(op string, conditions ...string) string {
	return `{"op":"` + op + `","conditions":[` + strings.Join(conditions, ",") + `]}`
}

func TestCompile(t *testing.T) {
	t.Parallel()

	t.Run("numbers the placeholders from the list's next free one", func(t *testing.T) {
		t.Parallel()
		d := definition("or",
			group("and", condition("number", "gt", "3"), condition("subject", "contains", `"a_b"`)),
			group("and", condition("unread", "eq", "false")))

		out, err := compiler(t).Compile(d)
		require.NoError(t, err)

		sql, args := out.Predicate(4)
		assert.Contains(t, sql, "c.number > $4")
		assert.Contains(t, sql, "c.subject ILIKE $5")
		assert.Contains(t, sql, ") OR (")
		assert.Equal(t, []any{int64(3), `%a\_b%`}, args[:2])
		assert.Len(t, args, 4)
	})

	t.Run("names where a definition is wrong", func(t *testing.T) {
		t.Parallel()

		for path, d := range map[string]querybuilder.Definition{
			"op":                          definition("xor"),
			"groups":                      definition("and"),
			"groups.0.conditions":         definition("and", group("and")),
			"groups.0.conditions.0.field": definition("and", group("and", condition("x", "eq", "1"))),
			"groups.0.conditions.0.op":    definition("and", group("and", condition("number", "gte", "1"))),
			"groups.0.conditions.0":       definition("and", group("or", condition("text", "matches", `"a"`))),
		} {
			_, err := compiler(t).Compile(d)

			var def *querybuilder.DefinitionError
			require.True(t, errors.As(err, &def), path)
			assert.Equal(t, path, def.Path)
		}
	})

	t.Run("keeps one search where every join is and", func(t *testing.T) {
		t.Parallel()
		out, err := compiler(t).Compile(definition("and", group("and", condition("text", "matches", `"refund"`))))
		require.NoError(t, err)
		require.NotNil(t, out.Search)
		assert.Equal(t, "refund", *out.Search)
	})
}

func FuzzCompile(f *testing.F) {
	for _, seed := range []string{
		group("and", condition("status", "in", `["open"]`)),
		group("or", condition("created_at", "older_than", `"P1W2DT3H"`)),
		group("and", condition("contact.attr.plan", "eq", `{"a":1}`)),
	} {
		f.Add(`{"op":"and","groups":[` + seed + `]}`)
	}

	c := compiler(f)

	f.Fuzz(func(t *testing.T, raw string) {
		var d querybuilder.Definition
		if json.Unmarshal([]byte(raw), &d) != nil {
			return
		}

		out, err := c.Compile(d)
		if err != nil {
			return
		}

		sql, args := out.Predicate(1)
		assert.Equal(t, strings.Count(sql, "$"), len(args)+strings.Count(sql, "$$"))
	})
}

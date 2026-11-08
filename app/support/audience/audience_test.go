package audience_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
compile parses and compiles an audience for the workflow surface.
*/
func compile(t *testing.T, raw string) *audience.Program {
	t.Helper()

	a, err := audience.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	p, err := audience.Compile(a, audience.Base(), audience.WorkflowAudience())
	require.NoError(t, err)

	return p
}

/*
codes returns the problem codes of an error.
*/
func codes(t *testing.T, err error) []string {
	t.Helper()

	var pe *audience.ProblemsError
	require.ErrorAs(t, err, &pe)

	out := make([]string, 0, len(pe.Problems))
	for _, p := range pe.Problems {
		out = append(out, p.Code)
	}

	return out
}

/*
leaf builds a one-condition audience.
*/
func leaf(field, op, value string) string {
	if value == "" {
		return `{"rules":{"field":"` + field + `","op":"` + op + `"}}`
	}

	return `{"rules":{"field":"` + field + `","op":"` + op + `","value":` + value + `}}`
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("an empty audience matches everyone", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []string{"", "null", "{}"} {
			a, err := audience.Parse(json.RawMessage(raw))
			require.NoError(t, err)
			assert.Nil(t, a.Rules)
			assert.True(t, compile(t, raw+"").Evaluate(audience.Env{}) || raw == "")
		}
	})

	t.Run("refuses unknown keys", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Parse(json.RawMessage(`{"rules":{"field":"contact.name","op":"exists","oops":1}}`))
		assert.Equal(t, []string{audience.CodeSchema}, codes(t, err))
	})

	t.Run("refuses a node mixing a group and a leaf", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Parse(json.RawMessage(
			`{"rules":{"all":[{"field":"contact.name","op":"exists"}],"field":"contact.name","op":"exists"}}`))
		assert.Equal(t, []string{audience.CodeSchema}, codes(t, err))
	})

	t.Run("refuses an empty group with its path", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Parse(json.RawMessage(`{"rules":{"all":[{"any":[]}]}}`))

		var pe *audience.ProblemsError
		require.ErrorAs(t, err, &pe)
		assert.Equal(t, "rules.all[0].any", pe.Problems[0].Path)
		assert.Contains(t, err.Error(), "schema at rules.all[0].any")
	})

	t.Run("refuses a leaf without an operator", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Parse(json.RawMessage(`{"rules":{"field":"contact.name"}}`))
		assert.Equal(t, []string{audience.CodeSchema}, codes(t, err))
	})

	t.Run("refuses an unknown visitor group", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Parse(json.RawMessage(`{"visitors":"robots"}`))
		assert.Equal(t, []string{audience.CodeSchema}, codes(t, err))
	})

	t.Run("refuses broken json", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Parse(json.RawMessage(`{"rules":`))
		assert.Equal(t, []string{audience.CodeSchema}, codes(t, err))
	})

	t.Run("refuses groups nested five deep", func(t *testing.T) {
		t.Parallel()

		raw := `{"rules":{"all":[{"any":[{"all":[{"any":[{"not":{"field":"contact.name","op":"exists"}}]}]}]}]}}`
		_, err := audience.Parse(json.RawMessage(raw))
		assert.Equal(t, []string{audience.CodeLimitExceeded}, codes(t, err))
	})

	t.Run("refuses more than fifty conditions", func(t *testing.T) {
		t.Parallel()

		leaves := make([]map[string]string, 51)
		for i := range leaves {
			leaves[i] = map[string]string{"field": "contact.name", "op": "exists"}
		}

		raw, err := json.Marshal(map[string]any{"rules": map[string]any{"any": leaves}})
		require.NoError(t, err)

		_, err = audience.Parse(raw)
		assert.Equal(t, []string{audience.CodeLimitExceeded}, codes(t, err))
	})

	t.Run("refuses rules over eight kilobytes", func(t *testing.T) {
		t.Parallel()

		big := make([]byte, audience.MaxBytes+1)
		for i := range big {
			big[i] = ' '
		}

		_, err := audience.Parse(append([]byte(`{}`), big...))
		assert.Equal(t, []string{audience.CodeLimitExceeded}, codes(t, err))
	})

	t.Run("reads a bare rule tree", func(t *testing.T) {
		t.Parallel()

		r, err := audience.ParseRule(json.RawMessage(`{"field":"contact.name","op":"exists"}`))
		require.NoError(t, err)
		assert.Equal(t, "contact.name", r.Field)

		r, err = audience.ParseRule(nil)
		require.NoError(t, err)
		assert.Nil(t, r)

		_, err = audience.ParseRule(json.RawMessage(`{"field":1}`))
		require.Error(t, err)
	})
}

func TestCompile(t *testing.T) {
	t.Parallel()

	refused := []struct {
		name, raw, code string
	}{
		{"an unknown field", leaf("contact.shoe_size", "exists", ""), audience.CodeUnknownAttribute},
		{"an operator of another type", leaf("contact.name", "gt", "3"), audience.CodeOpInvalidForType},
		{"a value on a presence check", leaf("contact.name", "exists", `"x"`), audience.CodeValueInvalid},
		{"an empty text", leaf("contact.name", "eq", `" "`), audience.CodeValueInvalid},
		{"a list that is not a list", leaf("contact.name", "in", `"x"`), audience.CodeValueInvalid},
		{"a list with an empty item", leaf("contact.name", "in", `["a",""]`), audience.CodeValueInvalid},
		{"a list with an object", leaf("contact.name", "in", `[{}]`), audience.CodeValueInvalid},
		{
			"a reversed range", `{"rules":{"field":"contact.custom.seats","op":"between","value":[5,1]}}`,
			audience.CodeValueInvalid,
		},
		{"a text where a number goes", leaf("contact.custom.seats", "gt", `"x"`), audience.CodeValueInvalid},
		{"negative days", leaf("contact.last_seen_at", "within_last_days", "-1"), audience.CodeValueInvalid},
		{"fractional days", leaf("contact.last_seen_at", "within_last_days", "1.5"), audience.CodeValueInvalid},
		{"a malformed date", leaf("contact.created_at", "before", `"soon"`), audience.CodeValueInvalid},
		{"a date that is not text", leaf("contact.created_at", "before", "3"), audience.CodeValueInvalid},
		{"a null equality on a custom field", leaf("contact.custom.plan", "eq", `[1]`), audience.CodeValueInvalid},
		{"a page field on a server surface", leaf("page.path", "eq", `"/x"`), audience.CodeFieldUnavailable},
		{
			"within days on a contact field", `{"rules":{"field":"contact.name","op":"exists","within_days":3}}`,
			audience.CodeValueInvalid,
		},
	}

	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			t.Parallel()

			a, err := audience.Parse(json.RawMessage(tc.raw))
			require.NoError(t, err)

			_, err = audience.Compile(a, audience.Base(), audience.WorkflowAudience())
			assert.Equal(t, []string{tc.code}, codes(t, err))
		})
	}

	t.Run("refuses claimed fields on an access surface", func(t *testing.T) {
		t.Parallel()

		access := audience.Surface{
			Key: "help.article", Gate: audience.GateAccess, Where: []audience.Location{audience.Server},
		}
		a, err := audience.Parse(json.RawMessage(leaf("contact.email", "eq", `"a@b.c"`)))
		require.NoError(t, err)

		_, err = audience.Compile(a, audience.Base(), access)
		assert.Equal(t, []string{audience.CodeClaimedOnAccessGate}, codes(t, err))

		for _, field := range []string{"company.custom.plan", "contact.custom.plan"} {
			a, err = audience.Parse(json.RawMessage(leaf(field, "eq", `"pro"`)))
			require.NoError(t, err)

			_, err = audience.Compile(a, audience.Base(), access)
			require.NoError(t, err, field)
		}
	})

	t.Run("fails every condition on a missing value when access is at stake", func(t *testing.T) {
		t.Parallel()

		a, err := audience.Parse(json.RawMessage(leaf("contact.custom.plan", "neq", `"free"`)))
		require.NoError(t, err)
		p, err := audience.Compile(a, audience.Base(), audience.HelpAccess())
		require.NoError(t, err)

		assert.True(t, p.Evaluate(audience.Env{Facts: audience.Values{}}))
		assert.False(t, p.Evaluate(audience.Env{Facts: audience.Values{}, Strict: true}))
	})

	t.Run("reports every bad condition at once", func(t *testing.T) {
		t.Parallel()

		a, err := audience.Parse(json.RawMessage(`{"rules":{"all":[
			{"field":"contact.nope","op":"exists"},
			{"field":"contact.name","op":"gt","value":1}]}}`))
		require.NoError(t, err)

		_, err = audience.Compile(a, audience.Base(), audience.WorkflowAudience())
		assert.Equal(t, []string{audience.CodeUnknownAttribute, audience.CodeOpInvalidForType}, codes(t, err))
	})

	t.Run("refuses a shape problem before compiling", func(t *testing.T) {
		t.Parallel()

		_, err := audience.Compile(audience.Audience{Visitors: "robots"}, audience.Base(), audience.WorkflowAudience())
		assert.Equal(t, []string{audience.CodeSchema}, codes(t, err))
	})

	t.Run("compiles a bare rule", func(t *testing.T) {
		t.Parallel()

		p, err := audience.CompileRule(&audience.Rule{Field: "contact.name", Op: "exists"},
			audience.Base(), audience.WorkflowBranch())
		require.NoError(t, err)
		assert.True(t, p.Evaluate(audience.Env{Facts: audience.Values{"contact.name": "Ada"}}))
	})

	t.Run("lets a catalog extend another", func(t *testing.T) {
		t.Parallel()

		c := audience.Base().With(
			audience.Field{Key: "conversation.priority", Type: audience.TypeList, Where: audience.Conversation},
			audience.Field{Key: "run.vars.", Prefix: true, Type: audience.TypeAuto, Where: audience.Conversation},
		)

		f, ok := c.Lookup("run.vars.order_id")
		require.True(t, ok)
		assert.Equal(t, audience.TypeAuto, f.Type)

		_, ok = audience.Base().Lookup("conversation.priority")
		assert.False(t, ok)

		_, ok = c.Lookup("run.vars.")
		assert.False(t, ok)
	})
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	facts := audience.Values{
		"contact.kind":         "user",
		"contact.name":         "Adéla Novák",
		"contact.email":        "ada@Example.com",
		"contact.custom.plan":  "Pro",
		"contact.custom.seats": 12,
		"contact.custom.beta":  true,
		"contact.custom.score": 4.5,
		"contact.custom.when":  "2026-11-01T10:00:00Z",
		"contact.custom.unix":  float64(now.Add(-48 * time.Hour).Unix()),
		"contact.tags":         []string{"VIP", "beta"},
		"contact.last_seen_at": now.Add(-36 * time.Hour),
		"contact.created_at":   now.AddDate(0, -3, 0),
		"company.name":         "Crème Corp",
		"session.context.cart": int64(3),
		"contact.custom.big":   int32(7),
	}

	cases := []struct {
		name, raw string
		want      bool
	}{
		{"text equality ignores case", leaf("contact.custom.plan", "eq", `"pro"`), true},
		{"text inequality", leaf("contact.custom.plan", "neq", `"team"`), true},
		{"in a list", leaf("contact.custom.plan", "in", `["team","PRO"]`), true},
		{"not in a list", leaf("contact.custom.plan", "not_in", `["team"]`), true},
		{"contains", leaf("contact.email", "contains", `"example"`), true},
		{"does not contain", leaf("contact.email", "not_contains", `"gmail"`), true},
		{"starts with", leaf("contact.email", "starts_with", `"ADA@"`), true},
		{"ends with", leaf("contact.email", "ends_with", `".com"`), true},
		{"keywords ignore accents and match whole words", leaf("company.name", "contains_any", `["creme"]`), true},
		{"keywords refuse part of a word", leaf("company.name", "contains_any", `["cre"]`), false},
		{"no keyword present", leaf("company.name", "not_contains_any", `["refund"]`), true},
		{"number greater", leaf("contact.custom.seats", "gt", "10"), true},
		{"number at least", leaf("contact.custom.seats", "gte", "12"), true},
		{"number less", leaf("contact.custom.seats", "lt", "12"), false},
		{"number at most", leaf("contact.custom.score", "lte", "4.5"), true},
		{"number between", leaf("contact.custom.seats", "between", "[10, 12]"), true},
		{"number equality on an unknown type", leaf("contact.custom.seats", "eq", "12"), true},
		{"number inequality on an unknown type", leaf("contact.custom.seats", "neq", "11"), true},
		{"a text never equals a number", leaf("contact.custom.seats", "eq", `"12"`), false},
		{"a number never equals a text", leaf("contact.custom.plan", "eq", "12"), false},
		{"int64 values compare as numbers", leaf("session.context.cart", "gte", "3"), true},
		{"int32 values compare as numbers", leaf("contact.custom.big", "eq", "7"), true},
		{"boolean true", leaf("contact.custom.beta", "is_true", ""), true},
		{"boolean false", leaf("contact.custom.beta", "is_false", ""), false},
		{"boolean equality", leaf("contact.custom.beta", "eq", "true"), true},
		{"boolean inequality", leaf("contact.custom.beta", "neq", "true"), false},
		{"set has", leaf("contact.tags", "has", `"vip"`), true},
		{"set has any", leaf("contact.tags", "has_any", `["x","beta"]`), true},
		{"set has none", leaf("contact.tags", "has_none", `["x"]`), true},
		{"seen within the last two days", leaf("contact.last_seen_at", "within_last_days", "2"), true},
		{"not seen within the last day", leaf("contact.last_seen_at", "within_last_days", "1"), false},
		{"seen more than a day ago", leaf("contact.last_seen_at", "more_than_days_ago", "1"), true},
		{"seen one day ago", leaf("contact.last_seen_at", "days_ago", "1"), true},
		{"not seen two days ago", leaf("contact.last_seen_at", "days_ago", "2"), false},
		{"created before a day", leaf("contact.created_at", "before", `"2026-10-01"`), true},
		{"created after a day", leaf("contact.created_at", "after", `"2026-08-06"`), true},
		{"not created after its own day", leaf("contact.created_at", "after", `"2026-08-07"`), false},
		{"created on a day", leaf("contact.created_at", "on", `"2026-08-07"`), true},
		{"created before a moment", leaf("contact.created_at", "before", `"2026-08-07T13:00:00Z"`), true},
		{"created after a moment", leaf("contact.created_at", "after", `"2026-08-07T11:00:00Z"`), true},
		{"created on the day of a moment", leaf("contact.created_at", "on", `"2026-08-07T23:00:00Z"`), true},
		{"text timestamps read as dates", leaf("contact.custom.when", "within_last_days", "7"), true},
		{"unix seconds read as dates", leaf("contact.custom.unix", "days_ago", "2"), true},
		{"a future date within the next days", leaf("contact.custom.when", "within_next_days", "3"), false},
		{
			"a past date is not far in the future", leaf("contact.custom.when", "more_than_days_from_now", "1"),
			false,
		},
		{"a text that is no date never matches", leaf("contact.custom.plan", "within_last_days", "3"), false},
		{"exists", leaf("contact.name", "exists", ""), true},
		{"not exists", leaf("contact.name", "not_exists", ""), false},
		{"identified visitors", `{"visitors":"identified"}`, true},
		{"leads only", `{"visitors":"leads"}`, false},
		{"all group", `{"rules":{"all":[` + inner("contact.custom.plan", "eq", `"pro"`) + `,` +
			inner("contact.tags", "has", `"vip"`) + `]}}`, true},
		{"any group", `{"rules":{"any":[` + inner("contact.custom.plan", "eq", `"team"`) + `,` +
			inner("contact.tags", "has", `"vip"`) + `]}}`, true},
		{"not group", `{"rules":{"not":` + inner("contact.custom.plan", "eq", `"pro"`) + `}}`, false},
		{"visitors and rules together", `{"visitors":"identified","rules":` +
			inner("contact.custom.plan", "eq", `"pro"`) + `}`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := audience.Env{Facts: facts, Now: now}
			assert.Equal(t, tc.want, compile(t, tc.raw).Evaluate(env))
		})
	}

	t.Run("dates on a day follow the workspace location", func(t *testing.T) {
		t.Parallel()

		berlin, err := time.LoadLocation("Europe/Berlin")
		require.NoError(t, err)

		late := audience.Values{"contact.created_at": time.Date(2026, 3, 1, 23, 30, 0, 0, time.UTC)}
		p := compile(t, leaf("contact.created_at", "on", `"2026-03-02"`))
		assert.True(t, p.Evaluate(audience.Env{Facts: late, Location: berlin}))
		assert.False(t, p.Evaluate(audience.Env{Facts: late}))
	})
}

/*
inner is one condition as a JSON node.
*/
func inner(field, op, value string) string {
	return `{"field":"` + field + `","op":"` + op + `","value":` + value + `}`
}

func TestMissingValues(t *testing.T) {
	t.Parallel()

	negatives := map[string]string{
		"not_exists": "", "neq": `"pro"`, "not_in": `["pro"]`, "not_contains": `"pro"`, "not_contains_any": `["pro"]`,
	}
	for op, value := range negatives {
		t.Run("a negative "+op+" matches people with no value", func(t *testing.T) {
			t.Parallel()

			assert.True(t, compile(t, leaf("contact.custom.plan", op, value)).Evaluate(audience.Env{}))
		})
	}

	positives := map[string]string{
		"exists": "", "eq": `"pro"`, "in": `["pro"]`, "contains": `"pro"`, "starts_with": `"p"`, "gt": "1",
	}
	for op, value := range positives {
		t.Run("a positive "+op+" refuses people with no value", func(t *testing.T) {
			t.Parallel()

			assert.False(t, compile(t, leaf("contact.custom.plan", op, value)).Evaluate(audience.Env{}))
		})
	}

	t.Run("no tags has none of them", func(t *testing.T) {
		t.Parallel()

		assert.True(t, compile(t, leaf("contact.tags", "has_none", `["vip"]`)).Evaluate(audience.Env{}))
		assert.False(t, compile(t, leaf("contact.tags", "has", `"vip"`)).Evaluate(audience.Env{}))
	})

	t.Run("a nil value counts as missing", func(t *testing.T) {
		t.Parallel()

		var never *time.Time
		facts := audience.Values{"contact.custom.plan": nil, "contact.last_seen_at": never}
		assert.True(t, compile(t, leaf("contact.custom.plan", "not_exists", "")).Evaluate(audience.Env{Facts: facts}))
		assert.False(t, compile(t, leaf("contact.last_seen_at", "exists", "")).Evaluate(audience.Env{Facts: facts}))
	})

	t.Run("a value of an unsupported kind never matches", func(t *testing.T) {
		t.Parallel()

		facts := audience.Values{"contact.custom.plan": map[string]any{"a": 1}}
		assert.False(t, compile(t, leaf("contact.custom.plan", "eq", `"pro"`)).Evaluate(audience.Env{Facts: facts}))
	})
}

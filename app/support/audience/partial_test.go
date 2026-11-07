package audience_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/audience"
)

func TestPartial(t *testing.T) {
	t.Parallel()

	opener := func(t *testing.T, raw string) *audience.Program {
		t.Helper()

		a, err := audience.Parse(json.RawMessage(raw))
		require.NoError(t, err)

		p, err := audience.Compile(a, audience.Base(), audience.WorkflowOpener())
		require.NoError(t, err)

		return p
	}
	onServer := func(f audience.Field) bool { return f.Where == audience.Client }
	rule := `{"rules":{"any":[{"all":[` + inner("contact.custom.plan", "eq", `"pro"`) + `,` +
		inner("contact.kind", "eq", `"user"`) + `]},` + inner("page.path", "starts_with", `"/pricing"`) + `]}}`

	t.Run("a pro user is decided on the server", func(t *testing.T) {
		t.Parallel()

		facts := audience.Values{"contact.custom.plan": "pro", "contact.kind": "user"}
		got, residual := opener(t, rule).Partial(audience.Env{Facts: facts, Unknown: onServer})
		assert.Equal(t, audience.True, got)
		assert.Nil(t, residual)
	})

	t.Run("anyone else gets only the page condition", func(t *testing.T) {
		t.Parallel()

		got, residual := opener(t, rule).Partial(audience.Env{Facts: audience.Values{}, Unknown: onServer})
		assert.Equal(t, audience.Unknown, got)
		require.NotNil(t, residual)
		assert.Equal(t, "page.path", residual.Field)

		raw, err := json.Marshal(residual)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "plan")
	})

	t.Run("a false server condition decides an all group", func(t *testing.T) {
		t.Parallel()

		r := `{"rules":{"all":[` + inner("contact.custom.plan", "eq", `"pro"`) + `,` +
			inner("page.path", "eq", `"/a"`) + `]}}`
		got, residual := opener(t, r).Partial(audience.Env{Facts: audience.Values{}, Unknown: onServer})
		assert.Equal(t, audience.False, got)
		assert.Nil(t, residual)
	})

	t.Run("open conditions stay grouped", func(t *testing.T) {
		t.Parallel()

		r := `{"rules":{"all":[` + inner("page.path", "eq", `"/a"`) + `,` +
			`{"not":` + inner("page.host", "eq", `"x.com"`) + `}]}}`
		got, residual := opener(t, r).Partial(audience.Env{Unknown: onServer})
		assert.Equal(t, audience.Unknown, got)
		require.Len(t, residual.All, 2)
		require.NotNil(t, residual.All[1].Not)

		r = `{"rules":{"any":[` + inner("page.path", "eq", `"/a"`) + `,` + inner("page.path", "eq", `"/b"`) + `]}}`
		_, residual = opener(t, r).Partial(audience.Env{Unknown: onServer})
		assert.Len(t, residual.Any, 2)
	})

	t.Run("evaluating with unknown values does not match", func(t *testing.T) {
		t.Parallel()

		assert.False(t, opener(t, leaf("page.path", "eq", `"/a"`)).Evaluate(audience.Env{Unknown: onServer}))
	})

	t.Run("names results for traces", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "true", audience.True.String())
		assert.Equal(t, "false", audience.False.String())
		assert.Equal(t, "unknown", audience.Unknown.String())
	})
}

func TestExplain(t *testing.T) {
	t.Parallel()

	t.Run("records every condition and the first failing one", func(t *testing.T) {
		t.Parallel()

		p := compile(t, `{"visitors":"identified","rules":{"all":[`+inner("contact.custom.plan", "eq", `"pro"`)+`,`+
			inner("contact.custom.plan", "eq", `"team"`)+`]}}`)
		e := p.Explain(audience.Env{Facts: audience.Values{"contact.kind": "user", "contact.custom.plan": "pro"}})

		assert.Equal(t, audience.False, e.Result)
		require.Len(t, e.Leaves, 3)
		assert.Equal(t, "visitors.all[0]", e.Leaves[0].Path)

		failing, ok := e.FirstFailing()
		require.True(t, ok)
		assert.Equal(t, "rules.all[1]", failing.Path)
	})

	t.Run("an empty audience explains as a match", func(t *testing.T) {
		t.Parallel()

		e := compile(t, `{}`).Explain(audience.Env{})
		assert.Equal(t, audience.True, e.Result)

		_, ok := e.FirstFailing()
		assert.False(t, ok)

		var none *audience.Program
		assert.Equal(t, audience.True, none.Explain(audience.Env{}).Result)
	})
}

func TestSavedAudiences(t *testing.T) {
	t.Parallel()

	pro := compile(t, leaf("contact.custom.plan", "eq", `"pro"`))
	saved := func(id string) (*audience.Program, bool) {
		if id == "aud_PRO" {
			return pro, true
		}

		return nil, false
	}
	facts := audience.Values{"contact.custom.plan": "pro"}

	t.Run("a member of a saved audience is in it", func(t *testing.T) {
		t.Parallel()

		p := compile(t, leaf("audience", "in", `["aud_missing","aud_PRO"]`))
		assert.True(t, p.Evaluate(audience.Env{Facts: facts, Saved: saved}))
		assert.False(t, p.Evaluate(audience.Env{Facts: audience.Values{}, Saved: saved}))
	})

	t.Run("not in a saved audience", func(t *testing.T) {
		t.Parallel()

		p := compile(t, leaf("audience", "not_in", `["aud_PRO"]`))
		assert.False(t, p.Evaluate(audience.Env{Facts: facts, Saved: saved}))
		assert.True(t, p.Evaluate(audience.Env{Facts: facts}))
	})

	t.Run("a saved audience that references itself stops", func(t *testing.T) {
		t.Parallel()

		var self *audience.Program
		loop := func(string) (*audience.Program, bool) { return self, true }
		self = compile(t, leaf("audience", "in", `["aud_SELF"]`))
		assert.False(t, self.Evaluate(audience.Env{Saved: loop}))
	})

	t.Run("an unknown saved audience stays open", func(t *testing.T) {
		t.Parallel()

		a, err := audience.Parse(json.RawMessage(leaf("page.path", "eq", `"/a"`)))
		require.NoError(t, err)
		page, err := audience.Compile(a, audience.Base(), audience.WorkflowOpener())
		require.NoError(t, err)

		p := compile(t, leaf("audience", "in", `["aud_PAGE"]`))
		got, _ := p.Partial(audience.Env{
			Saved:   func(string) (*audience.Program, bool) { return page, true },
			Unknown: func(f audience.Field) bool { return f.Where == audience.Client },
		})
		assert.Equal(t, audience.Unknown, got)
	})
}

func TestProblemsError(t *testing.T) {
	t.Parallel()

	t.Run("is an error naming each problem", func(t *testing.T) {
		t.Parallel()

		var err error = &audience.ProblemsError{Problems: []audience.Problem{{Code: "schema", Path: "rules"}}}
		var pe *audience.ProblemsError
		assert.True(t, errors.As(err, &pe))
		assert.Equal(t, "audience rule invalid: schema at rules", err.Error())
	})
}

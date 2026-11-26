package audience_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
companies are the facts of a person who belongs to several companies,
each with its plan.
*/
type companies []any

/*
plan is the company field the cases read.
*/
const plan = "company.custom.plan"

func (c companies) Lookup(key string) (any, bool) {
	if key != plan || len(c) == 0 || c[0] == nil {
		return nil, false
	}

	return c[0], true
}

func (c companies) LookupEach(key string) ([]any, bool) {
	return c, key == plan && len(c) > 1
}

func TestSeveralValues(t *testing.T) {
	t.Parallel()

	pro, notPro := leaf(plan, "eq", `"pro"`), leaf(plan, "neq", `"pro"`)

	for name, tc := range map[string]struct {
		rule  string
		facts companies
		want  bool
	}{
		"any company meets a positive condition":   {pro, companies{"free", "pro"}, true},
		"no company meets it":                      {pro, companies{"free", "team"}, false},
		"a negative condition needs every company": {notPro, companies{"free", "pro"}, false},
		"every company meets the negative one":     {notPro, companies{"free", "team"}, true},
		"a company without the value counts":       {leaf(plan, "exists", ""), companies{nil, "pro"}, true},
		"not exists needs every company bare":      {leaf(plan, "not_exists", ""), companies{nil, "pro"}, false},
		"one company reads as before":              {pro, companies{"pro"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, compile(t, tc.rule).Evaluate(audience.Env{Facts: tc.facts}), name)
		})
	}
}

package workflowspec_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

func TestCheckSplit(t *testing.T) {
	t.Parallel()

	const fm = workflowspec.TriggerFirstMessage

	split := func(settings, paths string) string {
		return def(fm, `{"id":"s_split1","kind":"split","settings":`+settings+`,"paths":[`+paths+`]}`)
	}
	two := `{"id":"p_varnta","steps":[]},{"id":"p_varntb","steps":[]}`
	variants := func(a, b int) string {
		return `{"variants":[{"id":"p_varnta","label":"Short","weight":` + strconv.Itoa(a) +
			`},{"id":"p_varntb","label":"Long","weight":` + strconv.Itoa(b) + `}]`
	}

	accepted := []struct{ name, raw string }{
		{"an even split", split(variants(50, 50)+`}`, two)},
		{"an uneven split", split(variants(90, 10)+`}`, two)},
		{"a winner", split(variants(50, 50)+`,"winner":"p_varntb"}`, two)},
	}

	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, errorCodes(t, tc.raw))
		})
	}

	refused := []struct{ name, raw, code string }{
		{"shares that miss 100", split(variants(50, 40)+`}`, two), workflowspec.CodeInvalidSetting},
		{"a share of 0", split(variants(100, 0)+`}`, two), workflowspec.CodeInvalidSetting},
		{
			"a winner that is no variant", split(variants(50, 50)+`,"winner":"p_other1"}`, two),
			workflowspec.CodeInvalidSetting,
		},
		{"one path", split(`{"variants":[{"id":"p_varnta","label":"A","weight":100}]}`,
			`{"id":"p_varnta","steps":[]}`), workflowspec.CodeLimitExceeded},
		{"a variant with no path", split(`{"variants":[{"id":"p_varnta","label":"A","weight":50},
			{"id":"p_varntc","label":"C","weight":50}]}`, two), workflowspec.CodeInvalidSetting},
		{
			"a path with no variant", split(`{"variants":[{"id":"p_varnta","label":"A","weight":100}]}`, two),
			workflowspec.CodeMissingSetting,
		},
		{"a variant with no label", split(`{"variants":[{"id":"p_varnta","label":" ","weight":50},
			{"id":"p_varntb","label":"B","weight":50}]}`, two), workflowspec.CodeInvalidSetting},
		{"an unknown setting", split(`{"variants":[],"shares":1}`, two), workflowspec.CodeInvalidSetting},
	}

	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, errorCodes(t, tc.raw), tc.code)
		})
	}
}

func TestSplitPick(t *testing.T) {
	t.Parallel()

	s := workflowspec.SplitSettings{Variants: []workflowspec.SplitVariant{
		{ID: "p_varnta", Label: "A", Weight: 50}, {ID: "p_varntb", Label: "B", Weight: 50},
	}}

	t.Run("gives a conversation the same variant every time", func(t *testing.T) {
		t.Parallel()

		first, winner := s.Pick("conversation-1", "s_split1")
		assert.False(t, winner)
		assert.NotEmpty(t, first.ID)

		for range 5 {
			again, _ := s.Pick("conversation-1", "s_split1")
			assert.Equal(t, first, again)
		}
	})

	t.Run("spreads conversations over the variants by share", func(t *testing.T) {
		t.Parallel()

		got := map[string]int{}
		for i := range 1000 {
			v, _ := s.Pick("conversation-"+strconv.Itoa(i), "s_split1")
			got[v.ID]++
		}

		assert.InDelta(t, 500, got["p_varnta"], 80)
		assert.InDelta(t, 500, got["p_varntb"], 80)
	})

	t.Run("sends everyone down the winner", func(t *testing.T) {
		t.Parallel()

		won := s
		won.Winner = "p_varntb"

		for i := range 20 {
			v, winner := won.Pick("conversation-"+strconv.Itoa(i), "s_split1")
			assert.True(t, winner)
			assert.Equal(t, "p_varntb", v.ID)
		}
	})
}

package pipeline

import (
	"testing"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

func TestUnpricedFlagsOnlyUsageTheTableCannotPrice(t *testing.T) {
	rec := &Recorder{prices: &aiprices.Table{Models: []aiprices.ModelPrice{
		{Provider: "openai", Match: "exact", Model: "known", InputPerMtok: "1.00", OutputPerMtok: "2.00"},
	}}}
	used := contracts.LLMUsage{InputTokens: 100, OutputTokens: 10}

	cases := []struct {
		name            string
		rec             *Recorder
		provider, model string
		usage           contracts.LLMUsage
		want            bool
	}{
		{"a listed model", rec, "openai", "known", used, false},
		{"an unlisted model", rec, "openai", "other", used, true},
		{"a listed model on another provider", rec, "baseten", "known", used, true},
		{"no tokens", rec, "openai", "other", contracts.LLMUsage{}, false},
		{"no model", rec, "openai", "", used, false},
		{"no table", &Recorder{}, "openai", "other", used, false},
	}
	for _, c := range cases {
		if got := c.rec.unpriced(c.provider, c.model, c.usage); got != c.want {
			t.Errorf("%s: unpriced = %v, want %v", c.name, got, c.want)
		}
	}

	if cost := rec.price("openai", "other", used); cost != 0 {
		t.Errorf("an unlisted model cost %d", cost)
	}
}

func TestFirstUnpricedReportsEachModelOnce(t *testing.T) {
	if !firstUnpriced("test-provider", "test-model") {
		t.Fatal("the first sighting was not reported")
	}
	if firstUnpriced("test-provider", "test-model") {
		t.Fatal("the second sighting was reported again")
	}
	if !firstUnpriced("test-provider", "another-model") {
		t.Fatal("another model was not reported")
	}
}

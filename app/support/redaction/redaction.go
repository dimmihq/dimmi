/*
|--------------------------------------------------------------------------
| Redaction
|--------------------------------------------------------------------------
|
| What a customer or teammate should never have stored is removed before
| the text is written, and nothing keeps the original. The built-in rules
| run first, then the workspace's own patterns in their order. Patterns
| are Go's RE2 expressions, which run in time linear to the text, so a
| pattern can never stall a write. A hit is counted by rule and never
| carries the text it replaced.
|
*/

package redaction

import (
	"errors"
	"fmt"
	"regexp"
)

/*
Built-in rule names and the limits on a workspace's own rules: the
longest pattern and the replacement written when a rule sets none.
*/
const (
	RuleCardNumbers    = "card_numbers"
	RuleUSSSN          = "us_ssn"
	MaxPatternLength   = 256
	DefaultReplacement = "[redacted]"
)

/*
ErrInvalidPattern is returned for a pattern that is empty, too long or
not a valid expression.
*/
var ErrInvalidPattern = errors.New("redaction: invalid pattern")

/*
Settings turns the built-in rules on or off.
*/
type Settings struct {
	CardNumbers bool `json:"card_numbers"`
	USSSN       bool `json:"us_ssn"`
}

/*
DefaultSettings are the built-ins a workspace starts with: card numbers
on, social security numbers off.
*/
func DefaultSettings() Settings {
	return Settings{CardNumbers: true}
}

/*
Rule is one of the workspace's own patterns. Key names it in the hits,
and Replacement is what each match becomes.
*/
type Rule struct {
	Key         string
	Pattern     string
	Replacement string
}

/*
Hit is how many matches one rule replaced.
*/
type Hit struct {
	Rule  string `json:"rule"`
	Count int    `json:"count"`
}

/*
Redactor applies a workspace's rules. The zero Redactor changes nothing.
*/
type Redactor struct {
	settings Settings
	rules    []compiled
}

type compiled struct {
	key         string
	re          *regexp.Regexp
	replacement string
}

/*
Compile checks a pattern and returns it compiled.
*/
func Compile(pattern string) (*regexp.Regexp, error) {
	if pattern == "" || len(pattern) > MaxPatternLength {
		return nil, ErrInvalidPattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPattern, err)
	}

	return re, nil
}

/*
New returns a redactor for the settings and the rules, in their order.
*/
func New(settings Settings, rules []Rule) (Redactor, error) {
	r := Redactor{settings: settings}

	for _, rule := range rules {
		re, err := Compile(rule.Pattern)
		if err != nil {
			return Redactor{}, err
		}

		replacement := rule.Replacement
		if replacement == "" {
			replacement = DefaultReplacement
		}

		r.rules = append(r.rules, compiled{key: rule.Key, re: re, replacement: replacement})
	}

	return r, nil
}

/*
Empty reports whether no rule is on, so callers can skip the work.
*/
func (r Redactor) Empty() bool {
	return !r.settings.CardNumbers && !r.settings.USSSN && len(r.rules) == 0
}

/*
Apply returns the text with every rule applied and the hits, in rule
order.
*/
func (r Redactor) Apply(text string) (string, []Hit) {
	var hits []Hit

	if r.settings.CardNumbers {
		text, hits = matcher{RuleCardNumbers, cardPattern, maskCard}.replace(text, hits)
	}

	if r.settings.USSSN {
		text, hits = matcher{RuleUSSSN, ssnPattern, maskSSN}.replace(text, hits)
	}

	for _, rule := range r.rules {
		text, hits = matcher{rule.key, rule.re, func(string) (string, bool) {
			return rule.replacement, true
		}}.replace(text, hits)
	}

	return text, hits
}

/*
ApplyAll applies the rules to each text in place, a nil one skipped, and
returns the hits of all of them added up by rule.
*/
func (r Redactor) ApplyAll(texts ...*string) []Hit {
	if r.Empty() {
		return nil
	}

	var total []Hit

	for _, text := range texts {
		if text == nil {
			continue
		}

		var hits []Hit

		*text, hits = r.Apply(*text)
		total = Add(total, hits...)
	}

	return total
}

/*
Add adds the hits to the totals, one entry per rule, in first-seen order.
*/
func Add(totals []Hit, hits ...Hit) []Hit {
	for _, hit := range hits {
		found := false

		for i := range totals {
			if totals[i].Rule == hit.Rule {
				totals[i].Count += hit.Count
				found = true
			}
		}

		if !found {
			totals = append(totals, hit)
		}
	}

	return totals
}

/*
Count returns how many matches the hits replaced in all.
*/
func Count(hits []Hit) int {
	total := 0
	for _, hit := range hits {
		total += hit.Count
	}

	return total
}

/*
matcher is one rule as it runs: its name, what it finds, and what a
match becomes, with whether the match counts at all.
*/
type matcher struct {
	rule string
	re   *regexp.Regexp
	mask func(string) (string, bool)
}

/*
replace swaps each match that the mask accepts and counts it under the
rule.
*/
func (m matcher) replace(text string, hits []Hit) (string, []Hit) {
	count := 0
	out := m.re.ReplaceAllStringFunc(text, func(match string) string {
		masked, ok := m.mask(match)
		if ok {
			count++
		}

		return masked
	})

	if count == 0 {
		return text, hits
	}

	return out, append(hits, Hit{Rule: m.rule, Count: count})
}

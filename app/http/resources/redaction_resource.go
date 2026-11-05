/*
|--------------------------------------------------------------------------
| Redaction Resources
|--------------------------------------------------------------------------
|
| A workspace's redaction as the API shows it: the built-in switches and
| its own rules in the order they apply. A tried text comes back as it
| would be stored, with the rules that changed it, and nothing else.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/redaction"
	"github.com/dimmihq/dimmi/app/models"
	supportredaction "github.com/dimmihq/dimmi/app/support/redaction"
)

/*
RedactionRule is one of the workspace's own rules.
*/
type RedactionRule struct {
	ID          string    `json:"id"`
	Object      string    `json:"object"`
	Name        string    `json:"name"`
	Pattern     string    `json:"pattern"`
	Replacement string    `json:"replacement"`
	Enabled     bool      `json:"enabled"`
	Position    int32     `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

/*
NewRedactionRule shapes the rule.
*/
func NewRedactionRule(r models.RedactionRule) RedactionRule {
	return RedactionRule{
		ID: models.PublicID(models.PrefixRedactionRule, r.ID), Object: "redaction_rule", Name: r.Name,
		Pattern: r.Pattern, Replacement: r.Replacement, Enabled: r.Enabled, Position: r.Position,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

/*
RedactionRules is the workspace's redaction: BuiltIns the switches of
the built-in rules and Rules its own rules in the order they apply.
*/
type RedactionRules struct {
	Object   string                    `json:"object"`
	BuiltIns supportredaction.Settings `json:"built_ins"`
	Rules    []RedactionRule           `json:"rules"`
}

/*
NewRedactionRules shapes the workspace's redaction.
*/
func NewRedactionRules(r redaction.Rules) RedactionRules {
	out := RedactionRules{Object: "redaction", BuiltIns: r.Settings, Rules: make([]RedactionRule, 0, len(r.Rules))}
	for _, rule := range r.Rules {
		out.Rules = append(out.Rules, NewRedactionRule(rule))
	}

	return out
}

/*
RedactionTest is a text as the rules would store it. Redactions names
each rule that changed it and how often.
*/
type RedactionTest struct {
	Object     string                 `json:"object"`
	Text       string                 `json:"text"`
	Redactions []supportredaction.Hit `json:"redactions"`
}

/*
NewRedactionTest shapes a tried text.
*/
func NewRedactionTest(t redaction.Tried) RedactionTest {
	hits := t.Hits
	if hits == nil {
		hits = []supportredaction.Hit{}
	}

	return RedactionTest{Object: "redaction_test", Text: t.Text, Redactions: hits}
}

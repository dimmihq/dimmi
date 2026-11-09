/*
|--------------------------------------------------------------------------
| Contact List Conditions
|--------------------------------------------------------------------------
|
| The dashboard's filters narrow the contact list with conditions, sent as
| a JSON list in the conditions parameter: a field, a comparison and a
| value. Each is checked here, so the database only ever sees fields and
| comparisons it knows: tags and companies are named by their public ids
| and passed on as ids, and "N days ago" becomes an instant from the
| application's clock. Saved audiences are taken apart, since their rules
| are judged outside the database. A condition that does not fit is
| refused, never ignored.
|
*/

package v1

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of a list's conditions.
*/
const (
	maxConditions     = 10
	maxConditionValue = 255
	maxConditionItems = 20
)

/*
ErrInvalidConditions refuses conditions that are not a list of known
fields, comparisons and values.
*/
var ErrInvalidConditions = fault.New(fault.Invalid, "invalid_parameter",
	"The conditions must be a JSON list of fields, comparisons and values this list knows.").
	With("parameter", "conditions")

/*
condition is one condition, as the client sends it and as the database
reads it. Type is a custom attribute's type; Days counts back from now
for the time fields.
*/
type condition struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Value  string   `json:"value,omitempty"`
	Values []string `json:"values,omitempty"`
	Type   string   `json:"type,omitempty"`
	Days   int      `json:"days,omitempty"`
}

func textOps() []string {
	return []string{"is", "is_not", "contains", "not_contains", "starts_with", "ends_with", "unknown", "known"}
}

func presence() []string {
	return []string{"unknown", "known"}
}

/*
conditionsOf reads the conditions parameter: the conditions the
database checks, nil when there are none, and the audiences the people
must be in or out of, judged apart since a rule can read anything.
*/
func conditionsOf(raw string, registry *ids.Registry, clk clock.Clock,
) (json.RawMessage, *contacts.AudienceFilter, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil, nil
	}

	var in []condition
	if json.Unmarshal([]byte(raw), &in) != nil || len(in) > maxConditions {
		return nil, nil, ErrInvalidConditions
	}

	now := clk.Now()
	out := make([]condition, 0, len(in))
	var members *contacts.AudienceFilter
	for _, c := range in {
		if c.Field == "audience" {
			if members != nil {
				return nil, nil, ErrInvalidConditions
			}

			var err error
			if members, err = audienceFilter(c, registry); err != nil {
				return nil, nil, err
			}

			continue
		}

		checked, err := checkCondition(c, registry, now)
		if err != nil {
			return nil, nil, err
		}

		out = append(out, checked)
	}

	return encodeConditions(out), members, nil
}

func encodeConditions(out []condition) json.RawMessage {
	if len(out) == 0 {
		return nil
	}

	encoded, _ := json.Marshal(out)

	return encoded
}

/*
audienceFilter reads a condition on saved audiences: in any of them, or
in none.
*/
func audienceFilter(c condition, registry *ids.Registry) (*contacts.AudienceFilter, error) {
	if (c.Op != "in_any" && c.Op != "in_none") || len(c.Values) == 0 || len(c.Values) > maxConditionItems {
		return nil, ErrInvalidConditions
	}

	out := &contacts.AudienceFilter{None: c.Op == "in_none"}
	for _, value := range c.Values {
		id, err := registry.Decode(string(models.PrefixAudience), value)
		if err != nil {
			return nil, ErrInvalidConditions
		}

		out.IDs = append(out.IDs, id)
	}

	return out, nil
}

/*
checkCondition checks one condition and writes it as the database
reads it.
*/
func checkCondition(c condition, registry *ids.Registry, now time.Time) (condition, error) {
	if len(c.Value) > maxConditionValue || len(c.Values) > maxConditionItems {
		return c, ErrInvalidConditions
	}

	switch {
	case c.Field == "tags":
		return referenced(c, registry, models.PrefixTag, []string{"has_any", "has_all", "has_none"})
	case c.Field == "company":
		return referenced(c, registry, models.PrefixCompany, []string{"is", "is_not", "unknown", "known"})
	case c.Field == "last_seen" || c.Field == "created":
		return dated(c, now)
	case c.Field == "kind":
		return kind(c)
	case slices.Contains([]string{"name", "email", "phone", "user_id"}, c.Field):
		return text(c)
	case strings.HasPrefix(c.Field, "custom.") && len(c.Field) > len("custom."):
		return custom(c)
	default:
		return c, ErrInvalidConditions
	}
}

func allowed(c condition, ops []string) (condition, error) {
	if !slices.Contains(ops, c.Op) {
		return c, ErrInvalidConditions
	}

	return condition{Field: c.Field, Op: c.Op, Value: c.Value, Values: c.Values, Type: c.Type}, nil
}

func needsValue(op string) bool {
	return !slices.Contains(presence(), op)
}

func kind(c condition) (condition, error) {
	if c.Value != "user" && c.Value != "lead" {
		return c, ErrInvalidConditions
	}

	return allowed(c, []string{"is", "is_not"})
}

func text(c condition) (condition, error) {
	if needsValue(c.Op) && c.Value == "" {
		return c, ErrInvalidConditions
	}

	return allowed(c, textOps())
}

/*
custom checks a custom attribute's condition by the attribute's type:
numbers and booleans compare as such, the rest as text.
*/
func custom(c condition) (condition, error) {
	switch c.Type {
	case "number":
		if _, err := strconv.ParseFloat(c.Value, 64); needsValue(c.Op) && err != nil {
			return c, ErrInvalidConditions
		}

		return allowed(c, []string{"eq", "neq", "gt", "lt", "unknown", "known"})
	case "boolean":
		return allowed(c, []string{"true", "false", "unknown", "known"})
	case "string", "list", "":
		c.Type = "string"
		return text(c)
	default:
		return c, ErrInvalidConditions
	}
}

/*
referenced checks a condition naming tags or a company by public id and
passes them on as ids.
*/
func referenced(c condition, registry *ids.Registry, prefix models.Prefix, ops []string) (condition, error) {
	if needsValue(c.Op) && len(c.Values) == 0 {
		return c, ErrInvalidConditions
	}

	decoded := make([]string, 0, len(c.Values))
	for _, value := range c.Values {
		id, err := registry.Decode(string(prefix), value)
		if err != nil {
			return c, ErrInvalidConditions
		}

		decoded = append(decoded, id.String())
	}

	c.Values = decoded

	return allowed(c, ops)
}

/*
dated turns "more" or "less than N days ago" into before or after an
instant; unknown and known pass as they are.
*/
func dated(c condition, now time.Time) (condition, error) {
	if !needsValue(c.Op) {
		return allowed(c, presence())
	}

	if c.Days < 1 || c.Days > 36_500 {
		return c, ErrInvalidConditions
	}

	at := now.Add(-time.Duration(c.Days) * 24 * time.Hour).UTC().Format(time.RFC3339)
	switch c.Op {
	case "more_than":
		return condition{Field: c.Field, Op: "before", Value: at}, nil
	case "less_than":
		return condition{Field: c.Field, Op: "after", Value: at}, nil
	default:
		return c, ErrInvalidConditions
	}
}

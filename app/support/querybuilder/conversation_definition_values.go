/*
|--------------------------------------------------------------------------
| View Definition Values
|--------------------------------------------------------------------------
|
| The fields of a saved view that compare values rather than name rows:
| priority by level, yes or no flags, numbers, moments absolute or
| relative to now, words in the subject, and the custom attributes of the
| customer and their company.
|
*/

package querybuilder

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/*
priorityLevel returns the stored position of a level, lowest first.
*/
func priorityLevel(name string) (int16, bool) {
	level, ok := map[string]int16{"none": 0, "low": 1, "medium": 2, "high": 3, "urgent": 4}[name]

	return level, ok
}

/*
priorityList reads a list of levels.
*/
func priorityList(raw json.RawMessage) ([]int16, error) {
	names, err := stringList(raw)
	if err != nil {
		return nil, err
	}

	levels := make([]int16, 0, len(names))

	for _, name := range names {
		level, ok := priorityLevel(name)
		if !ok {
			return nil, errors.New("unknown priority " + name)
		}

		levels = append(levels, level)
	}

	return levels, nil
}

func priorityField() field {
	return field{compile: func(_ Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op == "in" {
			levels, err := priorityList(cond.Value)

			return compiled{sql: "c.priority = ANY (?::smallint[])", args: []any{levels}}, nil, err
		}

		comparison, ok := map[string]string{"eq": " = ", "gte": " >= ", "lte": " <= "}[cond.Op]
		if !ok {
			return compiled{}, nil, errUnsupported
		}

		var name string
		if err := json.Unmarshal(cond.Value, &name); err != nil {
			return compiled{}, nil, errors.New("a priority level is required")
		}

		level, ok := priorityLevel(name)
		if !ok {
			return compiled{}, nil, errors.New("unknown priority " + name)
		}

		return compiled{sql: "c.priority" + comparison + "?", args: []any{level}}, nil, nil
	}}
}

/*
boolField tests a yes or no condition, written as the predicate that is
true for yes.
*/
func boolField(yes func(c Compiler) compiled) field {
	return field{compile: func(c Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op != "eq" {
			return compiled{}, nil, errUnsupported
		}

		var want bool
		if err := json.Unmarshal(cond.Value, &want); err != nil {
			return compiled{}, nil, errors.New("true or false is required")
		}

		p := yes(c)
		if !want {
			p.sql = "NOT (" + p.sql + ")"
		}

		return p, nil, nil
	}}
}

/*
unreadSQL is true while the conversation holds something someone other
than the caller wrote after the caller's read marker.
*/
func unreadSQL(c Compiler) compiled {
	return compiled{
		sql: "EXISTS (SELECT 1 FROM messages m WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id" +
			" AND m.type <> 'event' AND m.deleted_at IS NULL AND m.undone_at IS NULL" +
			" AND NOT (m.author_type = 'member' AND m.author_id IS NOT DISTINCT FROM ?::uuid)" +
			" AND m.id > coalesce((SELECT r.last_read_message_id FROM conversation_reads r" +
			" WHERE r.workspace_id = c.workspace_id AND r.conversation_id = c.id" +
			" AND r.workspace_member_id = ?::uuid)," +
			" '00000000-0000-0000-0000-000000000000'::uuid))",
		args: []any{c.MemberID, c.MemberID},
	}
}

func numberField(column string) field {
	return field{compile: func(_ Compiler, cond Condition) (compiled, []Reference, error) {
		comparison, ok := map[string]string{"eq": " = ", "gt": " > ", "lt": " < "}[cond.Op]
		if !ok {
			return compiled{}, nil, errUnsupported
		}

		var n int64
		if err := json.Unmarshal(cond.Value, &n); err != nil {
			return compiled{}, nil, errors.New("a whole number is required")
		}

		return compiled{sql: column + comparison + "?", args: []any{n}}, nil, nil
	}}
}

/*
timeField tests a moment: before or after a time, or older or newer
than a duration ago. A conversation without the moment matches none.
*/
func timeField(column string) field {
	return field{compile: func(c Compiler, cond Condition) (compiled, []Reference, error) {
		var raw string
		if err := json.Unmarshal(cond.Value, &raw); err != nil {
			return compiled{}, nil, errors.New("a time or duration is required")
		}

		switch cond.Op {
		case "before", "after":
			at, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return compiled{}, nil, errors.New("an ISO 8601 time is required")
			}

			sql := column + map[string]string{"before": " < ", "after": " > "}[cond.Op] + "?"

			return compiled{sql: sql, args: []any{at}}, nil, nil
		case "older_than", "newer_than":
			d, err := parseDuration(raw)
			if err != nil {
				return compiled{}, nil, err
			}

			sql := column + map[string]string{"older_than": " < ", "newer_than": " > "}[cond.Op] + "?"

			return compiled{sql: sql, args: []any{c.Now.Add(-d)}}, nil, nil
		}

		return compiled{}, nil, errUnsupported
	}}
}

/*
durationPattern reads the ISO 8601 durations a view uses: weeks, days,
hours, minutes and seconds.
*/
var durationPattern = regexp.MustCompile(`^P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

/*
parseDuration reads an ISO 8601 duration such as PT2H or P3D.
*/
func parseDuration(raw string) (time.Duration, error) {
	parts := durationPattern.FindStringSubmatch(raw)
	if parts == nil || raw == "P" || strings.HasSuffix(raw, "T") {
		return 0, errors.New("an ISO 8601 duration such as PT2H is required")
	}

	units := []time.Duration{7 * 24 * time.Hour, 24 * time.Hour, time.Hour, time.Minute, time.Second}

	var total time.Duration

	for i, unit := range units {
		if parts[i+1] == "" {
			continue
		}

		n, err := strconv.ParseInt(parts[i+1], 10, 32)
		if err != nil {
			return 0, errors.New("duration is too long")
		}

		total += time.Duration(n) * unit
	}

	return total, nil
}

func subjectField() field {
	return field{compile: func(_ Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op != "contains" {
			return compiled{}, nil, errUnsupported
		}

		var words string
		if err := json.Unmarshal(cond.Value, &words); err != nil || strings.TrimSpace(words) == "" {
			return compiled{}, nil, errors.New("text is required")
		}

		escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(words)

		return compiled{sql: "c.subject ILIKE ?", args: []any{"%" + escaped + "%"}}, nil, nil
	}}
}

/*
The tables an attribute condition reaches the customer's attributes
through.
*/
const (
	contactAttribute = "EXISTS (SELECT 1 FROM contacts ct WHERE ct.workspace_id = c.workspace_id" +
		" AND ct.id = c.contact_id AND "
	companyAttribute = "EXISTS (SELECT 1 FROM contacts ct JOIN companies co ON co.id = ct.company_id" +
		" AND co.workspace_id = ct.workspace_id WHERE ct.workspace_id = c.workspace_id AND ct.id = c.contact_id AND "
)

/*
attributeKey is what an attribute key may look like.
*/
var attributeKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func validKey(key string) bool {
	return attributeKey.MatchString(key)
}

/*
attributeField tests one custom attribute: equal to a value, through
containment so the attributes' index serves it, or present at all.
*/
func attributeField(column, through, key string) field {
	return field{compile: func(_ Compiler, cond Condition) (compiled, []Reference, error) {
		switch cond.Op {
		case "eq":
			var value any
			if err := json.Unmarshal(cond.Value, &value); err != nil || value == nil {
				return compiled{}, nil, errors.New("a value is required")
			}

			doc, err := json.Marshal(map[string]any{key: value})
			if err != nil {
				return compiled{}, nil, errors.New("value cannot be stored")
			}

			return compiled{sql: through + column + " @> ?::jsonb)", args: []any{string(doc)}}, nil, nil
		case "exists":
			return compiled{sql: through + "jsonb_exists(" + column + ", ?))", args: []any{key}}, nil, nil
		}

		return compiled{}, nil, errUnsupported
	}}
}

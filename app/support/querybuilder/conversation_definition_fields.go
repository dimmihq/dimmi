/*
|--------------------------------------------------------------------------
| View Definition Fields
|--------------------------------------------------------------------------
|
| The fields a saved view can test and the operators each accepts. Each
| field writes its own predicate with ? for every value, so a view never
| puts what someone typed into SQL text. A field naming rows reports
| them, so a view whose tag or team was deleted can say which condition
| no longer matches anything; until then the condition simply matches
| nothing.
|
*/

package querybuilder

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
field compiles the conditions on one field.
*/
type field struct {
	compile func(c Compiler, cond Condition) (compiled, []Reference, error)
}

/*
fieldFor returns the field of the name, the attribute fields of contacts
and companies included.
*/
func fieldFor(name string) (field, bool) {
	if key, ok := strings.CutPrefix(name, "contact.attr."); ok && validKey(key) {
		return attributeField("ct.custom_attributes", contactAttribute, key), true
	}

	if key, ok := strings.CutPrefix(name, "company.attr."); ok && validKey(key) {
		return attributeField("co.custom_attributes", companyAttribute, key), true
	}

	f, ok := fields()[name]

	return f, ok
}

/*
fields returns the registry of plain fields.
*/
func fields() map[string]field {
	return map[string]field{
		"status":             choiceField("c.status", []string{"open", "snoozed", "closed"}),
		"channel":            channelField(),
		"channel_connection": refField("c.channel_connection_id", "chn"),
		"inbox":              refField("c.inbox_id", "inbox"),
		"contact":            refField("c.contact_id", "ctc"),
		"assignee":           optionalRefField("c.assignee_member_id", "mem"),
		"team":               optionalRefField("c.assignee_team_id", "team"),
		"tag":                tagField(),
		"company":            companyField(),
		"priority":           priorityField(),
		"unread":             boolField(unreadSQL),
		"waiting": boolField(func(Compiler) compiled {
			return compiled{sql: "c.waiting_since IS NOT NULL"}
		}),
		"imported": boolField(func(Compiler) compiled {
			return compiled{sql: "coalesce((c.source->>'imported')::boolean, false)"}
		}),
		"number":          numberField("c.number"),
		"message_count":   numberField("c.message_count"),
		"created_at":      timeField("c.created_at"),
		"last_message_at": timeField("c.last_message_at"),
		"waiting_since":   timeField("c.waiting_since"),
		"snoozed_until":   timeField("c.snoozed_until"),
		"closed_at":       timeField("c.closed_at"),
		"subject":         subjectField(),
		"text":            {},
	}
}

/*
negate wraps a predicate in NOT when the operator is a negative one.
*/
func negate(op string, p compiled) compiled {
	if op == "not_in" || op == "not_exists" {
		p.sql = "NOT (" + p.sql + ")"
	}

	return p
}

/*
stringList reads a non-empty list of text values.
*/
func stringList(raw json.RawMessage) ([]string, error) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || len(values) == 0 {
		return nil, errors.New("a list of values is required")
	}

	return values, nil
}

func choiceField(column string, allowed []string) field {
	return field{compile: func(_ Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op != "in" && cond.Op != "not_in" {
			return compiled{}, nil, errUnsupported
		}

		values, err := stringList(cond.Value)
		if err != nil {
			return compiled{}, nil, err
		}

		for _, v := range values {
			if !slices.Contains(allowed, v) {
				return compiled{}, nil, errors.New("unknown value " + v)
			}
		}

		return negate(cond.Op, compiled{sql: column + " = ANY (?::text[])", args: []any{values}}), nil, nil
	}}
}

func channelField() field {
	return field{compile: func(_ Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op != "in" && cond.Op != "not_in" {
			return compiled{}, nil, errUnsupported
		}

		values, err := stringList(cond.Value)
		if err != nil {
			return compiled{}, nil, err
		}

		return negate(cond.Op, compiled{
			sql: "EXISTS (SELECT 1 FROM channel_connections cc WHERE cc.workspace_id = c.workspace_id" +
				" AND cc.id = c.channel_connection_id AND cc.type = ANY (?::text[]))",
			args: []any{values},
		}), nil, nil
	}}
}

/*
decodeRefs reads public ids of the prefix, me standing for the caller
among members, and reports the rows named.
*/
func decodeRefs(c Compiler, raw json.RawMessage, prefix string) ([]ids.UUID, []Reference, error) {
	values, err := stringList(raw)
	if err != nil {
		return nil, nil, err
	}

	out := make([]ids.UUID, 0, len(values))

	var refs []Reference

	for _, v := range values {
		if prefix == "mem" && v == "me" {
			out = append(out, c.MemberID)

			continue
		}

		id, err := c.Decode.Decode(prefix, v)
		if err != nil {
			return nil, nil, errors.New("unknown id " + v)
		}

		out = append(out, id)
		refs = append(refs, Reference{Prefix: prefix, ID: id})
	}

	return out, refs, nil
}

/*
refField tests a column naming a row: in or not in.
*/
func refField(column, prefix string) field {
	return field{compile: func(c Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op != "in" && cond.Op != "not_in" {
			return compiled{}, nil, errUnsupported
		}

		values, refs, err := decodeRefs(c, cond.Value, prefix)
		if err != nil {
			return compiled{}, nil, err
		}

		p := compiled{sql: "coalesce(" + column + " = ANY (?::uuid[]), false)", args: []any{values}}

		return negate(cond.Op, p), refs, nil
	}}
}

/*
optionalRefField tests a column that may name no row: also exists and
not exists.
*/
func optionalRefField(column, prefix string) field {
	named := refField(column, prefix)

	return field{compile: func(c Compiler, cond Condition) (compiled, []Reference, error) {
		if cond.Op == "exists" || cond.Op == "not_exists" {
			return negate(cond.Op, compiled{sql: column + " IS NOT NULL"}), nil, nil
		}

		return named.compile(c, cond)
	}}
}

/*
existsField tests rows linked through a subquery: in and not in name
rows, exists and not exists ask for any.
*/
func existsField(prefix, anyRow, named string) field {
	return field{compile: func(c Compiler, cond Condition) (compiled, []Reference, error) {
		switch cond.Op {
		case "in", "not_in":
			values, refs, err := decodeRefs(c, cond.Value, prefix)
			if err != nil {
				return compiled{}, nil, err
			}

			return negate(cond.Op, compiled{sql: named, args: []any{values}}), refs, nil
		case "exists", "not_exists":
			return negate(cond.Op, compiled{sql: anyRow}), nil, nil
		}

		return compiled{}, nil, errUnsupported
	}}
}

func tagField() field {
	const tagged = "EXISTS (SELECT 1 FROM conversation_tag ctg WHERE ctg.workspace_id = c.workspace_id" +
		" AND ctg.conversation_id = c.id"

	return existsField("tag", tagged+")", tagged+" AND ctg.tag_id = ANY (?::uuid[]))")
}

func companyField() field {
	const company = "EXISTS (SELECT 1 FROM contacts ct WHERE ct.workspace_id = c.workspace_id" +
		" AND ct.id = c.contact_id AND ct.company_id"

	return existsField("cmp", company+" IS NOT NULL)", company+" = ANY (?::uuid[]))")
}

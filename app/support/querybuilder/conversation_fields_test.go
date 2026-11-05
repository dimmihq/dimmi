package querybuilder_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
public writes a public id of the prefix the test compiler reads.
*/
func public(tb testing.TB, prefix string) string {
	tb.Helper()

	registry := ids.NewRegistry()
	require.NoError(tb, registry.Register(prefix, prefix))

	return registry.Encode(prefix, ids.New())
}

func TestCompileFields(t *testing.T) {
	t.Parallel()

	t.Run("compiles each field and operator it has", func(t *testing.T) {
		t.Parallel()

		for _, c := range []struct{ field, op, value, sql string }{
			{"status", "not_in", `["open","closed"]`, "NOT (c.status = ANY"},
			{"channel", "in", `["email"]`, "cc.type = ANY"},
			{"channel", "not_in", `["api"]`, "NOT (EXISTS"},
			{"channel_connection", "in", `["` + public(t, "chn") + `"]`, "c.channel_connection_id = ANY"},
			{"inbox", "not_in", `["` + public(t, "inbox") + `"]`, "NOT (coalesce(c.inbox_id"},
			{"contact", "in", `["` + public(t, "ctc") + `"]`, "c.contact_id = ANY"},
			{"assignee", "in", `["me"]`, "c.assignee_member_id = ANY"},
			{"assignee", "not_exists", `null`, "NOT (c.assignee_member_id IS NOT NULL)"},
			{"team", "exists", `null`, "c.assignee_team_id IS NOT NULL"},
			{"tag", "in", `["` + public(t, "tag") + `"]`, "ctg.tag_id = ANY"},
			{"tag", "not_exists", `null`, "NOT (EXISTS (SELECT 1 FROM conversation_tag"},
			{"company", "exists", `null`, "ct.company_id IS NOT NULL"},
			{"company", "not_in", `["` + public(t, "cmp") + `"]`, "ct.company_id = ANY"},
			{"priority", "in", `["low","urgent"]`, "c.priority = ANY"},
			{"priority", "gte", `"high"`, "c.priority >= "},
			{"priority", "lte", `"none"`, "c.priority <= "},
			{"unread", "eq", "true", "m.undone_at IS NULL"},
			{"waiting", "eq", "true", "c.waiting_since IS NOT NULL"},
			{"imported", "eq", "false", "NOT (coalesce((c.source"},
			{"message_count", "lt", "4", "c.message_count < "},
			{"created_at", "before", `"2026-11-01T00:00:00Z"`, "c.created_at < "},
			{"closed_at", "after", `"2026-11-01T00:00:00Z"`, "c.closed_at > "},
			{"waiting_since", "older_than", `"PT2H"`, "c.waiting_since < "},
			{"snoozed_until", "newer_than", `"P1D"`, "c.snoozed_until > "},
			{"company.attr.tier", "eq", `"gold"`, "co.custom_attributes @>"},
			{"contact.attr.plan", "exists", `null`, "jsonb_exists(ct.custom_attributes"},
		} {
			out, err := compiler(t).Compile(definition("and", group("and", condition(c.field, c.op, c.value))))
			require.NoError(t, err, c.field+" "+c.op)

			sql, args := out.Predicate(1)
			assert.Contains(t, sql, c.sql, c.field+" "+c.op)
			assert.Equal(t, strings.Count(sql, "$"), len(args), c.field+" "+c.op)
		}
	})

	t.Run("lists what each named row is, me aside", func(t *testing.T) {
		t.Parallel()
		tag := public(t, "tag")

		out, err := compiler(t).Compile(definition("and",
			group("and", condition("tag", "in", `["`+tag+`"]`), condition("assignee", "in", `["me"]`))))
		require.NoError(t, err)

		require.Len(t, out.References, 1)
		assert.Equal(t, "tag", out.References[0].Prefix)
		assert.Equal(t, "groups.0.conditions.0", out.References[0].Path)
	})

	t.Run("refuses operators and values a field does not take, naming where", func(t *testing.T) {
		t.Parallel()

		for _, c := range []struct{ field, op, value, path string }{
			{"status", "eq", `"open"`, "op"},
			{"status", "in", `[]`, "value"},
			{"status", "in", `["pending"]`, "value"},
			{"channel", "eq", `"email"`, "op"},
			{"channel", "in", `"email"`, "value"},
			{"inbox", "eq", `"x"`, "op"},
			{"inbox", "in", `["nope"]`, "value"},
			{"assignee", "in", `"me"`, "value"},
			{"tag", "eq", `"x"`, "op"},
			{"tag", "in", `["nope"]`, "value"},
			{"priority", "gt", `"high"`, "op"},
			{"priority", "in", `["huge"]`, "value"},
			{"priority", "eq", `3`, "value"},
			{"priority", "eq", `"huge"`, "value"},
			{"unread", "in", `true`, "op"},
			{"unread", "eq", `"yes"`, "value"},
			{"number", "eq", `"7"`, "value"},
			{"created_at", "before", `7`, "value"},
			{"created_at", "before", `"yesterday"`, "value"},
			{"created_at", "older_than", `"two hours"`, "value"},
			{"created_at", "older_than", `"P"`, "value"},
			{"created_at", "older_than", `"PT99999999999H"`, "value"},
			{"created_at", "eq", `"P1D"`, "op"},
			{"subject", "eq", `"x"`, "op"},
			{"subject", "contains", `"  "`, "value"},
			{"contact.attr.plan", "eq", `null`, "value"},
			{"contact.attr.plan", "in", `["a"]`, "op"},
			{"contact.attr.Bad-Key", "eq", `"a"`, "field"},
			{"text", "contains", `"a"`, "op"},
			{"text", "matches", `"  "`, "value"},
		} {
			_, err := compiler(t).Compile(definition("and", group("and", condition(c.field, c.op, c.value))))

			var def *querybuilder.DefinitionError
			require.True(t, errors.As(err, &def), c.field+" "+c.op+" "+c.value)
			assert.Equal(t, "groups.0.conditions.0."+c.path, def.Path, c.field+" "+c.op+" "+c.value)
			assert.Contains(t, def.Error(), "invalid definition at groups.0.conditions.0")
		}
	})

	t.Run("refuses a second search and one the search cannot read", func(t *testing.T) {
		t.Parallel()
		twice := definition("and", group("and",
			condition("text", "matches", `"a"`), condition("text", "matches", `"b"`)))

		_, err := compiler(t).Compile(twice)
		require.Error(t, err)

		broken := compiler(t)
		broken.Search = func(string) (string, error) { return "", errors.New("no terms") }
		_, err = broken.Compile(definition("and", group("and", condition("text", "matches", `"a"`))))

		var def *querybuilder.DefinitionError
		require.True(t, errors.As(err, &def))
		assert.Equal(t, "no terms", def.Reason)
	})

	t.Run("anchors relative times at the compiler's now", func(t *testing.T) {
		t.Parallel()
		c := compiler(t)

		out, err := c.Compile(definition("and", group("and", condition("created_at", "older_than", `"P1W"`))))
		require.NoError(t, err)

		_, args := out.Predicate(1)
		assert.Equal(t, c.Now.Add(-7*24*time.Hour), args[0])
	})
}

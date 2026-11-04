package conversations_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestCustomViewFields(t *testing.T) {
	t.Parallel()

	d := open(t)
	team := d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
	refund := d.customer(t, "ann", "Refund please", "I was charged twice")
	d.tc.Clock.Travel(2 * time.Hour)
	login := d.customer(t, "bea", "Login trouble", "The code never arrives")
	snoozed := d.customer(t, "cid", "Later", "Whenever")

	d.owner.PatchJSON("/api/v1/conversations/"+refund, testkit.JSON{
		"assignee_id": "me", "team_id": team, "priority": "high",
	}).AssertOK()
	d.owner.PostJSON("/api/v1/conversations/"+refund+"/replies", testkit.JSON{"body": "Refunded"}).AssertCreated()
	d.owner.PatchJSON("/api/v1/conversations/"+snoozed, testkit.JSON{
		"status": "snoozed", "snooze_preset": "tomorrow", "priority": "urgent",
	}).AssertOK()
	contact := d.owner.GetJSON("/api/v1/conversations/" + login).String("data.contact.id")
	d.owner.PatchJSON("/api/v1/contacts/"+contact, testkit.JSON{"custom_attributes": testkit.JSON{"plan": "pro"}}).
		AssertOK()
	inbox := d.inboxID()
	number, err := strconv.Atoi(d.owner.GetJSON("/api/v1/conversations/" + login).String("data.number"))
	require.NoError(t, err)
	connection := d.owner.GetJSON("/api/v1/conversations/" + login).String("data.channel.id")
	hourAgo := d.tc.Clock.Now().Add(-time.Hour).UTC().Format(time.RFC3339)

	everyone := []string{refund, login, snoozed}

	for name, tc := range map[string]struct {
		field, op string
		value     any
		want      []string
	}{
		"status in":                {"status", "in", []string{"snoozed"}, []string{snoozed}},
		"status not in":            {"status", "not_in", []string{"snoozed"}, []string{refund, login}},
		"channel in":               {"channel", "in", []string{"api"}, everyone},
		"channel not in":           {"channel", "not_in", []string{"api"}, nil},
		"connection in":            {"channel_connection", "in", []string{connection}, everyone},
		"inbox in":                 {"inbox", "in", []string{inbox}, everyone},
		"contact in":               {"contact", "in", []string{contact}, []string{login}},
		"assignee is me":           {"assignee", "in", []string{"me"}, []string{refund}},
		"assignee exists":          {"assignee", "exists", nil, []string{refund}},
		"assignee not exists":      {"assignee", "not_exists", nil, []string{login, snoozed}},
		"team in":                  {"team", "in", []string{team}, []string{refund}},
		"team not in":              {"team", "not_in", []string{team}, []string{login, snoozed}},
		"team not exists":          {"team", "not_exists", nil, []string{login, snoozed}},
		"tag not exists":           {"tag", "not_exists", nil, everyone},
		"tag exists":               {"tag", "exists", nil, nil},
		"company not exists":       {"company", "not_exists", nil, everyone},
		"priority eq":              {"priority", "eq", "high", []string{refund}},
		"priority at least high":   {"priority", "gte", "high", []string{refund, snoozed}},
		"priority at most high":    {"priority", "lte", "high", []string{refund, login}},
		"priority in":              {"priority", "in", []string{"urgent", "none"}, []string{login, snoozed}},
		"unread":                   {"unread", "eq", true, []string{login, snoozed}},
		"read":                     {"unread", "eq", false, []string{refund}},
		"waiting":                  {"waiting", "eq", true, []string{login, snoozed}},
		"not imported":             {"imported", "eq", false, everyone},
		"number eq":                {"number", "eq", number, []string{login}},
		"number gt":                {"number", "gt", 1, []string{login, snoozed}},
		"number lt":                {"number", "lt", 2, []string{refund}},
		"message count gt":         {"message_count", "gt", 1, []string{refund}},
		"created before":           {"created_at", "before", hourAgo, []string{refund}},
		"created after":            {"created_at", "after", hourAgo, []string{login, snoozed}},
		"created older than":       {"created_at", "older_than", "PT1H", []string{refund}},
		"created newer than":       {"created_at", "newer_than", "PT1H", []string{login, snoozed}},
		"last message newer":       {"last_message_at", "newer_than", "P1D", everyone},
		"snooze ends after":        {"snoozed_until", "after", hourAgo, []string{snoozed}},
		"closed before":            {"closed_at", "before", hourAgo, nil},
		"waiting since older than": {"waiting_since", "older_than", "PT1H", nil},
		"subject contains":         {"subject", "contains", "fund", []string{refund}},
		"subject stays data":       {"subject", "contains", "'; DROP TABLE conversations; --", nil},
		"wildcards stay data":      {"subject", "contains", "%", nil},
		"text matches":             {"text", "matches", "code arrives", []string{login}},
		"contact attribute eq":     {"contact.attr.plan", "eq", "pro", []string{login}},
		"contact attribute exists": {"contact.attr.plan", "exists", nil, []string{login}},
		"company attribute exists": {"company.attr.plan", "exists", nil, nil},
	} {
		t.Run(name, func(t *testing.T) {
			id := saveView(t, d.owner, "personal", oneCondition(tc.field, tc.op, tc.value))

			got := listed(d.owner.GetJSON("/api/v1/conversations?filter[status]=any&filter[view]=" + id).AssertOK())

			assert.ElementsMatch(t, tc.want, got, name)
		})
	}

	t.Run("joins groups and conditions both ways", func(t *testing.T) {
		id := saveView(t, d.owner, "personal", testkit.JSON{"op": "or", "groups": []testkit.JSON{
			{"op": "and", "conditions": []testkit.JSON{
				{"field": "priority", "op": "eq", "value": "high"}, {"field": "unread", "op": "eq", "value": false},
			}},
			{"op": "or", "conditions": []testkit.JSON{{"field": "contact.attr.plan", "op": "eq", "value": "pro"}}},
		}})

		assert.ElementsMatch(t, []string{refund, login},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[status]=any&filter[view]="+id)))
	})

	t.Run("refuses text matching inside an or", func(_ *testing.T) {
		d.owner.PostJSON("/api/v1/conversation-views", testkit.JSON{
			"name": "Text or", "scope": "personal", "definition": testkit.JSON{"op": "or", "groups": []testkit.JSON{
				{"op": "and", "conditions": []testkit.JSON{{"field": "text", "op": "matches", "value": "refund"}}},
			}},
		}).AssertStatus(422).AssertJSONPath("path", "groups.0.conditions.0")
	})
}

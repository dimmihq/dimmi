package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

func TestOperatorUsage(t *testing.T) {
	t.Parallel()

	t.Run("counts the month's runs by purpose, teammate and thread", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Where is my refund?")
		k.llm.Script(reply("Brief."), reply("It is on its way."))
		k.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/operator/brief", testkit.JSON{}).AssertOK()
		k.ask(k.thread(conv), "Where is it?").AssertOK()

		res := k.d.Owner.GetJSON("/api/v1/operator/usage").AssertOK()

		assert.Equal(t, "2", res.String("data.runs_count"))
		assert.ElementsMatch(t, []string{"ask", "brief"},
			[]string{res.String("data.by_purpose.0.purpose"), res.String("data.by_purpose.1.purpose")})
		assert.Equal(t, "2", res.String("data.by_member.0.runs_count"))
		assert.Equal(t, "ask", res.String("data.recent.0.purpose"))
		assert.NotEmpty(t, res.String("data.recent.0.thread_title"))
		assert.NotEmpty(t, res.String("data.recent.0.conversation_number"))
	})

	t.Run("counts a run about a conversation the teammate may not see, without naming it", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		conv := k.conversation("Where is my refund?")
		k.llm.Script(reply("Brief."))
		k.d.Owner.PostJSON("/api/v1/conversations/"+conv+"/operator/brief", testkit.JSON{}).AssertOK()
		member, row := k.d.Join(t, enums.RoleMember)
		k.d.Owner.PatchJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, row.ID), testkit.JSON{
			"conversation_access": "assigned",
		}).AssertOK()

		res := member.GetJSON("/api/v1/operator/usage").AssertOK()

		assert.Equal(t, "1", res.String("data.runs_count"))
		assert.Equal(t, "brief", res.String("data.recent.0.purpose"))
		assert.Empty(t, res.String("data.recent.0.conversation_number"))
		assert.Empty(t, res.String("data.recent.0.conversation_id"))
		owner := k.d.Owner.GetJSON("/api/v1/operator/usage").AssertOK()
		assert.NotEmpty(t, owner.String("data.recent.0.conversation_number"))
	})

	t.Run("answers an empty month with zeros", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)

		k.d.Owner.GetJSON("/api/v1/operator/usage?period=2020-01").AssertOK().
			AssertJSONPath("data.runs_count", float64(0)).AssertJSONPath("data.period_start", "2020-01-01")
	})

	t.Run("refuses a period that is no month", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)

		k.d.Owner.GetJSON("/api/v1/operator/usage?period=soon").AssertStatus(422)
	})

	t.Run("lets members read usage, like the agent's own", func(t *testing.T) {
		t.Parallel()
		k := newDesk(t)
		member, _ := k.d.Join(t, enums.RoleMember)

		member.GetJSON("/api/v1/operator/usage").AssertOK()
	})
}

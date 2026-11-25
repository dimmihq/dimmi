package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
TestProcedureExit covers leaving a procedure: the customer's call, as a
model reading the conversation judges it, never the agent's alone.
*/
func TestProcedureExit(t *testing.T) {
	t.Parallel()

	t.Run("keeps the procedure when the agent wants out but the customer has not moved on", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		conv, trigger := r.conversation("I want a refund")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"moved_on": false, "asks": true})
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			tools(call("update_procedure_status", `{"status":"exited_topic_changed","reason":"the lookup disagrees"}`)),
			reply("Which order is it?"),
		)

		r.run(conv, trigger)

		assert.Equal(t, "active", r.procedureState(conv)[0].Status, "the agent cannot walk away from the request")
		assert.Contains(t, r.toolResult(2), "still wants what this procedure is for")
	})
}

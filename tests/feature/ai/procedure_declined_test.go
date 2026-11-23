package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAiProcedureDeclinedStep(t *testing.T) {
	t.Parallel()

	t.Run("does not send a reply back for an action the customer turned down", func(t *testing.T) {
		t.Parallel()
		r := newRefundDesk(t)
		conv, trigger := r.conversation("Actually no, leave the order as it is. I don't want it looked up or refunded.")
		r.signIn(conv)
		r.llm.judge(map[string]bool{"declined": true})
		r.llm.answer.Script(
			tools(call("follow_procedure", r.follow())),
			reply("No problem, I've left your order as it is."),
		)

		res := r.run(conv, trigger)

		assert.Equal(t, "answered", res.Outcome, "the chat is not handed over for an action nobody wants")
		assert.Len(t, r.llm.answer.Sent(), 2, "the reply is sent as written, with no retry")
		assert.Equal(t, []string{"step_declined"}, guards(trace(t, r.load(conv, res.RunID).run)))
		assert.Empty(t, r.api.Sent(), "nothing ran")
	})
}

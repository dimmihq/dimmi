package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
liveProcedure publishes a procedure and returns its public id.
*/
func (h *harness) liveProcedure(name string, def testkit.JSON) string {
	h.t.Helper()
	res := h.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": name}).AssertCreated()
	id := res.String("data.id")
	h.d.Owner.PatchJSON("/api/v1/procedures/"+id, testkit.JSON{"definition": def}).AssertOK()
	h.d.Owner.PostJSON("/api/v1/procedures/"+id+"/publications", nil).AssertOK()

	return id
}

func TestAiProcedures(t *testing.T) {
	t.Parallel()

	signIn := testkit.JSON{
		"description": "The customer cannot sign in.",
		"steps": []testkit.JSON{
			{"id": "s1", "kind": "instruction", "text": "Ask which email address they sign in with."},
		},
		"handover": []string{"They lost access to the mailbox."},
	}

	t.Run("offers live procedures and follows the one the request matches", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		id := h.liveProcedure("Send a sign-in code", signIn)
		args, _ := json.Marshal(map[string]string{"procedure_id": id})
		h.llm.answer.Script(tools(call("follow_procedure", string(args))),
			reply("Which email address do you sign in with?"))

		conv, trigger := h.conversation("I can't sign in to my account")
		res := h.run(conv, trigger)
		require.Equal(t, "answered", res.Outcome)

		first := h.llm.answer.Sent()[0]
		var offered string
		for _, tool := range first.Tools {
			if tool.Name == "follow_procedure" {
				offered = tool.Description
			}
		}
		assert.Contains(t, offered, id+": Send a sign-in code. Use when: The customer cannot sign in.")
		assert.Contains(t, sentText(h.llm.answer), "1. Ask which email address they sign in with.")
		assert.Contains(t, sentText(h.llm.answer), "They lost access to the mailbox.")

		s := h.load(conv, res.RunID)
		assert.True(t, slicesHasPrefix(s.run.Flags, "procedure:"))
		h.d.Owner.GetJSON("/api/v1/procedures").AssertOK().AssertJSONPath("data.0.uses", float64(1))
	})

	t.Run("offers no procedure tool when none is live", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.d.Owner.PostJSON("/api/v1/procedures", testkit.JSON{"name": "Draft only"}).AssertCreated()
		h.llm.answer.Script(reply("Hello! How can I help?"))

		conv, trigger := h.conversation("hi")
		h.run(conv, trigger)
		for _, tool := range h.llm.answer.Sent()[0].Tools {
			assert.NotEqual(t, "follow_procedure", tool.Name)
		}
	})
}

/*
slicesHasPrefix reports whether any flag starts with the prefix.
*/
func slicesHasPrefix(flags []string, prefix string) bool {
	for _, f := range flags {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}

	return false
}

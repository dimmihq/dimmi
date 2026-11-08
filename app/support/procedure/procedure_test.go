package procedure_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/procedure"
)

func signIn() procedure.Definition {
	return procedure.Definition{
		Description: "The customer cannot sign in.",
		Steps: []procedure.Step{
			{ID: "s1", Kind: procedure.KindInstruction, Text: "Ask which email they sign in with."},
			{ID: "s2", Kind: procedure.KindAction, Text: "Look up the account.", ActionID: "act_1"},
			{ID: "s3", Kind: procedure.KindCondition, Text: "If no account matches, offer to check another address."},
		},
		Handover: []string{"They lost access to the mailbox."},
	}
}

func TestProcedure(t *testing.T) {
	t.Parallel()

	t.Run("reads a definition and refuses unknown keys", func(t *testing.T) {
		t.Parallel()
		raw, _ := json.Marshal(signIn())
		d, err := procedure.Parse(raw)
		require.NoError(t, err)
		assert.Len(t, d.Steps, 3)

		_, err = procedure.Parse(json.RawMessage(`{"steps":[],"extra":1}`))
		require.Error(t, err)

		empty, err := procedure.Parse(nil)
		require.NoError(t, err)
		assert.Empty(t, empty.Steps)
	})

	t.Run("finds nothing wrong with a finished procedure", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, procedure.Check(signIn()))
	})

	t.Run("lists what stops an unfinished one from being published", func(t *testing.T) {
		t.Parallel()
		d := signIn()
		d.Description = " "
		d.Steps = append(d.Steps,
			procedure.Step{ID: "s4", Kind: procedure.KindAction, Text: "Refund it."},
			procedure.Step{ID: "s4", Kind: "magic", Text: ""},
		)

		fields := map[string]bool{}
		for _, i := range procedure.Check(d) {
			fields[i.Code+":"+i.Field] = true
		}

		assert.True(t, fields["missing_setting:description"])
		assert.True(t, fields["missing_setting:action_id"])
		assert.True(t, fields["missing_setting:text"])
		assert.True(t, fields["duplicate_id:"])
		assert.True(t, fields["schema:"])
		assert.NotEmpty(t, procedure.Check(procedure.Empty()))
	})

	t.Run("writes the steps as dimmi reads them, naming each action's tool", func(t *testing.T) {
		t.Parallel()
		text := procedure.Render("Send a sign-in code", signIn(), map[string]string{"act_1": "look_up_account"})

		assert.Contains(t, text, "Procedure: Send a sign-in code\nUse when: The customer cannot sign in.")
		assert.Contains(t, text, "2. Do: Look up the account. (call the look_up_account tool)")
		assert.Contains(t, text, "3. Check: If no account matches")
		assert.Contains(t, text, "- They lost access to the mailbox.")
		assert.Contains(t, procedure.Render("x", signIn(), nil), "not available right now")
	})

	t.Run("lists each action once", func(t *testing.T) {
		t.Parallel()
		d := signIn()
		again := procedure.Step{ID: "s9", Kind: procedure.KindAction, Text: "Again", ActionID: "act_1"}
		d.Steps = append(d.Steps, again)
		assert.Equal(t, []string{"act_1"}, procedure.ActionIDs(d))
	})
}

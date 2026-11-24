package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
routineAt is a routine's session with 16 turns and 75 reading steps,
having used the reading steps given.
*/
func routineAt(used int) *session {
	return &session{
		toolCalls: used,
		routine:   &routineRun{row: models.OperatorRoutine{MaxModelCalls: 16, MaxToolCalls: 75}},
	}
}

func TestRunningBudget(t *testing.T) {
	t.Parallel()

	e := &Engine{}

	t.Run("says nothing while a routine has plenty left", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, e.running(routineAt(10), 3))
	})

	t.Run("says what is left once reading steps run low", func(t *testing.T) {
		t.Parallel()

		note := e.running(routineAt(60), 4)

		assert.Contains(t, note, "15 reading steps")
		assert.Contains(t, note, "12 turns")
	})

	t.Run("says what is left once turns run low", func(t *testing.T) {
		t.Parallel()

		assert.Contains(t, e.running(routineAt(5), 12), "4 turns")
	})

	t.Run("says nothing to a chat, or to a run with nothing left", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, e.running(&session{toolCalls: 70}, 15))
		assert.Empty(t, e.running(routineAt(75), 4))
		assert.Empty(t, e.running(routineAt(10), 16))
	})

	t.Run("adds the note to the last tool result and to nothing else", func(t *testing.T) {
		t.Parallel()

		messages := []contracts.ChatMessage{
			{Role: contracts.ChatRoleTool, Parts: textParts("first")},
			{Role: contracts.ChatRoleTool, Parts: textParts("second")},
		}

		remind(messages, "[note]")

		assert.Len(t, messages[0].Parts, 1)
		assert.Len(t, messages[1].Parts, 2)
		assert.Equal(t, "[note]", messages[1].Parts[1].Text)

		remind(messages, "")
		assert.Len(t, messages[1].Parts, 2)
	})
}

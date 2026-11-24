package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
)

func TestWorkflowCustomerMoves(t *testing.T) {
	t.Parallel()

	call := func(args string) *contracts.ChatResponse {
		return &contracts.ChatResponse{ToolCalls: []contracts.ToolCall{
			{Name: customerTool, Arguments: json.RawMessage(args)},
		}}
	}
	menu := CustomerTurn{
		Saved:   []SavedMessage{{Text: "Hi", Sent: true}},
		Run:     []SimMessage{{From: "customer", Text: "Hi"}, {From: "agent", Text: "What is it about?"}},
		Choices: []string{"Billing", "Bugs"}, CanWait: true,
	}

	move, err := readMove(call(`{"move":"tap","saved":0,"message":"","unknown":false,"choice":2}`), menu)
	require.NoError(t, err)
	assert.Equal(t, CustomerMove{Message: "Bugs", Choice: 2}, move, "a tap sends the button's label")
	assert.False(t, ownWords(move), "a tap is not held to the scenario like the customer's own words")

	move, err = readMove(call(`{"move":"wait","saved":0,"message":"","unknown":false,"choice":0}`), menu)
	require.NoError(t, err)
	assert.True(t, move.Waited)

	_, err = readMove(call(`{"move":"tap","saved":0,"message":"","unknown":false,"choice":3}`), menu)
	require.ErrorIs(t, err, ErrCustomerFailed, "a tap names one of the buttons")

	only := menu
	only.ChoicesOnly, only.CanWait = true, false
	assert.Equal(t, []string{moveTap, moveEnd}, movesNow(only), "only a tap is offered when typing is not")
	_, err = readMove(call(`{"move":"answer","saved":0,"message":"bugs","unknown":false,"choice":0}`), only)
	require.ErrorIs(t, err, ErrCustomerFailed)
	_, err = readMove(call(`{"move":"wait","saved":0,"message":"","unknown":false,"choice":0}`), only)
	require.ErrorIs(t, err, ErrCustomerFailed, "waiting needs a wait that can run out")

	var schema struct {
		Required []string `json:"required"`
	}
	require.NoError(t, json.Unmarshal(customerSpec(menu).Schema, &schema))
	assert.Contains(t, schema.Required, "choice")
	require.NoError(t, json.Unmarshal(customerSpec(CustomerTurn{}).Schema, &schema))
	assert.NotContains(t, schema.Required, "choice", "an agent's simulation keeps the tool it had")
	assert.Contains(t, customerStatus(menu), "1. Billing")

	waiting := CustomerTurn{
		Run:     []SimMessage{{From: "agent", Text: "We'll check back in a day."}},
		CanWait: true, Waiting: "1d",
	}
	assert.Equal(t, []string{moveAnswer, moveWait, moveEnd}, movesNow(waiting), "a wait lets the customer write or not")
	assert.Contains(t, customerStatus(waiting), "waits 1d before it goes on")
	assert.Contains(t, workflowMoves(waiting), "let the time pass")
	move, err = readMove(call(`{"move":"answer","saved":0,"message":"Any news?","unknown":false}`), waiting)
	require.NoError(t, err)
	assert.Equal(t, "Any news?", move.Message, "writing during a wait interrupts it")
}

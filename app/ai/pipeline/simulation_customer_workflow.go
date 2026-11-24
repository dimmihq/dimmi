/*
|--------------------------------------------------------------------------
| The Simulated Customer In A Workflow
|--------------------------------------------------------------------------
|
| A workflow talks to the customer with more than words: it offers
| buttons, asks for one value, and waits a while for an answer before
| it gives up. The simulated customer of a workflow simulation can do
| what a real one does with them: tap one of the buttons, type an
| answer instead when the workflow lets them, or say nothing and let
| the wait run out. Tapping is the only move when the buttons are all
| the workflow accepts.
|
| The tool the customer moves through is built here too, for agents and
| workflows alike: it offers only the moves the chat allows now.
|
*/

package pipeline

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
choiceNumbers are the numbers a tap may name, zero for none.
*/
func choiceNumbers(choices []string) []int {
	out := []int{0}
	for i := range choices {
		out = append(out, i+1)
	}

	return out
}

/*
workflowMoves describes the moves a workflow adds, for the tool.
*/
func workflowMoves(in CustomerTurn) string {
	out := ""
	if len(in.Choices) > 0 {
		out += " " + moveTap + ": tap one of the buttons the workflow offers."
	}
	if in.CanWait {
		out += " " + moveWait + ": say nothing and let the workflow's wait run out."
	}

	return out
}

/*
workflowMove holds a tap or a wait to what the workflow allows: a tap
must name one of the buttons, and waiting needs a wait that can run
out.
*/
func workflowMove(told toldMove, in CustomerTurn) (CustomerMove, bool) {
	switch told.Move {
	case moveTap:
		if told.Choice >= 1 && told.Choice <= len(in.Choices) {
			return CustomerMove{Message: in.Choices[told.Choice-1], Choice: told.Choice}, true
		}
	case moveWait:
		if in.CanWait {
			return CustomerMove{Waited: true}, true
		}
	}

	return CustomerMove{}, false
}

/*
workflowStatus tells the customer's model the buttons on offer and
whether it may let the wait run out.
*/
func workflowStatus(in CustomerTurn) string {
	if len(in.Choices) == 0 && !in.CanWait {
		return ""
	}
	var b strings.Builder
	if len(in.Choices) > 0 {
		b.WriteString("\nButtons the customer can tap:")
		for i, c := range in.Choices {
			b.WriteString("\n" + strconv.Itoa(i+1) + ". " + escape(c))
		}
		if in.ChoicesOnly {
			b.WriteString("\nThe customer can only tap a button here, not type.")
		}
	}
	if in.CanWait {
		b.WriteString("\nThe workflow waits for an answer; the customer may also say nothing.")
	}

	return b.String()
}

/*
movesNow lists the moves the chat allows the customer now: a saved
message only while one is unsent, and Confirm only while a card waits
for something this customer came for.
*/
func movesNow(in CustomerTurn) []string {
	moves := []string{moveAnswer}
	if in.ChoicesOnly && len(in.Choices) > 0 {
		moves = nil
	}
	if len(in.Choices) > 0 {
		moves = append(moves, moveTap)
	}
	if in.CanWait {
		moves = append(moves, moveWait)
	}
	if len(unsent(in.Saved)) > 0 && (!in.ChoicesOnly || len(in.Choices) == 0) {
		moves = append(moves, moveSendSaved)
	}
	if in.confirmable {
		moves = append(moves, moveConfirm)
	}

	return append(moves, moveEnd)
}

/*
customerSpec is the tool the customer moves through. It offers only
the moves the chat allows now, and only the saved messages still
unsent, so a move the chat does not allow cannot be asked for.
*/
func customerSpec(in CustomerTurn) contracts.ToolSpec {
	numbers := append([]int{0}, unsent(in.Saved)...)
	properties := map[string]any{
		"move": map[string]any{
			"type": "string", "enum": movesNow(in),
			"description": moveSendSaved + ": send a saved message that has not been sent, word for word. " +
				moveAnswer + ": write the customer's answer to what the agent asked. " +
				moveConfirm + ": press Confirm on the waiting card. " +
				moveEnd + ": the customer has nothing left to say." + workflowMoves(in),
		},
		"saved": map[string]any{
			"type": "integer", "enum": numbers,
			"description": "For " + moveSendSaved + ": the number of the saved message to send. Otherwise 0.",
		},
		"message": map[string]any{
			"type":        "string",
			"description": "For " + moveAnswer + ": what the customer writes. Otherwise empty.",
		},
		"unknown": map[string]any{
			"type":        "boolean",
			"description": "True when the agent asked for something the customer does not know.",
		},
	}
	required := []string{"move", "saved", "message", "unknown"}
	if len(in.Choices) > 0 {
		properties["choice"] = map[string]any{
			"type": "integer", "enum": choiceNumbers(in.Choices),
			"description": "For " + moveTap + ": the number of the button to tap. Otherwise 0.",
		}
		required = append(required, "choice")
	}
	schema, _ := json.Marshal(map[string]any{
		"type": "object", "properties": properties, "required": required, "additionalProperties": false,
	})

	return contracts.ToolSpec{
		Name: customerTool, Description: "Make the customer's next move in the chat.", Schema: schema,
	}
}

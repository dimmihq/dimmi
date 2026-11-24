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
*/

package pipeline

import (
	"strconv"
	"strings"
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

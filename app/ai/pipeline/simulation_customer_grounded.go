/*
|--------------------------------------------------------------------------
| Holding The Simulated Customer To Its Scenario
|--------------------------------------------------------------------------
|
| A saved message goes out word for word, so it is the customer's own.
| A message the customer's model writes itself is not: the small model
| that plays the customer sometimes invents that a step was tried and
| failed, agrees to an offer the customer never asked for, or raises a
| question of its own. Each changes what the simulation tests.
|
| So every message the model writes in its own words is put to the
| decision model with everything known about the customer: does it
| stay inside that? One that does not is sent back to the customer's
| model with the reason, once. When the second answer strays too, the
| customer has nothing of its own to say and the chat ends there. The
| question is resources/prompts/tasks/judgments/customer_answer.md.
|
| Where no decision model is set up, or it cannot answer, the message
| goes out as written: nothing here guesses at what a sentence means.
|
*/

package pipeline

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
ownWords reports whether a move is a message the customer's model
wrote itself, which is what has to be held to the scenario.
*/
func ownWords(move CustomerMove) bool {
	return move.Message != "" && move.Saved == 0 && move.Choice == 0
}

/*
grounded holds a move the customer's model wrote in its own words to
what is known about the customer. A move that strays is asked again,
with the first answer named as not the customer's; when the second
strays too, or cannot be had, the customer ends the chat.
*/
func (r *Runner) grounded(ctx context.Context, c customerCall, in CustomerTurn, move CustomerMove) CustomerMove {
	if !ownWords(move) || r.customerSupported(ctx, c.st, in, move.Message) {
		return move
	}
	again := in
	again.strayed = move.Message
	next, err := r.askCustomer(ctx, c, again)
	if err != nil {
		return CustomerMove{Done: true}
	}
	if !ownWords(next) || r.customerSupported(ctx, c.st, in, next.Message) {
		return next
	}

	return CustomerMove{Done: true}
}

/*
customerSupported reports whether a message the customer's model wrote
stays inside what is known about the customer. It is taken to when the
decision model cannot be asked.
*/
func (r *Runner) customerSupported(ctx context.Context, st *run, in CustomerTurn, answer string) bool {
	agent := ""
	if n := len(in.Run); n > 0 && in.Run[n-1].From == "agent" {
		agent = in.Run[n-1].Text
	}
	q := prompts.Judgment("customer_answer", prompts.Vars{
		"scenario": cmpOr(strings.TrimSpace(in.Scenario), "(nothing)"),
		"saved":    savedLines(in.Saved),
		"agent":    truncate(r.redactFor(st.env, agent), maxReplyChars),
		"answer":   answer,
	})
	answers := r.decideTwice(ctx, st, contracts.DecisionRequest{
		State: q.State,
		Questions: map[string]contracts.DecisionQuestion{"inside": {
			Type: contracts.DecisionNoul, Instructions: q.Ask, True: q.True, False: q.False,
		}},
	})
	likely, ok := answers["inside"]

	return !ok || likely >= judged
}

/*
strayedNote tells the customer's model that its last answer was not
the customer's, and what to do instead.
*/
func strayedNote(answer string) string {
	return "<not_the_customer>\nYour last answer was not sent: \"" + escape(answer) + "\"\nIt says something " +
		"this customer is not known to have said, done or wanted. Answer again from the scenario and the saved " +
		"messages only. When they do not cover what the agent asked, say the customer does not know and set " +
		"unknown to true; when the agent offered something the customer did not come for, decline; when there " +
		"is nothing left to say, end the chat.\n</not_the_customer>\n"
}

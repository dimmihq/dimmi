/*
|--------------------------------------------------------------------------
| Checking A Reply
|--------------------------------------------------------------------------
|
| Before a reply is sent a model reads it once and answers what code
| needs to know about it, in whatever language it is written: whether
| it tells the customer the conversation was passed to a person,
| whether it carries text never meant for them, such as a note the
| agent left to itself or a placeholder, and whether it talks about the
| agent's own knowledge and sources instead of only answering.
|
| A reply with text not meant for the customer is not patched. It goes
| back to the agent once, with the reason, to be written again; if the
| second reply still carries it, the conversation is handed to the team
| rather than sent.
|
| A reply that explains what the agent has or could not find goes back
| once too, and what comes back is sent whatever it says: that is about
| how a reply reads, never a reason to keep one from a customer.
|
| The questions are resources/prompts/tasks/judgments/reply_*.md.
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
replyCheck is what the model said about one reply. Judged is false when
no model could be asked.
*/
type replyCheck struct {
	claimsHandoff bool
	notForThem    float64
	aboutItself   float64
	judged        bool
}

/*
checkReply asks what code needs to know about a reply and keeps the
answer as the run's latest check.
*/
func (r *Runner) checkReply(ctx context.Context, st *run, reply string) replyCheck {
	check := replyCheck{}
	if strings.TrimSpace(reply) == "" {
		return check
	}
	vars := prompts.Vars{"reply": truncate(r.redactFor(st.env, reply), maxReplyChars)}
	answers := r.judge(ctx, st, contracts.DecisionRequest{
		State: prompts.Judgment("reply_claims_handoff", vars).State,
		Questions: map[string]contracts.DecisionQuestion{
			"claims_handoff":   question("reply_claims_handoff", vars),
			"not_for_customer": question("reply_not_for_customer", vars),
			"about_itself":     question("reply_about_itself", vars),
		},
	})
	if answers != nil {
		check.judged = true
		check.claimsHandoff, check.notForThem = answers["claims_handoff"] >= judged, answers["not_for_customer"]
		check.aboutItself = answers["about_itself"]
	}
	st.reply = &check

	return check
}

/*
withoutNotes sends back, once, a reply that carries text never meant
for the customer, with a note saying why, and returns the reply written
again. A second reply the model is sure still carries it is not sent:
the conversation goes to the team. The reply is read as the customer
would see it, without the source markers the pipeline takes out itself.
*/
func (r *Runner) withoutNotes(ctx context.Context, st *run, draft string) (string, error) {
	if st.handoff != nil {
		return draft, nil
	}
	check := r.checkReply(ctx, st, visible(draft, st.sources))
	if check.notForThem < judged {
		if check.aboutItself >= judged {
			return r.plainly(ctx, st, draft)
		}
		return draft, nil
	}
	st.flags = append(st.flags, "reply_regenerated")
	st.loop.messages = append(st.loop.messages,
		contracts.ChatMessage{Role: contracts.ChatRoleAssistant, Parts: textParts(draft)},
		userMessage("<system_note>That reply was not sent. It contains text that is not for the customer: a "+
			"note to yourself, a placeholder, or a marker that a source or detail is missing. Write the reply "+
			"again for the customer, saying only what you know. What your procedure tells you to say needs no "+
			"source.</system_note>"))
	next, err := r.agentLoop(ctx, st)
	if err != nil {
		return draft, err
	}
	if st.handoff == nil && r.checkReply(ctx, st, visible(next, st.sources)).notForThem >= sure {
		st.flags = append(st.flags, "reply_withheld")
		st.handoff = &handoffRequest{reason: "other", note: "dimmi's reply kept carrying text not meant for " +
			"the customer, so it was not sent. Handed over so a teammate can answer."}
		return "", nil
	}

	return next, nil
}

/*
plainly sends back, once, a reply that tells the customer about the
agent's own knowledge or sources, with a note saying why, and returns
the reply written again. Whatever comes back is sent: a reply is never
withheld for how it reads. A reply that came back empty, or with the
conversation now handed over, leaves the first one standing.
*/
func (r *Runner) plainly(ctx context.Context, st *run, draft string) (string, error) {
	st.flags = append(st.flags, "reply_reworded")
	st.loop.messages = append(st.loop.messages,
		contracts.ChatMessage{Role: contracts.ChatRoleAssistant, Parts: textParts(draft)},
		userMessage("<system_note>That reply was not sent. It tells the customer about your own knowledge or "+
			"sources: what you have, what you looked for, what you could not find or cannot confirm. They "+
			"asked about their own question and only need the answer. Write the reply again with every fact "+
			"and every step it had. Say what you know as plain fact. For something you don't know, name it in "+
			"one short sentence and stop, with no reason after it. Change nothing else.</system_note>"))
	next, err := r.agentLoop(ctx, st)
	if err != nil {
		return draft, err
	}
	if strings.TrimSpace(next) == "" {
		return draft, nil
	}
	if st.handoff == nil {
		r.checkReply(ctx, st, visible(next, st.sources))
	}

	return next, nil
}

/*
visible is a draft as the customer would read it: without the list of
earlier sources and the numbered source markers, which are the agent's
way of citing and are taken out before a reply is sent, and with a
reference where the ticket's will be written.
*/
func visible(draft string, sources []source) string {
	text, _ := citations(earlierMarker().ReplaceAllString(earlierSources().ReplaceAllString(draft, ""), ""), sources)
	return strings.ReplaceAll(text, ticketReference, sampleReference)
}

/*
claimsHandoff reports whether a reply tells the customer a person is
now involved, as the model that read it judged. The check the reply
went through before it got here answers; a reply that went through
none is checked now.
*/
func (r *Runner) claimsHandoff(ctx context.Context, st *run, reply string) bool {
	if st.reply != nil {
		return st.reply.claimsHandoff
	}
	return r.checkReply(ctx, st, reply).claimsHandoff
}

/*
replyAsks reports whether a reply asks the customer for something, as
the model that read it judged.
*/
func (r *Runner) replyAsks(ctx context.Context, st *run, reply string) bool {
	vars := prompts.Vars{"reply": truncate(r.redactFor(st.env, reply), maxReplyChars)}
	answers := r.judge(ctx, st, contracts.DecisionRequest{
		State:     prompts.Judgment("reply_asks", vars).State,
		Questions: map[string]contracts.DecisionQuestion{"asks": question("reply_asks", vars)},
	})

	return answers["asks"] >= judged
}

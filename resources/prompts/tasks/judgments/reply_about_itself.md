<!--
Who reads this: the decision model.
Asked by: checkReply in app/ai/pipeline/reply_check.go, in the same call as
reply_claims_handoff.md and reply_not_for_customer.md. This file has no State: all three questions
are asked about the state written in reply_claims_handoff.md, the reply as the customer would see
it.

What it is for: an agent that does not know something tends to explain itself. "I don't have a
comparison with them", "I couldn't find that in our information", "from what I can see". The
customer asked about their own problem and gets a sentence about the agent's sources. Every root
system prompt tells the agent not to do this, and a model does it anyway often enough that telling
it has stopped helping. So the reply is read once more before it goes out, and when this is likely
it goes back to the agent, once, to be written again (plainly in reply_check.go). It is never
withheld and the conversation is never handed over for it: this is about how a reply reads, not
about whether it is safe to send.

What it must get right: the line between not knowing and explaining why. "Not sure about Outlook."
is exactly what the agent is told to write and must be false. So must every plain statement about
the product, including what the product does not do, and every statement of what the agent did. A
wrong "true" costs a second generation and a short delay; a wrong "false" sends a reply that reads
like a search engine apologising.
-->
# Question
Does the reply talk about the agent's own knowledge, sources or searching, instead of only answering the customer?

# True when
Some part of the reply tells the customer about what the agent has, was given, can see, looked for or could not find: that it has no comparison, document, steps, details or information on something; that it searched or checked and found nothing; that it cannot confirm, verify or say something from what it has; that it does not want to guess; or it names where its facts come from, such as the documentation, the help center, an article, a page, its notes or its information. One such sentence is enough, however good the rest of the reply is.

# False when
The reply only tells the customer about their own question. Saying in a few words that it is not sure about a named thing, or does not know it, with no reason given, is false. So is stating what the product does or does not do, a price, a limit or a step; saying what the agent did or passed on; saying a person will help; saying it is an AI; declining something it will not do; and asking the customer a question. A link the customer can open is false too.

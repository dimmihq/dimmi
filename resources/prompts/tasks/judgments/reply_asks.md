<!--
Who reads this: the decision model.
Asked by: replyAsks in app/ai/pipeline/reply_check.go, and the simulated customer in
app/ai/pipeline/simulation_customer.go, which only writes a reply when this is true.

What it must get right: tell a reply that is waiting on the customer from one that is finished.
A closing courtesy ("let me know if you need anything else") waits on nothing. A wrong "true"
keeps a simulation going with a customer who has nothing to answer; a wrong "false" ends a chat
while the agent is still waiting for a detail.

Placeholder: {{reply}} is the agent's reply, already redacted and cut to length by the caller.
-->
# State
An AI support agent wrote this reply to a customer. Judge only the reply below, as the customer would read it.

REPLY:
{{reply}}

# Question
Does the reply need something back from the customer before the agent can go on?

# True when
The reply asks the customer a question, asks for a detail (such as an order number, an email address, a date or a screenshot), asks them to choose between options, or asks them to confirm or agree to something, and the agent cannot go on until they answer. A reply that both answers and then asks for something it still needs counts.

# False when
The reply asks the customer for nothing: it only answers, explains, or says what happens next. An open invitation that the agent does not depend on, such as "let me know if you need anything else", "does that help?" or "feel free to reach out", is not asking for something. A rhetorical question the agent answers itself is not asking either.

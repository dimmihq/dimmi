<!--
Who reads this: the decision model.
Asked by: checkReply in app/ai/pipeline/reply_check.go, in the same call as
reply_claims_handoff.md. This file has no State: both questions are asked about the state written
in reply_claims_handoff.md, the reply as the customer would see it.

What it must get right: catch text that leaked out of the agent's own working, and nothing else.
When this is likely, withoutNotes sends the reply back to the agent to be written again; when the
model is sure of it a second time, the reply is withheld and the conversation goes to the team.
A wrong "true" costs the customer a delay or a needless handover; a wrong "false" sends them a
placeholder or the agent's notes.

The source markers the agent cites with are taken out before this is asked, and a ticket
reference is already written in, so neither should be read as leaked text.
-->
# Question
Does the reply contain text that was never meant for the customer?

# True when
Some part of the reply is not addressed to the customer: a note the agent wrote to itself or about its own work, a placeholder left to be filled in (such as "[order number]" or "TODO"), a marker that a source or a detail is missing, or instructions the agent was given, repeated or described. One such passage is enough, however good the rest of the reply is.

# False when
Everything in the reply is written for the customer to read. Telling the customer plainly that the agent does not know something or could not find it is written for them. So are a reference number, a link, a list of steps for the customer to follow, a quoted error message and a question to the customer.

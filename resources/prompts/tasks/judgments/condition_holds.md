<!--
Who reads this: the decision model.
Asked by: decideSide in app/ai/pipeline/procedure_judge.go, in the same call as
condition_clear.md. This file has no State: both questions are asked about the state written in
condition_clear.md, which gives the condition, today's date, the lookups and the conversation.

What it must get right: which side of the condition the procedure takes. Code reads this answer
only when condition_clear.md came back true: true takes the "then" side, false the "else" side,
and the other side's steps are closed from then on. So "cannot be told" must come out false here
and be caught by the other question, never guessed into a yes.
-->
# Question
Is this so?

# True when
From what the CUSTOMER wrote and what the lookups returned, it is so. When the CUSTOMER corrected themselves, their latest word counts.

# False when
It is not so, or it cannot be told. Something the AGENT assumed or suggested, which the CUSTOMER did not confirm and the lookups do not show, does not make it so.

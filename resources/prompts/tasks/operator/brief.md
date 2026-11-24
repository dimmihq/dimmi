<!--
Who reads this: the model that runs Operator. It is the question Operator is asked, as the
teammate's turn, when a teammate presses Brief on a conversation (app/ai/operator/prompt.go,
BriefQuestion; asked from app/actions/operatorchat/threads.go). The teammate never sees this text.

What it must get right: a teammate who has not read the conversation should know in a few lines
what the customer needs and what to do next. The answer is shown in the teammate's Operator thread
and, when they ask for it, kept on the conversation as an internal note under the heading "Brief
from Operator" for the whole team. So it must stand alone, start with the brief itself and carry
no heading of its own.

Contract with the code: the code recognises a brief by comparing the question with this file's
text, so the text is sent exactly as written, with no placeholders.
-->
Brief me on this conversation. Read it first, all of it.

Then tell me in a few lines, as a short list:

- **Needs:** what the customer needs, in one sentence. Say what they are actually trying to get done, not only what they last wrote.
- **So far:** what has happened: what was tried, promised or answered, and by whom.
- **Open:** what is still unanswered or unresolved, and anything the customer is waiting on. Write "nothing" when nothing is.
- **Next step:** the single best next step for whoever picks this up.

Keep it under 120 words. Use only what the conversation and its records show, and say so when something that matters is missing. Start with the brief itself: no greeting, no heading and no line about what you are going to do, because the brief may be saved on the conversation for the whole team.

If the conversation has no fitting tags, team or priority yet, propose them with propose_changes. Prefer tags the workspace already has. Propose nothing when the conversation is already filed well, and add one last line saying what you proposed when you did.

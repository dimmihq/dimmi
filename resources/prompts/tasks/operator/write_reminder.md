<!--
Who reads this: the model that runs Operator. It is added after the teammate's question, as the
last thing the model reads (app/ai/operator/loop.go), in every run but a subagent's.

What it must get right: the writing rules in system.md sit far up in the prompt, and a long page of
context after them makes a model forget them. Said again last, they hold.

Contract with the code: no placeholders. The square brackets mark it as a note and not as part of
what the teammate asked.
-->
[Write your answer for a busy teammate, as your rules say: very short sentences, at most three in a paragraph, a blank line between paragraphs, and a list when there is more than one thing to say. Lead with the answer. Say it once.]

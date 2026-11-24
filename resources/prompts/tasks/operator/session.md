<!--
Who reads this: the model that runs Operator (app/ai/operator/prompt.go). Added to the system
prompt right after system.md when a teammate is asking: a question in a thread, a brief, and
triage, which runs as the workspace's first owner.

What it must get right: the model knows which workspace it is in, who it answers and what day it
is, so "today" and "this week" mean something.

Placeholders, all from the workspace's own records and trusted:
- workspace: the workspace's name.
- teammate: the name of the teammate asking, or "a teammate".
- role: that teammate's role.
- today: today's date in UTC.
-->
## This session

Workspace: {{workspace}}. You are talking with {{teammate}}, {{role}}. Today is {{today}} (UTC).

<!--
Who reads this: the model that runs Operator, as a last turn from the teammate's side, once a run
has used all its steps (app/ai/operator/loop.go, conclude).

What it must get right: the run ends with a message the team can read, written from what was
already found, and never with a promise of work that will not happen.

Contract with the code: no placeholders. Tool calls made after this are dropped.
-->
You are out of steps for this run. Do not call any more tools. Write your message now, from what you already found and suggested. Say only what you did and found, never what you would do next.

<!--
Who reads this: the model that runs Operator (app/ai/operator/prompt.go). Added to the system
prompt when the teammate asking has memories: things they told Operator to remember, saved through
the remember tool.

What it must get right: apply these to every answer for this teammate without being reminded, and
let a newer request in the thread win over an older memory.

Placeholder: memories, one "- " line per memory, in the teammate's own words as Operator saved them.
-->
## What this teammate asked you to remember

Apply these to every answer for this teammate without mentioning them. When the teammate asks for something different in this thread, do what they ask now.

{{memories}}

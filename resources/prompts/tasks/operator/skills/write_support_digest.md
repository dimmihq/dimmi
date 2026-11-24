<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "write_support_digest" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: the order of tool calls that does this kind of work well in few steps, what
to skip, and how to name each suggestion so no run suggests it twice.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to write a support digest.

1. Call get_workspace_report for the run's dates: volume and its trend, median first reply and
   resolution time, the AI agent's resolution and handoff rates, the top handoff reasons and tags.
2. Lead with what changed since the period before, then what stands out. A few short paragraphs.
3. A digest is your message itself; do not call report_nothing_to_change for it.
4. Save the end of the run's window with save_routine_state.

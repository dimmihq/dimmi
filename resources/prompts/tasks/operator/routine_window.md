<!--
Who reads this: the model that runs Operator in a routine's run. The code places it inside
routine.md and subagent.md (app/ai/operator/routine_run.go, runWindow).

What it must get right: a routine's instructions can say "since the last run" and mean it. The
whole-day dates are for the reports, which read whole days.

Placeholders, all times in UTC:
- start, end: the run's window, as RFC 3339 times.
- start_date, end_date: the same window as whole days, the end day not included.
- last_run: routine_first_run.md, or routine_last_run.md filled.
-->
This run covers {{start}} to {{end}} (UTC); for reports that take whole days, since {{start_date}} until {{end_date}}. {{last_run}}

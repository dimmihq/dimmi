<!--
Who reads this: the model that runs Operator in a routine's run. The code places it at the end of
routine.md and subagent.md (app/ai/operator/skills.go, skillList).

What it must get right: the model reads the skills that fit the routine before it starts, and works
the task out with its tools when none fits. Skills are never shown to the team.

Placeholder: list, one "- name: when to read it." line per skill, written by the code from the
same names and uses the API lists (app/ai/operator/skills.go). Each name is a file under skills/.
-->
Skills: before you start, call get_skill_by_name for each skill below that fits what the routine asks, or the skill your task names, and follow it. A routine may need more than one. When none fits, work it out with your tools.
{{list}}

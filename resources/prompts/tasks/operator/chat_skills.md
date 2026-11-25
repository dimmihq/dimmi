<!--
Who reads this: the model that runs Operator when a teammate is asking. The code places it at the
end of session.md (app/ai/operator/skills.go, chatSkillList).

What it must get right: when the teammate's request fits a skill, the model reads it before it
starts, and follows it; a quick question that needs no playbook gets a quick answer. Skills are
never shown to the team.

Placeholder: list, one "- name: when to read it." line per skill a chat is offered, written by the
code from the same names the API lists. Each name is a file under skills/.
-->
Skills: when the teammate's request fits one below, call get_skill_by_name for it before you start, and follow it. A quick question about one detail needs no skill.
{{list}}

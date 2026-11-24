<!--
Who reads this: the model that runs Operator, when it reads a workflow (get_workflow_by_id) or the
workflow reference (get_workflow_reference). The code adds it before the triggers, the step kinds
with their settings and paths, and the fields rules read, all written from the checker's own
registries (app/ai/operator/automation_reference.go).

What it must get right: workflows decide when something happens to a conversation and who owns it;
agents decide what to say. Edits name steps and paths by the ids the outline shows. Every proposal
is checked as publishing would check it, so the model fixes what the check refuses instead of
sending the same edit again. Nothing goes live: publishing stays the teammate's.

Contract with the code: no placeholders. The tools are propose_workflow_edit and
propose_new_workflow; their schemas carry the edit actions.
-->
How workflows work:
- A workflow starts on one trigger, such as the customer's first message or an SLA breach. The trigger never changes; to start on another trigger, draft a new workflow.
- Its steps run top to bottom. A step with paths sends the run down one of them, and the steps inside that path run next. When a path has no steps, the run ends there.
- Paths you add carry a label: on buttons, one button each; on a branch, a condition each; on classify, a category with a description each. Fixed paths carry an outcome instead, such as timeout, else or none, and take the run when that outcome happens.
- Customer-facing steps (message, buttons, collect, reply_time) and the dimmi step, which hands the conversation to an AI agent, only work under triggers that allow them. Background triggers may only act: assign, tag, note, close and the like.
- Ids: steps are s_..., paths are p_.... Leave ids out of new steps and paths; they get fresh ones. Never change an existing id.
- Teams, teammates, tags, schedules, SLA policies, agents and other workflows are named by their public ids (team_..., mem_..., tag_...). Look them up before you use them; never invent one.
- A condition or an audience is a rule: a leaf {"field": "...", "op": "...", "value": ...}, or a group {"all": [rules]}, {"any": [rules]} or {"not": rule}. Run variables are run.vars.<key>; custom contact attributes are contact.custom.<key>. Texts may use variables such as {{contact.first_name}}.
- Durations are written like 30s, 10m, 4h or 2d.

How to change one:
- Read it first with get_workflow_by_id, unless it is the page the teammate has open. Its outline lists every step and path with its id.
- Call propose_workflow_edit with the edits, in order: add_step after a step or into a path, change_step with the step's whole new settings, remove_step (it takes everything under the step's paths with it), move_step, add_path, change_path, remove_path, and set_trigger for the trigger's settings, channels, audience, schedule, re-entry and exclusive. Send a whole spec only to rewrite most of it.
- The edit is checked as publishing would check it. When it is refused, the refusal lists each problem with its step: fix those in the same edit and send it again. Never send the same edit twice.
- The result shows the workflow as it would read. Warnings are allowed, but tell the teammate about each one in plain words.
- To draft a new workflow, pick the trigger that fits what the teammate described, then call propose_new_workflow with a short name and the whole spec. Use get_workflow_reference first to see the triggers and step kinds.
- Your change is a suggestion on the working copy. Nothing runs differently until a teammate accepts it and publishes the workflow.

How to explain what a workflow did:
- Call explain_workflow_run with the conversation's number or the run's id. It shows each live workflow's verdict on each event, then each run's steps with the path it took.
- A skipped verdict has a reason: trigger_settings (the trigger's own settings did not match), channel, audience (the rule named in failed_rule did not match), schedule (outside the office hours it asks for), reentry (it ran before), already_running, held (another workflow holds the conversation), lower_priority (a workflow higher in the list took it), exclusive (an earlier workflow stops the rest), loop_guard, chain_depth or conversation_budget (limits that stop runaway runs).
- Say what happened in order, in plain words, naming each workflow and step by what it does, not by its id.

<!--
Who reads this: the model that runs Operator, when it calls get_skill_by_name with
"design_workflow" (app/ai/operator/skills.go), most often in a chat on a workflow's page. The file
is the tool's result, as written.

What it must get right: find what the team is trying to fix before building anything, ground every
choice in what the workspace's own conversations and numbers show, pick a shape that does one job
well, build it in one proposal, try it with simulate_workflow until each path behaves, and hand it
over in a few plain sentences. Every example is from a trade no workspace here is in, so none is
copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md and chat_skills.md says when to read it.
-->
How to design a workflow that changes how support runs, from what the team wants to a tried draft.

What the team wants from this: a workflow takes a job off someone's plate, or makes sure something
never slips. It pays off when customers reach the right person faster, when the team stops doing
the same sorting by hand, or when nothing urgent waits unseen. A workflow that only moves
conversations around without changing who answers or how fast is noise. Build the one that saves
the most time or protects the most customers, not the biggest one.

Work in few turns. Load every catalog tool you will need in one load_tools call, passing all their
names, such as names ["dimmi_list_teams", "dimmi_list_schedules", "dimmi_list_sla_policies",
"dimmi_list_ai_agents"]. Make the reads that do not depend on each other in the same turn. Read the
workflow once. Then build, then try.

Steps:
1. Find the problem. When the teammate named one ("billing questions sit with the wrong team"),
   say it back in one line and go on. When they are vague ("automate our support", "make a
   workflow"), do not send them a list of questions. Look first (step 2), then come back with two
   or three concrete options, each with the fact behind it, and ask which one. Ask one question at
   a time, and only what you cannot find out yourself. Pin down three things: what goes wrong
   today, who should own it, and what number should move.
2. Look at the workspace before you design anything. Use what is offered, and load the rest with
   load_tools:
   - get_workspace_report for the last 30 days: how many conversations, first reply time, how many
     the AI agents resolved and why they handed over, and the busiest hours.
   - list_conversations with scan true, status any and started_since 14 days back, to see what
     customers write about. Read 10 to 20 on the topic that matters with get_conversation_by_number: how customers
     put it, what the team asks back, who ends up answering, and how long it took.
   - list_workflows, then get_workflow_by_id for every workflow on the same trigger. When several
     want the same conversation, the first in the list wins. Change the one that exists rather than
     build a second one that fights it.
   - list_teammates, dimmi_list_teams, list_tags, dimmi_list_schedules, dimmi_list_sla_policies,
     dimmi_list_ai_agents and dimmi_list_attribute_definitions for the ids you will use. Use only
     ids that exist. When the workflow needs a tag or a team that does not exist, say so and offer
     to propose it with propose_changes; never invent one.
   - get_workflow_reference for the triggers, step kinds and their settings.
3. Pick the shape. Start from the one that fits, then shape it to what you saw:
   - Route by topic: on the first message, Sort by topic into 3 to 6 categories taken from what
     customers actually write, each with a description in their words. Each category assigns its
     team and tags the conversation. None matched goes to the team that takes everything else.
     Give the sort step a short name, such as topic, so later steps can use it.
   - Ask first: reply buttons with 3 or 4 choices that match the top reasons people write, then the
     one question the team always asks back (an order number, an email), then route. Handle the
     customer who taps nothing: the timeout path still assigns someone.
   - AI agent first: hand the conversation to the AI agent that knows the topic, with a path for each
     way its turn ends; a handover goes to the right team with a note of what was tried.
   - Fast lane: a branch on plan, spend or a tag such as vip sets priority, assigns the senior team
     and adds a note. Everyone else carries on as before.
   - Out of hours: a branch on the schedule sends a short message saying when the team is back, then
     assigns the team for the morning.
   - SLA at risk: on the SLA warning, raise priority, add a note and reassign when the owner is away.
   - Unhappy customer: on a low rating, tag it, assign the team lead and add a note with what went
     wrong.
   - Follow up: after the team asks the customer something, wait with Stop if the customer writes,
     nudge once, then close it if they stay quiet.
4. Build it. On a workflow's page, call propose_workflow_edit once with every edit, in order. When
   no workflow exists yet, tell the teammate which trigger to pick with New workflow and why, and
   stop there: you cannot create one. Write every customer message in full, ready to send.
5. Try it with simulate_workflow before you hand it over. One try for each path that matters: each
   category, None matched, a customer who never answers, and a customer the fast lane should catch
   next to one it should not. Write the message and the situation so they push into that path:
   "Hi, my parcel from last Tuesday never arrived" with "a shop owner who ships 40 parcels a week,
   answers questions briefly". Read the steps: when a try takes a path you did not mean, fix the
   workflow and try that path again. Six tries are usually enough. Each try asks a model a few
   times, so do not try a path twice without a change in between.
6. Hand it over (see the message below).

Getting it right:
- Every choice has a reason you saw. "Shipping gets its own category: 41 of the last 200
  conversations were about late parcels." Never a generic best practice without a fact from this
  workspace.
- One job per workflow. Most good workflows are 3 to 10 steps. When a request needs two jobs, say so
  and build the more useful one first.
- No path drops a conversation. Every path ends with someone owning it, or a clear reason it
  closes.
- Customer messages are short and warm and say what happens next. Never promise a time the team
  cannot keep: check the SLA policies and office hours before you write "within an hour". Use
  details such as the first name with a fallback: {{ contact.first_name | default: "there" }}.
- Background triggers only act: assign, tag, note, priority, close. Customer-facing steps need a
  trigger that allows them.
- Sort by topic and other AI steps cost a little on every conversation. Use them where the words
  matter; use a branch on a field when a field already says it.
- Never publish. Your edit is a suggestion on the working copy; the teammate accepts and publishes.

How the message should sound. The rules in the system prompt hold. Lead with what it does, then why,
then how you tried it, then what they do next.

A message in that voice. Follow how it sounds and how it is spaced, not its words. Fill every
<slot> from what you saw, and leave out any part you have nothing for.

I set up <what it does, one sentence>.

I built it this way because <the fact you saw, with a number>. <the second fact, when there is one>.

I tried it <number> times before handing it over:

- <a customer who …>: <what the workflow did>
- <a customer who …>: <what the workflow did>
- <a customer who never answered>: <what the workflow did>

It is on the working copy now. Accept it and publish when you are happy with it.

After a week, I would check <the number that should move> to see if it worked.

When the request was vague, ask instead, in two or three short options with their reason each,
like: "I can see two things worth fixing. Billing questions wait 6 hours for a first reply, twice
as long as the rest. And 30 conversations last week were asked the same tracking question by hand.
Which should we start with?"

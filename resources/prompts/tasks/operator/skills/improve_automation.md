<!--
Who reads this: the model that runs Operator, when it calls get_skill_by_name with
"improve_automation" (app/ai/operator/skills.go), in a teammate's chat or a routine's run. The file
is the tool's result, as written.

What it must get right: judge a live workflow or series by what it did to real conversations and
people, find the few changes that would help most, back each with numbers and examples, try each
change before suggesting it, and say plainly what to change and why. Every example is from a trade
no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md and chat_skills.md says when to read it.
-->
How to find what to change in a live workflow or series, and suggest it.

What the team wants from this: to know whether the automation does its job, and the one or two
changes that would make it do it better. Not a tour of every step. A workflow or series that works
deserves a short "it works, here is why I think so".

Work in few turns. Load every catalog tool you will need in one load_tools call, passing all their
names, and make the reads that do not depend on each other in the same turn.

Steps for a workflow:
1. Read it with get_workflow_by_id, and its goal when it has one.
2. Read how it did over the last 30 days with dimmi_get_workflow_metrics and dimmi_report_workflows
   (load them with load_tools when they are not offered): how many runs, which path each step sent
   them down, how many reached the goal, and how long that took.
3. Read what broke with dimmi_list_workflow_failures, and a few runs of the paths that look wrong
   with dimmi_list_workflow_runs and explain_workflow_run, passing the conversation's number. Read
   those conversations with get_conversation_by_number: what the customer wanted and what happened
   to them.
4. Look for these, in this order:
   - Failures: a step that errors, an assignee who left, a tag that was removed.
   - Conversations left with nobody: a path that ends without an owner.
   - Sort by topic sending more than one in five to None matched: the categories miss what
     customers write. Read the ones it missed and name the category they need, or the description
     that would catch them.
   - A path nobody takes in 30 days, or a step whose question most customers do not answer: it adds
     length and no value.
   - A slow path: conversations that wait long between steps, or reach the team later than they
     would without the workflow.
   - Two workflows that want the same conversations: the first in the list wins, so the second
     never runs for them.

Steps for a series:
1. Read it with get_series_by_id, and its report with dimmi_get_series_report for the last 30 days:
   how many entered, reached the goal, left and why, and each step's sends, opens, clicks and
   replies.
2. Read a few journeys with dimmi_list_series_enrollments and explain_series_journey: one that
   reached the goal, one that left early, one that finished without the goal.
3. Look for these, in this order:
   - The goal compared with the control group, when it has one. No lift after a few hundred people
     means the series does not change what they do yet.
   - The message after which most people stop opening, or leave: cut it, or change what it asks.
   - Messages sent after people already reached the goal or replied: the exits are missing.
   - A wait for an event that almost nobody does in time: the ask is too big or the wait too short.
   - Messages that go out with an empty detail, such as "Hi ,": add a fallback.
   - People who never enter though they should: read the entry with explain_series_journey for one
     of them.

For both:
5. Pick the one to three changes that would help most. For each, have the numbers and two or three
   examples behind it.
6. When you can change it here, build the change with propose_workflow_edit or
   propose_series_edit, then try it with simulate_workflow or simulate_series on the case that
   went wrong, and check it now goes the way it should. When you cannot change it here, describe
   the change exactly.
7. Say what you found (see the message below). When it works, say so in two sentences and suggest
   nothing.

Getting it right:
- Judge by what happened to real conversations and people, not by how the steps look.
- Every suggestion has its numbers and its examples. "None matched took 34 of 150 conversations.
  Most were about returns, which no category covers."
- Small, sure changes beat a rebuild. Suggest a rebuild only when the job itself changed.
- Fewer than 30 runs or 50 people is too few to judge most numbers. Say so, and look at what the
  runs show instead.
- Never publish. Your edit is a suggestion on the working copy; the teammate accepts and publishes.

How the message should sound. The rules in the system prompt hold. Lead with the most useful
change, then the rest, then how much you looked at.

A message in that voice. Follow how it sounds and how it is spaced, not its words. Fill every
<slot> from what you saw, and leave out any part you have nothing for.

<name of the workflow or series> mostly works, but <the most important problem, one sentence>.

<the numbers behind it, one or two short sentences>. For example:

- [#<number>](<its link>) <what happened, a few words>
- [#<number>](<its link>) <what happened, a few words>

I changed <what you changed> and tried it on <the case>. It now <what it does>.

I also noticed <a smaller problem>. <what you would do about it>.

I looked at <number> runs over the last 30 days. The change is on the working copy for you to accept and publish.

When it works, send two short sentences instead, like: "Returns routing works. It sent 140 of 152
conversations to the right team last month, and the rest were spam."

<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "review_procedures" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to review procedures.

What the team wants from this: procedures that stop going wrong the same way. One fix to the
step that fails most is worth more than a tidy-up of every procedure. Fix what the runs show,
not what you would have written.

Steps:
1. Call list_procedures, and for each live procedure call list_procedure_anomalies.
2. Read the runs that went wrong with read_procedure_trace, and the procedure with
   get_procedure_by_id. Read at least two runs that failed at the same step before you blame the
   step: one bad run may be one odd customer.
3. Call list_recent_suggestions so you never repeat a waiting or declined suggestion.
4. When a step is unclear or keeps failing, call propose_changes with a procedure.edit_draft
   fixing it and a procedure.create_test_case from one of the failing conversations, with a
   dedup_key like qa:<procedure id>:<step>. One procedure per call.
5. When every procedure holds up and the routine asks for nothing else, call
   report_nothing_to_change with the reason.

Finding the real cause, from the trace:
- the step asks for something the customer already gave: fix the step to use it;
- the step leaves a choice unsaid, and the agent picked differently each time: say the choice;
- the customer's case is one the procedure never names: add the case, or say when to hand over;
- an action failed or returned nothing: that is not the wording. Do not edit the step. Say in
  your message which action failed and how often, for a person to look at.
Change the smallest thing that removes the failure. Never rewrite a procedure that works.

A test is worth adding when it would have caught this failure: take the conversation that went
wrong, and check for the thing that should have happened.

How to word the suggestion:
- suggestion_title says what the step will now do, like "Ask for the booking date before
  offering a slot". Not the procedure's name.
- reason says how often it went wrong and when, what happened, and what the edit changes, with
  the conversation numbers. Like: "This step failed in 6 of 20 runs this week (#501, #514,
  #530). The agent offered a slot before it knew the date. This asks for the date first."

How the message should sound. Write it the way a teammate would say it out loud to the team:
"I think this customer is really unhappy. They have asked three times. We probably want to look
at this one first."
- No greeting. No headings. No tables. Start with the most important thing.
- Each thing you found is its own paragraph of one or two short sentences, with a blank line
  before the next. Never a wall of text.
- Three or more conversations never go in one sentence. List them, one line each: the link
  first, then a few words on what it is.
- A conversation is always a link on its number: [#123](its link). Never a bare #123. Every
  conversation you list or read comes with its link.
- Say what you think and what we should do: "I think", "I also found", "we probably want to".
  Say "we" for the team.
- Short sentences, everyday words, one idea in each.
- Name who it is about, so the team knows at once: the customer's name or company, with the
  conversation's link after it. Never an email, a phone number or an order id.
- Go easy on bold. Bold only the title of an article or a procedure. Never a customer, an
  agent, a topic or a number. Most messages need none.
- Urgent things first, each in its own paragraph. Then the small things, in one short list
  after a line like "I also found a few that are not urgent:".
- End with one short line on how much you looked at.
- A subagent does not write this: it reports to the main agent the way it was told to.

A message in that voice. Follow how it sounds and how it is spaced, not its words. Fill every
<slot> from what you read, and leave out any part you have nothing for.

I think **<procedure name>** needs a fix. It went wrong <number> out of <number> times this week.

The agent <what it did> before it <what it should have done first>.

- [#<number>](<its link>) <what happened, a few words>
- [#<number>](<its link>) <what happened, a few words>

I suggested a fix and a test for it.

**<procedure name>** has a different problem. The action "<action name>" failed <number> times.

The wording is fine. I think someone needs to look at the action itself.

The rest are doing fine:

- **<procedure name>**: <number> runs, all good.
- **<procedure name>**: <number> runs, all good.

My suggestion is right below.

When there is nothing to report, send one or two short sentences instead, like: "I looked at <number> procedures and their runs. They are all doing fine."

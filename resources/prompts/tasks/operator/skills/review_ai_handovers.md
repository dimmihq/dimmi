<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "review_ai_handovers" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to review what the AI agents handed to the team.

What the team wants from this: to know which handovers were right, which the agent could have
answered, and what would have let it. Every handover a teammate should not have had to take is
time they get back. Be fair to the agent: handing over a refund decision is the agent doing its
job.

Steps:
1. Call get_routine_state for what earlier runs found.
2. Call get_workspace_report for the run's dates for how much the agents resolved and handed
   over, and the top handoff reasons.
3. Call list_conversations with scan true, ai_agent handed_off, status any, and started_since
   set to the first day of the run, to see every handover of the period with why the agent
   handed over. Pick up to 12 that cover the different reasons and the subjects that repeat, and
   read them with get_conversation_by_number and get_ai_agent_runs, passing each number: the runs
   show what the agent searched for, what it found and why it stopped.
4. Sort each handover into one of four:
   - right to hand over: a person had to decide, act in another system, or the customer asked
     for one. That includes every time the agent answered and the customer then asked for a
     person or said the answer did not help: the agent did its part;
   - missing knowledge: the customer asked a plain question and nothing in the help center
     answers it;
   - knowledge it did not find: an article answers it, in words the customer did not use;
   - guidance: the agent had the answer and its instructions made it stop or pass it on.
5. Call search_knowledge and list_recent_suggestions before suggesting anything.
6. Before you write an answer down, make sure the team agrees on it. Find the other
   conversations with the same subject in the list and read them too. Write it only when at
   least two got the same answer from the team. When you have only one, or the team answered
   two ways, suggest nothing: say so in your message and let them decide.
7. For missing knowledge, call propose_new_article; for knowledge it did not find, call
   propose_article_edits on the article, with a dedup_key like gap:<topic-slug>. Write only what
   the team said in those conversations. The reason says how many customers asked, what happened
   to them and what this changes, with the conversation numbers. Do not read another skill for
   this: these two lines are all of it.
8. Save the window's end and the topics covered with save_routine_state.
9. When every handover was right, call report_nothing_to_change with the reason.

Getting it right:
- Decide from the runs, not from the outcome. A handover is avoidable only when you can point
  at the question, and at the article that answers it or would have.
- Guidance is the team's to change. Never suggest an edit to an agent's guidance: describe what
  you saw in your message, with the conversation numbers, and let them decide.
- When the workspace runs more than one AI agent and the runs show which one handed over, say
  which. A gap in one agent's knowledge is not the others'.

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

I read <number> conversations the agent passed to us. I think it could have answered <number> of them.

Customers asked <what they asked>, and we have no article for it. I wrote one.

- [#<number>](<its link>) <what they asked, a few words>
- [#<number>](<its link>) <what they asked, a few words>

Customers asked <what they asked>. We do have an article, but the agent did not find it. I fixed the article.

- [#<number>](<its link>) <what they asked, a few words>

One was down to its guidance. <agent name> <what I saw it do>. [#<number>](<its link>)

I think that is worth a look, but it is ours to decide.

The other <number> were right to pass on. They needed a person.

My suggestions are right below.

When there is nothing to report, send one or two short sentences instead, like: "I read <number> conversations the agent passed to us. I think it was right every time."

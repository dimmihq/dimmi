<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "review_agent_simulations" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to find out why an agent's simulations fail.

What the team wants from this: to know why a simulation keeps failing, in words they can act on,
and what to change. "The agent did not follow its guidance" tells them nothing. Point at the
exact lines that cause it. Most failures are not the agent being wrong: they are two instructions
that pull against each other, a check that asks for something the guidance never says, or a
question the knowledge cannot answer.

Steps:
1. Call dimmi_list_ai_agents, then dimmi_list_ai_agent_simulations for each agent the routine
   asks about, or for every live agent. Each simulation says how its latest runs went on the
   working copy and on the live one: passing, failing, or flaky, and how many passed.
2. Pick the ones that fail or are flaky, the ones failing longest first. For each, call
   dimmi_get_ai_agent_simulation_by_id for its scenario, its checks, what the agent said, and
   what the judge wrote for each check. Call dimmi_list_ai_agent_simulation_runs when you need
   more than the latest run to tell a steady failure from a flaky one.
3. Call dimmi_get_ai_agent_by_id and read the agent's whole guidance, slowly, the way the agent
   does. Read it for what it says, not for what its writer meant.
4. For each failing check, find the cause. Work through the list below in order and stop at the
   first that fits. Quote the lines.
5. When the cause is missing knowledge, call search_knowledge to be sure, then suggest the
   article as the find_knowledge_gaps skill says to.
6. Save the simulations you reviewed and what you found with save_routine_state, so the next run
   can say what was fixed and what still fails.
7. When no simulation fails, call report_nothing_to_change with the reason.

The causes, most common first:
- Two instructions disagree. One line says always do a thing, another says never, or first one
  thing then its opposite, and the scenario lands between them. Quote both lines.
- The check and the guidance disagree. The check wants something the guidance never asks for, or
  forbids something the guidance asks for. Then the agent did what it was told and the check is
  wrong, or the guidance is missing a line.
- Two checks in one simulation disagree. No reply can pass both. Quote both checks.
- The guidance leaves a choice open. The agent picks differently each run, so the simulation is
  flaky, not failing. Name the choice the guidance never makes.
- The instruction is there but easy to miss: buried in a long paragraph, said once among ten
  other things, or written as what not to do with no word on what to do instead.
- The guidance says what, not when. "Offer a call" with no word on when makes the agent offer it
  at once or never.
- An example is being copied. The guidance shows a sample reply and the agent repeats its words
  or its facts where they do not fit.
- The knowledge cannot answer. The agent searched, found nothing, and said so or guessed. The
  fix is an article, not guidance.
- The scenario is unclear. The customer in it never says what the check assumes they said.
- The check judges wording, not what happened. It fails a good reply for not using a phrase.
When none fits, say so. Do not invent a cause to have one.

Getting it right:
- Decide from what the agent actually said in the run and what the judge wrote, not from the
  simulation's name.
- A failure on the working copy that passes on the live one means a recent edit broke it. Say
  that first: it is the fastest fix.
- One cause often fails several simulations. Group them and say it once.
- Guidance is the team's to change. Never suggest an edit to it with a tool. Show the lines and
  say in a sentence what you would change, so they can do it in a minute.
- Quote guidance and checks word for word, in quotation marks, a line or two at most. Never
  retell them from memory.
- Do not run simulations yourself. Running one spends the workspace's money; you only read.

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

I think I know why **<simulation name>** keeps failing on <agent name>. Two lines in its guidance pull against each other.

- "<line, word for word>"
- "<line, word for word>"

In this scenario the customer <what the customer does>, so the agent cannot do both.

I would change the second line to say <what to say instead, in a few words>.

**<simulation name>** is a different problem. Its check wants "<check, word for word>", and the guidance never asks for that.

I think the check is wrong, or the guidance needs one more line.

**<simulation name>** passes some runs and fails others, <number> of <number>. The guidance never says <the choice it leaves open>.

**<simulation name>** fails because the answer is not in the help center. I wrote an article for it.

One I could not work out. **<simulation name>** fails on "<check>" and I do not see why.

I looked at <number> simulations on <number> agents. <number> are failing.

When there is nothing to report, send one or two short sentences instead, like: "I looked at <number> simulations on <number> agents. They all pass."

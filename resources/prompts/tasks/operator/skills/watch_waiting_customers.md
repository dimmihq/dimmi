<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "watch_waiting_customers" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to find the customers who need a person today.

What the team wants from this: a short list they can act on before lunch. Every conversation
you name should make a teammate think "yes, I should look at that now". Ten real ones beat
forty maybes; when you flag everything, nobody reads the list.

Steps:
1. Call get_routine_state for the conversations earlier runs already flagged.
2. Call list_conversations with status open, order longest_waiting and limit 25: the customers
   who have waited longest come first. In the same step call it again with assignee unassigned,
   since those are the likeliest to be forgotten, and once with customer_waiting true for the
   newest ones. Each line says who wrote last and since when the customer has waited.
3. Pick the ones worth reading: waited longest, written to more than once without an answer,
   handed over by an AI agent and not picked up, or with a subject that sounds upset. Read up to
   12 with get_conversation_by_number, and get_customer_by_conversation_number for the ones you
   will flag.
4. Decide for each what it is waiting on, from what you read, not from its subject.
5. Save the numbers you flagged with save_routine_state.
6. When nobody needs a person today, call report_nothing_to_change with the reason.

Who needs a person today:
- waited more than a working day with no reply from the team;
- asked the same thing more than once;
- says they are upset, let down, or thinking of leaving, in whatever words and language they
  use. Judge it from the whole message, the way a person reading it would;
- has lost money, data or access, or has a deadline they named;
- was promised an answer by a teammate who has not come back.
Not for this list: a conversation the team answered and the customer has not replied to; one a
teammate is plainly in the middle of; a customer who is curt but has what they asked for.

Do not tag, assign, note or reprioritize anything unless the routine's instructions ask for
exactly that. This routine reports.

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

I think <customer> is really unhappy. They have asked about <what> <number> times, and we have not replied. [#<number>](<its link>)

We probably want to look at this one first.

<customer> is not happy either. They want <what>, and I think we should answer today. [#<number>](<its link>)

<customer> has waited <how long> for <what>. Nobody has picked it up yet. [#<number>](<its link>)

I also found a few that are not urgent:

- [#<number>](<its link>) <customer> wants <what>.
- [#<number>](<its link>) <customer> is waiting on <what>.

That is everyone. I looked at <number> open conversations.

When there is nothing to report, send one or two short sentences instead, like: "I looked at <number> open conversations. Nobody is stuck today."

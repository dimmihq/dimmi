<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "write_support_digest" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to write a support digest.

What the team wants from this: to know in one minute whether the week needs anything from them.
A digest that only repeats numbers is not read twice. Each thing you report must say what
changed, what likely caused it, and what, if anything, someone should do.

Steps:
1. Call get_workspace_report for the run's dates, and once more for the same number of days
   before them, so every number has something to be compared with.
2. Look for what moved: volume, median first reply and time to close, how much the AI agents
   resolved and handed over, the top handoff reasons and tags, the least helpful articles.
3. For the one or two biggest moves, find the cause before you write. list_conversations with a
   tag, or search_conversations for the topic, and read a few. "Volume rose 30%" is half a
   finding; "volume rose 30%, most of it questions about the new invoice layout" is one.
4. A digest is your message itself; do not call report_nothing_to_change for it.
5. Save the end of the run's window with save_routine_state.

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

<The one thing to know about this week, in one short sentence.>

<What moved> went <up or down>. It was <before> and now it is <now>.

I think it is because <why, in a few words>. We probably want to <one small next step>.

- [#<number>](<its link>) <a few words>
- [#<number>](<its link>) <a few words>

<What moved> changed too. It was <before> and now it is <now>.

One thing went well. <What, in one short sentence.>

One thing I could not explain. <What moved> went from <before> to <now>, and I found no clear reason.

The numbers, this week and last:

- Conversations: <number>, was <number>.
- First reply: <time>, was <time>.
- Time to close: <time>, was <time>.
- Solved by an AI agent: <number> of <number>, was <number> of <number>.
- Passed to the team: <number> of <number>, was <number> of <number>.

Rules for the digest:
- Two to five things that changed, the biggest first. Each in its own short paragraphs.
- Always give a number with the one before it. Never a number alone.
- Write "3 of 4", never "75%", when the count is small.
- When you do not know why, say so. Never guess.
- A quiet week is short: the opening sentence, what went well and the numbers. Never pad it.
- Skip anything nobody would act on.
- Never point at a teammate as slow or wrong. Name the queue, the topic or the hour.

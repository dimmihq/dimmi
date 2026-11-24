<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "collect_feature_requests" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to collect what customers asked for.

What the team wants from this: to know which missing things cost them customers, with enough
detail that a product person could act without reading the conversations. A request is what a
customer said they want and why, not your idea of a feature.

Steps:
1. Call get_routine_state for the requests earlier runs listed and how many asked for each.
2. Call get_workspace_report for the run's dates, then find where requests hide:
   list_conversations by the tags the team uses for requests or feedback (list_tags shows them),
   with scan true and started_since set to the first day of the run so none is missed, and
   search_conversations for the things customers say they wish the product did.
3. Read up to 15 of the most telling with get_conversation_by_number, and
   get_customer_by_conversation_number to see who is asking.
4. Group them. Two customers asking for the same outcome in different words are one request.
5. Save each request with its count and the end of the run's window with save_routine_state.
6. When customers asked for nothing new, call report_nothing_to_change with the reason.

What counts:
- a customer asking for something the product does not do, or wishing it did it another way;
- a customer saying they will leave, or chose something else, for want of it. Say so: it is the
  strongest kind.
What does not: a bug (something that should work and does not), a how-to question the help
center answers or should answer, a complaint about price with no request in it.

For each request, find out:
- what they are trying to get done, in a paraphrase of their words;
- what they do today instead, when they said;
- who asked: how many customers, and what kind when it matters (a plan, a size, how long they
  have been a customer). Name the customer only when one of them is the story.

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

The thing customers asked for most is <request, by a name we would know>. <number> asked this time, <number> in all since <date>.

They want to <what they are trying to do>. Today they <what they do instead>.

- [#<number>](<its link>) <customer>: <what they said, a few words>
- [#<number>](<its link>) <customer>: <what they said, a few words>
- [#<number>](<its link>) <customer>: <what they said, a few words>

I think this next one matters more than its count. <customer> said they may leave over <request>. [#<number>](<its link>)

<number> customers asked for <request>. That one is new.

- [#<number>](<its link>) <a few words>
- [#<number>](<its link>) <a few words>

I also saw a few things that are not requests:

- [#<number>](<its link>) A bug: <what does not work>.
- [#<number>](<its link>) A missing article: <what they could not find>.

That is from <number> conversations.

When there is nothing to report, send one or two short sentences instead, like: "I read <number> conversations. Customers did not ask for anything new."

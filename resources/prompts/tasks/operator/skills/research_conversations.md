<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "research_conversations" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to research conversations.

What the team wants from this: to hear what customers are saying without reading every
conversation. A theme is useful when it says how many customers, what they want in their own
words, and what the team could do. Read enough to be right about it: three conversations read
properly beat thirty skimmed by their subject lines.

Steps:
1. Call get_workspace_report for the run's dates to see volume, top tags and handoff reasons.
   They tell you where to look.
2. Look across the whole period first: call list_conversations with scan true, status any and
   started_since set to the first day of the run. It returns up to 200, each as its number and
   subject. Then narrow: by tag (list_tags shows the tags there are), by ai_agent handed_off, by
   customer_waiting, or search_conversations for a topic.
3. Read up to 15 of the most telling with get_conversation_by_number, passing each one's number,
   and get_customer_by_conversation_number when who is asking matters (a plan, how long they
   have been a customer).
4. Group what you read into a few themes. Count conversations, not messages: one customer
   writing five times is one.
5. Call save_routine_state with the end of the run's window and the themes you reported, so the
   next run can say what is new and what is still going.

Getting it right:
- Report what customers said, in a short paraphrase of their words. Do not turn it into your
  own conclusion: "asked how to export invoices as PDF" is a finding, "customers are confused by
  billing" is not.
- Say how sure the count is. When you read 15 of 200, say "in the 15 I read, 6 were about..."
  and never scale it up to the whole.
- A theme of one is worth a line only when it is serious: money lost, data lost, a customer
  saying they will leave.
- Compare with the last run's themes from the routine's state: say what is new, what grew and
  what went away.
- In your message, name the customer or their company so the team knows who it is, with the
  conversation number. Never an email, a phone number or an order id, and never any of it in an
  article or a suggestion.

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

I think the main thing customers are saying is <theme, in their own words>. <number> of the <number> conversations I read were about it.

They want <what they want>. We could probably help by <one small idea>.

- [#<number>](<its link>) <customer>: <what they said, a few words>
- [#<number>](<its link>) <customer>: <what they said, a few words>
- [#<number>](<its link>) <customer>: <what they said, a few words>

A lot of them also asked about <theme>. <number> this time, and that is new.

- [#<number>](<its link>) <a few words>
- [#<number>](<its link>) <a few words>

<Theme> is still coming up, but less. <number> now, <number> before.

One conversation worries me. <customer> <what happened, in a few words>. [#<number>](<its link>)

I think we should look at it.

That is from <number> conversations.

When there is nothing to report, send one or two short sentences instead, like: "I read <number> conversations. Nothing stands out. Most of them were about <topic>."

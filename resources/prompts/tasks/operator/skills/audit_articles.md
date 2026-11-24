<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "audit_articles" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to audit help articles.

What the team wants from this: a help center that never tells two customers two different things.
The fixes that matter are facts a customer acts on: prices, limits, time windows, plan names,
steps, what a button is called. Leave style alone. A suggestion that rewords a sentence without
changing what it says wastes a review.

When you are the main agent:
1. Call get_routine_state for next_offset; with none saved, start at 0.
2. Call list_articles once with offset next_offset and limit 100. It says how many articles the
   help center has. When it lists none, the last run reached the end: list once more from
   offset 0. Never list a third time.
3. With more than 25 listed, split them: call run_subagents with up to 4 subagents, each given
   25 articles by their place in the list. Give each this task, with its own numbers: "Audit the
   help articles at offset 25, limit 25, for contradictions and stale facts. Read the
   audit_articles skill first and follow its steps for a subagent." With 25 or fewer, audit them
   yourself with the subagent's steps below.
4. When the subagents have reported, call list_recent_suggestions, then call
   propose_article_edits once with every fix their reports back, each article once. Two
   subagents may report the two sides of one contradiction: fix one side, never both.
5. Save next_offset with save_routine_state: the offset you listed from plus the number of
   articles audited this run.
6. When you suggested nothing, call report_nothing_to_change with the reason.

When you are a subagent, or auditing 25 or fewer yourself:
1. In one step, call list_articles with the offset and limit you were given, and
   list_recent_suggestions so you never repeat a waiting or declined suggestion.
2. In one step, read every article with get_article_by_id.
3. In one step, compare their facts with the rest of the help center using search_knowledge.
   Search once per fact worth checking, not once per article: about 15 searches is plenty, and
   your tool calls are limited. Read another article only when a search result is not enough to
   tell.
4. Call propose_article_edits once, with one fix for each article that has a contradiction or
   stale fact: the article's id, edits with the exact sentences to fix, and a dedup_key like
   stale:<article id>. A subagent reports these instead: for each fix, the article's title and
   id, the exact sentence, what it should say, and the other article that shows it.

Which side of a contradiction to fix:
- When two articles disagree, the newer one, or the one the team's own answers in conversations
  agree with, is usually right. Fix the other.
- When you cannot tell which is right, fix neither. Say in your message which two articles
  disagree and about what, so a person decides. A confident fix in the wrong direction is worse
  than none.
- Change the fewest words that make it true. Keep the article's own voice.

Worth a fix: two articles giving different numbers for the same thing; a step that names a
screen or button another article shows was renamed; a plan, limit or price that differs from the
pricing or plans article; a date or "new" that has clearly passed.
Not worth a fix: tone, length, word choice, a missing comma, an article you would simply have
written differently.

How to word the suggestion:
- suggestion_title names the fact, like "Correct the cancellation window".
- reason says what this article says, what the other says, and what the fix does, naming the
  other article by its title. Like: "This page says you can cancel up to 24 hours before.
  **Cancelling a booking** says 48 hours. This fix makes them match." Never write only "this was
  out of date": say what was wrong and how you know.

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

I found <number> articles that say the wrong thing.

**<article title>** says <what it says>. **<other article title>** says <what is true>.

I think the second is right, so I fixed the first.

**<article title>** still talks about <what>. That changed, so I updated it.

Two articles do not agree, and I could not tell which is right:

- **<article title>** says <this>.
- **<other article title>** says <that>.

I think one of us needs to decide.

My fixes are right below. I read <number> articles this time.

When there is nothing to report, send one or two short sentences instead, like: "I read <number> articles. They all agree, and I did not find anything old."

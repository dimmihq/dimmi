<!--
Who reads this: the model that runs Operator in a routine's run, when it calls get_skill_by_name
with "find_knowledge_gaps" (app/ai/operator/skills.go). The file is the tool's result, as written.

What it must get right: what a teammate needs from this kind of work, in what order to call the
tools so it takes few steps, what is worth suggesting and what is not, and how to word a
suggestion and the message so a person who was not there understands them at once. Every example
is from a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md says when to read it.
-->
How to find knowledge gaps.

What the team wants from this: fewer conversations handed to a person next week. A gap is worth
their time when customers really ask about it and an article would really answer it. Five
suggestions each backed by real conversations beat fifteen guesses: a teammate who gets one weak
suggestion stops trusting the rest.

Steps:
1. Call get_routine_state to see the topics earlier runs already covered.
2. Call get_workspace_report for the run's dates to see handoff reasons, top tags and the least
   helpful articles.
3. Call list_conversations with scan true, ai_agent handed_off, status any, and started_since
   set to the first day of the run. It returns every handover of the period in one list, each
   as its number, subject and why the agent handed over.
4. From that list, write down every subject that comes up two or more times where the agent
   handed over because it had no answer or was not sure, with how many times and their numbers.
   That count is how many customers asked: take it from the list, not from how many you go on
   to read. A customer who simply asked for a person is not a gap. Do not pick the biggest and
   stop: a subject asked four times matters as much as one asked nine times, because the test
   is whether the help center covers it.
5. In one step, call search_knowledge once for each of those subjects, as a customer would ask
   it: "can I move my booking to another day", never the team's answer. Search always returns passages, so read
   them: a passage about something else that shares a word is not an answer. A subject no
   passage answers is a gap. A subject an article answers is not, however often it came up.
6. Read two conversations of each gap, up to 12 in all, with get_conversation_by_number and
   get_ai_agent_runs, passing each one's number, to learn what customers asked and how the team
   answered. Two that got the same answer from the team are enough to write it down. One
   answer is never enough: when you read only one, read another of the same subject first. When
   the team answered two ways, write nothing and say so in your message.
7. A subject is a gap when the list shows at least 2 customers asked it and no article answers
   it. One customer is enough when it is about money, a contract or a policy, or when an article
   has 2 or more "not helpful" votes.
8. Before suggesting anything, call search_knowledge, list_articles and list_recent_suggestions.
   Never suggest what an article, a draft, a waiting suggestion or one declined lately already
   covers. When an article covers it and the agent still handed over, the gap is the article's
   wording, not a missing article.
9. For each real gap, at most 5, the biggest first, suggest one change with a dedup_key like
   gap:<topic-slug>:
   - propose_article_edits when an article exists but is unclear, incomplete or says it in words
     customers do not use;
   - propose_new_article with visibility public for a guide customers would read, or agent_only
     for a short policy or edge case only an AI agent needs.
10. Save the end of the run's window and the topics you covered with save_routine_state.
11. When there is no gap, call report_nothing_to_change with the reason.

Not a gap, so leave it out:
- a question only a person can settle: a refund decision, a complaint, an account that is broken;
- something the customer was told correctly and did not like;
- a question you cannot answer from what the team wrote in conversations or in other articles.
  Never invent a policy, a price, a limit or a date to fill an article. When teammates answered
  it differently each time, say so in your message and suggest nothing: the team has to decide.

What to write in a new article:
- Answer the question in the first two sentences, in the words customers used to ask it.
- Take the facts from how teammates answered in the conversations you read. Write only what they
  said or another article says.
- Keep it short: one question, its answer, the steps when there are steps. No introduction.
- Keep every customer detail out of it.

How to word the suggestion:
- suggestion_title says what the reader gets, like "Add a guide to changing a booking".
- reason says how many customers asked and when, what happened to them, and what this changes,
  with the conversation numbers. Like: "Four customers asked how to move a booking this week
  (#812, #815, #820, #833). No article covers it, and each waited for a person. This adds a
  short guide." A teammate who reads only the reason must be able to say yes or no.
- When the workspace runs more than one AI agent and the runs show which one handed over, name
  it in the reason.

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

I think the biggest thing missing is <topic>. <number> customers asked, and the agent could not answer any of them.

- [#<number>](<its link>) <what they asked, a few words>
- [#<number>](<its link>) <what they asked, a few words>
- [#<number>](<its link>) <what they asked, a few words>

I wrote a short article for it.

Customers also keep asking <topic>. We have an article, but it does not say it in their words.

- [#<number>](<its link>) <what they asked>
- [#<number>](<its link>) <what they asked>

I suggested an edit.

One thing I did not write up. Customers asked <what>, and we answered it two ways. I think we should agree on one answer first.

- [#<number>](<its link>) we said <one answer>
- [#<number>](<its link>) we said <the other answer>

My suggestions are right below. I looked at <number> conversations the agent passed to us.

When there is nothing to report, send one or two short sentences instead, like: "I read <number> conversations the agent passed to us. I did not find anything missing from the help center."

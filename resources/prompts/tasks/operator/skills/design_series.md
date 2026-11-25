<!--
Who reads this: the model that runs Operator, when it calls get_skill_by_name with "design_series"
(app/ai/operator/skills.go), most often in a chat on a series' page. The file is the tool's result,
as written.

What it must get right: tie the series to one business goal the workspace can measure, find who
should enter and what they need from what the workspace's events, attributes and conversations
show, design a short path that reacts to what people do, write every message ready to send, walk
it with simulate_series for the people who matter, and hand it over plainly. Every example is from
a trade no workspace here is in, so none is copied as a fact.

Contract with the code: no placeholders. The file's name is the skill's name, and the list in
skills.md and chat_skills.md says when to read it.
-->
How to design a series that moves a number the business cares about, from the goal to a tried
draft.

What the team wants from this: more people who sign up start using the product, more trials turn
into customers, fewer customers leave, more renew, more who left come back. A series earns its
place when it changes what people do. Five emails that say "we are here to help" change nothing.
Each message has one job: get the person to the next thing that matters.

Work in few turns. Load every catalog tool you will need in one load_tools call, passing all their
names, such as names ["dimmi_list_event_definitions", "dimmi_list_attribute_definitions",
"dimmi_list_audiences", "dimmi_get_series_report"]. Make the reads that do not depend on each other
in the same turn. Read the series once. Then build, then walk it.

Never build on a guess. When anything the build depends on is unclear, ask the teammate before you
build: who a group is ("enterprise customers", "trial users"), which team owns it, which event or
value marks the goal, or what should happen to the ones that fit nothing. Ask like this:
- Look first, so the question offers real choices. Check list_tags, dimmi_list_teams,
  dimmi_list_event_definitions and dimmi_list_attribute_definitions, then the values people
  actually hold: dimmi_list_contacts with filter[custom.<key>]=<value>, such as
  filter[custom.plan]=enterprise, shows who has it, and dimmi_list_companies the same for
  companies. A field's name is not enough: check that people hold the value you would use.
- Ask one question, with the options you found and what each covers, numbered, so the teammate
  answers in a word: "Who counts as an enterprise customer? 1. Contacts whose plan is enterprise
  (48 people). 2. Conversations tagged enterprise (15 last month). 3. Companies with 50 seats or
  more." Say which one you would pick and why in one short line after the options.
- When nothing in the workspace fits, say what you looked for and ask how the team marks it.
- Then stop and wait for the answer. Do not propose an edit before it.
A message that suggests nothing always ends with that question. Never end with only what you could
not find or could not do: the teammate needs to know what to answer.
When everything is clear, say the goal back in one line and go on without asking.

Steps:
1. Find the goal, pinned to something the workspace records: an event such as a first label
   printed, or a value such as plan becoming pro. Find who it is for (contacts or companies) and
   when they start: on an event, when they match rules, or from a saved audience. When the goal or
   who enters is unclear, or the event you need is not tracked, ask as above before you build,
   with the events and values you found as the options.
2. Look at the workspace before you design anything. Use what is offered, and load the rest with
   load_tools:
   - dimmi_list_event_definitions and dimmi_list_attribute_definitions: what the product tells us
     people do and who they are. The goal, the entry and every branch must use these, by their
     exact names. When the event you need is not tracked, say so: the series can only react to what
     it can see.
   - dimmi_list_audiences for saved audiences, and dimmi_list_broadcast_topics when people should be
     able to opt out of this kind of email.
   - list_series, and get_series_by_id and dimmi_get_series_report on live series that reach the
     same people. Do not send someone two series at once about the same thing; change the one that
     exists when it fits. Its report is your benchmark: how many reach the goal, and which message
     loses people.
   - search_conversations and get_conversation_by_number on what these people ask the team: their
     questions are what the messages should answer before they have to ask.
   - list_tags when a step should tag people, and get_series_reference for the step kinds, their
     settings and the shapes of entry, exit and goal.
3. Design the path:
   - Entry: the moment the need starts, such as signed up or trial started. Leave out people who
     already match when it goes live, unless the teammate asks to include them.
   - Goal: the event or rules from step 1, within the days the series runs. When many people enter
     (several hundred a month), hold out a control group of 10 percent, so the team can see what
     the series adds. Leave when the goal is reached.
   - Leave early on a reply: a person who writes back is talking to the team now.
   - 3 to 6 messages over 2 to 3 weeks: day 0, then about 2, 5, 9 and 14 days. Never two in one day.
     Quiet hours on.
   - React to what people do, not only to time. After a message that asks them to do something,
     wait up to a few days for that event; whoever did it moves on, whoever did not gets help with
     it. Check opens or clicks only to choose between a nudge and a different angle.
   - An AI path where people differ in ways rules cannot see, such as how they use the product, with
     a short description of each path. A split test only when the team wants to learn which of two
     approaches works; give it enough people.
   - Tag or set an attribute at the points the team will want to filter on later, such as a person
     who reached the goal or one who never started.
4. Write every message in full:
   - One job per message, and one link or button for it.
   - A subject of 30 to 50 characters that says what the person gets, not "Newsletter #2".
   - Short: 50 to 120 words. Plain words. The person's first name with a fallback:
     {{ contact.first_name | default: "there" }}.
   - Written as a person on the team, the way they would write one customer.
   - Answer what these people ask the team (step 2) before they have to ask.
   - Never invent a feature, a price, a discount or a deadline. Use only what you read in the
     workspace, or ask.
5. Build it. On a series' page, call propose_series_edit once with every edit, in order. When no
   series exists yet, tell the teammate to click New series and who it is for, and stop there: you
   cannot create one.
6. Walk it with simulate_series before you hand it over. Make each person match who enters: pass a
   real contact_id, or a person whose attributes match the entry rules. Walk three or four people
   who should travel it differently, each with a scenario in plain words:
   - the eager one: "opens every email the day it arrives and does what it asks";
   - the quiet one: "never opens anything";
   - the one who replies: "replies to the first email with a question";
   - the one who reaches the goal early: "upgrades the day after the second email";
   - and one who should not enter, to check the entry keeps them out.
   A scenario says how the person behaves, the way a teammate would describe a real customer. It
   never names the branch, path or ending you expect ("goes down the discount path"): the test is
   whether the series gets there on its own. To walk a person again after a change, send the same
   scenario.
   Read each walk: did each person get what they needed and nothing more, did the goal or a reply
   end it at the right time, did every branch go where you meant? Fix and walk again when one does
   not. Each walk asks a model a few times, so do not walk the same person twice without a change
   in between.
7. Hand it over (see the message below).

Getting it right:
- Every choice has a reason you saw. "Day 2 asks them to connect a carrier: 70 percent of the
  people who connected one in their first week became customers, and 12 percent of the rest."
  Never a generic best practice without a fact from this workspace.
- Fewer, better messages. When in doubt, cut one.
- Every path ends somewhere on purpose: the goal, a reply, a last message, or a clear exit.
- Respect people: no message after they reached the goal, no message after they replied, an
  unsubscribe path through a topic when the email is not essential.
- Never publish. Your edit is a suggestion on the working copy; the teammate accepts and publishes.

How the message should sound. The rules in the system prompt hold. Lead with what it does, then why,
then how you walked it, then what they do next.

A message in that voice. Follow how it sounds and how it is spaced, not its words. Fill every
<slot> from what you saw, and leave out any part you have nothing for.

I set up a series that <what it does, one sentence>. It ends when someone <the goal>.

I built it around <the fact you saw, with a number>. <the second fact, when there is one>.

It sends <number> emails over <number> days, and changes course when people <what it reacts to>.

I walked <number> people through it:

- <someone who …>: <what they got and how it ended>
- <someone who …>: <what they got and how it ended>
- <someone who should not enter>: <kept out, and why>

It is on the working copy now. Accept it and publish when you are happy with it.

After two weeks, compare how many reach <the goal> against the control group to see what it adds.

When something was unclear, ask instead, with numbered options and the one you would pick, like:
"What should count as reaching the goal? 1. The booking_made event (half of new sign-ups never
do it). 2. Plan becoming paid. I would pick 1: people who book in their first week stay three
times as long."

A question in that voice, when something is unclear. Fill every <slot> from what you found:

<what you found that matters, one or two short sentences>.

<the question, one sentence>?

1. <option> (<what it covers, with a number>)
2. <option> (<what it covers, with a number>)

I would pick <number>: <why, one short line>.

# 25 Series: competitor notes

Read 2026-11-10 from public help pages. Facts here come from the cited pages; where a page was unclear it says so.

## Intercom Series

Sources: [Series explained](https://www.intercom.com/help/en/articles/4425207-series-explained), [Series FAQs](https://www.intercom.com/help/en/articles/8780861-series-faqs), [Measure performance](https://www.intercom.com/help/en/articles/4425234-measure-how-well-your-series-performs), [Goals](https://www.intercom.com/help/en/articles/4425229-measure-the-impact-of-your-series-with-goals), [Best practices](https://www.intercom.com/help/en/articles/4799207-series-best-practices-tips-examples), [Performance summary report](https://www.intercom.com/help/en/articles/6926571-view-a-series-performance-summary-report).

- **Blocks**: rule, content (chat, post, email, push, tours, carousels, bots, banners), wait, tag (tag and untag, existing tags only), notes.
- **Paths**: linear, parallel, or diverge and rejoin. Up to 7 paths after a block, 250 blocks, 10 entry rule blocks, 500 live series per workspace. No loops.
- **Rule blocks**: "try to match once" (attribute rules) or "try to match for X" with an optional not-matched path; without it the person disengages. Event triggers hold the person at the block until the event fires (default 30 days). Content events ("clicked the message") match any earlier version of that message, even from earlier passes.
- **Send window**: each in-app message waits up to a period (default 30 days) for the person to come online, then the person leaves that path. A fallback path is possible.
- **Wait block**: a period or a date. Waiting for in-app delivery while people keep entering stacks messages, delivered at once when the user comes online.
- **Entry**: any entry rule. Check timing is stated two ways (about every 10 minutes then hourly; or hourly plus on Messenger ping): unclear. Event entry is checked at the moment of the event, so attributes written seconds later are missed. A person matching exit rules cannot enter. Several series at once are allowed.
- **Exit rules**: any attribute or event; exit is complete. No exit on reply (workaround with a rule block, a tag and an exit rule on the tag). No stop date (workaround with a wait-until path and a tag).
- **Disengaged**: no match within the window, or a message that cannot be delivered (no email, unsubscribed, bounced, push off).
- **Goal**: one per series, rules within a window from entry (default 30 days). Does not exit. Changing it applies to new entrants only. Only counts people who received content after it was set. Segments cannot be goals.
- **Re-entry**: off by default; "every time they match" with spacing, or a fixed schedule. A re-entry series must have one entry block.
- **Editing live**: new content ahead of someone reaches them; content behind does not. Deleting a block someone waits at drops them from there. Changed waits apply to everyone waiting, measured from their arrival. Content edits reach waiting people. No version history documented.
- **Pause**: stops all messages, people stay put. Delete removes everyone and cannot be undone.
- **Reporting**: started, finished, disengaged, exited, goal; per block pending and trying-to-match counts; per person drill-down; CSV export; webhook topic `content_stat.series`.
- **Companies**: rules on company data are evaluated per contact-company pair, so a contact in two matching companies gets two copies, unless "company priority" is set.
- **Frequency**: no per-user cap or sending limit documented for series.

## Customer.io automations

Sources: [Triggers](https://docs.customer.io/messaging/send/automations/triggers.md), [Editing live automations](https://docs.customer.io/messaging/send/automations/edit-live/changes.md).

- Triggers: event, attribute or segment, date attribute, object and relationship.
- Frequency: event triggers once, every event, or once within a period; attribute triggers once, fixed intervals (from entry), or every re-match.
- A person cannot re-trigger while their journey is active (except object triggers). One copy of a message per pass.
- Editing live: trigger changes leave current people in place; filter changes are re-checked through exit conditions; trigger type can only change while stopped. Their docs warn that live changes "can cause unexpected behaviors".
- Object journeys cap at 50,000 people per object and date, dropping the rest.

## Braze Canvas

Sources: [Change after launch](https://www.braze.com/docs/user_guide/messaging/canvas/managing_canvases/change_your_canvas_after_launch/), [Exit criteria](https://www.braze.com/docs/user_guide/messaging/canvas/create_a_canvas/exit_criteria/), [Winning path](https://www.braze.com/docs/user_guide/messaging/canvas/canvas_components/experiment_step/winning_path/), [Delay](https://www.braze.com/docs/user_guide/messaging/canvas/canvas_components/delay_step/), [Decision split](https://www.braze.com/docs/user_guide/messaging/canvas/canvas_components/decision_split/). Pages were read through summaries, so these are paraphrases.

- Delay: duration up to 730 days, a date, or a weekday; personalised delays exit the user when the date has passed.
- Decision split: yes or no on segments or filters, checked right after the previous step.
- Experiment paths with an automatic winning path after a window; ties go to the first path; no significance test is named.
- Exit criteria apply while users wait in delays.
- After launch: people in delays keep their timers; split and experiment steps cannot be added, removed or reordered; rate limit changes apply only to later arrivals; Braze advises against editing live canvases.

## What we take

- From Intercom: the canvas, rule blocks with a timeout path, goal window from entry, pause semantics, per-block counts and per-person drill-down.
- From Customer.io: one active journey per person per series, one copy per pass, re-entry with a cooldown.
- From Braze: exit criteria checked while waiting, experiment paths with a winner, delays by date and weekday.
- Where all three are weak: entry latency (Intercom), live editing (all three), account journeys (Intercom duplicates, Customer.io caps), Slack, replies becoming conversations, AI steps, a public API (Intercom has none for series).

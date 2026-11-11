# 25 Series

Date 2026-11-10. Owner approved; built the same day (see "State of the build" at the end). Read with `specs/00-conventions/README.md`, `specs/24-broadcasts/README.md`, `specs/17-workflows/README.md`, `specs/18-audiences/README.md`. Competitor notes with sources: [competitors.md](competitors.md).

## Goal

A series is an automated journey: people (or whole accounts) enter when they match rules or do something, then move through messages, waits, checks and actions over days or weeks, until they reach the goal, leave, or finish. Onboarding, trial ending, adoption nudges, win-back, renewals, NPS follow-up.

Broadcasts send one message once. Series send the right next message to each person when they are ready for it. Every message is a broadcast-grade send (verified address, consent, pacing, tracking, replies become conversations), so a series is the broadcasts machinery driven by a per-person cursor instead of a one-off freeze.

## Competitors in one paragraph each

- **Intercom Series**: a canvas of rule, content, wait and tag blocks. Entry rules (up to 10), exit rules, one goal with a window from entry, re-entry off by default, "try to match for X" rule blocks with a not-matched path, message send windows ("wait up to 30 d for them to come online"), a separate "disengaged" state, edits apply live to people mid-series, 250 blocks, 7 paths per block, no loops. Weak spots, all from their own docs: entry checked roughly hourly (two articles disagree), one goal only, no exit on reply, no stop date, goal does not exit, contacts in two companies get duplicates, stacked in-app messages delivered at once when a waiting user comes online, A/B is a separate split block with manual reading, no Slack.
- **Customer.io**: event, attribute, segment, date and object triggers; frequency per trigger (once, every time, within a period, fixed intervals); "one copy of a message per pass"; edits to filters re-checked against exit conditions; object (account) journeys exist but cap at 50,000 per object date and fan out per person.
- **Braze Canvas**: delays (duration, date, weekday), decision splits, audience and action paths, experiment paths with an automatic winning path, exit criteria checked even during delays, rate limits, quiet hours. In-flight users stay on the steps they entered; paths cannot be added or removed after launch; Braze itself advises against editing live canvases.

## What we match and where we beat them

| Area | Match | Beat |
|---|---|---|
| Entry | rules (saved audience or inline), an event, by hand or API | seconds, not hourly: entry is checked on the change that makes someone match, plus a 15-minute sweep for rules that turn true with time ("trial ends within 5 days") |
| Accounts (B2B) | company rules in conditions | **account-level series**: the company enrolls once, messages go to chosen people in it (admins, the champion) and to its Slack channel, goals and exits are per account, reports per account. No contact-company duplicates (a contact has one company) |
| Channels | email, Messenger chat and post | **Slack** steps into the customer's shared or community channel, and replies there become conversations |
| Replies | replies become conversations | **exit on reply** as a toggle; "replied" as a path; a teammate's reply pauses the person's series messages for a cool-off |
| Goal | one goal, window from entry | goal can **exit** in one toggle; goal by event or rules; per-step conversion on every message |
| Waits | duration, until date | business hours, a time of day in the contact's zone, until an attribute date (trial_ends_at minus 3 days), wait for rules with a timeout path |
| Messenger | chat and post | no "wait to come online" stacking: a Messenger step is a closed conversation the person sees whenever they open the Messenger, so nothing piles up |
| A/B | split with percentages | split test with a holdout, results with confidence on the goal, promote a winner in one click |
| Editing live | edits apply | **versions**: publish shows who sits on changed steps and lets you choose what happens to them; no silent surprises, roll back in one click |
| Testing | "add a rule for your email" | **try it on me**: enroll yourself, skip waits with a button, see every step's decision |
| Pacing | none documented | per-contact message cap across all series and broadcasts, quiet hours, install-wide provider quota and warm-up from broadcasts |
| AI | none in series | **AI path** (dimmi reads the person and the account and picks a path, with a fallback), **AI line** (one personal sentence per recipient, previewed), dimmi answers replies when allowed, Operator drafts whole series |
| API | none (Intercom series have no public API) | everything by REST, MCP and `dimmi series export/apply` JSON |

## Decisions

| Question | Decision |
|---|---|
| Who travels | a series is for contacts or for companies, chosen at creation and fixed (`subject` `contact` or `company`) |
| Graph | nodes and edges, acyclic. Paths may rejoin. No loops (checked at publish). Max 200 nodes, 8 outgoing paths per node |
| Node kinds | `entry`, `email`, `chat`, `post`, `slack`, `wait`, `wait_for`, `condition`, `split`, `ai_path`, `tag`, `untag`, `set_attribute`, `topic`, `webhook`, `notify`, `exit`. Banners, tours and push are out |
| Entry | one entry node holding any of: rules (saved audience or inline, 18), an event name (contact events, 18), manual and API enrollment. Several triggers OR together. Contacts who already match when the series goes live enter only when "include people who already match" is on (default off, with a count shown) |
| Re-entry | `once` (default), or `again` after the person left plus a cooldown in days. At most one active enrollment per series and subject |
| Exit rules | rules checked on every relevant change and before every step. Toggles: exit on goal, exit on reply to any series message, exit on unsubscribe from the series topic. An exit is immediate; queued sends that did not leave are withdrawn |
| Goal | optional, one per series: an event or rules, reached within N days of entry (default 30). Counted per enrollment; optional exit on goal |
| Conditions | `condition` evaluates once, in path order, first match wins, plus an "everyone else" path. `wait_for` holds until rules match or an event happens or the previous message is opened, clicked or replied, up to a timeout, with matched and timed-out paths |
| Rules | the audience language (18), server fields only. Page and session fields are refused at save. Extra leaves: "received/opened/clicked/replied to step X of this series", "is in series Y", "reached goal of series Y" |
| Waits | duration (minutes to 365 days), optionally counted in business hours of a schedule (17 schedules); until a time of day or weekday in the contact's zone (fallback workspace zone); until a date attribute plus or minus an offset (past date: go on at once) |
| Messages | each message node is a hidden `broadcasts` row (`kind = 'series'`), each send a `broadcast_recipients` row. Same senders, rendering, variables, tracking, links, unsubscribe, preference page, bounce handling and reply handling as broadcasts |
| Send failures | a message that cannot be sent (no address, suppressed, opted out, no Messenger, no Slack channel, bounced) ends the enrollment `undeliverable`, unless the node has a "could not send" path, which the person then takes. In an account series the step counts as sent when at least one person or room in the account got it |
| Advance after a message | when the channel accepted it (`sent`), not when it is opened |
| Consent | a series has a topic (default: a topic named after the series, opt-out) or is `service`. Marketing opt-outs skip non-service email; suppressions always skip |
| Pacing | install-wide provider quota and address warm-up from broadcasts apply unchanged. New per-contact cap: at most `series.max_messages_per_day` (default 2) series messages per contact per day across all series; over the cap the message waits for the next day, it is never dropped. Broadcasts' frequency cap counts series email too |
| Quiet hours | per series: send only inside a schedule in the contact's zone (default: off for Messenger, 8:00 to 20:00 for email). A message outside waits for the window |
| Human first | a teammate's public reply to a conversation that came from this series holds the enrollment's next message for `series.reply_cooloff_hours` (default 48), unless exit on reply is on |
| Account series | the company enrolls. Email and Messenger steps name who in the account receives them by rules (default: every contact of the company with an email; typical: `contact.custom.role = admin`). Slack steps go to the company's active rooms. Conditions see company fields plus "someone in the account" leaves. The step advances when every recipient row settled |
| Splits | `split` sends each enrollment down a path by percentage (2 to 4 variants, optional holdout that skips to the rejoin node). Sticky by `hash(enrollment_id, node_id)`. Winner promoted by hand; then 100% go down it |
| AI | `ai_path`: a prompt plus labeled paths; dimmi classifies from the contact, company, last conversations and event summaries; the fallback path takes errors, timeouts and spent caps. AI line: a message option that writes one sentence per recipient, max 200 characters, logged, shown in the person's trace. Both run on the workspace's AI key under the 07 cap. Operator drafts series as drafts only |
| Editing live | the working copy autosaves; publishing makes a new immutable version. Enrollments move to the new version at their current node when that node still exists (most edits). People waiting on a removed node: the publish dialog says how many and offers "move them to node X" (default: the removed node's first surviving successor) or "let them leave". Changed wait durations recompute from the time they arrived |
| Pause | stops every send and every advance at once; people stay where they are; waits that come due while paused run on resume, released at the pace of the channels. Stop (archive) ends every enrollment with `series_stopped` |
| Delete | only drafts that never went live. Live series are archived, never deleted, so reports remain |
| Permissions | `series:read`, `series:write` (draft, test), `series:publish` (publish, pause, resume, archive, enroll and remove people). Owner and admin hold all; members read |
| Metering | none |

## Data model

Migrations in one expand migration, `create_series_tables`, plus `add_series_to_broadcasts`. Prefixes in `models.PublicID`.

### series (`srs_`)

workspace_id, name (1 to 120), description, subject `contact|company`, status `draft|live|paused|archived`, working jsonb (the draft `SeriesSpec`, max 256 KB), working_hash, live_version_id, lock_version (409 `edit_conflict` on stale saves), topic_id, service bool, email_address_id, sender_member_id, slack_as, reply_team_id, reply_member_id, ai_replies, quiet_schedule jsonb, counts jsonb (entered, active, finished, exited, goal), created_by_member_id, published_at, paused_at, archived_at, timestamps. Unique `(workspace_id, lower(name)) WHERE archived_at IS NULL`.

### series_versions (`srv_`)

workspace_id, series_id, number (unique per series), spec jsonb (frozen), spec_hash, compiled jsonb (nodes by id, edges, path order, topological order, rule dependency set, referenced audiences, tags, schedules, topics, events), note, published_by_member_id, migration jsonb (what the publish did with people on removed nodes), created_at. Immutable.

### SeriesSpec (jsonb, Go type in `app/models/seriesspec`)

```json
{
  "entry": { "rules": {...}, "audience_id": "aud_...", "event": "signed_up",
             "include_existing": false, "reentry": { "kind": "once" } },
  "exit": { "rules": {...}, "on_goal": true, "on_reply": false, "on_unsubscribe": true },
  "goal": { "event": "messenger_installed", "days": 14 },
  "nodes": [
    { "id": "n_welcome", "kind": "email", "settings": { "subject": "...", "body": "...", "track_clicks": true },
      "position": { "x": 0, "y": 160 } },
    { "id": "n_wait", "kind": "wait", "settings": { "for": "P2D", "business_hours": false } },
    { "id": "n_installed", "kind": "condition",
      "settings": { "paths": [ { "id": "p_yes", "label": "Installed", "rules": {...} } ] } }
  ],
  "edges": [
    { "from": "entry", "to": "n_welcome" },
    { "from": "n_installed", "path": "p_yes", "to": "n_tips" },
    { "from": "n_installed", "path": "else", "to": "n_nudge" }
  ]
}
```

Node ids are stable strings made by the builder; they survive edits, which is what lets enrollments follow a new version.

### series_enrollments (`sre_`)

workspace_id, series_id, version_id, subject `contact|company`, contact_id, company_id, status `active|waiting|finished|exited`, node_id (cursor), seq bigint (bumped on every move; jobs carry the expected seq), wait jsonb (`{kind: timer|match|send|cooloff|ai, until, rules_hash, recipients}`), wake_at, entered_at, entered_by `rules|event|manual|api|test`, trigger_ref (event idempotency key or the change that matched), goal_at, left_at, end_reason (`finished`, `exit_rules`, `goal`, `replied`, `unsubscribed`, `removed`, `series_stopped`, `contact_erased`, `node_removed`, `step_budget`), variants jsonb (split node id to path), test bool, steps_executed, timestamps.

Indexes: partial unique `(series_id, contact_id) WHERE status IN ('active','waiting') AND subject = 'contact'` and the same for company; `(wake_at) WHERE status = 'waiting'` (reconciler); `(workspace_id, contact_id) WHERE status IN ('active','waiting')` and the company twin (change listeners); `(series_id, node_id) WHERE status IN ('active','waiting')` (who waits where); `(series_id, entered_at)`.

### series_steps

Trace, one row per node visit: workspace_id, enrollment_id, series_id, version_id, node_id, kind, status `done|waiting|skipped|failed`, path_id, outcome (`sent`, `skipped:<reason>`, `matched`, `timed_out`, `else`, `variant`, `ai:<label>`, `ai_fallback`, `tagged`, `http_<status>`), detail jsonb (max 4 KB: recipient ids, AI line, failed rule), entered_at, left_at. Index `(workspace_id, enrollment_id, entered_at)`, `(series_id, node_id, entered_at)`. Retention `series.trace_retention_days` (180), counts survive in the rollup.

### series_node_stats_daily

series_id, version_id, node_id, day (workspace zone), reached, left, outcomes jsonb (path or outcome to count). Built by `rollup_series_stats` every 5 minutes from `series_steps` over the last 2 days (idempotent recompute, the 17 pattern). Message engagement comes from the node's broadcast counts, not from here.

### Changes to broadcasts

- `broadcasts.kind` `one_off|series` (default one_off), `series_id`, `series_node_id`. Series rows have status `sending` while the series is live, `paused` while paused, `stopped` when archived; they never pass through freeze or dispatch, never appear in `GET /broadcasts`, and are never marked `sent`.
- `broadcast_recipients.series_enrollment_id`, `series_visit` (the series_steps id). The unique indexes `(broadcast_id, contact_id)` and `(broadcast_id, room_id)` gain `WHERE series_enrollment_id IS NULL` (new migration drops and recreates them; broadcasts already merged are append-only, so this ships as a new migration); series rows are unique by `(series_visit, contact_id)` and `(series_visit, room_id)`. The address dedupe index stays one-off only.
- On publish, the node's broadcast row is updated from the version (subject, body, sender settings). People who already got it keep the email they got; the report shows the current text with a "changed on" note.

## Engine

Package `app/series` (engine, like `app/workflows`) and `app/actions/series` (draft, publish, enroll, report). Queue jobs in `app/jobs/series.go`, queue `series`.

### Entering

1. **Changes**: listeners on `contact.created`, `contact.updated`, `company.updated`, contact and company tag changes, and a new in-process `contact_event.tracked` (18 names it; `Track` does not dispatch it yet, so this spec adds it). Each change is filtered by the compiled dependency set of the workspace's live series (one set lookup; workspaces without series pay nothing), then one `check_series_entry{workspace, subject}` job, unique per subject for 5 seconds so a burst of updates is one check.
2. **The check** loads the person once, evaluates every live series' entry rules and exit rules with `audiences.Evaluate`, and inserts enrollments for the ones that match and allow entry (re-entry rule, not already active, exit rules not matching). Insert is `ON CONFLICT DO NOTHING` on the active-enrollment index, so two parallel checks enroll once. Each new enrollment enqueues `advance_series_enrollment` in the same transaction.
3. **Event entry** enrolls in the transaction that records the event, keyed by the event's idempotency key (`trigger_ref`), so a retried event does not enroll twice. Rules on the entry node are evaluated after the event's summary row is written, so "did X 3 times" sees this time.
4. **Time-based rules**: `sweep_series_entry` every 15 minutes pages through `audiences.Match` for each live series whose entry rules contain a relative date or time leaf, enrolling new matches. Series without such leaves are never swept.
5. **Going live with "include existing"**: a `backfill_series` job pages `audiences.Match` (1,000 per page) and enrolls at most `series.backfill_per_minute` (default 2,000) so a large audience does not land in one minute.
6. **Manual and API**: `POST /series/{srs}/enrollments` with contact or company ids, an audience, or a contact list; same rules as automatic entry unless `force: true` (still respects exit rules and suppressions).

### Advancing

`advance_series_enrollment{enrollment_id, expected_seq}`, unique by enrollment while queued:

1. `BEGIN`; load the enrollment `FOR UPDATE`; stale seq, ended, or series not live: no-op.
2. Load the version from the immutable cache (otter, by version id).
3. Guard: load the subject, evaluate exit rules; matching ends the enrollment `exit_rules`.
4. Loop: execute the cursor node; write its `series_steps` row; move the cursor along the chosen edge; `seq + 1`. A node returns `Next(path)`, `Wait(spec)` or `End(reason)`. Stop at a wait, an end, 25 nodes in one job (re-enqueue self), or `series.max_steps_per_enrollment` (500, ends `step_budget`).
5. A node with no outgoing edge ends the enrollment `finished`.
6. `COMMIT`. Realtime and webhooks after commit.

Executors are small and one per kind; DB effects (tag, untag, set attribute, topic) run in the advance transaction, so they happen once.

### Message nodes

1. Check quiet hours and the per-contact daily cap (Valkey counter through `framework/ratelimit`, key per contact per day; the database count of today's series recipients rebuilds it). Outside the window or over the cap: `Wait(cooloff, until)`.
2. Pick recipients: the contact (contact series) or the account's people by the node's rules, or the company's Slack rooms (account series). Reach and consent use the broadcasts `facts` code unchanged, refactored to take a recipient list instead of a frozen page.
3. Insert `broadcast_recipients` rows (`queued`, `due_at = now`), skipped rows with their reason; enqueue `send_broadcast_recipient` per queued row with `InsertTx`; `Wait(send, recipients)`.
4. `SendRecipient` is unchanged except `load`: a series row is sendable while its series is live. When a series recipient settles (sent, failed, bounced, skipped), broadcasts dispatches `broadcast_recipient.settled` in the same transaction; the series listener enqueues the advance. When all recipient rows of the visit are settled, the node returns `Next` when at least one was sent; otherwise it takes the "could not send" path when the node has one, else the enrollment ends `undeliverable`.
5. Pacing (`PacedError`) is not settlement: the row stays queued and is retried at its time, as for broadcasts.

So every series email is a broadcast email: same verified address, same one-click unsubscribe, same Message-ID and reply token, same bounce and complaint handling, same tracking pixel and click redirect, same reply into a conversation routed to the series' reply team. Opens, clicks and replies update the recipient row; a listener wakes `wait_for` enrollments that wait for engagement.

### Waits

- `wait`: compute `wake_at` once on arrival (business hours with `officehours`, zone from the contact, attribute date plus offset); schedule the advance job at `wake_at` with the expected seq. Reconciler `reconcile_series_timers` every minute re-pushes enrollments with `wake_at < now - 2 min` (unique jobs make duplicates harmless).
- `wait_for`: store the rules hash and timeout as `wake_at`. Change listeners, event tracking and engagement events find waiting enrollments of the subject (index) and enqueue an advance; the node re-evaluates and takes the matched path, or stays waiting. At `wake_at` it takes the timed-out path.
- Changed wait durations on publish: enrollments waiting on that node get `wake_at = entered_at of the visit + new duration` (past: now).

### Goals and exits

- Goal check runs in the same change listener as entry and exit, for active enrollments and for ended ones still inside the goal window (goal is counted even after they finished). Event goals compare the event summary's `last_at` with `entered_at`. `goal_at` set once. With exit on goal, the enrollment ends `goal`.
- Exit on reply: the broadcasts reply handlers already find the recipient; they dispatch `broadcast_recipient.replied`; the series listener ends the enrollment `replied` in the reply's transaction, before anything else is sent.
- Exit on unsubscribe: the preference page and one-click handlers dispatch `contact.topic_changed`; the listener ends enrollments of series on that topic.
- Ending an enrollment withdraws its queued recipient rows (`status skipped`, reason `left_series`) in the same transaction; a send job that runs later finds nothing.

### AI path

Phase 1 in the advance transaction: `Wait(ai)` and enqueue `classify_series_enrollment`. The worker builds a compact profile (contact and company fields, tags, last 3 conversations' summaries, event summaries), calls the 07 provider with the node's prompt and labels, stores the label in `wait.result` and enqueues the advance with the seq. Timeouts (20 s), refusals, spent caps and unknown labels take the fallback path. One billed call per visit; the advance reuses the stored result on retry.

### Idempotency

| Effect | Guarantee | How |
|---|---|---|
| Enrollment | once per series and subject while active; once per event | partial unique index; `trigger_ref` |
| Node execution | once per seq | `expected_seq` under the row lock |
| Tag, attribute, topic | exactly once | same transaction as the cursor move |
| Message | exactly once per visit and recipient | unique `(series_visit, contact_id)`; send job keyed by recipient; email `Key: broadcast:<recipient>` as today |
| Webhook | at least once | stable `webhook-id` `sre_<id>:<visit>` |
| AI classify | one call per visit | result stored before advance |
| Timers | at least once, idempotent | seq on the job; reconciler |

### Mid-series changes

| Change | Effect |
|---|---|
| Contact merged | enrollments follow the surviving contact; if both were in the same series, the one that entered first stays, the other ends `merged` |
| Contact erased or blocked | enrollments end `contact_erased`; recipient rows are erased by the existing eraser |
| Company of a contact changes (contact series) | nothing moves; conditions read the current company |
| Contact leaves the company (account series) | not picked for later messages |
| Email address disconnected or unverified | email nodes skip with `no_sending_address`; the series shows a blocking warning and the publish check refuses until fixed |
| Slack room archived or app removed | Slack node skips `channel_unavailable` |
| Saved audience used by entry edited | applies to later entry checks; nobody is removed (exit rules do that) |
| Series paused | advances no-op, sends hold (`load` refuses), waits keep their times |
| New version published | enrollments move to it at their node; removed nodes per the publish choice |
| Teammate public reply in a series conversation | the enrollment's next message waits for the cool-off, or exit on reply |

## HTTP API (workspace API, `/api/v1`)

- `GET/POST /series`, `GET/PATCH/DELETE /series/{srs}` (PATCH saves the working copy with `lock_version`; DELETE only never-published drafts)
- `POST /series/{srs}/check` (validation problems: loops, dangling paths, missing senders, unknown variables, page fields in rules)
- `POST /series/{srs}/publications` (`{note, removed_nodes: {node_id: "move:<node>" | "leave"}}`), `GET /series/{srs}/publications/preview` (who sits on changed or removed nodes)
- `GET /series/{srs}/versions`, `POST /series/{srs}/versions/{srv}/restore`
- `POST /series/{srs}/pause`, `/resume`, `/archive`, `/duplicate`
- `GET /series/{srs}/estimate` (how many match entry now, by channel reach)
- `GET/POST /series/{srs}/enrollments?node=&status=&company=`, `GET/DELETE /series/{srs}/enrollments/{sre}` (DELETE removes the person, `removed`), `GET /series/{srs}/enrollments/{sre}/steps` (trace)
- `POST /series/{srs}/tests` (enroll yourself or a test contact, `test: true`), `POST /series/{srs}/enrollments/{sre}/skip` (test enrollments only: end the current wait now)
- `GET /series/{srs}/report?from=&to=`, `GET /series/{srs}/nodes/{node}/report`, `GET /series/{srs}/report.csv`
- `POST /series/{srs}/nodes/{node}/winner` (`{path}` for splits)
- `GET /contacts/{con}/series`, `GET /companies/{cmp}/series` (current and past enrollments)
- Realtime: `series.progress` on the workspace channel (counts, coalesced to one a second per series). Webhooks: `series.enrollment.started`, `.finished`, `.exited`, `.goal`.
- MCP toolset `series` (list, show, draft, check, estimate; never publish). CLI `dimmi series export/apply` with the spec JSON.

## UI, mapped onto the prototype

The prototype in `resources/js/features/series/` already has the right shape: list, builder with Build and Results modes, canvas on React Flow, inspector, motion. It moves from `sample.ts` to TanStack Query hooks in `resources/js/lib/api/series.ts`.

| Prototype | Becomes |
|---|---|
| `series-list.tsx` on `seriesList` | `GET /series`: name, entry summary (rendered from the entry spec on the server), status chip, active, entered, goal %. "New series" opens templates (onboarding, trial ending, adoption, win-back, renewal for accounts, NPS follow-up) or blank, and asks contact or company |
| `SeriesNode` tree with `next[]` | nodes and edges from the spec; the tree layout stays for trees, joins draw as a line into the shared node. `NodeKind` drops `banner`, adds `slack`, `wait_for`, `condition`, `split`, `ai_path`, `untag`, `set_attribute`, `topic`, `notify` |
| `Plus` button under a card | a menu of node kinds grouped Messages, Timing, Logic, Actions, AI |
| `series-inspector.tsx` settings per kind | typed settings forms; message nodes reuse the broadcasts composer and preview (`Broadcast` preview already used); rules reuse the audience builder from 18 |
| `EntrySettings` | triggers (rules, event, manual), include existing (with count), re-entry, exit rules and toggles, goal, topic, sender, address, quiet hours, reply team |
| `StepResults` and `resultLine` on sample stats | node report: reached, waiting now, sent, opened, clicked, replied, converted, paths with counts; "people here" list with the trace drawer |
| `Actions`: Set live, Pause, Resume, Publish changes (toast only) | real calls; Publish opens the dialog with the check result, the note and the removed-nodes choice |
| Build and Results switch | kept; Results overlays counts on cards, live over realtime |
| new | "Try it on me" button with a test panel (your enrollment, skip wait, the trace as it runs); version history with restore; problems badge per card from `/check`; contact and company profile sections "Series" with remove |

All BoardUI rules apply: semantic tokens, composite type, `cx()`, Remix icons, ScrollFade for the inspector and lists.

## Reporting

- Overview: entered, active now, finished, exited by reason, goal reached (count, %, over time), median time to goal, per channel sends and engagement.
- Per node: reached, waiting now, left, outcomes per path; message nodes add the broadcast counts (sent, delivered, opened estimated, clicked, replied, bounced, unsubscribed) and conversion to the series goal among those who got it.
- Split nodes: per variant entered, goal %, difference against the first variant with a 95% interval (two-proportion test), sample size warning under 100 per variant.
- Account series: the same report per company, and a company list with where each account stands.
- Per person: the trace (every node, decision, message with links to the email or conversation).
- CSV export of enrollments and of message recipients.

## Edge cases

- A person in many series at once: allowed; the per-contact daily cap and quiet hours keep the total sane; the cap queue is first come first served by due time.
- Entry and exit both match on the first check: they do not enter.
- Exit rules match while a message is queued but not sent: it is withdrawn.
- Event arrives before the contact exists: the event is refused by 18 today; no enrollment.
- Wait until an attribute date that is missing or not a date: take the node's "no date" path when present, else go on at once and trace `no_date`.
- Split weights changed on a live version: only later arrivals use the new weights; sticky assignments remain.
- Removing a split variant with people on it: treated like a removed node in the publish dialog.
- Series archived while people wait: all end `series_stopped`, queued sends withdrawn.
- Very large backfill: paced by `backfill_per_minute`; the report shows "enrolling".
- Clock: every time comes from `framework/clock`; tests move the fake clock to run waits.
- Test enrollments never count in reports and can enroll the same person again.
- Workspace zone or contact zone change after a wait was computed: the wait keeps its time (documented, as 17).

## Config

`series.max_messages_per_day` (2), `series.reply_cooloff_hours` (48), `series.backfill_per_minute` (2000), `series.entry_sweep_every` (15m), `series.max_steps_per_enrollment` (500), `series.trace_retention_days` (180), `series.max_nodes` (200), `series.ai_timeout` (20s).

## Milestones

1. **M1 Model and drafts**: migrations, `SeriesSpec`, validation (`/check`), CRUD and autosave with lock_version, versions and publish (no engine), list and builder on real data. Feature tests for CRUD, validation, versions.
2. **M2 Engine core**: enrollments, manual and API entry, advance loop, `wait`, `condition`, `tag/untag/set_attribute`, `exit` and finish, reconciler, pause and resume, trace. Tests with the fake clock walking a full contact series.
3. **M3 Messages through broadcasts**: `broadcasts.kind`, recipient columns and index migration, refactor of reach and `load`, `broadcast_recipient.settled`, email, chat, post and Slack nodes, quiet hours, per-contact cap, consent and topic. Tests against Mailpit and the Slack test driver.
4. **M4 Automatic entry, exits, goals**: change listeners with dependency sets, `contact_event.tracked`, event entry, entry sweep, backfill, exit rules and toggles, reply exit, unsubscribe exit, goals, `wait_for` with engagement.
5. **M5 Account series**: company subject, recipient rules per node, Slack rooms, per-company report.
6. **M6 Splits and AI**: split with holdout and winner, `ai_path`, AI line, Operator drafting through MCP tools, dimmi replies.
7. **M7 Live editing and reporting**: publish preview and removed-node moves, version restore, node and series reports, realtime, CSV, contact and company profile sections, webhooks, `dimmi series export/apply`.
8. **M8 Try it on me and templates**: test enrollments with skip, test panel, templates, e2e coverage and the manual e2e flow table.

Each milestone ends with `make lint test` (related packages locally, CI for the rest), Biome clean, and its rows in `docs/testing/manual-e2e.md`.

## Testing end to end

- **Feature tests** (`tests/feature/series`, `tc := tests.New(t)`): real Postgres, Valkey, River and Mailpit; fake clock moves time to run waits, sweeps and goal windows; RLS run as the unprivileged role. One test per behaviour: entry once, re-entry cooldown, exit withdraws a queued send, goal exits, wait in business hours, attribute-date wait, split stickiness, publish moving people, removed node choices, pause holding sends, per-contact cap, quiet hours, account fan-out, reply exit, idempotent retries (run every job twice).
- **Fakes only at boundaries**: LLM provider for AI path (fake that passes the provider contract), Slack HTTP through `httpclienttest` in feature tests.
- **Seeded demo data**: a seeder with a workspace, 300 contacts across 20 companies with plans, roles, events and Slack rooms, three live series (contact onboarding, account renewal with Slack, trial ending with a split) and enrollments spread over every node, so screens open full.
- **Browser** (Playwright in `tests/e2e`, light and dark, failing on any console error or CSP violation): create a series from a template, edit nodes, publish, see counts move, try it on me and skip waits, pause and resume, publish with a removed node.
- **Manual run on the release binary** (`docs/testing/manual-e2e.md` rows), with screenshots: a test enrollment receiving the welcome email in Mailpit with unsubscribe headers; replying to it and the reply becoming a conversation and ending the enrollment; the chat step appearing in the widget in a real browser, and the post card; an account series posting into the hivy Slack test workspace and a thread reply becoming a conversation; a `wait_for` taking the matched path when the event is tracked by API; the goal reached and counted; the results view filling live; the publish dialog moving people off a removed node.
- **No real LLM spend** without the owner's approval for that run; the AI path is shown with the fake provider unless a real run is approved.

## Owner answers (2026-11-10)

- Per-contact cap: 2 series messages a day.
- A message that cannot be sent ends the enrollment unless the node has a "could not send" path.
- Account-level series are part of the first build.

- Decided while building: broadcasts' 72-hour frequency cap counts series email; the webhook node ships with the engine; the notify node comes later.

## State of the build (2026-11-10)

Built and covered by feature tests (`tests/feature/series`, also run as the unprivileged application role), a Vitest suite for the builder's graph model and a Playwright flow (`tests/e2e/series.spec.ts`):

- Tables, versions, working copy with lock version, problems per step, publish with travelers following the new version (removed steps move to the next remaining step, changed waits count from arrival), pause, resume, archive, restore, duplicate.
- Engine: manual, API, rule, saved-audience and event entry; include-existing baseline; re-entry with cooldown; exit rules, exit on goal, reply and unsubscribe; goals inside the window, also after a journey ended; email, chat, post and Slack steps through broadcast senders with consent, topics, warm-up and provider pacing, quiet hours and the per-contact daily cap; could-not-send path, else the journey ends `undeliverable`; waits by duration, time of day and date attribute; wait-for rules, event or engagement with a timeout; checks; splits with sticky variants and a winner; AI path through the cheap model with a fallback; tag, untag, set attribute, topic and webhook steps; account series picking people per step and posting to the company's Slack channels; a sweep for time-based matches, lost jobs, late goals and held sends.
- Try it on me: test journeys that skip waits, never count, and replace an earlier test.
- API under `/api/v1/series`, permissions `series:read`, `series:write`, `series:publish`, journeys of a contact or a company.
- Builder, list and results on the real API; `series:demo` dev command.

Not built yet:

- The notify step, the AI line inside messages, rule leaves on series membership ("is in series Y").
- `series_node_stats_daily`: step counts are computed from `series_steps` on read; trace retention is not pruned.
- Realtime `series.progress`: the results view polls every 10 seconds.
- The publish dialog's per-step choice for removed steps (the API takes `moves`; the dialog uses the default).
- Business-hours waits on a schedule, MCP toolset, `dimmi series export/apply`, CSV export, the Series section on contact and company profiles (the API exists), series webhooks.

# 01 Messenger widget: connecting the static widget

Style: caveman. `resources/widget` is finished design running on sample content. This file: how it meets real API, plus live features customers see. Wire shapes in [api.md](api.md); public SDK surface in [sdk-contract.md](sdk-contract.md).

Stack as built: own small JSX runtime (`resources/widget/jsx`), Stimulus-style controllers (`lib/controller.ts`), Shadow DOM, constructed stylesheets. Not Preact: smaller, already done, stays. Older docs saying Preact are superseded.

## Protocol layer

New `resources/widget/client/`: no DOM imports, injected `fetch`, storage, event source, clock. Same source builds `@dimmi/headless` (20), so web core and headless never drift.

| File | Does |
|---|---|
| `http.ts` | base URL, bearer, error mapping to codes, `Dimmi-Server-Time` offset |
| `session.ts` | handshake table (README "Boot handshake"), storage keys, lazy create, PoW worker call |
| `outbox.ts` | `client_id`, persist, backoff, flush on `online` |
| `realtime.ts` | connect token, EventSource, offset and epoch, backoff, polling fallback, dedupe |
| `store.ts` | conversations, messages, unread, typing, availability; emits to controllers |
| `identity.ts` | identity sources (track 09 turns it on; M3 ships anonymous path) |

## File-by-file

| File | Today | After |
|---|---|---|
| `settings.ts` | `demo` on unless `data-demo="off"` | remove `demo`, `latency`. Add `preview` from `data-preview` |
| `loader.ts` | filled by server | gains error capture listeners; `requestIdleCallback` before core per bar |
| `core.ts` | `sample`, `simulate`, 4 s fake message | config fetch, then `store`; `draw` reads store. `simulate` and sample only in `preview` chunk. Commands per sdk-contract Global API. Fails per Loud console errors |
| `options.ts` | keys of config | `defaults` remain only for first paint fallback; `Texts`, `Appearance`, `Launcher` gain [settings.md](settings.md) keys; `merge` unchanged. Config response merged over defaults |
| `content.ts` | types + sample | types only; `Conversation.number` keyed views use `id`; `lib/map.ts` maps API resources to view types. Sample moves to `preview/` |
| `render.ts` | sets `--brand`, spacing | also `--header`, `--button`, `--header-image`, gradient, inks, `dir`, `lang` |
| `views/home.tsx` | `Mark` tile, team sample | logo or letter, team from availability, `team_intro`, `home_links`, branding from `config.branding`. Copy "Powered by Dimmi" becomes "Powered by dimmi", link `branding.url` |
| `views/messages.tsx`, `conversation.tsx`, `components/thread.tsx` | sample | store; status chip, seen, typing, streaming bubble, cards (07, 15, 17) |
| `views/help.tsx`, `pages.tsx`, `changelog.tsx`, `search_controller.ts` | sample | 08, 13 routes via `client/http`; lazy per chunk rule when owners land, tab hidden until then |
| `controllers/composer_controller.ts` | fake reply | `outbox.send`, attachments flow, typing indicator, email capture when `email_required` |
| `controllers/voice_controller.ts` | fake | MediaRecorder output uploaded as `audio/webm` or `audio/mp4` attachment, chunk `voice`; hidden when `features.attachments` false or no MediaRecorder |
| `controllers/messenger_controller.ts` | `receive(Incoming)` | `receive` fed from store; `read` calls read-state; tab rules from config `tabs`, `open_to`, page rules on route change |
| `lib/sound.ts` | plays | respects `appearance.sound`; never before visitor gesture (browser autoplay rule) |
| `components/launcher.tsx` | bubble, hidden | tab style, image, label from `texts.launcher_label` |
| `styles/*.css` | some physical props | logical properties throughout; `[dir=rtl]` checks; launcher `position` stays physical (customer choice) |

Unknown or missing tab, feature off, chunk 404: view hidden and console code, never blank panel.

Tests: `resources/widget/widget.test.ts` keeps design coverage through `preview` chunk; new `tests/e2e/widget.spec.ts` against release binary: load, send, teammate reply through API arrives without reload, attach file, reconnect, polling when SSE route blocked, strict CSP page, no sample text in production bundle (grep in build check).

## Several widgets, same person

Rules (binding, X8):
- Verified customer = one contact across workspace's widgets (`contacts.external_id` unique per workspace, 02 step 2). Teammates see full history on that contact in inbox.
- Anonymous visitors separate per widget: storage key `dimmi:{app}:*`, one lead per widget, `previous_token` honored only on same widget, same email typed on two widgets never merges (02 R2).
- Conversation stays on widget where it started: `conversations.source.widget_id` never changes. Visitor list on widget W: own conversations with `source.widget_id = W`, plus non-widget origin (`email`, `api`, `import`) when conversation inbox equals W's inbox. Others 404 on show and post.
- Realtime: one channel `widget_contact:{ctc}` per contact. Every conversation-scoped publication carries `widget_id` (`source.widget_id`, null for non-widget origin), added by `app/integrations/widget/payloads.go`; all publishers go through it (delta: 03 `RecordTyping`, 07 `ai.*`). Client drops publications with other `widget_id`. Why one channel: same person on both sides so no privacy boundary, all specs already publish there, uni-SSE cannot filter. Null `widget_id` for conversation not in list: client fetches it, 404 dropped.
- Unread count, `open_conversation_ids` in session resource scoped to widget same way.
- Teammate replies route by conversation's own channel connection: widget A conversation pushes to A's sessions via shared channel, offline email link uses its stored `page_url`.

## Customer-visible status

Field `customer_status` on `widget_conversation` and on `message.created`, `conversation.closed`, `conversation.reopened` payloads. Computed in `widget_conversation_resource` from 03 state, nothing stored.

| Value | When | Default label key |
|---|---|---|
| `with_team` | open or snoozed; last message from contact, or none | `status_with_team` |
| `waiting_on_you` | open, last `type=message` from member or agent | `status_waiting_on_you` |
| `closed` | closed | `status_closed` |

Snoozed shows `with_team`: customer never learns internal state. Spam shows normal value (E33 stands). `ai_state = active` shows existing "AI answering" chip instead, `ai_answering` key. `customer_status_enabled = false`: field still sent, chip hidden. Teammate-defined states wait on tickets, not specified.

## Seen on chat

Both directions yes, this is widget and chat only; email never (privacy, X12).
- Teammate sees visitor read: `PATCH read-state` -> `message.delivery_updated` status `read` (exists).
- Visitor sees team read: `conversation.seen_by_team` (03) -> `texts.seen` under latest team-visible message, gated by 03 workspace toggle `seen_receipt` (Inbox settings) and name by `seen_receipt_shows_name`. Toggle off: 03 does not emit; polling synthesis in `/updates` checks same toggle. Builds on 03 settings page making `seen_receipt` real (track 02 inbox settings); until then default on.

## Typing to customers

Teammate typing reaches widget via 03 `RecordTyping` relay (built). Client shows name + avatar, 6 s auto-expire, dropped on message from same author. Builds on F88 fix: whichever shape fix takes, widget relay and its payload `{author: {type, name, avatar_url}}` stay. Visitor typing to teammates: REST (exists). Polling mode shows none.

## AI token streaming

07 emits `ai.reply_started`, `ai.reply_delta` (ephemeral), `ai.reply_discarded`, then normal `message.created`.

- `reply_started {run_id, conversation_id, agent, streaming}`: open bubble under run id. `streaming: false` (verify mode): typing indicator only.
- `reply_delta {run_id, seq, text}`: append in `seq` order; gap: hold up to 500 ms, then wait for final. Plain text while streaming, `markdown` chunk renders on final, no half links (07 releases by sentence).
- `message.created` for same `run_id` (`metadata.ai.run_id`, widget-safe key): replace bubble, once.
- `reply_discarded`: remove bubble.
- No delta or final for 60 s: remove bubble, refetch messages.
- Reconnect or polling: deltas never replayed; indicator, then final message. Budget: under 400 B in core.
- Screen readers: live region announces final message only, not deltas.

## Developer context

Standout: console errors and failed requests attached to conversations, for teammates and agent. Off by default per widget; customer turns on, knows own privacy duty (setting row says so).

- Loader always holds `error`, `unhandledrejection` ring (10 newest, in memory, no network, no cost when off).
- Config `features.dev_context` on: core loads chunk `dev-context`. `console: true` wraps `console.error`, `console.warn`. `network: true` wraps `fetch`, `XMLHttpRequest`; records only status 400 and up or network failure. Own Dimmi calls excluded. Originals called first, exceptions swallowed, nothing sent until visitor writes.
- Sent on `POST /conversations` and `POST .../messages` as `dev_context`: `{console: [{level, message, source, at}], network: [{method, url, status, duration_ms, at}], since}`. Max 20 + 20 entries, 8 KB, only what is new since last send. Over size: oldest dropped, never 422.
- Never: request or response bodies, headers, cookies. URL: origin + path + query with secret-like keys stripped (same regex as `current_page`). Console text: first 500 chars, JWT, bearer, `key=`, card-like and email-like patterns replaced with `[redacted]`, client side then again server side (`RedactDevContext`).
- Stored `messages.metadata.dev_context` (documented key). Widget resources never return it. Member resources do, `conversations:read` path via 03 message resource. 07 may read for agent context (07 owner decides).
- Inbox: section "Developer context" in customer panel, `resources/js/features/inbox/customer-panel/developer-context.tsx`, shows entries for selected conversation grouped by message. Needs track 04 customer panel milestone; until it lands, section mounts in thread message detail popover.
- Hosted page same. Native: no patching, SDK method `addDeveloperContext` (20).
- Erase: gone with messages (03).

## Team introduction and delay notice

- `team_intro`, faces from availability `team`, shown on home and top of new thread. Hidden when `show_team_avatars` false and text empty.
- Delay notice: after visitor's first message, `delay_notice_enabled`, availability says `online = false` or `within_hours = false`: render `texts.delay_notice` under the message, client only. Combined with email capture rule. Availability re-read on panel open (30 s cache).

## Workflow touchpoints (17 owns)

Unchanged, compatible: config `workflow_openers` replaces `welcome_message` when one matches; `POST /conversations` `opener`; `workflow_answer`; `metadata.workflow.buttons` quick replies; composer hidden when step disables typing. `office_hours_text` stays free text. `schedule_id` feeds `within_hours` on availability once `app/support/officehours` exists; before that `within_hours` always true. Auto-replies by schedule are workflows, no widget feature.

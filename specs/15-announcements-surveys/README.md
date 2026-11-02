# 15 Announcements and surveys ("Engage")

Style: caveman. Date 2026-11-02. Backend only. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`, `specs/01-messenger-widget/*`.

Files in this folder:

| File | What |
|---|---|
| README.md | this: scope, decisions, data model, rule language, eligibility engine, code map, events, jobs, config, permissions, budgets, metering, cross-spec edits |
| [api.md](api.md) | admin REST API, widget API additions, CSAT email rating page, request/response examples, error codes |
| [edge-cases.md](edge-cases.md) | decided behavior per edge case (136) |
| [tasks.md](tasks.md) | ordered build checklist, demo script |

Umbrella name in code and API: **engage** (announcements, surveys, segments, CSAT). Product UI may say "Announcements" and "Surveys".

## Goal

Targeted in-app messages (banners, modals, chat-style messages) and surveys (NPS, CSAT, CES, choice, text) shown inside customer's app through existing Messenger widget, plus CSAT after conversation close on widget and email. Zero extra network cost for workspaces not using it. Never metered.

Why: product owner rule "anything we'll use ourselves as a business is MVP". Dimmi itself must announce launches, post urgent notices, run NPS, and rate support. Pulls forward parts of roadmap "post-MVP: reports" (CSAT) and "post-MVP: upmarket set" (outbound banners). Competitor class: Intercom Posts/Banners/Surveys, Chameleon, Userpilot, Refiner, Featurebase surveys.

## Decisions (summary)

| Question | Decision |
|---|---|
| Formats at MVP | announcements: `banner` (top/bottom, inline push-down or floating), `modal` (post; small or large), `chat` (message bubble from teammate above launcher, reply opens conversation). Surveys: `popover` (corner card, one question per step), `modal`, `conversation` (CSAT card in thread / email rating link). No Dimmi-hosted survey page (later: hosted-survey-page) |
| Tooltips, product tours, checklists | post-MVP (selector-anchored, fragile, own spec). Roadmap "post-MVP: upmarket set (tours)" keeps tours |
| Email delivery of announcements | in-app only at MVP. Bulk email needs consent, unsubscribe, list hygiene, 04 caps (200/2,000 per day). CSAT email is transactional, in conversation thread, ships: 03 `SendSystemMessage` -> email `Channel.Send` (04), `outbound_emails.kind system`. Out-of-thread survey mail not at MVP; if added later it uses 04 `email.SendContactMail` with a new `Kind` added by 04, never `app/mail` Mailables |
| CSAT after close | ships here (was 03 Out, "post-MVP: reports"). Survey with `display = conversation`, triggered by `conversation.closed`. Channels widget + email only. Slack/Discord/Telegram/WhatsApp: later: chat-app-csat |
| Segments | owned here (`segments` table). Contact-oriented saved filters, reusable by 03 contacts list (`filter[segment]`), 13 changelog, 14 feedback. 03 saved conversation views stay "post-MVP: ticket fields" |
| Contact tags | not in core (only conversation tags). Not added. Custom attributes cover flags (`custom.beta = true`). Open question 1 |
| Event tracking (`Dimmi('track')`) | client-side trigger only. No network, no storage. Matches live items with `trigger.on = track_event` already in browser. Event-based audience ("did X 3 times") post-MVP with `contact_event_summaries` |
| Eligibility evaluation | hybrid. Server evaluates audience (segment, kind, attributes, schedule, frequency, throttle, receipts) per request; never materializes audience. Client evaluates triggers (URL rules, delay, track event, device) per page. URL rules are targeting, not access control |
| Anonymous visitors without session | CDN-cacheable per-app list of items whose audience allows anonymous; frequency in `localStorage`; aggregate-only impression counters. Answering survey lazily creates session (01 rules, PoW) |
| A/B tests, goals (conversion events) | post-MVP. MVP measures CTA click rate, dismiss rate, response rate |
| Branching | yes, simple: per-question rules on closed answers, forward-only jumps (no loops), max 20 questions |
| Attribute write-back | yes: question may write answer into `contacts.custom_attributes[key]` on completion |
| AI | none at MVP. Open-text summary later: survey-ai-summary. Embedding clustering post-MVP |
| Localization | manual translations per item, up to 10 locales; fallback per field to default locale. Auto-translate "post-MVP: auto-translation" |
| Preview | reuse 01 widget test sessions; `preview` param shows one item regardless of audience/status; stats excluded |
| Changelog (13) boundary | 13 changelog = public product-updates feed (page, feeds, widget "What's new" tab, `spotlight` card inside Messenger, subscriber email). 15 announcement = targeted, time-boxed in-app message outside Messenger panel (banner, modal, chat bubble). Announcement promotes entry via CTA action `open_changelog_entry` (`changelog_entry_id` `cle_`, must be published; opens widget What's new entry). 13 `spotlight` is non-interruptive (shown on panel open): not counted by engage throttle. Lookup `changelog.FindPublishedEntry(ctx, workspaceID, entryID)` (13) validates CTA; widget opens via SDK `show('changelog', {entry})` on 13 route `GET /widget/v1/changelog/entries/{cle}`. 13 reserved `changelogs.audience` can adopt 15 segments later |
| State placement (Valkey always required, D11) | Postgres = durable truth: `engage_receipts` (frequency, show-once, dismissals, throttle for identified contacts: item must never reappear after Valkey flush), responses, items, stats daily. Valkey = rebuildable only: `cache.Store` (results cache, rendered list L2), `ratelimit.Limiter` (widget engage-events, responses, CSAT rating page posts), `pubsub.PubSub` (engage version / config bust to otter L1 on every replica), centrifuge Redis broker (realtime). Anonymous frequency stays client `localStorage` |
| Abilities | new `engage:read`, `engage:write`; permission `engage:manage` (group `engage:write`, owner + admin; 00-accounts catalog) |
| Metering | never metered. Impressions, clicks, responses, CSAT requests: no ledger lines (D2, D7). Chat-format reply creates normal contact-initiated conversation, billed by D1 like any other |

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Cost when unused | workspace with zero live items: zero extra widget requests, zero extra bytes beyond 1 config field, zero DB rows |
| Widget bundle | core adds at most 1.5 KB gz (scheduler, URL matcher, frequency store). Lazy chunks: `engage-banner` at most 6 KB gz, `engage-modal` at most 10 KB gz (shares renderer), `engage-survey` at most 14 KB gz. Content arrives pre-rendered safe HTML: no Markdown parser in browser. CI size budget |
| Eligibility fetch | `GET /widget/v1/engage-items` p95 under 20 ms server, at most 3 queries, at 1M contacts and 70 live items. Anonymous list p95 under 5 ms (otter), CDN `s-maxage=60` |
| Time to show | banner visible under 300 ms after core ready on repeat visit (cached list, ETag 304) |
| Throttle | contact never sees more than `max_interruptive_per_day` (default 2) modals/popovers/chat per 24 h across devices (identified) or per browser (anonymous), except items flagged `bypass_throttle` |
| No double asks | completed survey never shown again in same cycle, across tabs and devices (identified); CSAT at most once per conversation close, never within `cooldown_days` for same contact |
| Response durability | zero lost answers: partial answers saved per step, `client_id` dedupe, outbox retry like messages |
| Accessibility | UI contracts meet WCAG 2.2 AA (section "Accessibility contract"); CSAT email rating page works with JS disabled |
| Results | `GET /surveys/{svy}/results` p95 under 150 ms at 100k responses, under 400 ms at 1M (cached 60 s) |
| Agent-native | create announcement from Markdown file in one CLI command; MCP drafts, never publishes without confirm |

## Scope

### In (MVP)

- Segments: CRUD, rule language (contact + company fields, custom attributes, kind, dates), preview count + sample, list matching contacts.
- Announcements: banner, modal, chat formats; Markdown body; cover image; up to 2 CTAs with actions `url`, `sdk_event`, `open_messenger`, `open_article`, `open_survey`, `dismiss` (+ reserved `open_changelog_entry`); audience; triggers; schedule; frequency; priority; translations; status lifecycle; duplicate; preview; stats; receipts list.
- Surveys: question types `nps`, `rating`, `scale`, `single_choice`, `multiple_choice`, `text`, `statement`; templates NPS, CSAT, CES, feature feedback; branching; attribute write-back; intro + thank-you; displays popover/modal/conversation; responses list, CSV export, per-question results, NPS score, funnel; API-submitted responses.
- CSAT on close: conversation survey, delay, channel selection (widget, email), minimal signed email rating page (`/csat-ratings/{svr}`), cooldown, AI-involved split, rating shown on conversation, negative-rating notification (06).
- Engage settings: global throttle, survey cooldown, quiet while messenger open.
- Widget API additions, SDK additions (`track`, `show('announcement', {id})`, `show('survey', {id})`, events with camelCase payloads), anonymous CDN list, batched engage events.
- Assets: images for announcements and surveys (public, immutable URLs).
- MCP toolset `engage`, CLI `dimmi announcements`, `dimmi surveys`, `dimmi segments`.
- Webhook `survey_response.completed`. Realtime events for dashboard.
- GDPR eraser, contact merge handling, audit log entries.

### Out (later)

| Item | When |
|---|---|
| Product tours, tooltips, checklists, hotspots | "post-MVP: upmarket set (tours)" |
| Email/push delivery of announcements, broadcasts | post-MVP; needs 04 broadcast pipeline + consent |
| Event-based audiences (`contact_event_summaries`), server-side `track` ingestion | post-MVP |
| A/B tests, holdout groups, goals/conversion tracking | post-MVP |
| Contact tags | open question 1 |
| Auto-translation of items | "post-MVP: auto-translation" |
| Embedding-based theme clustering, sentiment per answer | post-MVP |
| Native mobile SDK rendering | "post-MVP: mobile SDKs" (API already headless) |
| Survey logic beyond forward jumps (piping, computed variables, quotas) | not planned |
| Send in contact local time window ("only 09:00-18:00 their time") | post-MVP |
| Reports dashboards across surveys | "post-MVP: reports"; MVP per-survey results + CSV |
| AI summary of open-text answers (themes, verbatim quotes, BYOK, spend cap, cache) | later: survey-ai-summary. Scope audit C11: IC/FB unclear, PL/FD none |
| Dimmi-hosted survey page (public or signed link, no-JS form, `/survey-forms/*`, `POST /surveys/{svy}/links`, `link_access`, cloud daily cap until paid via 12 `HasPaid`) | later: hosted-survey-page. C12: IC partial (link opens own page with Messenger), FB/PL/FD none. MVP keeps only CSAT email rating page |
| `anonymize_responses` survey setting | later: anonymous-survey-responses. C13: none found in IC/FB survey docs. API responses may still omit contact |
| Product-market-fit survey template | later: pmf-survey-template. C14: not in IC survey types or FB templates (NPS/CSAT/CES). Custom survey covers it |
| CSAT on Slack/Discord/Telegram/WhatsApp (plain link or buttons) | later: chat-app-csat. B15: IC/FB unclear. MVP: widget + email |

## User stories

Workspace member:
1. Publish "We shipped dark mode" modal to identified users on Pro plan who signed up before Oct 1, only on `/dashboard*`, once, with CTA to changelog page.
2. Show red top banner "Payments degraded, we are on it" to everyone (incl. anonymous) now, bypassing throttle, end it in one PATCH.
3. Run NPS every 90 days for users active in last 30 days, max 1 survey per 14 days per contact; see score, trend, comments.
4. Turn on CSAT: 10 minutes after close, ask widget and email customers "How did we do?"; see rating in conversation sidebar; get notified on 1-2 star rating.
5. Preview any draft in my real app with test session before publishing.
6. Export responses CSV for spreadsheet.

End customer:
7. See banner once, dismiss it, never see it again on any device when logged in.
8. Answer 3-question survey on phone with keyboard or screen reader; close tab mid-way, answers kept.
9. Rate support from email in one click plus submit, without account.
10. Reply to chat-style announcement and land in normal conversation.

Developer:
11. `Dimmi('track', 'export_completed')` triggers survey asking about export feature. No network for track itself.
12. CTA with `sdk_event` calls my handler `Dimmi('on', 'engage_action', fn)` to open my upgrade dialog.
13. Receive `survey_response.completed` webhook and pipe NPS to my warehouse; read results via REST.

AI agent:
14. Claude Code drafts release announcement from merged PR (`dimmi announcements create --file`), human publishes.
15. MCP `dimmi_get_survey_results` summarizes NPS for weekly report.

## Data model

New tables owned by 15. Conventions apply: UUIDv7 `id`, `workspace_id` first in every index, `created_at`/`updated_at`, text + CHECK, RLS in cloud. No core table columns added (see "Core table changes").

### New public ID prefixes (add to registry)

| Prefix | Table |
|---|---|
| seg_ | segments |
| ann_ | announcements |
| svy_ | surveys |
| svq_ | survey_questions |
| svr_ | survey_responses |
| erc_ | engage_receipts |
| eas_ | engage_assets |

Checked against registry and all spec folders 2026-11-02: no collision. `survey_answers`, `engage_stats_daily`, `engage_settings` have no public id (never addressed alone).

### Migrations (block `2026110100150x`)

| File | What |
|---|---|
| `20261101001501_create_engage_settings_table.sql` | |
| `20261101001502_create_segments_table.sql` | |
| `20261101001503_create_engage_assets_table.sql` | |
| `20261101001504_create_announcements_table.sql` | |
| `20261101001505_create_surveys_table.sql` | |
| `20261101001506_create_survey_questions_table.sql` | |
| `20261101001507_create_survey_responses_table.sql` | |
| `20261101001508_create_survey_answers_table.sql` | generated `search` tsvector (`dimmi_search` config from 03) |
| `20261101001509_create_engage_receipts_table.sql` | |
| `20261101001510_create_engage_stats_daily_table.sql` | |

All tenant tables: RLS policy, `workspace_id uuid not null references workspaces on delete cascade`.

### engage_settings

One row per workspace, created lazily (`INSERT ... ON CONFLICT DO NOTHING`) on first engage write or read. No initializer needed.

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid pk | |
| version | bigint not null default 1 | bumped in same tx as any item/segment/settings change that can alter widget output. Feeds 01 config `engage.version`, ETags, otter keys |
| live_summary | jsonb not null default '{}' | `{active: bool, anonymous: bool, next_transition_at: ts|null}` recomputed on bump and by `TransitionEngageSchedules` job. `active` = any announcement/survey `status = live` whose window not ended (CSAT excluded: server-side only). `anonymous` = any of those allows anonymous |
| max_interruptive_per_day | smallint not null default 2 CHECK 0..20 | modal, popover, chat. 0 = only `bypass_throttle` items |
| min_minutes_between | smallint not null default 30 CHECK 0..1440 | between interruptive items |
| survey_cooldown_days | smallint not null default 14 CHECK 0..365 | no in-app survey after contact completed or dismissed one within N days (CSAT exempt) |
| quiet_while_messenger_open | boolean not null default true | client defers interruptive items while panel open |
| created_at, updated_at | timestamptz | |

### segments

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `seg_` |
| workspace_id | uuid not null | |
| name | text not null | 1..80, unique `(workspace_id, lower(name)) WHERE deleted_at IS NULL` |
| description | text | max 500 |
| rules | jsonb not null | rule tree (section "Audience rule language"), max 8 KB, validated |
| rules_schema | smallint not null default 1 | language version |
| last_estimate | jsonb | `{count, approximate, at}` from last preview |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| created_at, updated_at, deleted_at | timestamptz | soft delete; refused while referenced by live/paused item (409 `segment_in_use`) |

Index `(workspace_id, deleted_at, name)`. Max entitlement `engage.max_segments` (cloud 200) per workspace; over: 422 `entitlement_limit_reached`.

### engage_assets

Public, long-lived images. Not core `attachments` (24 h orphan purge, signed URLs) and not `help_center_assets` (bound to help center).

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `eas_` |
| workspace_id | uuid not null | |
| disk, path, filename | text not null | |
| content_type | text not null CHECK in ('image/png','image/jpeg','image/gif','image/webp') | SVG refused (`svg_not_supported`); AVIF refused (`framework/image` `ErrUnsupportedFormat`; browser-side conversion later if needed) |
| size_bytes | bigint not null | max `engage.assets.max_mb` (5) |
| sha256 | text not null | unique `(workspace_id, sha256)` dedupe |
| width, height | int not null | from decode; max 40 MP (bomb guard, shared `app/support/images` (08 owner) on `framework/image`) |
| alt_text | text | max 300 |
| scan_status | text not null CHECK in ('pending','clean','infected','skipped') | infected: never served |
| uploaded_by_type, uploaded_by_id | text, uuid | Actor |
| referenced_at | timestamptz | first reference by item; unreferenced after 24 h purged |
| created_at, updated_at | timestamptz | |

Served at `GET /widget/v1/engage-assets/{eas}/{filename}` (public, immutable, CORS `*`, CDN). EXIF orientation applied, then metadata stripped at upload (`framework/image` re-encode).

### announcements

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `ann_` |
| workspace_id | uuid not null | |
| name | text not null | internal, 1..120 |
| format | text not null CHECK in ('banner','modal','chat') | immutable after first publish (422 `format_locked`) |
| status | text not null default 'draft' CHECK in ('draft','live','paused','ended','archived') | state machine below |
| default_locale | text not null | BCP 47, default workspace locale |
| content | jsonb not null | typed `models.AnnouncementContent` (below), max 32 KB |
| translations | jsonb not null default '{}' | `{"de": {partial content}}`, max `engage.max_translations` (10) locales |
| rendered | jsonb not null default '{}' | cache: `{locale: {body_html, renderer_version}}`; regenerated on write and renderer bump |
| audience | jsonb not null | `models.Audience` |
| trigger | jsonb not null | `models.Trigger` |
| frequency | jsonb not null | `models.Frequency` |
| starts_at, ends_at | timestamptz | null start = on publish; null end = open-ended; `ends_at > starts_at` |
| timezone | text not null default workspace tz | display only (dashboard renders local) |
| priority | smallint not null default 50 CHECK 0..100 | higher first |
| bypass_throttle | boolean not null default false | owner/admin only; urgent notices |
| revision | int not null default 1 | bumped on content/audience/trigger change |
| published_at, published_by_member_id | | first publish |
| ended_at | timestamptz | |
| end_reason | text CHECK in ('schedule','manual','segment_deleted','workspace_suspended') | |
| metadata | jsonb not null default '{}' | max 4 KB, API-writable (`source`, external refs) |
| created_by_type, created_by_id, updated_by_type, updated_by_id | | Actor |
| created_at, updated_at, deleted_at | timestamptz | soft delete (rest.md rule 7), purge after 30 days |

Indexes: `(workspace_id, status, deleted_at)`, `(workspace_id, updated_at desc, id)`. Live cap: entitlement `engage.max_live_announcements` (cloud 50); over: 422 `entitlement_limit_reached`.

`content` per format:

```json
{
  "title": "Dark mode is here",
  "body_markdown": "Toggle it in **Settings > Appearance**.",
  "cover_asset_id": "eas_01jb...",
  "sender_member_id": "mem_01jb...",
  "ctas": [
    { "id": "c1", "label": "Try it", "style": "primary", "action": "url", "url": "https://app.acme.com/settings/appearance", "new_tab": false },
    { "id": "c2", "label": "Later", "style": "secondary", "action": "dismiss" }
  ],
  "banner": { "position": "top", "style": "inline", "dismissible": true, "tone": "info" },
  "modal": { "size": "small" },
  "theme": { "accent_color": null }
}
```

- `title` max 120 (banner: optional, max 80). `body_markdown` max 10 KB (banner: max 280 chars, inline Markdown only: bold, italic, links, code).
- `cover_asset_id`: modal only; requires `alt_text` on asset to publish (422 `missing_alt`), decorative allowed with `"cover_decorative": true`.
- `sender_member_id`: chat only, required; falls back to workspace name + logo when member deactivated.
- CTAs: max 2 (banner 1 + dismiss control). `id` `[a-z0-9_]{1,24}` unique in item. `label` 1..40. Actions: `url` (http/https only, `new_tab`), `sdk_event` (`event_name` `[a-z0-9_.:-]{1,64}`, fires SDK event `engage_action {id, ctaId, eventName}`), `open_messenger` (`prefill` max 280; SDK `startConversation(prefill, {context: {announcement_id}})`, never sends by itself), `open_article` (`article_id` `art_`, needs widget help tab; SDK `show('help', {article})`), `open_survey` (`survey_id` `svy_` live; SDK `show('survey', {id})`), `dismiss`, `open_changelog_entry` (`changelog_entry_id` `cle_` published; SDK `show('changelog', {entry})`; needs widget `changelog_tab_enabled` (13), else falls back to entry public URL). Wire JSON keys stay snake_case (`article_id`, `cta_id`); SDK options and event payloads camelCase (`sdk-contract.md`). Target missing at click time (article unpublished, entry unpublished, survey ended): SDK opens fallback (Help home, entry public URL or nothing) and server hides CTA on next fetch.
- `banner.tone` in (`info`, `success`, `warning`, `critical`) maps to theme colors with contrast guaranteed. `style` `inline` (push page content down) or `floating`.
- `theme.accent_color` `#RRGGBB` or null (widget primary). Publish warning `low_contrast` when below 4.5:1 against text color.
- Variables in title/body/CTA label: `{{contact.first_name | fallback: "there"}}`, `{{company.name}}`, same allowlist and renderer as 03 saved replies. Rendered server-side per contact (session list) or with fallback (anonymous list).

### surveys

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `svy_` |
| workspace_id | uuid not null | |
| name | text not null | internal 1..120 |
| kind | text not null CHECK in ('nps','csat','ces','custom') | headline metric in results; from template |
| display | text not null CHECK in ('popover','modal','conversation') | `conversation` = CSAT on close |
| status | text not null default 'draft' CHECK in ('draft','live','paused','ended','archived') | same machine as announcements |
| default_locale, translations, rendered | | as announcements; translations cover intro, thank-you, question prompts, option labels |
| intro | jsonb not null default '{}' | `{title, body_markdown}` optional; popover skips when empty |
| thank_you | jsonb not null default '{}' | `{title, body_markdown, cta}` (one CTA, same action set) |
| settings | jsonb not null default '{}' | `{dismissible: true, show_progress: true, accent_color: null}` |
| audience, trigger, frequency | jsonb | as announcements; ignored for `display = conversation` |
| csat | jsonb | only `display = conversation`: `{delay_minutes: 10 (0..1440), channels: ["widget","email"] (subset of these two), include_ai_only: true, require_member_reply: false, cooldown_days: 7 (0..90), inbox_ids: [] (empty = all), expire_days: 14 (1..30)}` |
| starts_at, ends_at, timezone, priority, bypass_throttle, revision, published_at, published_by_member_id, ended_at, end_reason | | as announcements |
| created_by_*, updated_by_*, created_at, updated_at, deleted_at | | |

Indexes: `(workspace_id, status, deleted_at)`; partial unique `(workspace_id) WHERE display = 'conversation' AND status IN ('live','paused') AND deleted_at IS NULL` (one CSAT survey active per workspace). Live cap: entitlement `engage.max_live_surveys` (cloud 20); over: 422 `entitlement_limit_reached`.

### survey_questions

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `svq_` |
| workspace_id, survey_id | uuid not null | survey fk cascade |
| position | smallint not null | unique `(survey_id, position)` deferrable initially deferred (reorder in one tx) |
| type | text not null CHECK in ('nps','rating','scale','single_choice','multiple_choice','text','statement') | immutable once answers exist (422 `question_type_locked`) |
| prompt | text not null | 1..300, plain text + variables |
| description | text | max 500, inline Markdown |
| required | boolean not null default true | `statement` always false |
| config | jsonb not null default '{}' | per type below |
| branching | jsonb not null default '[]' | ordered rules `[{when: {op, value}, goto: "svq_..."|"end"}]`, max 10 |
| attribute_key | text | write-back target `custom_attributes` key `[a-z0-9_]{1,64}`; reserved `claimed_user_id` refused |
| archived_at | timestamptz | question removed after answers exist: archived (kept for results), not deleted |
| created_at, updated_at | timestamptz | |

Max `engage.max_questions` (20) non-archived per survey. `config` per type:

| Type | Config | Answer value |
|---|---|---|
| nps | `{min_label, max_label}` fixed 0..10 | `value_number` 0..10 |
| rating | `{scale: 3|5, style: "stars"|"emoji"|"numbers"}` | 1..scale |
| scale | `{min: 0|1, max: 2..10, min_label, max_label}` (CES = 1..7) | min..max |
| single_choice | `{options: [{id, label}], randomize, allow_other, display: "list"|"dropdown"}` options 2..20, label 1..120 | `value_choices` 1 id (or `other` + `other_text`) |
| multiple_choice | same + `min_select`, `max_select` | 1..n ids |
| text | `{multiline: true, max_length: 1..2000 (default 1000), placeholder}` | `value_text` |
| statement | `{cta_label}` | none |

Option ids `[a-z0-9_]{1,24}`; removing option that has answers archives it (`archived: true`, hidden from new respondents, kept in results).

Branching: evaluated after answer. `op` in (`eq`, `in`, `gte`, `lte`, `between`, `answered`, `skipped`); choice questions use option ids. First match wins; no match = next position. `goto` must reference later non-archived question or `end` (422 `branch_backwards`). Forward-only guarantees termination.

### survey_responses

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `svr_` |
| workspace_id, survey_id | uuid not null | survey fk cascade |
| survey_revision | int not null | revision at start |
| contact_id | uuid null references contacts on delete set null | null for anonymous API response (`contact: null`) or erased contact |
| widget_session_id | uuid null references widget_sessions on delete set null | |
| conversation_id | uuid null references conversations on delete set null | CSAT |
| request_message_id | uuid null references messages on delete set null | CSAT request message |
| source | text not null CHECK in ('widget','conversation_widget','conversation_email','api') | |
| status | text not null CHECK in ('requested','partial','completed','expired') | `requested` only CSAT (row created at send) |
| cycle | int not null default 1 | recurring frequency cycle number for contact |
| client_id | text | unique `(workspace_id, survey_id, client_id) WHERE client_id IS NOT NULL` |
| locale | text | locale shown |
| page_url | text | scrubbed like 01 `current_page` |
| country | text | from session |
| ai_involved | boolean | CSAT: conversation had any agent message |
| is_test | boolean not null default false | test session or test conversation; excluded from results by default |
| context | jsonb not null default '{}' | max 2 KB: widget page context snapshot, `assignee_member_id`, `inbox_id` (CSAT) |
| requested_at, started_at, completed_at, expires_at | timestamptz | |
| created_at, updated_at | timestamptz | |

Indexes: `(workspace_id, survey_id, status, completed_at desc, id)`, `(workspace_id, contact_id) WHERE contact_id IS NOT NULL`, `(workspace_id, conversation_id) WHERE conversation_id IS NOT NULL`, unique `(workspace_id, survey_id, contact_id, cycle) WHERE contact_id IS NOT NULL AND source IN ('widget','api')` (one in-app response per cycle), unique `(workspace_id, conversation_id, request_message_id) WHERE conversation_id IS NOT NULL`.

### survey_answers

| Column | Type | Notes |
|---|---|---|
| response_id | uuid not null references survey_responses on delete cascade | pk part |
| question_id | uuid not null references survey_questions on delete cascade | pk part |
| workspace_id, survey_id | uuid not null | denormalized for indexes |
| value_number | numeric(4,1) | nps, rating, scale |
| value_choices | text[] | option ids |
| value_text | text | max 2,000 (text answers and `other_text`) |
| skipped | boolean not null default false | |
| answered_at | timestamptz not null | |
| search | tsvector generated stored from `value_text` (`dimmi_search`) | |

Primary key `(response_id, question_id)`. Indexes: `(workspace_id, question_id, value_number)`, GIN `(workspace_id, search)` (btree_gin), GIN `(workspace_id, value_choices)`.

Sensitive data redaction before insert: digit runs 13..19 passing Luhn replaced by `[redacted card number]`; strings matching 07/08 secret patterns replaced `[redacted secret]`; `metadata` on response gets `redacted: true`.

### engage_receipts

Per contact per item state. Only contacts with widget session (leads or users). Anonymous visitors without session never get rows.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `erc_` |
| workspace_id, contact_id | uuid not null | contact fk cascade |
| subject_type | text not null CHECK in ('announcement','survey') | |
| subject_id | uuid not null | polymorphic, no FK; deleted with item purge |
| cycle | int not null default 1 | recurring frequency |
| first_seen_at, last_seen_at | timestamptz | |
| impressions | int not null default 0 | |
| dismissed_at, clicked_at | timestamptz | |
| last_action_id | text | CTA id |
| completed_at | timestamptz | survey completed; announcement: CTA clicked or chat replied |
| next_eligible_at | timestamptz | precomputed from frequency; null = eligible now; `infinity` = never |
| is_test | boolean not null default false | |
| created_at, updated_at | timestamptz | |

Unique `(workspace_id, contact_id, subject_type, subject_id)`. Index `(workspace_id, subject_type, subject_id, first_seen_at desc, id)` (recipients list). Eligibility reads all receipts of one contact: bounded by live items (`PruneEngageReceipts` removes receipts of items ended over `engage.receipts_retention_days` (365) ago).

### engage_stats_daily

Aggregates, no PII. Written only by in-memory tracker flush (upsert add). Counters buffered in process, flushed to Postgres (durable; not Valkey: analytics survive Valkey flush).

| Column | Type | Notes |
|---|---|---|
| workspace_id, subject_type, subject_id, day (date, UTC) | | pk |
| impressions, unique_viewers, anonymous_impressions, dismissals, clicks, replies, responses_started, responses_completed, requests_sent, requests_expired | bigint not null default 0 | `unique_viewers` = new receipts that day (session contacts only) |
| cta_clicks | jsonb not null default '{}' | `{cta_id: n}` |
| is_test | boolean | pk part: test traffic counted separately, hidden by default |

### Core table changes

None. 15 adds no columns to core tables. Uses existing jsonb keys:
- `messages.metadata.survey_request` (new documented key, written by 15 through 03 action): `{survey_id, response_id}`.
- `messages.event` name `announcement.replied` (event row at start of conversation created from chat-format reply), payload `{announcement_id, title_snapshot}`.
- `conversations.source.announcement_id` (new documented key) for conversations started by replying to chat announcement.

Each listed under "Required from other specs" for owner acknowledgement.

## Audience rule language

`models.Audience`:

```json
{
  "visitors": "identified",
  "segment_id": "seg_01jb...",
  "rules": { "all": [
    { "field": "contact.custom.plan", "op": "in", "value": ["pro", "team"] },
    { "field": "contact.custom.signed_up_at", "op": "before", "value": "2026-10-01T00:00:00Z" },
    { "any": [
      { "field": "contact.last_seen_at", "op": "within_last_days", "value": 30 },
      { "field": "company.custom.seats", "op": "gte", "value": 10 }
    ] }
  ] },
  "locales": ["en", "de"]
}
```

- `visitors` in (`everyone`, `identified` (kind user, verified), `leads` (kind lead with session), `anonymous_and_leads`). Anonymous no-session visitors only reachable when `visitors` in (`everyone`, `anonymous_and_leads`) AND no `segment_id`, AND `rules` reference only anonymous-safe fields (`session.locale`, `session.country`). Else item never in anonymous list.
- `segment_id` and inline `rules` combine with AND. Segment rules resolved at evaluation time (segment edit applies to live items immediately).
- Tree: `all`/`any` nodes, max depth 3, max 30 leaf conditions total (segment + inline).
- `locales`: optional allowlist matched against resolved contact locale primary subtag.

Fields (`contact.custom.signed_up_at` is plain custom attribute; 02 convention makes it a date):

| Field | Type | Ops |
|---|---|---|
| contact.kind | enum user, lead | eq, neq |
| contact.email | string | exists, not_exists, ends_with (domain), eq |
| contact.email_verified | bool | is_true, is_false |
| contact.name, contact.phone, contact.external_id | string | exists, not_exists |
| contact.locale, contact.timezone | string | eq, neq, in, starts_with |
| contact.country | ISO 3166 | eq, in, not_in |
| contact.created_at, contact.first_seen_at, contact.last_seen_at | date | before, after, within_last_days, more_than_days_ago |
| contact.custom.`<key>` | by JSON type of value | string: eq, neq, in, not_in, contains, starts_with, exists, not_exists; number: eq, neq, gt, gte, lt, lte, between; bool: is_true, is_false; date (ISO 8601 string or unix seconds number, op decides): before, after, within_last_days, more_than_days_ago |
| company.name, company.domain | string | eq, contains, ends_with, exists, not_exists |
| company.created_at | date | date ops |
| company.custom.`<key>` | as contact.custom | |
| contact.has_company | bool | is_true, is_false |
| session.locale, session.country | string | eq, in (anonymous-safe) |

Semantics (identical in Go evaluator and SQL compiler; property test compares both on 10,000 random contacts):
- Missing value: every op false except `not_exists`, `neq`, `not_in` (true). Type mismatch (custom string compared with `gt`): false, never error.
- Strings compared case-insensitively after NFC, `contains` max 100 chars value.
- Dates: unix seconds when number (02 claim `signed_up_at` convention), else RFC 3339; relative ops use request time (`a.Clock`, `framework/clock`).
- `in` max 50 values.

SQL compiler (`app/support/segments/compile.go`) emits parameterized predicates over `contacts` left join `companies` via `app/support/querybuilder` only. Used by preview, segment contacts list, 03 `filter[segment]`. Go evaluator (`app/support/segments/evaluate.go`) used by widget eligibility.

## Trigger and frequency

`models.Trigger`:

```json
{ "on": "page_view", "url_rules": [ { "op": "starts_with", "value": "/dashboard" }, { "op": "not_contains", "value": "/dashboard/billing" } ], "url_match": "all", "delay_seconds": 5, "device": "any", "event_name": null }
```

- `on` in (`page_view`, `track_event`, `manual`). `manual` = only via SDK `show('announcement', {id})` / `show('survey', {id})`, CTA `open_survey`, or deep link `#dimmi-announcement=ann_...` / `#dimmi-survey=svy_...`.
- `url_rules` max 10; ops `equals`, `contains`, `not_contains`, `starts_with`, `matches` (glob: `*` any chars, `**` across `/`; no regex, no ReDoS). Matched against URL path + query (host ignored unless value starts with `https://`). Empty = every page.
- `delay_seconds` 0..3600 on page (timer reset on SPA navigation that fails rules).
- `device` in (`any`, `desktop`, `mobile`) from client viewport width (640 px).
- `event_name` for `track_event`, same charset as SDK event names.

`models.Frequency`:

```json
{ "mode": "until_engaged", "max_impressions": 3, "min_hours_between": 24, "recur_days": null }
```

- `once` (default modal/chat/survey): show until first impression; then never.
- `until_engaged` (default banner): show on each eligible page view until dismissed, CTA clicked, or completed; at most `max_impressions` (1..20) impressions counted at most once per `min_hours_between` (0..720) for counting.
- `recurring` (surveys only): after completion or dismissal, eligible again after `recur_days` (7..365) as new cycle.
- Survey always stops for cycle on completion; dismissal ends cycle when `settings.dismissible`.

`next_eligible_at` precomputed on each receipt write so eligibility check is one comparison.

## Eligibility engine

Action `engage.ResolveEligibleItems(ctx, contact, session, opts)`; one shared code path for widget session list, preview, and tests.

```
items  = LiveItemsCache.Get(workspace, engage_settings.version)   // otter L1 per replica (version in key; bust via Valkey pubsub), compiled rules; miss: 2 queries (announcements, surveys) + segments
now    = a.Clock.Now()
recs   = receipts by (workspace, contact)                          // 1 indexed query, bounded
ctxVal = contact + company (already loaded by auth:widget) + session locale/country
throttle = count recs with last_seen_at > now-24h where interruptive and not bypass; last interruptive seen time
survey_cooldown = max(completed_at, dismissed_at) of in-app survey receipts
for item in items (priority desc, published_at asc):
  skip if now outside [starts_at, ends_at)
  skip if display == conversation                                  // CSAT is server-driven only
  skip if audience.visitors excludes contact.kind/session kind      // test sessions see live items like real contacts; receipts is_test
  skip if !Evaluate(audience rules, ctxVal)
  skip if rec.next_eligible_at > now
  skip if interruptive and !bypass and (throttle >= max or last interruptive within min_minutes)
  skip if survey and !bypass and survey_cooldown active
return items with per-item state {cycle, impressions, rendered content for locale}
```

- Throttled items omitted, not flagged. `meta.throttle = {remaining_today, next_slot_at}` tells client when refetch can return more.
- Response size cap 64 KB; over cap: lowest priority items dropped (`meta.truncated`).
- ETag = hash(engage version, contact attributes hash, company `updated_at`, receipts max `updated_at`, locale). 304 when unchanged.
- Client refetch: on full page load (after handshake, idle), on `identity_changed`, on `Dimmi.update({context|locale})`, on throttle `next_slot_at`, at most every `engage.client_refresh_interval` (600 s) in long SPA sessions.
- Client shows at most one interruptive item at a time and one banner per position. Ties: priority, then server order.

Anonymous list `GET /widget/v1/apps/{app}/engage-items`: items whose audience is anonymous-safe, rendered with variable fallbacks, `Cache-Control: public, s-maxage=60`, ETag = engage version + locale. Frequency, throttle, cooldown kept in `localStorage` key `dimmi:{app}:engage` (receipts mirror, 50 newest, pruned). After visitor gets session (first write), next fetch uses session list; SDK uploads local receipts once in `POST /widget/v1/engage-events` (`kind: "seen"` with original timestamps, max 7 days old) so server receipts stay consistent.

Config hook (01): config resource gains `engage: {active, anonymous, version}` from `engage_settings.live_summary`. SDK fetches lists only when `active` (session) or `anonymous` (no session). Workspaces not using engage: zero requests.

## Engage events (impressions, clicks, dismissals)

`POST /widget/v1/engage-events` (batched, 202). Kinds: `seen`, `dismissed`, `clicked` (`cta_id` on wire; SDK event `announcement_clicked {id, ctaId}`), `closed` (closed without dismiss, e.g. navigation; counts nothing). Session present: upsert receipts in one statement (`INSERT ... ON CONFLICT DO UPDATE`), recompute `next_eligible_at`, add to tracker. No session: tracker counters only (`anonymous_impressions`, dismissals, clicks), no rows, no identity.

In-memory tracker `app/support/engagestats` (pattern of 01 install tracker): per replica map `(workspace, subject, day, is_test) -> counters`, flushed every `engage.stats_flush_interval` (60 s) as one batched upsert; flush on shutdown. Crash loses at most 60 s of counters (analytics, not billing). Receipts are durable (written per request).

## CSAT on conversation close

Flow:
1. Listener `ScheduleCsatRequest` (Queued, `default`) on `conversation.closed`: when workspace has live `display = conversation` survey, insert job `SendCsatRequest` `ScheduledAt = closed_at + delay_minutes`, unique by `(conversation_id, closed_at)`.
2. Worker re-checks (all must hold, else skip with reason metric): conversation still closed, not spam, not deleted, `is_test` false (unless survey preview), contact not blocked, contact not erased, channel type in `csat.channels`, inbox in `csat.inbox_ids` (if set), at least one contact message, at least one workspace reply (`require_member_reply` true: member reply; `include_ai_only` false: skip conversations with only agent replies), no existing request for this close cycle, contact has no CSAT request within `cooldown_days`, workspace not suspended, survey still live, close not from import (16 writes imported conversations without `conversation.closed`; defense: skip when conversation `source.imported` and no live message after import, 16 E45). Email channel: contact email not suppressed (04 `email.IsSuppressed(ctx, ws, email)`, default scope `all`).
3. Create `survey_responses` row `status = requested`, `expires_at = now + expire_days`, then call 03 `SendSystemMessage` (one tx): message `type message`, `author_type system`, `substantive false`, body Markdown with rating links (email), `metadata.survey_request = {survey_id, response_id}`. Channel outbound pipeline delivers (widget realtime card on `widget_contact:{ctc}`, email via 04 `Channel.Send` in same thread, `outbound_emails.kind system`; no `SendContactMail` in thread).
4. Answer paths: widget card (`POST/PATCH /widget/v1/surveys/{svy}/responses` with `response_id`), email link to minimal CSAT rating page with signed URL (`/csat-ratings/{svr}?score=4&expires=...&signature=...`, `a.URL.SignedRoute`; signature binds `svr` + `expires`, `score` unsigned preselect only). GET only renders: first question with score preselected plus next question when it is `text` (comment). POST to same signed URL stores answers, completes response (later questions `not_shown`), renders thanks. Link scanners never answer. Not a general hosted survey page (later: hosted-survey-page).
5. Completion: `survey_response.completed` event; realtime (centrifuge Redis broker on Valkey, `framework/broadcasting`) to `conversation:{conv}` (inbox shows rating) and `workspace:{ws}`; 06 notifies on negative rating (first question value at most 2 of 5 or 0..6 NPS).
6. Expiry: `ExpireSurveyRequests` marks `requested` past `expires_at` as `expired` (rating page then shows "This survey closed").

Email body (rendered by 04 as normal outbound reply in thread): question + 5 linked labels ("1 Very bad" ... "5 Very good"), plain-text twin lists URLs. No tracking pixel.

Customer replies to CSAT email with text: 04 inbound threads into conversation as customer message (normal reopen rules). 15 does nothing special.

## Chat-format reply

Widget sends `POST /widget/v1/conversations` with `announcement_id` and first message (01 edit). 01 `StartWidgetConversation` validates announcement live/recent (format chat, contact has receipt), passes to 03 `ReceiveInboundMessage`: conversation `initiated_by = contact`, `source.announcement_id`, timeline begins with `type event` row `announcement.replied` (title snapshot, sender), then contact message. Receipt `completed_at` set, stats `replies + 1`. Billing: ordinary D1 contact-initiated conversation; no D7 fair-use count (not teammate-initiated).

## Code map

```
routes/api.go        # + engage routes (api.md)
routes/widget.go     # + engage widget routes, engage-assets route
routes/web.go        # + /csat-ratings/{svr} (CSAT email rating page)
routes/console.go    # + transition_engage_schedules (every minute), expire_survey_requests (hourly), prune_engage_receipts (daily 03:40 UTC), purge_engage_items (daily)
routes/mcp.go        # + toolset engage
routes/channels.go   # none (uses workspace + conversation channels)

config/engage.go     # EngageConfig

app/enums/
  engage_status.go, announcement_format.go, survey_display.go, survey_kind.go,
  question_type.go, response_status.go, response_source.go, frequency_mode.go, trigger_on.go,
  audience_visitors.go, cta_action.go, engage_event_kind.go, end_reason.go

app/models/  (sqlc + behavior)
  announcement.go (AnnouncementContent, typed jsonb), survey.go, survey_question.go (QuestionConfig per type),
  audience.go (Audience, RuleNode), trigger.go, frequency.go (NextEligibleAt), engage_settings.go

app/support/segments/
  rules.go            # parse + validate rule tree, field registry, limits
  evaluate.go         # Go evaluator (contact, company, session)
  compile.go          # SQL compiler via querybuilder
app/support/engagestats/  # in-memory counters + flush
app/support/urlmatch/     # glob matcher shared with SDK test vectors (JSON fixtures consumed by widget tests)
app/support/redact/       # Luhn card + secret pattern redaction (shared with 08 secret scanner patterns)

app/actions/engage/
  ensure_engage_settings.go          # EnsureEngageSettings (lazy upsert)
  update_engage_settings.go
  bump_engage_version.go             # BumpEngageVersion: version+1, recompute live_summary (called in every writing tx)
  create_segment.go, update_segment.go, delete_segment.go, preview_segment.go, list_segment_contacts.go
  upload_engage_asset.go, update_engage_asset.go, delete_engage_asset.go   # bytes via `app/support/images` (08) on `framework/image`, scan `a.Scanner`, disk `a.Storage.Disk("public")`
  create_announcement.go, update_announcement.go, delete_announcement.go, restore_announcement.go, duplicate_announcement.go
  publish_engage_item.go             # PublishEngageItem: validate publish rules (alt text, CTA targets, caps, segment exists), status live, published_at
  pause_engage_item.go, resume_engage_item.go, end_engage_item.go, archive_engage_item.go
  create_survey.go, update_survey.go, delete_survey.go, duplicate_survey.go
  create_survey_question.go, update_survey_question.go, delete_survey_question.go (archive when answered), reorder_survey_questions.go
  validate_survey_graph.go           # branching forward-only, required paths
  render_engage_content.go           # Markdown -> safe HTML per locale (`framework/markdown` + `framework/sanitize` with 08 `helpmarkdown.Engage()` options), variables
  resolve_eligible_items.go          # eligibility engine
  resolve_anonymous_items.go
  record_engage_events.go            # receipts upsert + tracker
  start_survey_response.go           # create partial (or attach to requested CSAT row)
  answer_survey_response.go          # validate answers vs questions + branching path, redact, save step
  complete_survey_response.go        # status completed, receipt completed, attribute write-back, event
  submit_api_survey_response.go      # REST source api (one-shot)
  schedule_csat_request.go, send_csat_request.go, expire_survey_requests.go
  compute_survey_results.go          # aggregates, NPS, funnel, cached 60 s in `cache.Store` (Valkey) keyed by survey + filters + response max id
  export_survey_responses.go         # CSV stream
  csat_rating_url.go                 # signed CSAT email rating URL via `a.URL.SignedRoute` (framework/routing)
  submit_csat_rating.go              # CSAT rating page POST: answers first question (+ text comment), completes response
  create_engage_preview.go           # calls 01 MintWidgetTestSession, returns preview URL
  repoint_engage_contact_data.go     # contact.merged
  erase_engage_contact_data.go       # GDPR eraser hook
  transition_engage_schedules.go     # end items past ends_at, recompute live_summary

app/http/controllers/api/v1/
  engage_settings_controller.go             # Show, Update                 /engage-settings
  segment_controller.go                     # Index, Store, Show, Update, Destroy
  segment_contact_controller.go             # Index                        /segments/{seg}/contacts
  segment_preview_controller.go             # Store                        /segment-previews
  engage_asset_controller.go                # Index, Store, Update, Destroy
  announcement_controller.go                # Index, Store, Show, Update, Destroy
  announcement_duplicate_controller.go      # Store
  announcement_preview_controller.go        # Store
  announcement_stats_controller.go          # Show
  announcement_receipt_controller.go        # Index
  survey_controller.go                      # Index, Store, Show, Update, Destroy
  survey_question_controller.go             # Index, Store, Update, Destroy, UpdateMany (order)
  survey_duplicate_controller.go, survey_preview_controller.go
  survey_response_controller.go             # Index (json/csv), Store (api source), Show, Destroy
  survey_result_controller.go               # Show                         /surveys/{svy}/results
  survey_receipt_controller.go              # Index
  survey_template_controller.go             # Index
  conversation_survey_response_controller.go # Index                       /conversations/{conv}/survey-responses
  contact_survey_response_controller.go     # Index                        /contacts/{ctc}/survey-responses
  contact_engage_receipt_controller.go      # Index                        /contacts/{ctc}/engage-receipts

app/http/controllers/widget/
  app_engage_item_controller.go             # Index  GET /apps/{app}/engage-items (anonymous, CDN)
  engage_item_controller.go                 # Index  GET /engage-items (session)
  widget_announcement_controller.go         # Show   GET /announcements/{ann} (manual/deep link)
  widget_survey_controller.go               # Show   GET /surveys/{svy}
  engage_event_controller.go                # Store  POST /engage-events
  widget_survey_response_controller.go      # Store, Update
  engage_asset_file_controller.go           # Show   GET /engage-assets/{eas}/{filename}

app/http/controllers/web/
  csat_rating_controller.go                 # Show GET /csat-ratings/{svr}; Store POST /csat-ratings/{svr} (signed URL)

resources/views/csat_ratings/               # templ: page.templ, closed.templ, thanks.templ (zero JS)

app/http/requests/
  update_engage_settings_request.go, store_segment_request.go, update_segment_request.go, store_segment_preview_request.go,
  store_engage_asset_request.go, update_engage_asset_request.go,
  store_announcement_request.go, update_announcement_request.go, store_engage_preview_request.go,
  store_survey_request.go, update_survey_request.go, store_survey_question_request.go, update_survey_question_request.go,
  reorder_survey_questions_request.go, index_survey_responses_request.go, store_api_survey_response_request.go,
  show_survey_results_request.go,
  index_widget_engage_items_request.go, store_engage_events_request.go,
  store_widget_survey_response_request.go, update_widget_survey_response_request.go, store_csat_rating_request.go

app/http/resources/
  engage_settings_resource.go, segment_resource.go, segment_preview_resource.go, engage_asset_resource.go,
  announcement_resource.go, announcement_stats_resource.go, engage_receipt_resource.go,
  survey_resource.go (with questions), survey_question_resource.go, survey_response_resource.go (answers, contact compact gated by contacts:read),
  survey_results_resource.go, survey_template_resource.go, engage_preview_resource.go,
  widget_engage_item_resource.go (widget-safe: no audience, no internal name, no stats), widget_survey_resource.go, widget_survey_response_resource.go

app/policies/
  segment_policy.go, announcement_policy.go, survey_policy.go (incl. publish, bypassThrottle, deleteResponse, export), engage_settings_policy.go

app/events/
  segment_created.go, segment_updated.go, segment_deleted.go,
  announcement_created.go, announcement_updated.go, announcement_status_changed.go, announcement_deleted.go,
  survey_created.go, survey_updated.go, survey_status_changed.go, survey_deleted.go,
  survey_response_created.go, survey_response_completed.go, survey_response_deleted.go,
  engage_settings_updated.go

app/listeners/
  bump_widget_config_for_engage.go          # AfterCommit: engage version change -> 01 config cache bust (`pubsub.PubSub` on Valkey, same topic 01 uses)
  schedule_csat_request.go                  # Queued: conversation.closed
  cancel_csat_on_reopen.go                  # Queued: conversation.reopened -> mark pending job skip (worker re-check covers; listener cancels River job when possible)
  repoint_engage_on_contact_merge.go        # InTx: contact.merged
  end_items_on_segment_delete.go            # InTx guard: segment delete refused when in use (no listener needed); kept for archived references
  pause_engage_on_workspace_suspend.go      # AfterCommit: workspace.suspended -> live_summary inactive (items keep status)

app/jobs/
  send_csat_request.go, expire_survey_requests.go, transition_engage_schedules.go,
  prune_engage_receipts.go, purge_engage_items.go, process_engage_asset.go

app/exceptions/engage.go   # ErrSegmentInUse, ErrFormatLocked, ErrQuestionTypeLocked, ErrBranchBackwards, ErrSurveyNotLive, ErrResponseCompleted, ErrResponseExpired, ErrMissingAlt, ...

app/mcp/tools/
  dimmi_list_announcements.go, dimmi_create_announcement.go, dimmi_update_announcement.go,
  dimmi_list_surveys.go, dimmi_create_survey.go, dimmi_get_survey_results.go, dimmi_list_survey_responses.go,
  dimmi_preview_segment.go

app/client/commands/   # CLI mode (09 owns framework)
  announcements.go (list, show, create --file, update, publish, pause, end), surveys.go (list, results, responses --csv), segments.go (list, preview)

database/migrations/20261101001501..1510_*.sql
database/queries/engage_settings.sql, segments.sql, engage_assets.sql, announcements.sql, surveys.sql,
  survey_questions.sql, survey_responses.sql, survey_answers.sql, engage_receipts.sql, engage_stats_daily.sql
database/factories/segment_factory.go, announcement_factory.go, survey_factory.go, survey_response_factory.go, engage_receipt_factory.go
database/seeders/engage_seeder.go   # demo: banner, NPS survey, CSAT survey

tests/feature/api/engage/*_test.go          # one per controller
tests/feature/widget/engage_*_test.go
tests/feature/web/csat_rating_test.go
tests/feature/engage/segments_test.go        # rules validation, Go evaluator, SQL compiler + parity property test (Go vs SQL on real Postgres, 10,000 factory contacts)
tests/feature/engage/urlmatch_test.go        # shared JSON vectors with widget (pure, table-driven, no mocks)
tests/feature/engage/*_test.go               # eligibility, frequency, throttle, branching, write-back, CSAT gating, redaction: real App (`testkit.New(t)`), real Postgres, real Valkey (per-test key prefix, never faked), real dispatcher + River (`tc.Queue.Drain`); fakes only at boundaries (`tc.Mail`, `tc.Clock`). No unit tests with mocks (testing.md). Test names carry edge ids (`t.Run("E62 ...")`)
tests/e2e/widget_engage_protocol_test.go     # Node: config flag, list, events, response, anonymous CDN list
```

## HTTP API (summary)

Full shapes in [api.md](api.md). Auth letters per `00-foundation/api.md`. All admin routes `A` + `workspace` + `throttle:api`; POST routes `idempotent`.

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | /engage-settings | engage-settings.show | A engage:read | throttle settings, live summary |
| PATCH | /engage-settings | engage-settings.update | A engage:write + `engage:manage` | |
| GET | /segments | segments.index | A engage:read | |
| POST | /segments | segments.store | A engage:write | |
| GET/PATCH/DELETE | /segments/{seg} | segments.show/update/destroy | A engage:read / engage:write | delete refused when in use |
| GET | /segments/{seg}/contacts | segments.contacts.index | A engage:read + contacts:read | matching contacts, cursor |
| POST | /segment-previews | segment-previews.store | A engage:read | count + 10 sample (rules or segment_id) |
| GET | /engage-assets | engage-assets.index | A engage:read | |
| POST | /engage-assets | engage-assets.store | A engage:write | multipart, 5 MB |
| PATCH/DELETE | /engage-assets/{eas} | engage-assets.update/destroy | A engage:write | alt text; delete refused when referenced by non-archived item |
| GET | /announcements | announcements.index | A engage:read | filters status, format, q |
| POST | /announcements | announcements.store | A engage:write | creates draft (or `status: live` with `engage:manage`) |
| GET | /announcements/{ann} | announcements.show | A engage:read | |
| PATCH | /announcements/{ann} | announcements.update | A engage:write | fields + `status` (live, paused, ended, archived), `trashed` |
| DELETE | /announcements/{ann} | announcements.destroy | A engage:write | soft delete; live must be ended first |
| POST | /announcements/{ann}/duplicates | announcements.duplicates.store | A engage:write | copy as draft |
| POST | /announcements/{ann}/previews | announcements.previews.store | A engage:read | test session + preview URL |
| GET | /announcements/{ann}/stats | announcements.stats.show | A engage:read | totals + daily |
| GET | /announcements/{ann}/receipts | announcements.receipts.index | A engage:read + contacts:read | who saw/clicked/dismissed |
| GET | /surveys | surveys.index | A engage:read | |
| POST | /surveys | surveys.store | A engage:write | optional `template`, inline `questions` |
| GET/PATCH/DELETE | /surveys/{svy} | surveys.show/update/destroy | A | as announcements |
| GET | /surveys/{svy}/questions | surveys.questions.index | A engage:read | |
| POST | /surveys/{svy}/questions | surveys.questions.store | A engage:write | |
| PATCH | /surveys/{svy}/questions | surveys.questions.update-many | A engage:write | `{order: [...]}` (rest.md rule 9) |
| PATCH/DELETE | /surveys/{svy}/questions/{svq} | surveys.questions.update/destroy | A engage:write | delete archives when answered |
| POST | /surveys/{svy}/duplicates, /surveys/{svy}/previews | surveys.duplicates.store, surveys.previews.store | A | |
| GET | /surveys/{svy}/responses | surveys.responses.index | A engage:read | JSON or `format=csv`; filters |
| POST | /surveys/{svy}/responses | surveys.responses.store | A engage:write | API-submitted response (`source api`) |
| GET/DELETE | /surveys/{svy}/responses/{svr} | surveys.responses.show/destroy | A engage:read / engage:write + `engage:manage` | |
| GET | /surveys/{svy}/results | surveys.results.show | A engage:read | aggregates, NPS, funnel |
| GET | /surveys/{svy}/receipts | surveys.receipts.index | A engage:read + contacts:read | |
| GET | /survey-templates | survey-templates.index | A engage:read | static templates |
| GET | /conversations/{conv}/survey-responses | conversations.survey-responses.index | A conversations:read | CSAT on conversation |
| GET | /contacts/{ctc}/survey-responses | contacts.survey-responses.index | A contacts:read + engage:read | |
| GET | /contacts/{ctc}/engage-receipts | contacts.engage-receipts.index | A contacts:read + engage:read | |

Widget (`/widget/v1`; `app` = public app id + origin; `widget` = bearer session):

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | /apps/{app}/engage-items | widget.apps.engage-items.index | app | anonymous-safe items, CDN 60 s |
| GET | /engage-items | widget.engage-items.index | widget | eligible items for contact, ETag |
| GET | /announcements/{ann} | widget.announcements.show | widget or app | manual/deep link; audience still enforced |
| GET | /surveys/{svy} | widget.surveys.show | widget or app | manual, CSAT card, `open_survey` |
| POST | /engage-events | widget.engage-events.store | app or widget | batched seen/dismissed/clicked, 202 |
| POST | /surveys/{svy}/responses | widget.surveys.responses.store | widget | start or answer (creates or attaches CSAT row) |
| PATCH | /surveys/{svy}/responses/{svr} | widget.surveys.responses.update | widget | more answers, `status: completed` |
| GET | /engage-assets/{eas}/{filename} | widget.engage-assets.show | none | public image |

Web (`routes/web.go`, rest.md exception row requested):

| Method | Path | Route name | Auth |
|---|---|---|---|
| GET | /csat-ratings/{svr} | csat-ratings.show | signed URL (`svr`, `expires`) |
| POST | /csat-ratings/{svr} | csat-ratings.store | same signed URL |

## MCP tools (toolset `engage`, under 2,000 tokens)

| Tool | Ability | Risk | REST |
|---|---|---|---|
| `dimmi_list_announcements` | engage:read | read | GET /announcements (+ stats summary) |
| `dimmi_create_announcement` | engage:write | write | POST /announcements (always draft via MCP) |
| `dimmi_update_announcement` | engage:write | write; `status: live`/`paused`->`live` and `bypass_throttle` changes are customer-visible: 09 confirm gate (MVP always confirms; send modes later: agent-send-modes) | PATCH /announcements/{ann} |
| `dimmi_list_surveys` | engage:read | read | GET /surveys |
| `dimmi_create_survey` | engage:write | write | POST /surveys (draft, template, questions) |
| `dimmi_get_survey_results` | engage:read | read | GET /surveys/{svy}/results |
| `dimmi_list_survey_responses` | engage:read | read | GET /surveys/{svy}/responses; text answers in untrusted envelope |
| `dimmi_preview_segment` | engage:read | read | POST /segment-previews (count only, no sample PII) |

Publishing (`status: live`, `resume`) via MCP is customer-visible: 09 confirm gate rule applies (elicitation then `confirm` param). No MCP delete.

## CLI (09 framework, client mode)

```
dimmi announcements list [--status live] [--json]
dimmi announcements create --file launch.md [--format modal|banner|chat] [--segment seg_...] [--publish] [--dry-run]
dimmi announcements update ann_... --file launch.md
dimmi announcements publish|pause|end ann_...        # PATCH status
dimmi surveys list
dimmi surveys results svy_... [--since 30d] [--json]
dimmi surveys responses svy_... --csv > nps.csv
dimmi segments list | dimmi segments preview seg_...
```

`--file` Markdown with YAML front matter: `name`, `format`, `title`, `ctas`, `audience`, `trigger`, `starts_at`, `ends_at`, `locale`; body = Markdown after front matter. `--publish` requires interactive confirm unless `--yes`.

## Events

Emitted (new, add to catalog):

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| segment.created / segment.updated / segment.deleted | yes (workspace) | no | |
| announcement.created / announcement.updated / announcement.deleted | yes (workspace) | no | |
| announcement.status_changed | yes | no | `{from, to, reason}`; audit |
| survey.created / survey.updated / survey.deleted | yes | no | |
| survey.status_changed | yes | no | audit |
| survey_response.created | yes (`workspace:{ws}`; `conversation:{conv}` when CSAT) | no | partial start or CSAT `requested` |
| survey_response.completed | yes (`workspace:{ws}`; `conversation:{conv}` when CSAT) | **yes** | resource with answers, survey compact, contact compact (null for anonymous API response), conversation id. Brief alias "survey.response_created" maps here |
| survey_response.deleted | yes | no | admin delete or erase |
| engage_settings.updated | yes | no | audit |

Widget channel (`widget_contact:{ctc}`): no new events. CSAT card arrives as `message.created` (metadata `survey_request`); card state after answering in other device via `message.updated`? No: widget fetches `GET /widget/v1/surveys/{svy}?response_id=` on render; completed shows thanks state.

Consumed: `conversation.closed` (CSAT schedule), `conversation.reopened` (cancel pending CSAT), `contact.merged` (repoint), `contact.deleted` (eraser runs via hook, not listener), `workspace.suspended` / `workspace.unsuspended` (live summary), `member.deactivated` (chat sender fallback, cache bust), `article.unpublished` / `article.deleted` (CTA `open_article` validity warning; item keeps running, CTA hidden in widget).

Audit (WriteAuditLog): status changes, `bypass_throttle` changes, deletes, response deletes, CSV exports, engage settings, segment deletes.

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| SendCsatRequest | default | listener, scheduled `closed_at + delay` | 5, backoff | unique `(conversation_id, closed_at)` |
| ExpireSurveyRequests | maintenance | hourly | 3 | periodic singleton; batches 1,000 |
| TransitionEngageSchedules | default | every minute | 3 | periodic singleton; ends items past `ends_at` (status ended, reason schedule), recompute `live_summary` when `next_transition_at <= now`, bump version |
| ProcessEngageAsset | default | upload | 3 | per asset: decode, dims, EXIF strip, scan |
| PruneEngageReceipts | maintenance | daily 03:40 UTC | 3 | receipts of items ended over 365 days, orphan receipts of purged items |
| PurgeEngageItems | maintenance | daily | 3 | soft-deleted items over 30 days: hard delete (responses cascade), orphan assets unreferenced 24 h |

## Config keys

`config/engage.go`, env `DIMMI_ENGAGE__`.

| Key | Default | Notes |
|---|---|---|
| engage.enabled | true | false: config `engage.active=false`, routes 404 `feature_disabled` |
| engage.max_questions | 20 | |
| engage.max_translations | 10 | |
| engage.assets.max_mb | 5 | |
| engage.stats_flush_interval | 60 s | |
| engage.items_cache_ttl | 30 s | otter L1, plus version key; Valkey pubsub bust |
| engage.client_refresh_interval | 600 s | returned to SDK |
| engage.anonymous_max_age | 60 s | CDN `s-maxage` |
| engage.list_max_bytes | 65536 | |
| engage.receipts_retention_days | 365 | after item end |
| engage.response_text_max | 2000 | |
| engage.csat.request_expiry_days_max | 30 | |
| engage.rate.events_per_session_minute | 60 | `a.Limiter` (Valkey), all `engage.rate.*` keys |
| engage.rate.events_per_ip_minute | 300 | anonymous |
| engage.rate.responses_per_session_hour | 30 | |
| engage.rate.csat_posts_per_ip_hour | 30 | CSAT rating page |
| engage.segment_preview_timeout | 3 s | exact count statement timeout, then sampled estimate |
| engage.segment_contacts_timeout | 5 s | |

Self-host: tunables above configurable, `0` disables a limit.

Entitlement catalog keys (cloud caps, `Entitlements.Limit`, extensibility.md section 6; owner 15; self-host default `-1` unlimited; staff override per customer, 12): `engage.max_live_announcements` (cloud 50, per workspace), `engage.max_live_surveys` (cloud 20), `engage.max_segments` (cloud 200). Over limit: 422 `entitlement_limit_reached` with `key`, `limit`. Validation limits (`engage.max_questions`, `engage.max_translations`, `engage.assets.max_mb`) stay config: same in every edition, not caps.

## Permissions matrix

| Action | owner | admin | member | token ability |
|---|---|---|---|---|
| view items, segments, stats, results, responses, export CSV | yes | yes | yes | engage:read (+ contacts:read for contact details) |
| create/edit drafts, segments, assets, questions | yes | yes | yes | engage:write |
| publish (`status: live`), resume, edit live item content/audience | yes | yes | no | engage:write + permission `engage:manage` |
| pause, end live item | yes | yes | yes | engage:write (stopping is always safe) |
| set `bypass_throttle`, change engage settings | yes | yes | no | engage:write + `engage:manage` |
| delete items, delete responses | yes | yes | no | engage:write + `engage:manage` |
| CSAT survey create/publish | yes | yes | no | engage:write + `engage:manage` |

Policies call `authz.Can(principal, permission)` (never role names). Catalog `app/auth/permissions.go` (00-accounts owner): `engage:read`, `engage:write`, `engage:manage` (rows marked admin+: publish/resume, `bypass_throttle`, settings, deletes, CSAT survey); role grants `app/auth/roles.go`: owner/admin all, member read + write. Token abilities `engage:read`, `engage:write` only.

Visitor (widget session): read only own eligible items, write only own receipts and responses. Response ids of other contacts: 404.

## Accessibility contract (WCAG 2.2 AA, binding for widget UI and CSAT rating page)

Server enforces what it can; UI contract for frontend team:
- Modal: `role="dialog"`, `aria-modal="true"`, `aria-labelledby` title, focus moves to dialog, focus trap, Escape closes when dismissible, focus returns to previous element (2.4.3). Never opens while user typing in input (focus check) to avoid hijack.
- Banner: `role="region"` with `aria-label`, inline style pushes content (no obscuring focused element, 2.4.11), dismiss button `aria-label="Dismiss announcement"`, 24x24 CSS px min target (2.5.8).
- No auto-dismiss or auto-advance timers (2.2.1). Animations respect `prefers-reduced-motion`.
- Survey: rating/NPS as `radiogroup` with labeled radios ("7, likely"), arrow-key navigation, visible focus ring (2.4.7), required marked in text not color only, error messages linked via `aria-describedby`, progress announced `aria-live="polite"`.
- Color: server computes contrast for `accent_color` vs text; publish warning `low_contrast` (below 4.5:1); `banner.tone` palette pre-validated.
- Images: cover image needs alt or explicit decorative flag (publish error `missing_alt`).
- CSAT rating page: semantic HTML form, works without JS, `lang` attribute = locale, labels on every input, server-side validation messages, 320 px reflow (1.4.10).
- Text sizes in rem; zoom 200% without loss.

## Localization

- Locale resolution: contact.locale, else session locale, else `Accept-Language` (CSAT rating page), else `default_locale`. Primary-subtag fallback (`de-AT` -> `de`).
- Missing translated field falls back per field to default locale; never empty UI.
- Option ids stable across locales; results aggregate across locales; CSV includes `locale` column.
- Widget built-in strings (Next, Submit, Skip, Close, "Thanks!") from 01 dictionary (`en` at MVP).

## Performance budgets

| Path | Budget | How |
|---|---|---|
| config `engage` block | +0 ms | read from cached `live_summary` with 01 config cache |
| `GET /widget/v1/apps/{app}/engage-items` | p95 5 ms origin | otter L1 by `(workspace, version, locale)` (Valkey pubsub bust), CDN 60 s |
| `GET /widget/v1/engage-items` | p95 20 ms | items from otter L1; receipts 1 query; contact+company preloaded; evaluator under 1 ms for 70 items x 30 conditions |
| `POST /widget/v1/engage-events` | p95 10 ms | one upsert statement for up to 50 events; tracker in memory |
| response save | p95 50 ms | one tx: response row + answers upsert |
| results | p95 150 ms at 100k, 400 ms at 1M responses | index-only scans on `(workspace_id, question_id, value_number)`, Valkey cache 60 s keyed by filters (shared across replicas) |
| segment preview | 3 s max | exact count with statement timeout, else `TABLESAMPLE SYSTEM (1)` estimate `approximate: true` |
| scale assumptions | 1M contacts, 5M page loads/day, 70 live items | server rows only for session contacts that see items; no audience materialization, no fan-out jobs |

Storage estimate: receipt row about 120 bytes; announcement seen by 1M contacts = about 120 MB plus index; pruned 365 days after end. Answer row about 80 bytes plus text.

## Observability

Metrics: `dimmi_engage_items_requests_total{list=session|anonymous,result=200|304}`, `dimmi_engage_eligibility_seconds`, `dimmi_engage_events_total{kind}`, `dimmi_engage_stats_flush_total{result}`, `dimmi_engage_responses_total{source,status}`, `dimmi_engage_csat_requests_total{result,skip_reason}`, `dimmi_engage_segment_preview_seconds{approximate}`, `dimmi_engage_redactions_total{kind}`.

Logs: item public id, contact public id, outcome. Never answer text at info. Never signed link signatures.

## Security and privacy

- Audience rules are targeting, not authorization. Content of live items readable by any contact matching audience; URL rules evaluated client-side. Docs and dashboard note: do not put secrets in announcements.
- Anonymous list exposes only items explicitly anonymous-safe; rendered with variable fallbacks (no contact data).
- Widget resources hide audience, segment, internal name, stats, creator.
- CSAT rating page: CSP `default-src 'self'; img-src 'self' https:; style-src 'self'; form-action 'self'; frame-ancestors 'none'`. Signed URLs via `framework/routing` (HMAC-SHA256, app key), bind `svr` + `expires` (max 30 days); POST reuses same signature (no cookies, no form token). Only `display = conversation` responses with `source conversation_email`.
- Link scanners: GET never writes (no seen, no answer).
- Text answers rendered escaped everywhere.
- Attribute write-back writes only `custom_attributes` keys declared on question; identified contact keys may be overwritten later by JWT claims (02 rule, documented).
- Page URL scrubbing reuses 01 rule.
- GDPR: eraser `EraseEngageContactData` deletes contact's survey responses (answers cascade), receipts, and scrubs `contact_id` from `requested` CSAT rows; stats rows keep counts (no PII). Contact export (03) includes engage receipts and survey responses (export hook).
- Tenant isolation: all queries with `workspace_id`; widget routes resolve workspace from session/app.

## Metering impact (pricing.md section 2)

| Rule | Effect |
|---|---|
| D2 | CSAT request messages `substantive = false`, `author_type system`: never activate billing. 12 contract test covers writer |
| D7 | Announcements, surveys, impressions, clicks, responses: never ledger lines. No conversations created by publishing. Not counted in outbound fair-use (no `conversation.created` with `initiated_by` member/agent/api) |
| D1 | Chat-format reply creates contact-initiated conversation; billed only when workspace replies, like any widget conversation |
| D10 | Test sessions and preview: receipts and responses `is_test`; CSAT never sent on `is_test` conversations |
| Abuse (cloud) | live item caps, widget rate limits, CSAT rating post limits; none billable |

12 verification: metering code never reads engage tables; `IsActivity` ignores `substantive=false`; fixture test `csat_request_not_billable` and `announcement_reply_billing` added to 12 replay suite.

## Dependencies

| Spec | Needs |
|---|---|
| 00-foundation | prefixes, ownership map, events, abilities, rest.md web exception row, eraser hook entry |
| 00-accounts | audit log, member roles, workspace suspended events |
| 01-messenger-widget | config `engage` block, SDK commands/events, lazy chunks, test session mint, preview page `preview` param, `announcement_id` on conversation create, widget message resource exposes `survey_request` |
| 02-widget-identity | verified contact attributes and company claims (targeting input); `signed_up_at` convention |
| 03-shared-inbox | `SendSystemMessage` action, `conversation.closed/reopened`, contacts `filter[segment]`, `include=csat`, eraser + export hooks, `announcement.replied` event row, contact merge event |
| 04-email-channel | deliver system survey request message in thread with `Auto-Submitted: auto-generated`; suppression check API |
| 06-notifications | trigger `csat_negative` |
| 07-dimmi-agent | agent never triggered by system messages (CSAT request) |
| 08-help-center | Markdown options profile `engage` (`app/support/helpmarkdown`, on `framework/markdown`), shared `app/support/images` (on `framework/image`), Help tab opens article by id (`show('help', {article})`) |
| 09-agent-native | toolset `engage`, CLI commands, confirm gate for publish |
| 10-webhooks | catalog entry `survey_response.completed` with fixture |
| 11-self-host | backups include engage tables (automatic), `dimmi doctor` none |
| 12-cloud-billing | replay fixtures `csat_request_not_billable`, `announcement_reply_billing`; confirm no metering |
| 13-changelog | widget entry route `GET /widget/v1/changelog/entries/{cle}` (opened by SDK `show('changelog', {entry})`) for CTA `open_changelog_entry`; entry lookup `changelog.FindPublishedEntry(ctx, workspaceID, entryID)`; boundary (Decisions table) |
| 14-feedback-roadmap | may reuse segments (not at MVP); survey answers linking to feature requests later |

## Required from other specs (exact edits)

15 is read-only on other folders. Owners apply:

### 00-foundation
1. `data-model.md` ownership map: add row `engage_settings, segments, engage_assets, announcements, surveys, survey_questions, survey_responses, survey_answers, engage_receipts, engage_stats_daily | 15-announcements-surveys`.
2. `data-model.md` prefix registry (catalog `app/models/public_ids.go`, `framework/ids`): add `seg_` segments, `ann_` announcements, `svy_` surveys, `svq_` survey_questions, `svr_` survey_responses, `erc_` engage_receipts, `eas_` engage_assets.
3. `data-model.md` "Core table changes" jsonb row: `messages.metadata.survey_request`, `conversations.source.announcement_id`, `messages.event` name `announcement.replied` (15). Migration range note: 15 uses `20261101001501-001599`.
4. `events-and-realtime.md` "Events added by feature specs": row 15 with events in Events table above; note `survey_response.completed` webhook yes, others realtime only. Standard listeners: `WriteAuditLog` covers engage status/settings/exports.
5. `api.md` abilities: add `engage:read`, `engage:write`. Endpoint ownership: row "announcements, surveys, segments, engage settings, engage assets, survey templates, `/conversations/{conv}/survey-responses`, `/contacts/{ctc}/survey-responses`, `/contacts/{ctc}/engage-receipts`, widget engage routes, `/csat-ratings/{svr}` | 15".
6. `rest.md` exceptions table: add web row `/csat-ratings/{svr}` (CSAT email rating page, GET renders, POST acts, HMAC-signed query). Replaces earlier `/survey-forms/*` row (later: hosted-survey-page).
7. `contracts.md` contact eraser hooks: add "15: survey responses + answers, engage receipts, CSAT request rows (`EraseEngageContactData`)".
8. `security.md` Abuse (cloud): remove public survey links line (later: hosted-survey-page); keep card/secret redaction of answers.

### 01-messenger-widget
1. Config resource adds `engage: {active, anonymous, version, refresh_interval_ms}`; config cache key includes `engage_settings.version`; listener bust on `engage_settings` version change (15 `BumpWidgetConfigForEngage` publishes same PubSub invalidation 01 uses).
2. `sdk-contract.md` (already carries these; listed for check): `track(event, properties?)` client-only (no network; matches cached items with `trigger.on = track_event`; properties ignored at MVP, max 1 KB); views `show('announcement', {id})`, `show('survey', {id})` (no separate show commands); CTA mappings `show('help', {article})`, `show('changelog', {entry})`, `startConversation(prefill, {context: {announcement_id}})`; boot option `engage: false` (opt out per page); events (snake_case names, camelCase payloads) `announcement_shown {id}`, `announcement_dismissed {id}`, `announcement_clicked {id, ctaId}`, `engage_action {id, ctaId, eventName}` (CTA `sdk_event`), `survey_shown {id}`, `survey_completed {id, responseId}` (no answer values); deep links `#dimmi-announcement=`, `#dimmi-survey=`; storage key `dimmi:{app}:engage`.
3. Bundle: lazy chunks `engage-banner` (6 KB gz), `engage-modal` (10 KB), `engage-survey` (14 KB); core budget unchanged (engage scheduler at most 1.5 KB inside 35 KB).
4. `POST /widget/v1/conversations` accepts `announcement_id` (chat-format reply); `StartWidgetConversation` passes `source.announcement_id` and event row to 03.
5. Widget message resource exposes `survey_request: {survey_id, response_id}` for system messages; widget renders CSAT card.
6. Preview page `/widget-preview/{widget}` reads `#preview=ann_...|svy_...` and forwards to SDK; test sessions may request `preview` param on engage routes.
7. Out table: remove "CSAT prompt on close" and change `dimmi.track` row to "server-side track ingestion and event audiences".
8. Eraser `EraseWidgetContactData` unchanged; contact export lists engage data (via 03 export hook).

### 02-widget-identity
1. Document `custom_attributes.signed_up_at` (unix seconds or RFC 3339) convention recognized as date by segments; validator hint when `signed_up_at` sent as non-date.
2. Note: identity claim changes trigger SDK engage refetch (no server change).

### 03-shared-inbox
1. New action `conversations.SendSystemMessage(ctx, tx, input)`: inserts `type message`, `author_type system`, `substantive false`, metadata passthrough (allowlist key `survey_request`); runs outbound pipeline; never reopens, never changes `waiting_since`, `last_message_at`, `last_activity_at`, unread counts, assignment; never triggers 07; emits `message.created`.
2. `messages.event` allowed names add `announcement.replied`; `conversations.source` documents `announcement_id`.
3. `GET /contacts` adds `filter[segment]=seg_...` (15 compiler, statement timeout 5 s, 422 `segment_too_slow`).
4. Conversation resource `include=csat`: latest completed CSAT `{response_id, score, scale, comment_excerpt (140), completed_at}`; sidebar context endpoint includes it.
5. Eraser registry list and CI table add 15 `EraseEngageContactData`; contact export hook includes `survey_responses` + `engage_receipts`.
6. Out table: CSAT row changes to "ships in 15".

### 04-email-channel
1. Outbound for `author_type system` messages with `metadata.survey_request`: same thread headers, from conversation reply address, header `Auto-Submitted: auto-generated`, no open tracking, plain-text twin with URLs.
2. Done in 04: read helper `email.IsSuppressed(ctx, workspaceID, email, scopes ...string) bool` (outbound.md 16; no scopes = `all`) used for 15 pre-send check. In-thread CSAT mail = `outbound_emails.kind system` via email `Channel.Send` (04 README `outbound_emails.kind`). Listed for grep.

### 05-chat-app-channels
1. Remove system-authored CSAT message handling (README 120-122, edge 191, tasks): CSAT on chat apps deferred (later: chat-app-csat).

### 06-notifications
1. New trigger `csat_negative` (default on, in-app + push) to conversation assignee (fallback: assignee team members) when `survey_response.completed` CSAT score at most 2 of 5 (or NPS 0..6); skip `is_test`.

### 07-dimmi-agent
1. Remove `RecordAiSpend` kind `survey_summary` and 15 survey summary edge (later: survey-ai-summary).
2. Out list: "CSAT on AI answers" now covered by 15 CSAT with `ai_involved` split.

### 08-help-center
1. Expose `helpmarkdown.Engage()` (`app/support/helpmarkdown/options.go`, `framework/markdown` `Options` profile `engage`): 08 dialect minus headings above H3, footnotes, embeds-as-cards; images only `eas_` assets or https; same `framework/sanitize` policy.
2. Extract image processing into `app/support/images` (allowed types, caps, scan via `a.Scanner`) on `framework/image` (sniff, decode dims, bomb guard, metadata strip by re-encode), shared by `help_center_assets` and `engage_assets`.
3. Widget Help tab opens article by id: SDK `show('help', {article: 'art_...'})` -> `GET /widget/v1/help/articles/{art}` (for CTA `open_article`); article not public/published: Help home + `console.warn` `view_unavailable` reason `article_not_found` (sdk-contract code).

### 09-agent-native
1. Register toolset `engage` (8 tools above) in `routes/mcp.go`; `status: live` publish uses confirm gate list ("engage publish" row).
2. CLI commands `announcements`, `surveys`, `segments` in command map; Claude Code skill mentions `dimmi announcements create --file`.

### 10-webhooks
1. `webhook_catalog.go` entry `survey_response.completed` (object `survey_response`, fixture).

### 12-cloud-billing
1. Metering rules D2/D7 mapping: note 15 CSAT requests (`substantive=false`) and engage content never metered; add replay fixtures `csat_request_not_billable`, `announcement_reply_billing`.
2. Drop 15 as `BillingStatus.HasPaid` consumer and entitlement `engage.link.public_daily_limit_unpaid` (later: hosted-survey-page).

### 13-changelog
1. Expose `changelog.FindPublishedEntry(ctx, workspaceID, entryID)` for CTA validation; widget opens entry by id via SDK `show('changelog', {entry: 'cle_...'})` on route `GET /widget/v1/changelog/entries/{cle}`, used by CTA `open_changelog_entry`.
2. Out row "Popup/banner announcements outside Messenger": change to "ships in 15; announcement CTA links entry". Out row "Audience segmentation": "15 segments available; adopt later".
3. `spotlight` card stays inside Messenger; not counted by 15 throttle (no change needed, note only).

### specs/README.md and research/roadmap.md (orchestrator)
1. Add folder row `15-announcements-surveys | product-owner MVP add | announcements, surveys, segments, CSAT | 136 | 36`.
2. Roadmap: move "CSAT on close" out of "post-MVP: reports" and "banners" out of "post-MVP: upmarket set" into launch list (announcements + surveys launch item); keep tours in upmarket set; "outbound" removed from "Not at launch" for in-app only.

## Contradictions found

1. `01` Out lists CSAT ("post-MVP: reports") and `dimmi.track` (post-MVP); `03` Out lists CSAT. Product owner rule makes both MVP here; edits listed above.
2. `research/roadmap.md` "Not at launch" lists "outbound". Resolved: in-app outbound (announcements, surveys) MVP; email/broadcast outbound stays post-MVP.
3. `data-model.md` "One prefix per table" vs tables without public ids in other specs (`message_mentions`, `conversation_reads`). 15 follows practice: prefixes only for addressable tables.
4. `00-foundation/data-model.md` lists no contact tags while brief mentions tags as targeting; resolved by custom attributes (open question 1).

## Open questions (recommended default in bold)

1. Contact tags (Intercom-style) for targeting: **no at MVP**; custom attributes via JWT/API cover it; revisit with 03 "post-MVP: ticket fields".
2. Default global throttle 2 interruptive per day: **yes** (Chameleon docs use 2/day example).
3. CSAT channels: **widget + email** only (chat apps later: chat-app-csat).

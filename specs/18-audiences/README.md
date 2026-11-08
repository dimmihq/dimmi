# 18 Audiences

**Approved by the owner on 2026-11-04, with every recommended answer in "Decisions for you" accepted. Not built yet.**

Style: caveman. Date 2026-11-04. One file on owner's request for now. Read with `specs/00-conventions/*`, `specs/00-foundation/*`, `specs/15-announcements-surveys`, `specs/17-workflows`, `specs/02-widget-identity`. Research: `research/competitors/intercom-audiences.md`.

## Goal

One way to say "who" across the whole product. A workspace defines a rule once and uses it to decide who sees the launcher, who can start a conversation, which tabs appear, who gets an announcement or survey, which workflow runs, who can read an article, what dimmi may answer from and which actions dimmi may call. Same language, same evaluator, same builder, same preview, same "test as a person".

Why now: 15 and 17 both specify rule languages, 17 reuses 15, 13, 14, 07 and 08 all park "audience later". Build each alone and we end up where Intercom is: the same-looking builder on 10 surfaces with different limits, different timings, and three reusable objects that barely relate (research section 1).

## What exists today

| Thing | State | Evidence |
|---|---|---|
| `contacts.custom_attributes`, `companies.custom_attributes`, `contacts.trusted_attribute_keys` | built | migrations `...000033`, `...000032`; `app/actions/contacts/fields.go` (100 keys, 1,024 chars, key `^[a-z][a-z0-9_]{0,63}$`) |
| Trust bookkeeping | partial: REST and teammate writes join trusted keys | `app/actions/contacts/update_contact.go`, `create_contact.go` |
| Attribute definitions, types, registry | none. Type is whatever JSON arrives | |
| Contact and company tags | none in code. `tags` + `conversation_tag` only. Spec: 03 records.md, built by track 04 | `20261101000039`, data-model |
| Events | **not ingested.** `track()` is a spec only: client-side, no network, no storage (01 sdk-contract, 15). `contact_event_summaries` is a post-MVP name in 15. No table, no route | `routes/`, `database/migrations` |
| Segments, rule evaluator, compiler | spec only (15). `app/support/segments`, `app/support/querybuilder` do not exist | `ls app/support` |
| Workflows conditions | spec only (17 `app/support/conditions`). Prototype has 3 fixed audience presets | `resources/js/features/automation/inspector.tsx:65` |
| Widget API | spec only for sessions and config. Go serves loader and assets only; widget code in `resources/widget` | `routes/web.go:65`, `app/http/controllers/widget` |
| Help center visibility | spec: `public`, `agent_only`; audience "later" | 08 README line 91 |
| UA parser | already a dependency | `go.mod` `medama-io/go-useragent` |
| GeoIP | spec says optional, nothing in code | 01 sessions table `country` |

Good news: 15 and 17 are unbuilt, so unifying now costs spec edits, not migrations.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| One language | exactly one parser, one Go evaluator, one SQL compiler, one client residual evaluator. `tests/arch` fails if another package parses rule JSON |
| Parity | Go and SQL agree on 10,000 random contacts per field and op (15's test, extended). Go and TypeScript agree on a shared JSON corpus for every client field |
| Leak-free | widget responses contain no attribute names, attribute values, event names (except those admin marked `client_writable`), saved audience names, tag names or rule shapes beyond page leaves the admin was told are public. CI test greps every widget response in the feature suite against a canary rule |
| Spoof-proof | a browser can never pass an `access` gate by anything it sends. Proven by a test matrix over every widget write path |
| Cost when unused | workspace with no rules: zero extra requests, one absent config field, zero rows, no added query |
| Bundle | client evaluator at most 500 B gz inside the 35 KB core; loader stays under 3 KB with no rule code. CI size gate |
| Widget decision | session handshake with rules: under 2 ms added p95, at most 2 extra queries and only when rules need tags or events. Anonymous: zero extra requests |
| Navigation | residual re-checked on every SPA navigation locally: under 1 ms, no network, no throttle |
| Freshness | attribute, tag or event change to open widget reflecting it: p95 under 5 s. JWT claim to decision: same response. Event to rule visible to next evaluation: under 1 s |
| Event ingest | `POST /contact-events` p95 under 20 ms single, under 100 ms for a 100-event batch; sustained 2,000 events per second per replica |
| Preview | exact count at 1M contacts within 3 s, else sampled and labelled. Builder debounce 400 ms. p95 under 500 ms at 100k |
| Evaluation cost | 100 live rules against one contact under 1 ms |
| Explainable | every "does this person match" answer returns the rule tree with each leaf's actual value, result, and trust. Same explainer for test-as, workflows and help center |
| Mistakes caught | builder flags negative OR, impossible AND, missing-value traps, claimed fields, stale references before publish. Each has a test |
| API parity | everything in the builder through REST, CLI, MCP. Rules are readable and writable JSON |
| Cost | never metered. Events, rules, previews: no ledger lines |

## Scope

### In

- One rule language and field registry used by widget, engage (15), workflows (17), help center (08), dimmi knowledge and actions (07), changelog and feedback audiences (13, 14) when they adopt, and outbound.
- Saved audiences: CRUD, nesting, used-by, preview, members list.
- Attribute registry: types, labels, archive, conversion, client-writable flag, problems list.
- Contact and company tags.
- Standard fields: `last_heard_at`, `web_sessions`, device, browser, OS, location.
- Event ingestion: REST, SDK opt-in, definitions, per-contact rollups.
- Messenger surfaces: launcher, start conversation, tabs.
- Help center: audience on article and collection, signed-in help center visitors.
- Partial evaluation so browsers only ever get page leaves.
- Builder, preview, test as a person, explainer.
- Fixed audiences and CSV lists (last milestone).
- REST, MCP toolset, CLI, webhooks on definitions.

### Out (later)

| Item | Later | Why |
|---|---|---|
| Bulk email broadcasts | 15's post-MVP line, needs 04 pipeline | language ready; only `email_baseline` seam here |
| Raw event history | `contact-event-history` | rollups answer every rule; see decision 5 |
| Multi-company matching | with `contact_companies` | core has primary company; `company.*` means primary today, "any" later |
| Custom objects in rules | post-MVP | no custom objects yet |
| Conversation custom fields | 03 records.md conversation attributes | |
| Regex | never | ReDoS, explainability |
| AI-written audiences | later: `audience-from-sentence` | deterministic first; MCP already lets an agent write JSON |
| Enrichment attributes | integrations | any integration writes through REST |
| Per-field rule access controls | custom roles | |

## Terms

- **Rule**: a tree of `all`, `any`, `not` and leaves `{field, op, value}`.
- **Audience** (inline on a surface): `{ "visitors": ..., "rules": ... }`. Replaces 15's `models.Audience`.
- **Saved audience** (`audiences`, `adn_`): named rule, reusable, one word replacing "segment". See decision 1.
- **Surface**: a place that takes an audience (launcher, article, workflow, ...). Registered in Go.
- **Gate**: how strict a surface is: `display` or `access`.
- **Claimed** value: came from a browser. **Verified** value: came from a signed JWT, REST, teammate, trusted import or integration.
- **Residual**: what is left of a rule after the server plugs in everything it knows. Only page and device leaves.

## Rule language

Keeps 15 and 17's JSON. No migration of ideas, only one owner.

```json
{
  "visitors": "identified",
  "rules": { "all": [
    { "field": "company.custom.plan", "op": "in", "value": ["pro", "team"] },
    { "field": "audience", "op": "not_in", "value": ["adn_01jbtrial"] },
    { "field": "event.report_exported.count", "op": "gte", "value": 3, "within_days": 7 },
    { "any": [
      { "field": "contact.last_seen_at", "op": "within_last_days", "value": 30 },
      { "field": "page.path", "op": "starts_with", "value": "/billing" }
    ] }
  ] }
}
```

- Nodes: `all`, `any`, `not` (17's single-child node, now everywhere). Limits: depth 4, 50 leaves after expanding saved audiences, 8 KB per stored tree. `in` max 50 values.
- `visitors` stays as builder sugar for the Everyone / Users / Leads / Visitors checkboxes: `everyone`, `identified`, `leads`, `anonymous`, `anonymous_and_leads`. Compiles to a leading `all` leaf. Default `everyone`.
- 15's `segment_id` and `locales` go away: saved audience is a leaf (allows `not_in`, which Intercom users fake with two segments), locale is a leaf.
- Nesting: a saved audience may reference saved audiences, at most 2 levels, cycles refused at save (`audience_cycle`). Edits apply to live users immediately; publish-time expansion is only for validation.

### Operators (one table, every field type)

| Type | Ops |
|---|---|
| string | `eq`, `neq`, `in`, `not_in`, `contains`, `not_contains`, `starts_with`, `ends_with`, `contains_any`, `not_contains_any`, `exists`, `not_exists` |
| number | `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `between`, `exists`, `not_exists` |
| bool | `is_true`, `is_false`, `exists`, `not_exists` |
| date | `before`, `after`, `on`, `within_last_days`, `more_than_days_ago`, `days_ago`, `within_next_days`, `more_than_days_from_now`, `exists`, `not_exists` |
| list (one of fixed options) | `eq`, `neq`, `in`, `not_in`, `exists`, `not_exists` |
| set (tags) | `has`, `has_any`, `has_none` |
| url | `eq`, `contains`, `not_contains`, `starts_with`, `matches` (glob: `*`, `**`; 15's matcher) |

Semantics, identical in Go, SQL, client:

- Strings case-insensitive after NFC (Intercom's `is` is case-sensitive and bites people).
- Missing value: every op false except `not_exists`, `neq`, `not_in`, `not_contains`, `not_contains_any`, `has_none` (true). Kept from 15, with the builder saying it out loud on each negative leaf: "also matches people with no value".
- Type mismatch: false, never an error.
- Relative dates: rolling 24-hour days from `a.Clock.Now()`, future via named ops, never negative numbers. `on`: calendar day in workspace timezone. `days_ago`: N full days back (24-hour window).
- Dates stored as Unix seconds when the definition says `date` (see registry), so SQL compares numbers.
- `within_days` modifier on `event.*.count`, 1 to 90.
- `matches` glob: `*` any characters except `/`, `**` any characters. Trailing slash on `page.path` ignored. `page.url` never holds fragment.

### Field catalog

Each field registers: type, source, trust, where it can be evaluated (`client`, `server`, `conversation`), SQL compile yes/no, surfaces. `enums.AudienceField` Go registry (extensibility P1). 17's groups register in the same registry; its catalog section 3 becomes a list of registrations, not a second language.

| Field | Type | Evaluated | Trust | Notes |
|---|---|---|---|---|
| `contact.kind` | list (user, lead) | server | verified | |
| `contact.email`, `.phone`, `.name`, `.external_id` | string | server | email: by `email_verified` | |
| `contact.email_domain` | string | server | by `email_verified` | common filter |
| `contact.email_verified` | bool | server | verified | |
| `contact.locale`, `.timezone` | string | server | claimed for leads | |
| `contact.country`, `.region`, `.city` | string | server | observed | from `contacts.location` |
| `contact.created_at`, `.first_seen_at`, `.last_seen_at` | date | server | observed | `last_seen_at` throttled 60 s (01) |
| `contact.last_heard_at` | date | server | observed | new column. Pre-event rule below |
| `contact.web_sessions` | number | server | observed | new column |
| `contact.conversation_count` | number | server | observed | new column |
| `contact.has_company` | bool | server | verified | |
| `contact.tags` | set | server | verified | new |
| `contact.custom.<key>` | by definition | server | by definition and `trusted_attribute_keys` | |
| `company.name`, `.domain` | string | server | verified | |
| `company.created_at` | date | server | verified | |
| `company.tags` | set | server | verified | new |
| `company.custom.<key>` | by definition | server | verified: browser cannot write (02 R8) | |
| `event.<name>.count` | number | server | by definition | `within_days` |
| `event.<name>.first_at`, `.last_at` | date | server | by definition | |
| `audience` | saved audience ids, `in`, `not_in` | expands | worst of contents | |
| `session.verified` | bool | server | verified | |
| `session.country` | string | server | observed | GeoIP; forces dynamic for anonymous |
| `session.browser`, `.os` | string | server | claimed | parsed from user agent with the existing parser |
| `session.context.<key>` | by JSON type | server | claimed | 01 `Dimmi('update', {context})` |
| `visitor.locale` | string | client | claimed | browser language |
| `visitor.device` | list (desktop, mobile, tablet) | client | claimed | viewport: mobile under 640 px, tablet 640 to 1023 px, desktop 1024 px up |
| `page.url`, `.path`, `.query`, `.host`, `.referrer` | url | client | claimed | |
| `conversation.*`, `message.*`, `note.*`, `trigger.*`, `rating.*`, `dimmi.*`, `office_hours.*`, `time.*`, `run.vars.*` | as 17 catalog | conversation | as 17 | Go only; preview replays recent conversations |

Renames for consistency: 17 `session.page_url` to `page.url`; 17 `conversation.source.page_url` stays (fixed at creation).

15's `Trigger.url_rules` is a second language (`equals`, `url_match`). Replaced: trigger takes a `page` tree limited to `page.*` and `visitor.device`, same ops. `delay_seconds`, `on`, `event_name` remain trigger settings.

### Lint (warnings at save and publish, each with code and fix)

| Code | Catches | Research source |
|---|---|---|
| `negative_or` | `any` with two or more negatives on the same field: matches nearly everyone | research 3 |
| `impossible_and` | `all` with two different `eq` on the same field, or two `page.path eq` | research 7 |
| `missing_value_match` | negative op on a field where most contacts have no value (with the count) | research 3 |
| `claimed_field` | rule uses a claimed value on a `display` surface, informational | new |
| `page_rule_is_public` | rule includes page leaves: values reach browsers | new |
| `date_as_text` | custom attribute looks like dates but is typed string | research 6 |
| `matches_nothing`, `matches_everyone` | from preview | |
| `archived_reference`, `missing_reference` | deleted attribute, tag, audience | |
| `event_not_tracked` | event has no definition or no events in 30 days | |

Errors (publish refused): `claimed_field_on_access_gate`, `field_unavailable_for_surface`, `op_invalid_for_type`, `unknown_attribute`, `audience_cycle`, `limit_exceeded`, `geo_unavailable` when GeoIP missing on an `access` surface.

## Surfaces

`enums.AudienceSurface` registry: key, gate, allowed field sets, evaluation moment. New surface = one registration.

| Surface | Gate | Evaluated when | Unknown or stale | Owner |
|---|---|---|---|---|
| `messenger.launcher` | display | each boot, identity change, navigation | shows launcher (fail open: never lose support access) | 01 |
| `messenger.start` | access | server, at `POST /widget/v1/conversations`; button state from handshake | refuses 403 `conversation_start_restricted` | 01 |
| `messenger.tab.help`, `.changelog`, `.feedback` | display | boot, identity change | tab hidden | 01, 13, 14 |
| `engage.announcement`, `engage.survey` | display | each list fetch, residual per navigation | not shown | 15 |
| `workflow.audience`, `workflow.branch` | display | at trigger, at step | false | 17 |
| `help.article`, `help.collection` | access | every request | hidden | 08 |
| `dimmi.knowledge` | access | every run (not once) | excluded | 07, 08 |
| `dimmi.action` | access | every run | not offered | 07 `action-audiences` |
| `changelog.entry`, `feedback.board` | display | as their own read path | hidden | 13, 14 adopt later |
| `outbound.fixed` | display | at go-live snapshot | excluded | 15 |

- `display`: shows or hides something non-secret, or routes. Claimed values allowed, labelled.
- `access`: reveals data or spends something. Only verified values count; see Security.
- Workflows are `display` because a workflow audience is routing, not disclosure. Any step that reaches data has its own gate (07 actions bind only trusted keys, HTTP step bound by connection). Lint warns on claimed fields, because a lead could claim `plan=enterprise` and reach the VIP lane. Decision 3.
- Launcher rule hides the launcher only. Replies, engage items, `show()` still work. Same as Intercom's documented intent; we keep it clear in UI copy.

Per surface storage:

- `widget_settings.audiences jsonb`: `{launcher, start, tabs: {help, changelog, feedback}}`, each an inline audience. Absent = everyone. Column and launcher page leaves ship in 01 M8: 01 "Pages" control is simple form over `audiences.launcher` (01 settings.md), no second column, no second matcher.
- 15 items, 17 versions: existing `audience` columns.
- 08: `articles.audience_id`, `collections.audience_id`: saved only, so search can filter by id and bulk edit is one id.
- 07: `ai_actions.audience jsonb`, M7.

## Evaluation design

### One function, three outputs

`audience.Partial(ctx, tree, known) -> True | False | Residual(tree)`: walks the tree, plugs in known fields, simplifies by Kleene logic. `Evaluate` = `Partial` with everything known. `Explain` = same walk, recording each leaf.

- Server decisions: all server fields known, client fields unknown.
- Dashboard: all known, including a typed-in page for test-as.
- SQL: separate compiler for preview and member lists, proven equal.

### Why partial evaluation

```text
rule:     (plan = pro AND kind = user) OR page.path starts_with /pricing
pro user: True                        -> widget gets: launcher: true
other:    page.path starts_with /pricing -> widget gets: the one page leaf
visitor:  page.path starts_with /pricing -> same, in public config
rule:     plan = pro AND page.path = /a
free:     False                       -> widget gets: launcher: false
```

The browser gets a boolean or a page-only tree. No attribute name, value, saved audience name or event name ever leaves the server. An attacker reading the widget response learns that a page rule exists and nothing else. It also means the widget is never out of sync on page rules: re-checked locally on every navigation.

### Flows

**Config (public, cacheable, 01)** adds, only when any rule exists:

```json
"surface": { "version": 41, "dynamic": false, "writable_events": ["export_completed"],
  "anonymous": { "launcher": { "any": [ { "field": "page.path", "op": "starts_with", "value": "/pricing" } ] },
                 "start": true, "tabs": { "help": true } } }
```

- Computed at publish, cached with config, bumps with 15's `engage_settings.version` pattern.
- `dynamic: true` only when an anonymous-reachable rule uses `session.country`. Then the SDK makes one `GET /widget/v1/apps/{app}/decisions` (no session, no row, `private, max-age=300`, rate limited). Workspace without geo rules: zero requests, the 01 bar stays.
- `writable_events`: only names admin marked `client_writable`; public by that choice. Omitted when none.
- Loader optimisation: anonymous `launcher: false`, no identity source and no stored session: core is not fetched until `show()` is called. Saves 35 KB on marketing pages for workspaces that only serve signed-in customers.

**Session (handshake)** `GET /widget/v1/session` or `POST /widget/v1/sessions` responses add `surface` (same shape, no `dynamic`), computed after 02 applied claims, so a JWT's attributes count in the same response.

**Refetch** `GET /widget/v1/surface` (session auth, ETag). Called on `identity_changed`, on realtime `surface.changed`, on 15's refresh interval.

**Push** on `contact.updated`, `company.updated`, tag change, event rollup change: if `changed_attribute_keys` meets the workspace's compiled dependency set (attributes, tags, events used by live widget surfaces), broadcast contentless `surface.changed` on the contact's widget channel, coalesced to one per 2 s per contact. No open stream: picked up at next page load or 600 s refresh. Workspaces without rules: filter is one set lookup, nothing is sent.

**ETag bug fix (15).** 15's ETag hashes contact and version only. A rule like `within_last_days 30` flips with time while nothing changes, so a 304 would keep it stale. ETag gains an hour bucket when any live rule has a relative date leaf or `time.*`.

**Anonymous lists (15).** Item appears in the anonymous list when `Partial` with empty context is not `False`; item carries residual in place of `url_rules`. This replaces 15's "only `session.locale`, `session.country`" rule, which cannot work: its list is public on a CDN and country varies per request.

### Client evaluator

- `resources/widget/lib/rules.ts`. 01 M8 ships it with page ops for Pages control; 18 M5 extends. Input: residual tree. Handles `all`, `any`, `not`, url and string ops, device, locale. Under 500 B gz; shares 15's URL matcher.
- Runs on boot and each `history` change (01 already patches `history`), no throttle because no network.
- Malformed or unknown op from an older core: treated as false, one `console.error`, never throws.
- Holds decisions in memory; nothing identified goes to `localStorage`. Logout drops them.

### Evaluate-at rule, per kind

- Live surfaces evaluate against current data every time. No "frozen at conversation start". Mid-conversation upgrade reaches dimmi on the next reply (Intercom's Fin evaluates once).
- Workflows match at the event, with data freshly loaded (17 engine), branches at the step.
- Pre-event snapshot: message-triggered rules read `contact.last_heard_at` from the event payload's `previous_last_heard_at`, so "never heard from" matches the sender (Intercom's documented race).
- Page in a conversation context: `page.url` is what the SDK sent on the message at send time, not a session-start value, so no staleness.
- Fixed audiences: snapshot at go-live.

### Saved audience caching

- Live rule cache per replica (otter), keyed by workspace epoch, busted over `pubsub.PubSub` (15, 17 pattern). Holds compiled trees with saved audiences expanded and dependency sets.
- Contact and company already loaded by `auth:widget`. Tags and event rollups load only when the compiled set needs them: at most 2 indexed queries.
- Nothing materialised (15's decision stands). Membership is always computed. Only exception: fixed snapshots.

## Data model

Build numbers come from track range 13 (`20261201130001` up), see BUILD-TRACKS; `0018NN` below are labels.

Migration block `20261101001801`..`001899`. All tables: `workspace_id uuid not null references workspaces on delete cascade`, indexes lead with `workspace_id`, RLS `CREATE POLICY x_tenant ON x USING (workspace_id = ANY (dimmi_visible_workspaces()))`, `created_at`, `updated_at`. Prefixes (add to registry, no collision; `aud_` is audit logs, `evt_` is events): `adn_` audiences, `atd_` attribute definitions, `edf_` event definitions, `asn_` audience snapshots.

### audiences (`001801`, replaces 15 `segments`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `adn_` |
| name | text not null | 1..80; unique `(workspace_id, lower(name)) WHERE deleted_at IS NULL` |
| description | text | max 500 |
| rules | jsonb not null | tree, with `visitors`, max 8 KB |
| rules_schema | smallint not null default 1 | |
| last_estimate | jsonb | `{count, approximate, at}` |
| created_by_member_id, updated_by_member_id | uuid null | set null on member delete |
| lock_version | int not null default 1 | 409 `edit_conflict` |
| deleted_at | timestamptz | refused while referenced by live use, 409 `audience_in_use` |

### audience_references (`001802`)

| Column | Type | Notes |
|---|---|---|
| consumer_type | text not null | surface key |
| consumer_id | text not null | public id |
| target_type | text CHECK in (`audience`, `attribute`, `event`, `tag`) | |
| target_key | text not null | id or `contact.plan` |
| live | boolean not null | |

PK `(workspace_id, consumer_type, consumer_id, target_type, target_key)`; index `(workspace_id, target_type, target_key)`. Rewritten in the same transaction whenever a consumer's rule is saved. Drives used-by, archive refusal, `audience_in_use` and the push dependency set. A test asserts every registered surface writes it.

### attribute_definitions (`001803`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `atd_` |
| object | text CHECK in (`contact`, `company`) | conversation attributes: 03 records.md `conversation_attribute_definitions` (`cad_`) |
| key | text not null | `^[a-z][a-z0-9_]{0,63}$`, as code. Unique `(workspace_id, object, key)`. People and company namespaces are separate |
| label | text not null | editable, defaults from key |
| description | text | max 500 |
| data_type | text CHECK in (`string`, `number`, `boolean`, `date`, `list`) | locked; conversion below |
| options | jsonb | for `list`, max 200, `{value, label}` |
| client_writable | boolean not null default false | see Security |
| origin | text CHECK in (`created`, `observed`, `imported`) | |
| status | text CHECK in (`active`, `archived`) default `active` | |
| last_value_at | timestamptz | throttled |
| contacts_with_value | bigint | nightly estimate |

Seed: `signed_up_at` as `date` when first seen (02 convention).

### attribute_problems (`001804`)

Grouped log, one row per `(workspace_id, object, key, code)`: `code` (`type_mismatch`, `unknown_key_from_browser`, `not_client_writable`, `archived_write`, `key_limit_reached`, `invalid_option`), `count`, `last_at`, `sample` (shape only, no values). Rebuildable, pruned at 30 days. Surfaces silent failures, which Intercom has many of.

### tags: `contact_tag`, `company_tag` (`001805`, `001806` not used)

Spec and caps in 03 records.md (migration `20261201040005`), built by track 04: max 50 per contact or company, 10,000 tags per workspace. 18 adds rule fields `contact.tags`, `company.tags`, dependency filter, and proves merge union and erase in its tests. Resolves 15 open question 1.

### contacts additions (`001807`)

`last_heard_at timestamptz`, `web_sessions int not null default 0`, `conversation_count int not null default 0`. Expand migration, backfill in batches. Index `(workspace_id, last_heard_at)`. GIN `(workspace_id, custom_attributes jsonb_path_ops)` on contacts, built concurrently: companies have it (03 `...308`), contacts do not.

### event_definitions (`001808`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `edf_` |
| name | text not null | `^[a-z0-9_.:-]{1,64}$` (01 SDK charset); unique `(workspace_id, name)` |
| label, description | text | |
| client_writable | boolean not null default false | |
| origin | text | `created`, `observed` |
| status | text | `active`, `archived` |
| last_seen_at | timestamptz | throttled |
| total_count | bigint | approximate |

### contact_event_summaries (`001809`)

| Column | Type | Notes |
|---|---|---|
| contact_id | uuid not null on delete cascade | |
| event_definition_id | uuid not null on delete cascade | |
| count | bigint not null | |
| first_at, last_at | timestamptz not null | |
| daily | smallint[] not null | last 90 days, rolled by `last_at` on write; clamped 32,767 per day |
| last_metadata | jsonb | up to 20 scalar keys, 2 KB; latest values, shown on the contact |

PK `(workspace_id, contact_id, event_definition_id)`. `count_last_days` = sum of the window; no raw table. Index `(workspace_id, event_definition_id, last_at)` for SQL previews.

### Help center and widget (`001810`..`001813`)

- `articles.audience_id`, `collections.audience_id` uuid null references audiences on delete restrict (08).
- `knowledge_documents.effective_audience_ids uuid[]`, denormalised onto chunks (08 `kd_`, `kc_`) for filter without join. Recomputed when article or collection chain changes.
- `help_centers.login_url text` (https). Signed-in reading needs it.
- `widget_settings.audiences jsonb not null default '{}'`: created in 01 `20261201010001` (M8), label `001813` not used.

### audience_snapshots, audience_snapshot_members (`001814`, `001815`, last milestone)

`asn_`: `consumer_type`, `consumer_id`, `rules_hash`, `count`, `taken_at`. Members `(workspace_id, snapshot_id, contact_id)`. Written by batch job at go-live. Merge and erase hooks update it.

### Erase and merge

- 03 eraser hook deletes `contact_tag`, `contact_event_summaries` by cascade; attribute values already covered.
- Merge: tags union; summaries summed, `first_at` min, `last_at` max, `daily` added; values by existing rule; `trusted_attribute_keys` as today.

## Attribute registry

- **Who creates a definition.** Admin in UI, REST, CLI, or `observed`: a verified write (JWT claim, REST, import, integration) with an unknown key auto-registers with inferred type: integer or float `number`, `true/false` `boolean`, key ending `_at` or value parses RFC 3339 `date`, else `string`. Browser writes never create definitions. Intercom now requires pre-creation, which trips installs; we want first-JWT-just-works for developers, without letting a browser pollute the list.
- **Type lock.** Type fixed once set. Mismatch on write: value dropped, problem logged, write not failed. JWT: warning `claim_custom_attributes_invalid` already exists (02). REST: 422 only when caller asks `?strict=true`, default keeps accepted fields and returns `warnings`.
- **Dates.** On a `date` definition, input is ISO 8601 or Unix seconds or ms (ms detected and divided, Intercom's `_at` traps people); stored as Unix seconds number. Existing values normalised by backfill job on definition create.
- **Conversion.** `POST /attribute-definitions/{atd}/conversions` with `{to, dry_run}`: dry run reports convertible, unconvertible counts and samples; run is a River job in batches, unconvertible values cleared to a problem row, rules referencing it re-validated; refused when a live rule's op would become invalid unless `replace_refs`. Never silent.
- **Rename.** Label anytime. Key: `POST .../renames` job, copies values, updates rule references in drafts and live rules in one version, 90-day alias for writes. Intercom: impossible.
- **Archive.** Hidden from builder. Writes dropped with a problem row; values kept; refused with 409 `attribute_in_use` listing references. Unarchive anytime.
- **Delete.** Admin may delete an archived attribute: job removes values, definition goes, audit logged. Needs recent auth. Intercom: not possible even by support.
- **Limits.** Active definitions: 500 per object (Intercom soft 250); per contact stays 100 keys (data model); list options 200; value 1,024 chars. All entitlement keys, generous and same on self-host defaults.
- **Reserved.** `claimed_user_id`, `test_member_id`, `signed_up_at` semantics (01, 02) stay. Standard field names cannot be custom keys (`email`, `name`, ...), refusing Intercom's "Plan Name" clash class.

## Event ingestion

Why now: rule "did X 3 times in the last 7 days" and "never did X" are the top filter in this category. Today nothing ingests. 15's `Dimmi('track')` remains the client-side trigger; network is opt-in per event.

### Paths

| Path | Auth | Trust | Notes |
|---|---|---|---|
| `POST /api/v1/contact-events` | token `contacts:write` or `events:write` | verified | single or `{events: [...]}` up to 100; contact by `contact_id`, `external_id` or `email` |
| `@dimmi/server` `track()` helper | same | verified | thin |
| SDK `Dimmi('track', name, props)` | widget session | claimed | sends only when config lists name as `client_writable`; else stays client-only as 15 specifies. Batched, 1 per 2 s, `POST /widget/v1/contact-events`; no anonymous no-session network |

Body: `{name, contact: {...}, occurred_at?, metadata?, idempotency_key?}`. `occurred_at` default now, may be up to 30 days back (rollup updates; `daily` respects it), up to 1 hour ahead, else clamped with warning. Metadata flat scalars, 20 keys, 2 KB.

### Pipeline

1. Validate. Definition missing: verified write auto-creates `observed`; browser write dropped. Limit: 1,000 active definitions per workspace (Intercom 120); over: 422 `event_limit_reached`, shown in problems, never silent.
2. Idempotency: `SET NX` in Valkey for 48 h on `idempotency_key`. Rebuildable; a Valkey flush can double-count a retried event, accepted and documented. REST `Idempotency-Key` (foundation) also accepted.
3. Upsert `contact_event_summaries` single statement, `ON CONFLICT DO UPDATE`. Sync, because the row is per person and per event so low contention.
4. Emit in-process `contact_event.tracked`: push `surface.changed` (dependency filter), 15 engage triggers (later), 17 `workflow-contact-events` seam, 10 webhook `contact_event.tracked` when subscribed.

Rate: own bucket, 6,000 per minute per workspace default, entitlement. Not metered, not billed.

Visibility: next evaluation sees it. No "up to 24 hours for metadata" lag; metadata rules are not at MVP. Numbers sent as strings do not silently fail: no metadata rules, and `last_metadata` is display only.

### Not doing

No raw table at start; timeline shows rollups: "Exported a report, 12 times, last 3 h ago". See decision 5. No company-level events.

## Security

The core rule: **a browser can never unlock anything.**

### Trust levels

| Level | Source | Can pass `access`? |
|---|---|---|
| verified | 02 JWT claim, REST with token, teammate edit, trusted import mapping, integration | yes |
| observed | server measured: last seen, sessions, IP country | yes, except country when GeoIP missing |
| claimed | `PATCH /widget/v1/contact`, `session.context`, page, device, browser, locale, customer-typed answers via 15 write-back or 17 collect, unsigned events | never |

### Rules

1. **Locked by default.** `attribute_definitions.client_writable = false`. `PATCH /widget/v1/contact` with a non-writable key: key dropped, problem logged, no error to the browser (no oracle on what keys exist). Intercom is the reverse: open until you protect each one (research section 7).
2. **Key-level provenance stands.** Existing `trusted_attribute_keys` is unchanged; `access` evaluator reads only keys in it, regardless of `client_writable`.
3. **`access` mode.** Evaluator flag. Claimed fields refused at publish (`claimed_field_on_access_gate`). At runtime, an untrusted or missing value makes the leaf false, including negative ops: `plan neq free` with no verified `plan` is false. So staying anonymous cannot pass a negative rule. SQL compiler has the same mode. Unknown at the end is denied.
4. **`display` values from a browser only ever change what is shown.** Content behind a `display` surface is treated as public: builder copy says so. Secret in an announcement is not protected by its audience.
5. **Verified identity is the session, not the kind.** `identified` means session verified by 02. `Intercom` shim never verifies (01), so stays lead.
6. **Nothing about the rules reaches a browser.** See partial evaluation. Widget resources are allowlists (15 `widget_engage_item_resource` pattern). `start` returns only a boolean. 403 `conversation_start_restricted` carries no reason detail.
7. **Page leaves are public.** Builder labels them. Admin pasting `/internal-secret-admin` into a launcher rule sees `page_rule_is_public`. `access` surfaces cannot use page leaves at all.
8. **Enumeration.** Help center unknown and restricted slugs: same 404 for anonymous. Search, suggestions, sitemap, `llms.txt`, `.md` twins, public MCP, `ask` all filter. Timing: restricted branch does no extra rendering.
9. **Test-as** never impersonates: read-only, no writes, no receipts, no conversations visible, audit logged, needs `contacts:read` plus `audiences:manage`.
10. **Preview** needs `contacts:read` for sample and leaf values; counts alone need `audiences:read`. Tokens scoped accordingly (15's rule).
11. **SQL safety.** Compiler emits only parameters; field keys from registry, values bound; custom keys through `jsonb` extract with parameter. No string concatenation. Statement timeout 3 s, read-only transaction, tenant `SET LOCAL` as always.
12. **DoS.** Depth and leaf caps at save. Preview rate limit per member, one in flight per builder. Events rate limited separately.
13. **Prompt injection.** Attribute values never reach system prompts (07 keeps its allowlist). Audience only filters what dimmi can use, deterministically before generation.
14. **Stale identity.** Logout clears decisions. JWT `aud` per surface continues (02).
15. **Privacy.** Rules may hold emails. Rule JSON excluded from logs; audit stores hash and diff keys. Saved audience contacts export follows 03 `contacts:export`.

### Help center identity

Intercom needs a custom domain and cookie forwarding. We reuse 02's portal SSO:

- Admin sets `login_url`. Reader clicks "Sign in" on a restricted page: goes to `login_url?return_to=...`. Host app mints identity JWT `aud: "dimmi-help"`, single use, 300 s, delivers in URL fragment: `{help_host}/sso#token=...`.
- `hc.js` posts it; server verifies per 02, sets `__Host-dimmi_hc` httpOnly, 1 hour, re-mint silently via `login_url` on expiry. Works on the default subdomain and custom domains.
- Inside the widget, Help tab uses the widget session.
- 02 token surface enum gains `help_center`.

## Help center behaviour

- Effective audience = article AND every collection above it. Editing a collection recomputes all.
- A collection with no visible article disappears for that reader.
- Public pages unchanged: `public, max-age=0, must-revalidate` + ETag. Signed-in readers: page cache `variant` gains a hash of the allowed set, `Cache-Control: private, no-store` on restricted responses, public cache never sees restricted content. Test greps the anonymous response of every path for a canary title.
- Non-match signed in: 404. Anonymous on restricted or unknown slug when `login_url` set: same page with sign-in button, status 404.
- Teammate inserting a restricted article in a reply: chip "only visible to X" plus check against this contact; blocked with override per Intercom's pattern, since sending a link the customer cannot open only frustrates.
- Article has one audience for humans and dimmi. Dimmi must not tell a person what they cannot read. Internal policy remains `agent_only` + optional audience. Decision 4.
- `agent_only` with audience: dimmi only for those people.
- Knowledge search: `contracts.Knowledge.Search` takes `allowed_audience_ids`, filter `effective_audience_ids <@ $allowed`. Public `help_center` runs, anonymous MCP: empty set.
- Audience stays out of `content_hash`: changing it needs no review or re-embed, only a cache bump and the denormalised update.

## API

Rules per `00-foundation/rest.md`. Abilities new: `audiences:read`, `audiences:write`, `events:write`; `attributes:read`, `attributes:write` from 03 records.md track 12 (whichever ships first registers). Permissions `audiences:manage`, `attributes:manage` (owner, admin; 03 shares it). Entitlement `audiences.max_per_workspace` (cloud 200, self-host `-1`, owner 18): over 422 `entitlement_limit_reached`. Config `audiences.preview_timeout` 3 s, `audiences.contacts_timeout` 5 s (422 `audience_too_slow`). Metric `dimmi_audience_preview_seconds{approximate}`. Members read, can see preview counts, and with `contacts:read` samples.

| Method | Path | Name | Ability |
|---|---|---|---|
| GET, POST | `/api/v1/audiences` | audiences.index, .store | read, write |
| GET, PATCH, DELETE | `/api/v1/audiences/{adn}` | .show, .update, .destroy | |
| GET | `/api/v1/audiences/{adn}/contacts` | .contacts.index | `contacts:read` |
| GET | `/api/v1/audiences/{adn}/references` | .references.index | read |
| POST | `/api/v1/audience-previews` | audience-previews.store | read |
| POST | `/api/v1/audience-evaluations` | audience-evaluations.store | read + `contacts:read` |
| POST | `/api/v1/audience-lints` | audience-lints.store | read |
| GET | `/api/v1/audience-fields` | audience-fields.index | read |
| GET, POST | `/api/v1/attribute-definitions` | | attributes |
| GET, PATCH, DELETE | `/api/v1/attribute-definitions/{atd}` | archive = `PATCH {status}`; `DELETE` deletes | |
| POST | `/api/v1/attribute-definitions/{atd}/conversions` | | |
| POST | `/api/v1/attribute-definitions/{atd}/renames` | | |
| GET | `/api/v1/attribute-problems` | | |
| GET, POST | `/api/v1/event-definitions`, GET, PATCH `/{edf}` | | |
| POST | `/api/v1/contact-events` | contact-events.store | `events:write` |
| GET | `/api/v1/contacts/{ctc}/event-summaries` | | `contacts:read` |
| | tags on `/contacts`, `/companies` | 03 records.md: `PATCH` `add_tags`, `remove_tags`, `filter[tag]`; no separate routes | |
| | `filter[audience]=adn_` on `/contacts` and `/companies` | replaces 15's `filter[segment]` | |

Preview:

```json
POST /api/v1/audience-previews
{ "audience": { "visitors": "identified", "rules": { "all": [ ... ] } }, "surface": "workflow.audience", "sample": 10 }
```
```json
{ "data": { "object": "audience_preview", "count": 18234, "approximate": false, "total_contacts": 1002311,
  "anonymous_included": false, "sample": [ ... ], "lint": [ { "code": "negative_or", "path": "/all/1", "message": "..." } ],
  "computed_in_ms": 412 } }
```

Conversation-field rules replay the last 30 days, up to 1,000 conversations: `basis: "recent_conversations"`. Anonymous visitors are not counted: `anonymous_included: true` adds UI note "plus visitors with no profile". Over 3 s: sampled, rounded, `approximate: true`. Never errors with too many matches (Intercom's 50,000 limit).

Evaluation (test as a person):

```json
POST /api/v1/audience-evaluations
{ "subject": { "contact_id": "ctc_..." },
  "context": { "page_url": "https://app.acme.com/billing", "locale": "de", "country": "DE", "device": "mobile" },
  "audience": { "audience_id": "adn_..." } }
```
```json
{ "data": { "object": "audience_evaluation", "result": "match", "gate": "display",
  "tree": { "all": [ { "field": "company.custom.plan", "op": "in", "value": ["pro"],
      "actual": "pro", "trust": "verified", "result": true } ] },
  "residual_for_browser": { "..." : "..." } } }
```

`subject` also accepts `surface: "messenger"` with no audience: returns launcher, start, tabs, eligible engage items, matching workflows' audiences, visible help counts. That is the "see the product as this person" view.

Widget additions (01): `surface` block in config and session responses; `GET /widget/v1/surface`; `GET /widget/v1/apps/{app}/decisions`; `POST /widget/v1/contact-events`; realtime `surface.changed`; 403 `conversation_start_restricted`.

Webhooks: `audience.created/updated/deleted`, `attribute_definition.*`, `contact_event.tracked` (off unless subscribed, since high volume).

### MCP (toolset `audiences`, under 2,000 tokens)

`dimmi_list_audiences`, `dimmi_preview_audience`, `dimmi_test_audience`, `dimmi_save_audience` (always-confirm when it changes a live surface's reach), `dimmi_list_attributes`, `dimmi_define_attribute`, `dimmi_list_events`. Sample names come back enveloped as untrusted. MCP preview omits samples unless `contacts:read`.

### CLI

```text
dimmi audiences list|show|preview <file|adn>|test <adn> --contact ctc_... [--page URL] [--country DE]
dimmi audiences export --dir ./dimmi/audiences
dimmi audiences apply --dir ./dimmi/audiences [--dry-run]
dimmi attributes list|define|archive|convert
dimmi events list|send
dimmi audience explain <ctc>         # what this person sees across surfaces
```

JSON files round-trip like 17's workflows.

## Dashboard builder

Settings gets `Audiences` and `Data` (attributes, events, problems). New `resources/js/features/settings/pages/audiences.tsx`, `data.tsx`. Shared `resources/js/features/audiences/` component used by settings, 15 editors, 17's inspector (replacing `inspector.tsx:65` presets), 08's article panel, 01's Messenger settings. BoardUI Select, ComboBox, Tooltip, Badge; surface order per project rules. E2E per flow in `tests/e2e`, light and dark.

### Builder steps

1. **Who**: checkboxes Users, Leads, Visitors; or pick a saved audience. Copy states "Visitors have no profile: only page, device and language rules apply to them".
2. **Rules** rendered as sentences: "Plan is any of Pro, Team". Field picker searches by label, grouped (Person, Company, Behaviour, Tags, Saved audiences, Page, Conversation). Only fields valid for this surface; unavailable ones show why instead of vanishing. Value picker knows the type: list values from options, tag names, country names, date pickers with "more than N days ago", event count with "in the last N days".
3. **Grouping**: "All of" and "Any of" buttons at each level, indent shows nesting, drag to move. Max depth shown when reached. Plain English read-out at top: "Users on Pro or Team who were seen in the last 30 days".
4. **Preview pane**, always visible: count and proportion, sample of 10, each openable to see which leaves passed. Labelled when approximate. Refreshes 400 ms after edit.
5. **Test a person**: search box in pane. Pick a contact, set page, country and device; rule tree lights up leaf by leaf with their real values. "Open the Messenger as this person": a sandbox preview from 01's test session with the surface result of that contact; no writes, watermarked, stats excluded. Banner: "Showing what Ada would see, not her data."
6. **Warnings** inline next to the leaf, each with a one-click fix: `negative_or` offers to change `any` to `all`.
7. **Reach badge** before publish on live change: "This change adds 1,204 and removes 37 people" (diff of old and new).
8. **Save as audience** from any inline rule. Used-by list on each saved audience: per consumer, with status, so nobody edits a rule blind. Editing one used live asks: "Used by 3 live items, N people affected".

### Mistakes prevented

| Mistake | Prevention |
|---|---|
| "Not A or not B" matching everyone | `negative_or` with auto-fix |
| Excluding by custom email attribute | builder warns with `missing_value_match` count |
| Date attribute stored as text | `date_as_text` |
| Fixed vs dynamic chosen wrong | go-live card states who gets it, with counts now and growth since 30 days |
| Rule on data that never arrives | `event_not_tracked`, attribute last value date beside its name |
| Editing a rule used live | reach diff and used-by |
| Page rule leaking | `page_rule_is_public` |
| Trusting a lead's value | `claimed_field` badge on leaf, access refusal |
| Hidden launcher misread as "no Messenger" | copy under toggle: replies and announcements still arrive |

### Data page

Attributes list: label, key, type, object, last value, % contacts with value, used by, writable toggle, archive. Create drawer. Problems tab: the silent failures, grouped, last seen. Events list: count, last seen, writable. "Install" snippets for the first attribute and first event. Contact panel shows attributes with trust badge and event summaries.

## Performance targets

| Path | Target | How |
|---|---|---|
| Handshake | +2 ms p95, up to 2 queries | compiled cache, lazy tags and events |
| `GET /widget/v1/surface` | p95 under 15 ms, ETag | same |
| `decisions` | p95 under 5 ms | country from IP, config cached |
| `POST /contact-events` | p95 under 20 ms | one upsert |
| Evaluate 100 rules | under 1 ms | compiled tree, cached lower-case values |
| Preview | at 1M, 3 s then sample; index-backed common fields | GIN on attributes, statement timeout; `TABLESAMPLE` |
| Help page signed in | p95 under 80 ms miss, under 10 ms hit | variant cache per allowed set |
| Knowledge filter | under 2 ms overhead | array filter on existing index scan |
| Bundle | at most 500 B gz | CI size gate |

Load test harness in `tests/` extends foundation's: 1M contacts, 500 attributes defined, 100 live surfaces.

## Code map

```text
app/support/urlmatch/          glob matcher, shared JSON corpus with `lib/rules.ts`
app/support/audience/          parse.go, validate.go, evaluate.go, partial.go, compile.go, explain.go, lint.go, registry.go, doc.go
app/actions/audiences/         create, update, delete, preview, evaluate, list_contacts, rewrite_references
app/actions/attributes/        define, update, archive, delete, convert, rename, record_problem
app/actions/events/            track, define
app/jobs/                      convert_attribute, rename_attribute, backfill_attribute_dates, snapshot_audience
app/http/controllers/api/v1/   audience_*, attribute_definition_*, event_definition_controller, contact_event_controller
app/http/controllers/widget/   surface_controller, decision_controller, contact_event_controller
app/http/resources/            allowlisted widget surface resource
app/models/                    generated from database/queries/audiences.sql, attributes.sql, events.sql
app/mcp/tools/                 audiences.go
resources/widget/lib/rules.ts  client residual evaluator
resources/js/features/audiences/
tests/feature/audiences/, tests/e2e/audiences.spec.ts
```

`app/support/segments` and `app/support/conditions` are not created. 17's condition fields register into `audience`. Dynamic SQL: 15 names `app/support/querybuilder`, which doesn't exist; the compiler uses `framework/database` parameters and is the only dynamic predicate in the app.

## Required from other specs

All spec edits; nothing shipped is broken. Listed so owners acknowledge.

| Spec | Change |
|---|---|
| 15 | `segments` becomes `audiences`; `models.Audience` drops `segment_id` and `locales`; `Trigger.url_rules` replaced by `page` tree; ETag gets hour bucket; anonymous list by partial evaluation, no country-on-CDN; `app/support/segments` removed; `filter[segment]` renamed |
| 17 | section 3 "Conditions" becomes field registrations; `app/support/conditions` removed; `session.page_url` to `page.url`; audience builder shared; `collect` targets must be `client_writable` |
| 15 survey write-back | same `client_writable` rule; customer answers never trusted |
| 01 | `surface` block, routes, `PATCH /widget/v1/contact` drops non-writable keys silently, 403 on start, `Dimmi('track')` network when writable |
| 02 | `surface: help_center`, `aud: dimmi-help`; observed definitions from claims |
| 03 | new columns, tags on contacts and companies, event summaries on contact resource, `filter[audience]` |
| 07 | per-run evaluation, `allowed_audience_ids`, `action-audiences` becomes real |
| 08 | `audience_id`, `login_url`, `.md`, `llms`, sitemap and MCP filters, cache variant, public 404 rule |
| 09 | toolset and CLI |
| 10 | new webhook names |
| 13, 14 | reserved `audience` adopt the shared type when they use it |
| 16 | Intercom `data_attributes` map to definitions with types; segments import as draft audiences when rules are knowable (Intercom's API gives names only, so name-only stubs); protected attributes map to `client_writable = false` |
| foundation | prefixes, tables owner map, abilities |
| conventions | none |

## Edge cases

Case. Behaviour. All decided.

### Rule language

1. Rule references attribute later archived. Live rule stays evaluated on existing values; archive refused while referenced (409). Forced by delete: referencing leaf false, warning on consumer, 06 notifies admins once.
2. Saved audience edited while used live. Applies immediately; reach diff before save; audit logged.
3. Saved audience deleted while referenced. 409 `audience_in_use` listing references.
4. Cycle through three audiences. 422 `audience_cycle` with the path.
5. Leaf type mismatch, e.g. `gt` on string attribute. Publish error `op_invalid_for_type`; at runtime false.
6. `neq` on missing value in `display`. True, labelled in builder. In `access`: false.
7. Two negatives in `any`. Warning, not error: someone may mean it.
8. `in` with 51 values. 422 `limit_exceeded`; hint: tag them or save as list.
9. Leaf on custom key with no definition. Publish error `unknown_attribute`; builder offers to define.
10. Date value in ms via REST. Detected over 10^11, divided.
11. `on` during a DST change. Calendar day in workspace timezone by `framework/clock`.
12. Future date with `within_last_days`. False: past only. Use `within_next_days`.

### Anonymous and identified

13. Visitor with no session, rule with attributes only. `Partial` false, surface off, no request.
14. Visitor with country rule. `dynamic: true`, one uncached call.
15. No GeoIP on self-host, `session.country` in `display` rule. Leaf unknown then false; publish warning `geo_unavailable`. `access` surface: refused.
16. Visitor becomes lead (first message). Next surface fetch uses session; `visitors` re-applied. Conversation-start rule already passed at that moment.
17. Lead signs in. JWT claims applied before `surface` computed; same response.
18. JWT expired, widget signs out. Decisions dropped; anonymous surface from config.
19. Shared browser, next user. Logout drops; no decision persisted.
20. Invalid JWT. 02 behaviour, session anonymous, rule evaluates anonymous; no fail-open.
21. Lead with `claimed_user_id` set. Never a user; `identified` false.
22. Contact blocked. All widget surfaces as 01 (silent spam); rules not consulted.
23. Contact merged mid-session. Session moves to survivor (02); next fetch uses it.
24. Test session. Sees live rules with its own test lead, receipts `is_test` (15).

### Spoofing

25. Lead `PATCH`es `custom_attributes.plan=enterprise`. Not writable: dropped. If writable: stored, key removed from trusted, `access` false, `display` true with `claimed` label.
26. Browser sends `Dimmi('update', {context: {plan: 'pro'}})`. Only `session.context.plan`, never `contact.custom.plan`.
27. Forged `page` on message to pass a rule. `access` surfaces do not read page. `display`: harmless.
28. Direct `POST /widget/v1/conversations` with launcher hidden. Launcher is cosmetic: accepted unless `messenger.start` false, which is checked server side.
29. Spoofed `track` to unlock. Unsigned events are claimed; `access` refuses them.
30. Client sends modified residual. There is no return path: server never trusts client decisions.
31. Reader edits `__Host-dimmi_hc`. Encrypted, verified; invalid means anonymous.
32. Reading restricted article through `.md`, `llms-full.txt`, sitemap, public MCP, `ask`. All filtered; canary test.
33. Attribute value crafted as `{{...}}`. Rules read values; template rendering already escapes (03).
34. Rule JSON with SQL in a key. Keys validated against registry and bound as parameters.

### Evaluation timing

35. Attribute changes, tab closed. Next load.
36. Attribute changes during 304 window. `surface.changed` forces refetch; ETag changes with contact hash.
37. Relative date flips with no writes. Hour bucket in ETag. At most 1 hour late; test with `tc.Clock`.
38. SPA navigates 100 times in a minute. Local residual, no network.
39. Rule published while widget open. Publish bumps epoch; picked up at next refresh or 600 s, not pushed to all sessions (thundering herd).
40. Dependency set huge. Cap 2,000 keys; above: always send coalesced `surface.changed`.
41. Replica missed pubsub. Epoch in cache key checked against Valkey at most every 30 s.
42. Valkey down. Widget surface falls back to last cached compiled rules; if none, loads from Postgres; degraded, no fail-open on `access`.
43. Message trigger with `last_heard_at` rule. Reads previous value from payload.
44. Workflow in progress when audience edited. Runs keep their version (17); saved audience leaf resolved at match time only.
45. dimmi mid-conversation after downgrade. Next run excludes restricted content; earlier answers stay.
46. Attribute with `client_writable` turned off. Existing claimed values remain, not trusted, no new writes; label on contact.

### Attributes and events

47. New key via JWT, limit reached. Value dropped, `key_limit_reached` problem, 02 warning; token still valid.
48. Same key: number then string. String dropped, problem.
49. `date` definition created over existing strings. Backfill converts; unparsable moved to problem count, value kept until confirmed.
50. Convert string to number with 3 bad values. Dry run shows; run clears them, logs.
51. Attribute deleted with values at 1M contacts. Batched job; rule refs blocked first.
52. Contact erase. Summaries, tags cascade; snapshots removed.
53. Event burst: 10,000 identical for one contact. Rate limit 429, rollup correct for accepted; `daily` clamped.
54. Event out of order. `first_at` min, `last_at` max.
55. Event older than 90 days. Count and dates update; `daily` not.
56. Same `idempotency_key` retried. Dropped within 48 h; after Valkey flush double-counted, documented.
57. Event for unknown `external_id`. 404 by default; `upsert_contact: true` creates via 03 upsert.
58. Event on lead by email. Matches `email` per 03 rule; unverified email never merges.
59. Definition archived while events arrive. Dropped, `archived_write` problem; existing rollups stay.
60. Event name with capital letters. 422 from REST with hint; SDK `console.error`. No silent rewrite.

### Help center

61. Restricted page in search-engine index before. `noindex`, 404 for bots; old sitemap entry removed.
62. Collection audience excludes an article whose own audience allows. AND wins.
63. All articles restricted. Help center home shows empty state with sign-in, no 500.
64. `login_url` missing, reader on restricted link. Plain 404.
65. Article moved to a restricted collection. Effective recomputed, cache bumped, knowledge chunks updated in same job.
66. Saved audience edited: readers' allowed set changes. Next request, no cache leak: variant hash changes.
67. Teammate shares restricted link in reply. Warning and block, override with reason.
68. Changelog entry with audience (13 adopts). Follows same read filter.

### Preview and test-as

69. Rule with conversation fields. Replay; explicit `basis`.
70. Rule with page leaves only. Count `null`: "every person can match on those pages".
71. Preview on empty workspace. 0 plus guidance.
72. Sample includes deleted contact. Filtered.
73. Member without `contacts:read`. Count only, no sample.
74. Test-as on a contact never seen on the web. Standard fields empty; explanation per leaf.
75. Preview timeout. Sampled; labelled; retried off-request and stored as `last_estimate`.

### Fixed and CSV (last milestone)

76. Fixed audience, 1M members. Batched inserts; message starts when snapshot done; status `preparing`.
77. Fixed member erased or merged. Removed or carried over.
78. CSV with unmatched rows. Reports matched, unmatched; only existing contacts tagged.
79. CSV tagging in progress at go-live. Fixed waits; dynamic goes.

## Tasks

Ordered, each about 1 day, tests per `00-conventions/testing.md`: integration on real services, `tests.New(t)`, fake clock, widget e2e with Playwright. No real LLM in any test: 07 fake driver only. `make lint test` green per milestone.

### M0 Reconcile specs (no code, 1 day)

- [x] Apply "Required from other specs" to 01, 02, 03, 07, 08, 09, 10, 13, 14, 15, 16, 17, 20.
  - Accept: grep finds no `app/support/segments`, `app/support/conditions`, `segment_id`, `url_rules`, `filter[segment]` outside this file and `research/`. Owner signed off.
- [x] Foundation prefixes, tables owner map, abilities; folder in `specs/README.md`; BUILD-TRACKS.

Tracks: M1 to M4, M6 track 13 core. M5 track 13, needs 01 M2 session API per BUILD-TRACKS. M7 needs 08. M8 last.

### M1 Language (track 13)

- [ ] Migrations `001801`..`001802`, sqlc, factories, prefixes.
  - Accept: migrate up and down; RLS test workspace B cannot read A; prefix registry test.
- [ ] `app/support/urlmatch`, `app/support/audience`: parse, validate, registry, limits. Accept: table test per field and op; error per code with JSON path.
- [ ] Evaluate, Partial, Explain. Accept: Kleene table test; residual contains only client fields, property-tested on random trees; `access` mode table.
- [ ] SQL compiler, parity. Accept: 10,000 random contacts per field and op, Go equals SQL on real Postgres, both modes.
- [ ] Lint. Accept: one subtest per code.
- [ ] `tests/arch` rule: only this package parses rule JSON. Accept: fails on a planted second parser.

### M2 Data (track 13)

- [ ] Attribute definitions: table, CRUD, inference on verified writes, drop on browser writes, problems. Accept: REST and JWT auto-register; browser no; type mismatch logged; list; limit.
- [ ] Dates normalised, backfill job, conversions and renames, delete. Accept: dry run counts; edge cases 49, 50, 51.
- [ ] `client_writable`, reserved names. Accept: edge cases 25, 26, 46.
- [ ] Tag rule fields: tables, API built by track 04 (03 records.md). 18 adds `contact.tags`, `company.tags`, dependency filter. Accept: rule on tag; SQL parity; merge union, erase proven in rule tests.
- [ ] Contact columns: `last_heard_at`, `web_sessions`, `conversation_count`, backfill, GIN on `custom_attributes`. Accept: concurrent index migration; pre-event value in payload (edge case 43).

### M3 Saved audiences, preview, test-as (track 13)

- [ ] Audience CRUD, references, used-by, `filter[audience]`. Accept: 409s; edge cases 1, 2, 3, 4; every surface test asserts references.
- [ ] `/audience-previews` at scale. Accept: 1M seeded: under 3 s or labelled sample; conversation replay; lint inside.
- [ ] `/audience-evaluations`, messenger subject. Accept: leaf values and trust; permissions; audit row; edge cases 69 to 75.
- [ ] MCP toolset, CLI, export and apply. Accept: token budget test; round trip stable; confirm gate.

### M4 Builder UI (track 13)

- [ ] Shared rule builder, sentences, field picker per surface, values per type. Accept: unit tests; `pnpm lint` zero warnings; e2e light and dark.
- [ ] Preview pane, test a person, warnings with fixes, reach diff. Accept: e2e against seeded workspace: build rule, see count, test a contact, fix `negative_or`.
- [ ] Settings pages Audiences and Data. Accept: e2e create attribute, archive refused while used; problems tab shows a planted mismatch.
- [ ] Replace prototype presets in the automation inspector. Accept: `automation.spec.ts` updated.

### M5 Widget surfaces (track 13, needs 01 M2 session API)

- [ ] `surface` in config and session, `widget_settings.audiences`, launcher and tabs. Accept: canary test: response contains no attribute, event or audience names; zero-cost test.
- [ ] `messenger.start` server check. Accept: edge cases 25, 28, 29; 403.
- [ ] Extend 01 M8 `lib/rules.ts` to full residual with shared corpus, size gate. Accept: Go and vitest on same JSON; CI over 500 B gz fails; history navigation e2e.
- [ ] `decisions` route, loader skip. Accept: bundle not requested until `show()`; country e2e.
- [ ] `surface.changed` with dependency filter, ETag hour bucket. Accept: attribute REST write reaches open widget under 5 s; clock test edge case 37.
- [ ] 15 and 17 adopt surface hooks when those milestones run. Accept: their feature tests use `audience`.

### M6 Events (track 13)

- [ ] Definitions, rollups, `POST /contact-events`, idempotency, limits. Accept: benchmark; edge cases 53 to 60.
- [ ] Event fields in rules, window counts, SQL parity. Accept: parity on events; "3 times in 7 days" with fake clock.
- [ ] SDK `track` network opt-in, `surface.writable_events`. Accept: non-writable stays local (no request e2e); batching.
- [ ] Contact panel event summaries; Data page events. Accept: e2e.

### M7 Help center and dimmi (needs 08)

- [ ] `audience_id` on articles, collections, effective set, denormalise to chunks. Accept: edge cases 62, 65; `<@` filter.
- [ ] Read filter on all outputs, public 404 rule, cache variant. Accept: canary across HTML, `.md`, sitemap, `llms`, search, ask, MCP; edge cases 61, 63, 64, 66.
- [ ] SSO for help center, `login_url`. Accept: fragment token, single use, cookie attributes; expired token.
- [ ] 07 per-run `allowed_audience_ids`, actions audience. Accept: fake LLM driver; mid-conversation change; never a real model.
- [ ] Inbox article insert warning. Accept: e2e.
- [ ] Manual e2e flow table per `docs/testing/manual-e2e.md`.

### M8 Fixed and CSV (last)

- [ ] Snapshots, go-live job, statuses. Accept: edge cases 76, 77; no auto-pause at day 31.
- [ ] CSV list to tag. Accept: edge cases 78, 79.

## Where we beat Intercom

| Intercom pain (see research) | dimmi |
|---|---|
| Same-looking rules on each surface, limits and timings differ, three unrelated reusable objects | one language, one object, one builder, one preview |
| Attributes writable from the browser until protected, JWT only | locked by default; browser writes are opt-in and labelled |
| Rules and data maybe in the browser; docs silent | partial evaluation: browser gets a boolean or page-only tree; tested |
| `update` throttle, stale SPA URL, event needs a refresh | local per-navigation, no throttle; server push on data change |
| Fin evaluates once | every run |
| Two audiences per article | one audience, human and AI agree; `agent_only` for the difference |
| Help center audience needs custom domain and cookie forwarding | SSO fragment on any host |
| No collection audience | article AND collection chain |
| Shallow groups, 10 x 10 caps, negative OR | nesting depth 4, 50 leaves, `not`, saved audiences as leaves, negative OR warned with fix |
| Preview may be wrong, disabled with conversation fields, 50k error | explainer per leaf, replay on real conversations, never an error |
| "Test" means set live with your email | test a person with leaf-level result; sandbox Messenger as them |
| Attributes cannot be renamed, deleted; type locked with no path | rename, delete, conversion with dry run |
| Silent drops (nested, Jira clash, string numbers, brand id) | problems list; everything is logged |
| Fixed audience auto-pauses after 30 days | snapshot stays until you end it |
| Segments API read-only; rules UI-only | full CRUD, JSON rules, CLI export and apply, MCP |
| Dates by day; negative numbers for future | named ops, rolling days, calendar `on` |
| 250 attributes, 120 events | 500 per object, 1,000 events, never metered |
| Case-sensitive `is`, region capitals | case-insensitive everywhere; country picker |

Honest limits against Intercom: no multi-company "any" until `contact_companies`; no event metadata rules; no raw event history at start; no Series-like journeys (17 covers conversation flows, outbound journeys are not planned); enrichment only through integrations.

## Decisions (all recommended answers approved)

Recommended answer in bold.

1. **Rename "segments" to "audiences" everywhere, table and API.** Nothing is built, so it is one spec edit. Why: two words for one idea is Intercom's trap; we don't need segment vs audience. Alternative: keep `segments` in code and "Audiences" in UI: cheaper edit, permanent confusion.
2. **18 owns the language; 15 and 17 consume it.** Why: 17's catalog says "reuses 15" but adds fields and `not`, and 15's trigger has its own URL rule language: already drifting. Alternative: 15 keeps ownership: same result, wrong folder, because 01 and 08 will depend on it.
3. **Browser-written values never open `access` gates; workflows may read them but get a warning.** Why: blocking in workflows breaks "collect then branch", which only routes. A lead claiming `plan=enterprise` can reach a VIP lane until the admin chooses a verified attribute; lint says so. Stricter alternative: workflow audiences are `access` too, and branch conditions `display`. Pick this if you value strictness over zero-friction collect flows.
4. **One audience per article for humans and dimmi.** Why: if a person can't read it, dimmi shouldn't reveal it; Zendesk does it this way; `agent_only` already covers internal knowledge. Alternative: two settings like Intercom: flexible, and a known source of confusion.
5. **No raw event table at first: rollups with 90-day daily counts.** Why: answers every rule cheaply; self-host disk stays small. Cost: no event timeline or export, and we cannot add a new window after the fact. Alternative: partitioned `contact_events` with 90-day retention: about 1 week more and meaningful storage; add when someone asks for a timeline.
6. **Ship events in this feature (M6), not post-MVP.** Why: 01 and 15 park it, but "did X" rules are the most asked-for filter and every comparison includes them. Alternative: attributes first and let customers send counters as attributes; saves M6.
7. **Auto-register attributes on the first verified write; never from a browser.** Why: developers get first-JWT-just-works, and browsers cannot pollute the list. Intercom moved the other way; we keep the good half. Alternative: require pre-creation, as Intercom does now: tidier, worse first run.
8. **Add contact and company tags (resolves 15 open question 1).** Why: CSV lists and manual groups need them; importers carry them. Alternative: attributes as flags: no new tables, no manual list story.
9. **`messenger.start` is enforced server-side and takes no page rules.** Why: "start" is a capacity and access decision. Use the launcher rule for page-based hiding. Alternative: allow page rules as UI hint only: more flexible, misleading because a direct call passes.
10. **Help center SSO over fragment, no cookie forwarding, needs customer's `login_url`.** Why: works on our default domain, no proxy. Cost: customer adds one route, same as 14's portal SSO. Alternative: only widget session readers: no article audience on the public site.
11. **Page rules are visible to browsers, and that is accepted.** Why: required for instant local re-check and zero requests. Builder labels it; `access` surfaces cannot use them. Alternative: server-only page checks per navigation: leak-free, but a request per navigation breaks the zero-cost bar.
12. **Build order: M1 to M4 right after 03 and before 15 and 17; M5 with 01; M7 after 08.** Why: 15 and 17 both start by building the evaluator they'd otherwise duplicate. Alternative: build inside 15 first and extract: the drift we are trying to stop.

## Decisions 2026-11-08 (first build)

Workflows shipped first, so audiences start from the rule language they already use.

- **Built:** saved audiences (`audiences` table, CRUD under `/audiences`, lock version, nesting two levels deep, cycles refused with the path), used-by (other audiences, workflow drafts, live workflows), delete refused while another audience or a live workflow uses it, `POST /audience-previews`, `POST /audiences/{id}/previews` (keeps `last_estimate`), `POST /audience-evaluations` (test a person with page, device, language, country typed in), `GET /audience-fields` (registry plus custom attribute keys in use, typed from a value seen), lint `negative_or`, `impossible_and`, `page_rule_is_public`. Workflows evaluate saved audience leaves (loaded only when a rule names one) and refuse to publish one naming a missing audience.
- **Prefix `audn_`, not `adn_`.** The spelling linter reads `adn` as a typo of "and"; the linter's configuration stays as it is.
- **No `audience_references` table yet.** Used-by is a query over audience rules, workflow drafts and live versions' compiled references. Always right, never needs rewriting on save; the table comes when the push dependency set (M5) needs it.
- **Preview uses the Go evaluator, not a SQL compiler.** Up to 50,000 contacts are read in pages and evaluated; above that a random sample of about 20,000 is read and the count scaled (`approximate: true`). One evaluator means no parity gap. The SQL compiler comes with `filter[audience]` on contact lists and million-contact workspaces.
- **Trust in tests:** a custom attribute counts as verified when its key is in the contact's `trusted_attribute_keys`.
- **UI:** Customers, Audiences: list with last count, editor with "Who" (visitor groups), conditions and one level of groups matched all, any or none, live count 400 ms after each edit with a sample, warnings, test a person, used-by. Deeper trees written through the API are shown as a line and left alone. The workflow builder picks Everyone or one saved audience under "Who it runs for".
- **Not yet:** attribute definitions and problems (M2), event ingestion (M6), widget surfaces and the client evaluator (M5), help center and dimmi audiences (M7), fixed audiences and CSV (M8), `filter[audience]`, MCP and CLI, reach diff on save, conversation-field replay in previews.

## Decisions 2026-11-08 (attribute definitions)

- **Built:** `attribute_definitions` (`atd_`) and `attribute_problems`; every custom attribute write goes through one gate (`app/attributedefs`): REST contacts and companies, upserts, teammate edits, signed tokens and company claims are trusted and define a new key from its first value (500 active per object, else `key_limit_reached`); the messenger and workflow collect answers never define one. Values that do not fit their type, list values outside the list, and writes to archived keys are dropped and counted as problems; the write itself never fails. Dates arrive as ISO 8601 or Unix seconds or milliseconds and are stored as Unix seconds. API: `/attribute-definitions` (list, define, change label, description, list values, status, `client_writable`; delete an archived one with its values) and `/attribute-problems`. Listing defines keys already in use as `observed`, so existing workspaces start from their real data. Archive refused with 409 `attribute_in_use` while a saved audience or workflow reads the field. Rule builder fields take type, label and list values from definitions. Settings, Attributes page.
- **`client_writable` stays the sign-in settings' list** (20 keys), which the messenger already enforces; the definition shows and flips it. No second column.
- **Options are plain strings**, not `{value, label}`.
- **Not yet:** type conversion and key renames, backfilling existing values when a date is defined over text, nightly `contacts_with_value`, deleting in batches for very large workspaces (delete is one statement today), dropped keys from the browser listed as problems.

## Decisions 2026-11-08 (events)

- **Built:** `event_definitions` (`edf_`), `contact_event_summaries` (count, first and last time, 90 UTC daily counts from `daily_day`, last metadata), `POST /contact-events` (one or up to 100; person by `contact_id`, `external_id` or `email`; `idempotency_key` held 48 h in Valkey; times older than 30 days or over an hour ahead moved and marked `occurred_at_clamped`; metadata flat, 20 keys, 2 KB; names lowercase with a hint), `GET /event-definitions`, `PATCH` label, description, status, `GET /contacts/{id}/event-summaries`. Names are defined on their first event (1,000 per workspace, then `event_limit_reached`). Rules read `event.<name>.count` (with `within_days`), `.first_at`, `.last_at`; never did it reads as count 0. Workflows, previews and test a person load summaries only when a rule reads events. Builder: an Activity group and an "in the last N days" box on counts. Profile Activity card; Events section on Settings, Attributes.
- **Days are UTC calendar days**, not rolling 24 hours: "in the last 7 days" is today and the six days before, in UTC.
- **Ability:** recording takes `contacts:write`; no separate `events:write` group yet.
- **Not yet:** messenger `track()` over the network and `writable_events`, rate limit bucket, `upsert_contact`, webhooks `contact_event.tracked`, workflow triggers on events. A lead that signs in as a known customer adds its summaries to the customer's (totals add, first and last widen, days add, newest details win) and keeps none; erase cascades.

## Decisions 2026-11-08 (messenger rules)

- **Built:** `widget_settings.audiences` (`launcher`, `start`, `tabs.help`, `tabs.changelog`), `PATCH /widget-settings/{widget}/audiences` (a rule sent replaces that one, null removes it, one left out stays; checked: each rule compiles for its surface; saved audiences exist; the start rule and every saved audience inside it read only what the server knows and trusts), config `surface.anonymous` (only when rules exist), session `surface` for known visitors (contact, company, tags, events), 403 `conversation_start_restricted` at the first message. Saved audience leaves left open are replaced by their own open rules, so a browser never sees an audience id. The widget checks page and device conditions on load and on every route change, with no request; its rule checker is a separate chunk fetched only when rules exist (core stays under its 35 KB budget; the chunk is 0.9 KB gz, over the 500 B target). Settings, Messengers, "Who sees it" picks Everyone or a saved audience per part.
- **Fixed on the way:** a launcher styled "hidden" was still drawn; `.launcher[hidden]` now hides it.
- **Not yet:** `dynamic` country decisions and `GET /widget/v1/apps/{app}/decisions`, `GET /widget/v1/surface` with ETag and `surface.changed` push (a known visitor's outcome refreshes at the next session read), loader skipping the core when the anonymous launcher is false, the start button reflecting `surface.start` before sending, the hour bucket for relative dates, and a saved audience edit refreshing cached anonymous configs before their cache time ends.

## Decisions 2026-11-08 (help center and dimmi)

- **Built:** `articles.audience_id` and `collections.audience_id`; `PATCH /articles/{id}/audience` and `/collections/{id}/audience` (`null` opens to everyone). A reader sees an article only when they are in its audience and every collection above it. Readers who are not signed in (the public site, the messenger's Help tab) get a snapshot without what they may not read, so pages, `.md`, search, home and everything built from the snapshot agree. dimmi's knowledge search takes `AudienceFiltered` and `AllowedAudienceIDs`, worked out for the conversation's contact on every search; the playground, with no customer, is not filtered. Audiences in use by articles or collections cannot be deleted. Docs editor: an Audience picker that saves at once.
- **Access rules may read custom attributes, trusted ones only.** `contact.custom.*` has trust `by_key`: an access rule (start, articles) may use it, and evaluation then reads only keys in `trusted_attribute_keys`. Access evaluation is strict: a missing value fails every condition, `neq` included. The messenger start rule follows the same semantics.
- **Not yet:** signed-in help center readers (`login_url` and SSO), the messenger Help tab reading as the known visitor, restricted-article chips in the inbox, `Document` fetches filtered by audience (dimmi only fetches ids its filtered search returned), denormalised `effective_audience_ids` on chunks (filtering is in Go on candidates), `noindex` and sitemap cleanup for previously public pages, and a collection list that hides collections whose every article is hidden.

## Decisions 2026-11-08 (help center sign-in)

- **Built:** `settings.login_url` on help centers (https, or http on localhost). Pages show Sign in leading to it with `return_to` = `{help center}/sso?next=<page>`; the team's product sends the reader back with an identity token (the messenger's secrets and checks; `aud` may be `dimmi-help`; a nonce makes it single use) in the fragment, and the page's script posts it. Signing in identifies the customer as the messenger does (claims and custom attributes written, trusted). A signed, httpOnly, SameSite=Lax cookie `dimmi_help_reader` scoped to the help center's path remembers them for an hour. Signed-in pages and search show what the reader's audiences open, are never cached (`private, no-store`) and are built fresh rather than from the shared snapshot. Return addresses outside the help center go home. The header's Sign in button no longer points real help centers at the sample portal's page.
- **Cookie name:** `dimmi_help_reader`, not `__Host-`, since path-mode help centers (`/hc/{name}`) need a path and local development runs over http.
- **Not yet:** the inbox warning for sharing restricted articles. The sitemap and `llms.txt` always list only what anyone may read.

## Decisions 2026-11-08 (email code and messenger pass)

Competitors compared in `research/competitors/help-center-sign-in.md`: Intercom reuses the Messenger's session through a custom domain and cookie forwarding; Zendesk and Featurebase have their own reader accounts; Help Scout and GitBook only take a signed callback from the team.

- **Email code on by default** (`settings.email_sign_in`, unset means on). The Sign in button shows when audiences keep something from the reader or a login address is set, and leads to `/sso/sign-in`: "Continue with your account" when there is a login address, then the email form. Six digits, ten minutes, five tries, one send per 30 seconds and five an hour per address, twenty an hour per network: the messenger's rules. A good code proves the address only: the reader is the customer whose address the team's product gave (users before leads, then verified, then most recently seen), else a new lead with the address verified. Nobody is merged.
- **Messenger pass instead of shared cookies.** A visitor signed in by the team's product, or with an email proven in the messenger in the last day, gets a pass when opening an article from the Help tab: one use, one minute, in the fragment. It works on any domain, unlike Intercom's cookie forwarding. Passes go one way: signing in on the help center does not sign anyone in to the messenger.
- **Help tab as the known visitor.** `GET /widget/v1/help` with a session lists what the visitor's audiences open; the public `/apps/{app}/help` still lists only what anyone may read. Both carry the help center's `url`.
- **Seven days, renewed.** However a reader signed in, the cookie lasts seven days and is renewed on a visit once a day has passed.
- **Not yet:** social sign-in, OIDC and SAML for readers, and requiring sign-in for a whole help center.

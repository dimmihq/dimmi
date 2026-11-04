# Intercom audiences, in production detail (plus Zendesk and Crisp)

Research date: 2026-11-04. Input for `specs/18-audiences`. Public docs only: help center, developer docs, changelog, community forum. No sign-in, no workspace to test in.

How to read this:

- Everything is summarised in our own words. Every claim links its source; link targets are listed at the bottom.
- `[snippet]` = read through a search result summary, not the whole page. Treat as weaker.
- `[forum]` = a community thread; staff replies say so.
- `[conflict]` = two Intercom sources disagree; both are cited.
- Section 12 lists what public docs could not confirm. Read it before relying on anything here.

## 1. TL;DR

- "Audience" is not one thing at Intercom. It is the same rule builder pasted onto many surfaces, each with its own defaults, limits and evaluation moment. Rules set on one surface do not carry to another ([rules]).
- There are three reusable objects that barely relate: **segments** (saved contact filters, [segments]), **tags** (manual, [tags]) and **Fin audiences** (saved rules for AI content, workflows and connectors, [fin]). A segment can be used inside the others ([fin], [hc]).
- The rule model is shallow: a group of filters with one AND/OR connector, and groups joined by one connector ([rules], [filters]). Documented caps: 10 filters per OR group, 10 OR groups per workflow trigger ([rules]). The public search API documents 2 nesting levels and 15 filters per group ([search-api]).
- Data arrives as standard attributes, custom attributes (250 active soft limit), events (120 active) and tags ([cda], [track], [event-limits]).
- Evaluation is a patchwork. Email rules run hourly, Series about hourly or on ping, in-app on visit or `update`, Fin once at conversation start, workflow page rules on the device ([dynfix], [series-faq], [fin], [url-rules]).
- Identity: attributes are writable from the browser unless the admin locks each one per attribute. Locking only works with JWT ([cda], [jwt]).
- Public API: segments are read-only, with no rules in the payload. The rules themselves are UI-only ([segments-api], [segment-object], [connector-pr]).
- Biggest complaints: stale data until page refresh, SPA page URL staleness, negative OR always matching, no collection-level article audience, previews that do not predict matching ([event-delay], [troubleshoot], [conditional], [hc], [chats-faq]).

## 2. Where audiences appear

| Surface | What the audience controls | Options | Notes | Source |
|---|---|---|---|---|
| Messenger launcher | Whether the standard launcher shows | Visitors and/or users, plus display conditions (current page URL, geography, custom attributes, Clearbit data) | Only the launcher: outbound and replies still arrive, custom launchers keep working. `hide_default_launcher` in code overrides settings | [launcher] |
| Start conversation | "Control inbound volume" decides who may start a conversation; can be narrowed with attribute conditions | Shipped April 2020 | Blocks conversation-based workflow triggers for those people | [startconv], [setup], [troubleshoot] |
| Messenger spaces | Spaces on or off, order; launch directly into conversation per visitors, users, mobile users; CTA text per visitors and users | Workspace-level toggles | No documented per-attribute audience on a space itself | [setup], [faqs-msg] |
| Tickets space | Visibility set separately for visitors and users; only shows when the person has a ticket | | [snippet] | [tickets] |
| Tasks space | Shows only to people who received a checklist; audience per checklist, not per step | | [snippet] | [checklist] |
| News | Audience lives on the Newsfeed, not the item; a person sees only the highest-priority matching feed. Rules apply only inside the Messenger, a direct URL still opens | Users, leads, visitors plus attribute rules | [snippet] | [news], [faqs-msg] |
| Home apps | "+ Add audience rule" under each app | Article says rules apply to a whole group and cannot split subgroups, yet its example targets by a custom attribute `[conflict]` | Up to 19 apps | [home-apps], [faqs-msg] |
| Outbound (chat, post, banner, email, push, carousel, tour, survey, checklist) | Who (filters), where (page URL, time on page), when (one event rule), dynamic or fixed | Rules tab, same across types with small differences | Banners dynamic only | [message-rules], [dynfix], [rules] |
| Series | Entry rule blocks, mid-path rule blocks, exit rules, goals | | Section 7 | [series-faq], [series-explained] |
| Workflows | Trigger audience (predefined, custom) and branch conditions | Users, leads, visitors; mobile-only channels must target users | Section 7 | [trigger], [conditional], [url-rules] |
| Fin and Copilot | Saved audiences on content, guidance, escalation, data connectors, tasks, workflows. Several audiences on one item are OR. Nothing assigned means everyone | Rules from person, company, conversation attributes, tags, segments | Section 7 | [fin] |
| Help Center articles | Audience per article: leads, users, visitors plus filters | Not per collection | Needs custom domain and cookie forwarding | [hc] |
| Fin audience vs Help Center audience on an article | Two separate settings; Fin uses an article only when both match | | | [hc], [fin] |
| Spotlight | A Messenger variant replacing the widget for a chosen audience | | | [setup] |
| Copilot past conversations | Answers only from past conversations with matching attribute values; per attribute, not per conversation | | | [fin] |
| Bulk message permission | Separate permission for bulk messaging visitors, leads, users | | | [rules] |

Relation of the reusable objects:

- **Segment**: named saved filters; membership automatic ([segments]). Usable as audience rule on messages, articles ("Segment is not X" also works on workflows) ([hc], [trigger]).
- **Fin audience**: named saved rules, under Fin settings, usable on Fin content, guidance, tasks and workflows, with a "used by" view ([fin]). Different from a segment; can contain a segment ([fin]). Menu path differs between docs (Settings > AI & Automation vs Fin AI Agent > Fin settings) ([fin]).
- **Tag**: manual, on users, leads, companies, conversations. Tags have no automation ([tags]). Docs suggest tags as a way round attribute limits ([cda]).
- **CSV list**: "Filter audience from CSV" in outbound tags matching existing people, then the tag is the rule. Up to 5 columns (AND), tagging takes up to hours at peak ([csv]).

## 3. Rule model

### Grouping

- AND/OR rules on message audiences arrived April 2019, also in people and company filters ([andor]).
- UI flow: first filter, "+" adds a second joined by AND. For OR, add a filter group and set the group to OR ([filters]).
- A filter group and the connector between groups can each be AND or OR ([rules]). The URL rules article says the toggle applies to all rules in a group at once ([url-rules]).
- Documented depth: not stated for the builder ([rules]). Public search API: 2 levels, 15 filters per group ([search-api]).
- Workflow and audience caps: 10 filters per OR group; 10 OR groups per trigger; the add button disables at the cap ([rules], [conditional]).
- Very complex filters may run slowly or be refused, with a notice to simplify ([segments]).
- Older forum replies report duplicating a bot where OR was not available ([or-thread]) `[forum]`.

### Operators per type

From the filter docs (page is old, may have moved on) ([filters]):

| Type | Operators |
|---|---|
| Text | starts with, ends with, contains, contains exact word, is, is not, is unknown, has any value |
| Number | greater than, less than, is, is not, is unknown, has any value. No greater-or-equal |
| Boolean | true, false, is unknown, has any value |
| Date | more than, less than, exactly (days ago), after, on, before, is unknown, has any value |

Details that bite:

- "is" and "is not" are case sensitive ([filters]).
- "contains" matches inside words ([filters]).
- Region and country need full names and matching capitals ([filters], [segments]).
- Banner FAQ warns off "contains exact word" as a complex predicate ([chats-faq]).
- A filter on a tag needs a Person Tag condition picked first ([rules]).

Public search API operators are similar but not identical: `=`, `!=`, `IN`, `NIN`, `>`, `<`, `>=`, `<=`, `~`, `!~`, `^`, `$`; `tag_id` and `segment_id` only `=` and `!=`; IN capped at 15 values; strings under 256 chars ([search-api]).

### Relative dates

- A "day" is a rolling 24 hours from now. "Exactly 0 days ago" = under 24 hours ago ([filters], [contacts-faq]).
- Future dates need negative numbers: "more than -3 days ago" with "less than 0 days ago" ([filters], [contacts-faq]).
- "Less than" also matches future dates, so pair it with "more than 0" for past-only ([filters]).
- Date filters on custom attributes work by day only ([cda]). In the API, date custom attributes match by UTC calendar day and refuse `!=`, `>=`, `<=` ([search-api]).

### Missing values

- Negatives on empty custom attributes pass: a custom attribute that is empty makes "does not contain" true ([rules]).
- `!=` in the API also matches contacts with no value ([search-api]).
- Company "is not" with no linked company is not true; the Else path runs ([troubleshoot], [trigger]).
- "Company ID is unknown" does not return people without a company; workaround is a second segment excluding "has any value" ([segments], [filters]).
- Negative OR ("city is not Dublin OR city is not London") matches almost everyone. Intercom warns in several articles, no sign of the builder preventing it ([conditional], [rules]).

### Attribute sources

| Source | Examples | Notes | Source |
|---|---|---|---|
| Standard | Signed up, last seen, web sessions, first seen, browser version, browser language, language override, OS, city, country, recent page views | Updated automatically. Mobile adds last seen, sessions, device per platform | [data-tracked], [track] |
| Location | Country, region, city, continent code | From IP via MaxMind; cannot be edited; a custom attribute needed for manual country | [cda], [launcher] |
| Custom person and company | Plan, seats, sign-up date | Section 5 | [cda] |
| Company | Filter matches if any of a person's companies matches. One-off messages check at most 10 companies per user; a contact may have at most 1000 companies (API error sample) | Company "last seen" needs company in the ping | [filters], [segments], [company-limits], [contacts-faq] |
| Events | Count and recency; count 0 = never | Section 6 | [event-filters], [track] |
| Tags | Person tag, company tag, conversation tag | Conversation tag not on three triggers (visits a page, opens new conversation, clicks element); one Fin doc says conversation tags cannot be used at all `[conflict]` `[snippet]` | [trigger], [fin-mirror] |
| Segments | "Segment is / is not" | | [trigger], [hc] |
| Page | Current page URL (is, is not, contains; workflows also exact and starts-with), time on page, last page | Section 7 | [launcher], [url-rules], [message-rules] |
| Conversation | Conversation attributes (Fin audience disables preview), conversation starter (customer message, teammate direct, teammate to many, workflow, outbound), conversation tag | Copilot too | [fin], [trigger] |
| Enrichment | Clearbit Reveal firmographics (needs own account) | | [launcher] |
| Brand | Default attribute for Fin audiences; brand matched by internal id; mismatch silently drops content | | [fin] |
| Lifecycle | Visitor, lead, user checkboxes | Section 4 | [contacts-guide], [url-rules] |

Custom objects are not in Fin data access yet, per the Data Hub article ([data-hub]) `[snippet]`.

### Dynamic vs fixed

- Dynamic: everyone matching now and anyone matching later; each person once unless event-triggered or using a company variable ([dynfix], [segments]).
- Fixed: only those matching at go-live ([dynfix]).
- Type cannot change after go-live ([dynfix]).
- Fixed auto-pauses after 30 days, cannot be unpaused, cannot have an end date; shareable links and code triggers stop then ([dynfix]).
- Rulesets with no match or no update for over a year are suspended ([dynfix]).
- Banners are dynamic only; event-triggered messages must be dynamic ([dynfix], [message-rules]).
- Email rules can only be edited before live. After that: duplicate, edit, set live ([rules]).
- Dynamic audiences support sending from the account owner; fixed do not ([owner]) `[snippet]`.

### Page, send time and event interplay

- The targeting filter must already be true when an event fires. The event does not update the person in the same request ([event-filters]).
- Outside schedule, triggered sends are dropped, not held. Audience-rule messages are checked again later ([event-filters]).
- Audience rules can read the last visited page, which may be stale; "where to send" reads the live page ([troubleshoot]).
- Outbound messages "follow" visitors after triggering ([url-rules]).

## 4. Visitor, lead, user

- Visitor: unknown, logged out, no conversation history. Starting a conversation or replying to outbound turns a visitor into a lead. Lead becomes user on sign-up or login ([contacts-guide]) `[snippet]`.
- Visitor data ties to `intercom-id` / `intercom-session` cookies until identified ([contacts-guide]). Visitors unseen for 9 months are deleted ([data-tracked]).
- A first-time visitor who opens the Messenger is matched as a visitor snapshot, even though they then become a lead. Check all three types ([troubleshoot]).
- Users-only rules do not fire in incognito or for unrecognised browsers ([troubleshoot]).
- Identified users boot with `user_id`/`email`; logged-out boot with only `app_id` ([spa]).
- Default on custom workflow audience: Users only ([url-rules]).

## 5. Attribute registry

- Types: text, number (integer), boolean, date/time; list for qualification data ([cda]). The API enum adds float, datetime and relationship; a list comes back as string plus `options` ([api-attr-model]) `[snippet]`.
- Key names: case sensitive; no periods, dollar signs, special characters or NULL; unsupported chars become underscores ([cda]).
- Dates: keys ending `_at` treated as dates; Unix seconds, not ms ([cda]).
- Number values cannot start with 0 ([cda]). Text values capped at 255 chars (custom attributes only) ([cda]).
- Values must be scalars; nested and arrays not accepted ([cda]).
- Snippet: inline keys; a nested `custom_attributes` object in JS is silently dropped ([cda]).
- Clearing: empty string; in the Messenger, null ([cda]).
- Must exist first: Messenger, API, CSV and integrations can only update existing attributes ([cda]). A changelog entry says the same, but it now 404s to our fetcher ([changelog-create]) `[snippet]`. The article is internally unclear about unsupported characters creating attributes ([cda]).
- Limit: soft 250 active people plus company attributes, can be raised on request ([cda]). Conversation attributes capped at 5,000 ([cvda]) `[snippet]`.
- List options: 35, hard ([cda]); a 2021 staff reply said 20 `[conflict]` ([list-limit]) `[forum]`. Values set by API outside options are not permanent and can be lost on edit ([cda]).
- Rename: impossible; archive and create new ([cda]). People and company share a namespace, even after archive ([cda]).
- Archive: not deletable, even by support. Archived attributes leave the filters; unarchive at the bottom of the list ([cda]).
- Archive can fail when dependents exist, per API docs ([api-update]) `[snippet]`. Listing excludes archived by default ([api-list]) `[snippet]`.
- Type change: API says not possible, directs to the UI ([api-update]). Third-party doc claims the UI converts values silently ([revos]) `[snippet]`. Not confirmed by Intercom.
- Clash: a custom company "Plan Name" collides with the built-in one and shows twice ([cda]). A CDA named like a Jira field routes updates to Jira and fails silently ([cda]).
- Fresh in Fin: attributes must be synced to the Fin workspace before use ([fin-mirror]) `[snippet]`.

## 6. Events

- API: `POST /events`; JS `Intercom('trackEvent', name, metadata)`; associated with the logged-in user ([api-events], [js-methods]).
- Stored per person: first time, last time, total count ([track]). Person level only, no company events ([event-limits]).
- Dedup key: name, created timestamp, user, workspace; only first copy kept ([event-limits]). Send second-precision `created_at` ([api-events]) `[snippet]`.
- Active limit: 120 per workspace; above it not stored, returns an error, shown only in browser console for JS ([event-limits]). Another article states 500 unique per user `[conflict]`, likely a different limit ([event-filters]).
- Metadata: up to 20 per event in help and current API ([event-limits], [api-events]); older docs say 10 ([api-event-old]) `[snippet]`. Keys cannot change; further keys ignored ([api-events]) `[snippet]`.
- Timing: new metadata can take up to 24 hours to show in message filters ([event-limits]).
- Numeric metadata as string silently never matches `greater than` ([event-limits], [event-filters]).
- Nested metadata shows in list but cannot be used in messages ([event-limits]).
- Each event can trigger at most 50 live messages ([event-limits]).
- Export: none ([event-limits]). API rate limit: 10,000 per minute, split into 10-second windows ([event-limits], [rate-limit]).
- One event rule per message ([message-rules]). Don't use events for page views, use URL targeting ([event-filters]).
- Filter vs trigger: filters allow count and recency and several events but send once; triggers repeat, can read metadata, cannot do count or several events ([event-filters]).

## 7. Evaluation behaviour

### When rules are checked

| Surface | Moment | Source |
|---|---|---|
| Chat, post | When a person logs in or visits, matches, and it is during delivery times. A "last seen more than X days ago" rule matches on return | [dynfix] |
| Banner | When matching identified people are seen online | [chats-faq] |
| Email (dynamic) | Rules checked hourly; sent in delivery windows | [dynfix] |
| Series entry | About hourly, or near immediate when active with Messenger. Event entries only at the moment of the event | [series-faq], [series-explained] |
| Series mid rule block | About every 30 minutes; try-for-X or try-once | [series-faq], [series-explained] |
| Series mid-flight | No re-check; add a rule block or exit rule to recheck. Editing entry rules does not remove people | [series-faq], [series-explained] |
| Workflow trigger audience | At trigger; page URL rules evaluate on the device, which is why Fin's identity cannot show when URL targeted | [trigger], [url-rules] |
| Workflow updates | Rules at conversation start stand; no retroactive application | [troubleshoot] |
| Workflow branches with URL | At conversation creation; if the person moved, Else runs | [troubleshoot] |
| Fin audience | Once at conversation start; later changes to rules, attributes or content do not apply; Fin Procedures `@Switch` skips the check | [fin] |
| Help Center | On request, through cookie forwarding | [hc] |

### Client versus server

- Docs never state the split. The only direct hint: Current Page URL rules "evaluate on the visitor's device" ([url-rules]). Everything else (hourly checks, `update` checks for messages) implies server side ([dynfix], [spa]).

### Re-evaluation and staleness

- SPA: URL changes do not reach Intercom by themselves. Call `update`; it acts like a page refresh and checks for messages ([spa], [faqs-msg]).
- `update` throttle: 20 per user per page, reset every 30 minutes or by reload ([spa], [js-methods]).
- `update` with no data change registers once per 60 seconds ([dedupe]) `[forum]`, staff reply. Throttle is per user ([throttle-staff]) `[forum]`.
- Current Page URL holds the URL at Messenger session start, refreshes only on full page load ([troubleshoot]).
- API-created conversations may not have the URL in time ([troubleshoot]).
- Event-based rules need a page refresh or an explicit `update` after `trackEvent`. Staff calls this expected ([event-delay]) `[forum]`.
- "Last heard from" updates before matching, so "last heard from is unknown" never matches the sender ([troubleshoot]).
- Company attributes use the company linked at routing time; if none, the most recently updated one, which can misroute ([troubleshoot]).
- Tag can take up to 5 minutes to appear in the people list ([tags]). Search API recent contacts lag a few minutes ([search-api]).
- Custom attribute lag of 6+ hours reported, resolved in private ([attr-delay]) `[forum]`.
- Launcher: rules hide only the launcher ([launcher]); but a troubleshooting article says Messenger-hidden customers never get Messenger-based workflow triggers ([troubleshoot]) `[conflict]`.

### Identity

- `user_hash` Identity Verification is deprecated; toggle no longer changeable; hashes still accepted ([iv-deprecated]).
- JWT: server signs HS256 with `user_id` required; pass `intercom_user_jwt` at boot; company goes inside the token; boot-level company is ignored with JWT ([jwt]).
- Enforcement toggle requires a JWT or `user_hash` on every request for users. Without enforcement, boots without a JWT are accepted; an invalid JWT is always rejected ([jwt]).
- Protected attributes: per-attribute "Attribute Updates" setting disables Messenger updates; only JWT can change them. REST, CSV unaffected. Legacy `user_hash` requests are not processed for protected attributes ([jwt], [cda]).
- This does not stop bots collecting data from leads ([jwt]).
- Docs advise protecting email, phone, account ids and anything Fin uses in actions or workflows ([jwt]). Default is unprotected, by implication of the advice ([cda]) (inference).
- Visitors load without JWT; Identity Verification does not cover unidentified users ([jwt], [iv-android]).
- Session cookies: 7 days default, 1-hour grace; on expiry without JWT the person appears as a logged-out visitor ([jwt]).

### Previews and test

- Workflow trigger: audience preview = estimated number of existing contacts matching plus sample names ([trigger]).
- Preview reads current values and may include people who will not match at write-in ([troubleshoot]).
- Banner preview does not enforce audience or URL targeting ([chats-faq]). A message's "When to send" rule does not narrow preview: user saw everyone ([email-triggers]) `[forum]`.
- Fin audience preview disabled when conversation attributes are used ([fin]).
- 50,000 companies matched: preview fails with an error, but the send still goes ([segments]).
- News schedule confirmation shows the full count, not after-targeting ([faqs-msg]).
- API segment count: `include_count`; company segments report the count of a contact segment with the same name ([segments-api], [segment-object]).
- Test as: Help Center mentions "Test as specific user", simulates Messenger, no reports ([hc]). Fin previews: "Testing as" user or lead, or audience ([fin-preview]) `[snippet]`. Escalation filtering is not applied in that mode ([fin-preview]) `[snippet]`; batch testing likewise ([fin-mirror]) `[snippet]`.
- Workflow preview uses a "Preview User"; recommended against for matching tests ([wf-preview]). Common recipe: audience rule on own email, set live ([wf-preview]). Troubleshoot tool stops at the first mismatch ([troubleshoot]).

### Known complaints

Each one is a design input:

1. Stale until refresh: event rules do not fire first load ([event-delay]) `[forum]`.
2. Stale page URL on SPA and the 20-per-30-min throttle ([troubleshoot], [spa], [throttle-thread]) `[forum]`.
3. Negative OR: repeated warnings; still possible to build ([conditional], [rules]).
4. No collection audience, request is years old with per-article bulk workaround ([hc], [collection-thread]) `[forum]`.
5. Help Center needs custom domain and cookie forwarding ([hc]).
6. Suggested articles ignoring audience across two domains: support confirmed a bug ([suggested-bug]) `[forum]`.
7. Attribute lag ([attr-delay]) `[forum]`.
8. Previews vs real matching ([troubleshoot], [email-triggers]).
9. Silent failures: dropped nested attributes, string-number metadata, brand id mismatch, Jira clash ([cda], [event-limits], [fin]).
10. Attributes cannot be deleted or renamed ([cda]).
11. Fin evaluates once: a mid-conversation upgrade does not change content ([fin]).
12. Duplicates when a person has several companies in a Series, unless "Company priority" is on ([series-faq]).
13. Mismatch between newer News targeting and what admins used to have ([news-thread]) `[forum]`.

## 8. UI

Builder steps, as documented:

1. Open Rules > Audience (outbound), trigger settings (workflow), or Details > Help Center audience (article) ([rules], [trigger], [hc]).
2. Choose person types: visitors, leads, users. On workflows: Everyone, Custom (roles, default Users), or Predefined audience ([url-rules], [trigger], [hc]).
3. "+ Add audience rule", pick attribute or predicate, then value ([rules], [trigger]).
4. Add more filters; set AND/OR on groups and connector ([rules], [filters]).
5. Choose dynamic or fixed, for outbound ([dynfix]).
6. Review count and sample ([trigger], [fin]).
7. Set live; test with own-email audience ([wf-preview]).

What the UI prevents, documented:

- Workflow with URL rule cannot go live without a valid URL ([rules]).
- Mobile-only workflow must target users ([trigger]).
- Add rule button disables at the cap ([rules]).
- Teammates get a "[Restricted]" warning before inserting a restricted article, and cannot send it to someone outside the audience in the Inbox ([hc]).
- Required attribute fallback before going live ([wf-preview]).
- Duplicate fixed-audience warning if CSV tagging unfinished ([csv]).
- Workflow list filter by audience; Fin audience "used by" counts per item ([fin]).
- Audience filter on messages list ([dynfix]).

What it does not prevent, as far as docs show: negative OR, stale last-page rules, custom email attribute exclusion ([rules], [troubleshoot]).

## 9. REST API

| Object | Endpoints | Notes | Source |
|---|---|---|---|
| Segments | `GET /segments`, `GET /segments/{id}`, `GET /contacts/{id}/segments` | Read only. Fields: id, name, created_at, updated_at, person_type, count. No rule payload. Names HTML-escaped. `type=company` lists company segments | [segments-api], [segment-object] |
| Contact search | `POST /contacts/search` | Section 3. 150 per page. Can filter by `segment_id`, `tag_id`. Merged contacts excluded | [search-api] |
| Data attributes | list, create, update (archive, options) | Update cannot change data type; `PUT`/`PATCH`; `write_attributes` scope | [api-update], [api-list] |
| Events | `POST /events`, summaries by user | | [api-events] |
| Tags | `POST /tags` | Tag and untag users, companies, contacts in one call; under 50 per call advised in old docs | [api-tags] `[snippet]` |
| Fin data connector audience | Create and update accept audience ids; mode everyone, custom, fin_audiences; custom rules are UI only | From an OpenAPI PR, may not be final | [connector-pr] |
| Audience, Fin audience, workflow rule CRUD | Not found | | section 12 |

No public API for creating segments or audience rules appears in the current reference ([segments-api]). Workflows have no public API per our earlier research (`research/competitors/workflows-productlane-intercom-plain.md`).

## 10. Permissions, plans, limits

- Messenger settings: "Can access Messenger settings" ([setup]).
- Segments: everyone sees; each teammate chooses their own menu; only full-permission admins edit or delete ([segments]).
- Bulk messaging: "Can bulk message visitors, leads and users" ([rules]).
- Reusable audiences: "Can access workspace data" and "Can create and manage content in Knowledge" ([rules]).
- CSV audience: "Can manage tags" ([csv]).
- Plans: Launcher visibility needs an active subscription or trial ([launcher]). "At custom times" scheduling needs certain plans ([trigger]). Posts, surveys, tours, mobile push and carousels are in the Proactive Support Plus add-on available on any plan ([pricing-faq]) `[snippet]`. No plan-level gating of audiences themselves is documented.
- Limits collected: 10 filters per OR group, 10 OR groups per trigger ([rules]); 15 per group, 2 levels, 15 IN values in search ([search-api]); 250 attributes ([cda]); 35 list options ([cda]); 255 chars ([cda]); 120 events ([event-limits]); 20 metadata ([event-limits]); 50 triggers per event ([event-limits]); 19 Home apps ([home-apps]); 10 external links ([faqs-msg]); 50,000 companies preview ([segments]); 5 CSV columns ([csv]).

## 11. Others, briefly

**Zendesk Guide.** Per-article view permissions: everyone, signed-in, agents and admins, or custom user segment built from tags, organizations, groups ([zd-seg]). Same restriction per article not per section; bulk update to lock a section ([zd-limit]). "Require sign-in" locks the whole center ([zd-perm]). AI answers use restricted articles only for users matching ([zd-ai]). Segment-per-article count differs across pages ([zd-seg], [zd-limit]) `[conflict]`. Web Widget by segment: only via a forum workaround, placing the snippet in a `signed_in` block; a developer advocate said no hide API on the messaging widget at the time ([zd-widget]) `[forum]`.

**Crisp.** Segments are labels; custom data is key-values; both set from the JS SDK ([crisp-push]). Saved contact filters can combine profile fields, data and segments, reusable for campaigns, on paid plans ([crisp-filters], [crisp-crm]). Automatic triage rules can add a segment or write data ([crisp-triage]). We did not find a claim that Crisp protects browser-set data. Not confirmed.

Takeaway: Zendesk's model of "one permission for human and AI" is cleaner than Intercom's two settings. Crisp's two-word split (label vs key-value) is simple but offers less.

## 12. Could not confirm from public docs

1. Which rules run in the browser and which on the server, apart from page URL ([url-rules]). Whether rules or attribute names are ever sent to the browser.
2. Depth of nesting in the builder ([rules]). We saw one level of groups with a connector.
3. The complete operator list today; the filter article is old ([filters]).
4. Whether Home app rules can use any attribute; the article contradicts itself ([home-apps]).
5. Audience per space beyond visitors and users, and for the Help space ([setup], [faqs-msg]).
6. Maximum number of segments, Fin audiences, tags ([segments], [fin], [tags]).
7. Whether a data type can be converted and how; only an unofficial source says silent conversion ([api-update], [revos]).
8. Real propagation latency of attributes to rules. Only anecdotes ([attr-delay], [event-delay]).
9. Whether a hidden launcher stops Messenger-based triggers or only the launcher ([launcher], [troubleshoot]).
10. Whether enforcement of "who can start a conversation" is server-side or just hides the button ([startconv], [setup]).
11. Whether conversation tags work on Fin audiences; two docs differ ([fin-mirror], [trigger]).
12. Plan gating of audience features ([launcher], [pricing-faq]).
13. Performance limits other than the 50,000 companies preview and the vague "complex query" refusal ([segments]).
14. Whether Intercom has private API for audience CRUD. None public ([segments-api]).
15. A few help pages returned 404 to our fetcher (one fin.ai mirror, one changelog entry) so those claims rest on search result text ([fin-mirror], [changelog-create]).
16. Whether Intercom's protected attribute default is off: inferred from the advice to enable it ([cda], [jwt]).

## Sources

[rules]: https://www.intercom.com/help/en/articles/14654636-audience-rules-in-intercom
[launcher]: https://intercom.com/help/en/articles/1155368-control-who-can-see-the-messenger-launcher-on-your-site
[setup]: https://intercom.com/help/en/articles/6612589-set-up-and-customize-the-messenger
[startconv]: https://www.intercom.com/changes/en/control-who-can-start-new-conversations-with-your-team--
[faqs-msg]: https://www.intercom.com/help/en/articles/6612597-messenger-faqs
[home-apps]: https://www.intercom.com/help/en/articles/1827291-customize-your-messenger-home-with-apps
[tickets]: https://www.intercom.com/help/en/articles/8313203-tickets-space-in-the-messenger
[checklist]: https://www.intercom.com/help/en/articles/6661022-design-your-checklist
[news]: https://www.intercom.com/help/en/articles/6362267-getting-started-with-news
[news-thread]: https://community.intercom.com/messenger-8/rules-to-trigger-the-news-tab-in-messenger-10088
[message-rules]: https://www.intercom.com/help/en/articles/5296455-message-rules-when-how-and-to-whom-should-a-message-be-sent
[dynfix]: https://www.intercom.com/help/en/articles/3467924-dynamic-and-fixed-audiences
[owner]: https://www.intercom.com/help/en/articles/2567823-send-messages-to-users-from-account-owners
[chats-faq]: https://www.intercom.com/help/en/articles/8771588-chats-posts-and-banners-faqs
[series-faq]: https://www.intercom.com/help/en/articles/8780861-series-faqs
[series-explained]: https://www.intercom.com/help/en/articles/4425207-series-explained
[fin]: https://www.intercom.com/help/en/articles/9357948-manage-knowledge-audiences-and-targeting
[fin-mirror]: https://intercom.help/fin4all/en/articles/10903029-manage-audiences-and-targeting-for-fin
[fin-preview]: https://www.intercom.com/help/en/articles/12599471-use-fin-previews
[data-hub]: https://www.intercom.com/help/en/articles/16340024-using-the-data-hub-to-personalize-fin-s-responses
[hc]: https://www.intercom.com/help/en/articles/2982784-control-who-can-see-your-public-articles
[collection-thread]: https://community.intercom.com/knowledge-base-6/is-it-possible-to-only-show-help-center-collections-to-certain-users-32
[suggested-bug]: https://community.intercom.com/messenger-8/suggested-articles-in-messenger-with-audience-targeting-8741
[trigger]: https://www.intercom.com/help/en/articles/7851710-configure-workflow-trigger-settings
[conditional]: https://www.intercom.com/help/en/articles/11868426-conditional-logic-and-trigger-configuration-in-workflows
[url-rules]: https://www.intercom.com/help/en/articles/12819568-configure-page-url-targeting-rules-for-workflows
[troubleshoot]: https://www.intercom.com/help/en/articles/8263341-troubleshooting-when-a-workflow-doesn-t-trigger
[wf-preview]: https://www.intercom.com/help/en/articles/7836467-preview-and-set-workflows-live
[segments]: https://www.intercom.com/help/en/articles/324-how-to-segment-your-contacts
[filters]: https://www.intercom.com/help/en/articles/2410715-how-do-filters-work
[contacts-faq]: https://www.intercom.com/help/en/articles/8827723-contacts-faqs
[csv]: https://www.intercom.com/help/en/articles/6803933-filter-audience-from-a-csv
[tags]: https://www.intercom.com/help/en/articles/4393-group-your-users-by-tags
[andor]: https://www.intercom.com/changes/en/combine-and--or-rules-for-easier-message-targeting--11
[or-thread]: https://community.intercom.com/workflows-7/is-there-a-way-to-set-or-audience-rules-1880
[cda]: https://www.intercom.com/help/en/articles/179-create-and-track-custom-data-attributes-cdas
[track]: https://www.intercom.com/help/en/articles/320-tracking-user-data-in-intercom
[data-tracked]: https://www.intercom.com/help/en/articles/1722980-how-intercom-tracks-and-stores-data
[cvda]: https://www.intercom.com/help/en/articles/6546210-create-and-use-conversation-data-attributes-cvdas-in-the-inbox
[list-limit]: https://community.intercom.com/helpdesk-9/is-there-a-limit-to-the-number-of-list-items-you-can-add-to-an-attribute-list-671
[changelog-create]: https://intercom.com/changes/en/68371-changes-to-how-custom-data-attributes-are-created-with-intercom
[revos]: https://help.revos.ai/integrations/hHrHN7xqBo2gahRKcwcxPf/action---update-contact/65FnQdpdzDNjYbyg3z1X9R
[event-limits]: https://www.intercom.com/help/en/articles/5245634-event-limits-troubleshooting-and-f-a-q
[event-filters]: https://www.intercom.com/help/en/articles/5245581-using-events-as-filters-and-triggers
[spa]: https://www.intercom.com/help/en/articles/170-integrate-intercom-in-a-single-page-app
[js-methods]: https://developers.intercom.com/installing-intercom/web/methods/
[jwt]: https://www.intercom.com/help/en/articles/10589769-authenticating-users-in-the-messenger-with-json-web-tokens-jwts
[iv-deprecated]: https://www.intercom.com/help/en/articles/7946881-troubleshooting-identity-verification-deprecated
[iv-android]: https://developers.intercom.com/installing-intercom/ios/identity-verification
[contacts-guide]: https://developers.intercom.com/docs/guides/contacts
[dedupe]: https://community.intercom.com/messenger-8/are-there-hidden-update-throttling-limits-265
[throttle-thread]: https://community.intercom.com/messenger-8/are-there-hidden-update-throttling-limits-265
[throttle-staff]: https://community.intercom.com/messenger-8/how-to-infer-intercom-update-without-getting-throttled-up-to-20-times-per-30-minutes-if-its-per-user-per-same-user-or-per-workspace-basis-2022
[event-delay]: https://community.intercom.com/api-webhooks-23/event-propagation-delay-in-intercom-workflow-trigger-audience-rule-8111
[attr-delay]: https://community.intercom.com/proactive-support-11/custom-attribute-delay-7581
[email-triggers]: https://community.intercom.com/outbound-11/email-triggers-are-confusing-af-signed-up-10548
[segments-api]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/segments/listsegments
[segment-object]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/segments/segment
[search-api]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/contacts/searchcontacts
[api-attr-model]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/data-attributes/data_attribute
[api-update]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/data-attributes/updatedataattribute
[api-list]: https://developers.intercom.com/docs/references/2.11/rest-api/api.intercom.io/data-attributes
[api-events]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/data-events/createdataevent
[api-event-old]: https://developers.intercom.com/docs/references/1.4/rest-api/data-events/submitting-events
[api-tags]: https://developers.intercom.com/docs/references/1.3/rest-api/tags/tag-or-untag-users-companies-leads-contacts
[company-limits]: https://developers.intercom.com/docs/references/rest-api/api.intercom.io/models/contact_companies
[rate-limit]: https://developers.intercom.com/docs/references/rest-api/errors/rate-limiting
[connector-pr]: https://github.com/intercom/Intercom-OpenAPI/pull/474
[pricing-faq]: https://www.intercom.com/help/en/articles/8344190-intercom-pricing-faqs
[zd-seg]: https://support.zendesk.com/hc/en-us/articles/4408824005914-Setting-view-permissions-on-articles-with-user-segments
[zd-limit]: https://support.zendesk.com/hc/en-us/articles/4408893875610-How-can-I-limit-access-to-articles
[zd-perm]: https://support.zendesk.com/hc/en-us/articles/4408827797274-Understanding-Knowledge-user-permissions-for-knowledge-base-access
[zd-ai]: https://support.zendesk.com/hc/en-us/articles/8087943201306-Using-restricted-help-center-content-in-AI-agent-responses
[zd-widget]: https://support.zendesk.com/hc/en-us/community/posts/4428045260058-Show-web-widget-answer-bot-only-to-signed-in-users-
[crisp-push]: https://help.crisp.chat/en/article/how-to-automatically-push-segments-oerdox/
[crisp-filters]: https://help.crisp.chat/en/article/how-to-create-advanced-filter-vzw4sc/
[crisp-crm]: https://help.crisp.chat/en/category/contacts-crm-4e84h7/
[crisp-triage]: https://help.crisp.chat/en/article/how-to-qualify-categorize-route-or-mark-conversation-as-spam-with-the-automatic-triage-feature-1sihmjf

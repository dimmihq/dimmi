# 16 Importers: sources and field mapping

Style: caveman. Facts verified 2026-11-02 (links at bottom). Driver contract `contracts.ImportSource` (README code map). Every driver: thin JSON client on `framework/httpclient` (SSRF guard, timeouts, size caps), rate budget via `app/support/importkit/ratebudget.go`, cursor opaque in `import_run_stages.cursor`, recorded fixtures for `tests/contracts/import_source_test.go`. Field names below are mapping intent; driver pins exact shapes against recorded fixtures at build (fixtures refreshed by `dimmi imports:record-fixtures <source>` dev command against a sandbox account, never in CI).

## 1. Contract

```go
package contracts

type ImportSource interface {
    Key() string                                   // registry value: intercom ...
    Info() ImportSourceInfo                        // name, kind, objects, credential fields, page sizes, rate doc
    Connect(ctx context.Context, c ImportCredentials) (ImportAccount, error)          // validate, account id/name/region, scopes allowed
    Discover(ctx context.Context, s ImportSession) (ImportDiscovery, error)            // teammates, teams, inboxes, tags, attributes, statuses, boards, locales, counts
    Page(ctx context.Context, s ImportSession, stage string, cur ImportCursor, w ImportWindow) (ImportPage, error) // normalized records + next cursor + done
    Detail(ctx context.Context, s ImportSession, ref ImportRef) (ImportRecord, error)   // e.g. conversation with parts
    Open(ctx context.Context, s ImportSession, f ImportFileRef) (io.ReadCloser, ImportFileInfo, error) // attachment / image stream
    Budget(a ImportAccount) RateBudget             // documented limit, window, header names
}
```
Errors (`errors.Is`): `ErrImportUnauthorized` (pause `credentials_invalid`), `*ImportRateLimitedError{RetryAfter}`, `ErrImportNotFound` (record error `source_not_found`), `ErrImportTemporary` (retry), `ErrImportPermanent` (record error). Normalized types carry `ExternalID`, `UpdatedAt`, `CreatedAt` (UTC), never raw provider structs.

Rate budget: effective = documented limit x `rate_share`; Valkey token bucket key `dimmi:rl:import:<source>:<account id>` shared across workers and imports of same account (two workspaces importing same Intercom app share budget). Response headers lower budget live (remaining under 10%: slow to 1 req/s until reset). 429: wait `Retry-After` (or reset header, else 30 s), not counted as retry; waits over 2 min set run `paused` `rate_limited` with `resume_at`.

## 2. Intercom (`intercom`)

| Fact | Value |
|---|---|
| Auth | Bearer access token (Developer Hub private app). Token needs read scopes: users/companies, conversations, tags, admins, articles/help center |
| Hosts | `api.intercom.io` (us), `api.eu.intercom.io`, `api.au.intercom.io` (region option; Connect tries chosen region only) |
| Version | header `Intercom-Version` pinned (`imports.intercom.api_version`, 2.17 current at check) |
| Limits | 10,000 calls/min per private app (shared by workspace's private apps), 25,000/min per workspace, spread over 10 s windows; headers `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`; 429 `rate_limit_exceeded` (some without headers) |
| Pagination | cursor `pages.next.starting_after`; search `per_page` max 150 inside `pagination` object |
| Conversation parts | single conversation retrieve returns at most 500 parts (most recent 500); no pagination |

Stages:
- discover: `GET /admins` (teammates: id, name, email, away), `GET /teams`, `GET /tags`, `GET /data_attributes?model=contact|company` (custom only; types map to 18 `attribute_definitions`), Intercom segments list (names only, draft audience stubs), `GET /help_center/collections`, `GET /help_center/help_centers` (multi help center), counts via search `total_count`.
- companies: `GET /companies/scroll` (scroll param) or list; `company_id` -> `external_id`, name, website -> domain (host only), custom_attributes, `created_at`.
- contacts: `POST /contacts/search` sort `created_at asc` paginated (or `GET /contacts` list cursor). role `user` -> `kind user`, `external_id` from `external_id`; role `lead` -> `kind lead`. email, phone, name, avatar.image_url (URL kept, not downloaded), location (country, region, city), `signed_up_at`/`created_at` -> `first_seen_at`, `last_seen_at`, custom_attributes, companies (first -> `company_id`), tags (`import_tags`, into `contact_tag`), `unsubscribed_from_emails` -> ignored (no marketing in Dimmi; noted in custom attribute `intercom_unsubscribed` only when true).
- conversations: `POST /conversations/search` with `updated_at > window_since` (delta) or all, sort `created_at asc`, per_page 150 -> ids; `ImportConversationBatch` fetches `GET /conversations/{id}?display_as=plaintext` not used (HTML kept for conversion).
  - conversation: `source` (first message: type `conversation`, `email`, `push`, `facebook`, `twitter`...; author; subject for email; body), `state` open/closed/snoozed, `snoozed_until`, `admin_assignee_id`, `team_assignee_id`, `tags`, `priority` (`priority` -> level `high`, `not_priority` -> `none`), `contacts` (first -> contact, rest -> participants), `created_at`, `updated_at`, `statistics.last_close_at`, `ticket` (ticket type + attributes -> `metadata.import.ticket`), `custom_attributes` (conversation) -> `metadata.import.attributes`.
  - parts: `part_type` `comment` -> message (author type `user`/`lead`/`contact` -> contact, `admin` -> teammate mapping, `bot` -> author_type `agent` id null kind `bot`), `note` -> note, `assignment`/`open`/`close`/`snoozed`/`unsnoozed` -> type `event` rows (`conversation.assigned`, `.closed`, `.reopened`, `.snoozed`) with `metadata.imported`; `away_mode_assignment`, `participant_added`, `conversation_rating_changed` -> event rows or skipped (counted `unsupported_part_type`); empty body parts skipped.
  - attachments per part (`attachments[].url, name, content_type, filesize`) -> `DownloadImportAttachment`; inline images in body HTML -> attachments `inline = true`, body Markdown references `att_` signed link via 03 rendering.
  - total parts over 500: import available 500, event row "Older history not available from Intercom API (N parts)", warning error `parts_truncated`.
- articles: `GET /articles` (list), per article `translated_content` locales; `state` published/draft; `parent_id` -> collection; `url` -> redirect from path; `author_id` -> teammate mapping; body HTML -> Markdown; `created_at`, `updated_at`.
- collections: `GET /help_center/collections` (`parent_id` nesting, `url`, `name`, `description`, `icon` ignored).
- Redirect paths: `/{locale}/articles/{id}-{slug}` + `/{locale}/articles/{id}`; collections `/{locale}/collections/{id}-{slug}` + `/{locale}/collections/{id}`.

### Intercom shim mapping (implemented by 01)

| Intercom call | Dimmi |
|---|---|
| `Intercom('boot', {app_id, user_id, email, name, created_at, company, ...custom})` | `Dimmi('boot', {appId, userId: user_id, visitor: {name, email}})` (sdk-contract boot options); `user_id` -> unverified `claimed_user_id` (02 R5), `visitor` = unverified hints, unless Dimmi identity source configured (verified wins). `company`, `created_at`, custom attributes: ignored (browser never writes contact attributes, 02 R8); one `console.info` pointing to `identify()` attributes |
| `Intercom('update', attrs)` | `dimmi.update({})` (ping); `user_id` changed -> `dimmi.identify()`; other attrs ignored (same rule as boot) |
| `Intercom('shutdown')` | `dimmi.logout()` (Intercom shutdown clears visitor session on sign-out) |
| `Intercom('show' \| 'hide' \| 'showMessages')` | `dimmi.open()` / `close()` / `show('messages')` |
| `Intercom('showNewMessage', text)` | `dimmi.startConversation(text)` (composer with draft, never sends) |
| `Intercom('showArticle', id)` | resolve Intercom article id -> `art_` via import records, `dimmi.show('help', {article})`; unknown: open Help tab |
| `Intercom('onShow' \| 'onHide' \| 'onUnreadCountChange', fn)` | `dimmi.on('open' \| 'close' \| 'unread_count_changed', fn)` (count passed as number) |
| `Intercom('trackEvent', ...)`, `startTour`, `startSurvey`, `startChecklist` | no-op + one console info line |
| `Intercom('getVisitorId')` | `dimmi.visitorId` |
| `user_hash` | ignored (open question 1); dashboard warning when seen |

## 3. Zendesk (`zendesk`)

| Fact | Value |
|---|---|
| Auth | Basic `{email}/token:{api_token}`; host `https://{subdomain}.zendesk.com` (subdomain validated) |
| Limits | Support + Help Center API per minute by plan: Team 200, Professional 400, Enterprise 700, High Volume add-on 2,500 (Essential legacy 10); incremental export 10 req/min (30 with High Volume); headers `X-Rate-Limit`, `X-Rate-Limit-Remaining`, `ratelimit-reset`, `Retry-After` on 429 |
| Tickets | `GET /api/v2/incremental/tickets/cursor?start_time=` (first call), then `cursor`; up to 1,000 per page; `end_of_stream`; `start_time` at least 1 min in past |
| Comments | `GET /api/v2/incremental/ticket_events?start_time=&include=comment_events` (comments in `child_events`, 1,000 events per page) primary path; gap fill per ticket `GET /api/v2/tickets/{id}/comments` (cursor `page[size]=100`) |

Plan detection at Connect: read `X-Rate-Limit` header -> budget. ETA in preview uses it.

Mapping:
- agents (`/api/v2/users?role[]=agent&role[]=admin`) -> teammate mappings; groups -> team mappings; brands -> inbox mappings (default: all brands -> default inbox).
- organizations -> companies (`external_id` = `external_id` or `zd:<id>`, name, `domain_names[0]`, `organization_fields` -> custom attributes).
- end users -> contacts (`external_id` -> external_id, email, name, phone, `user_fields` -> custom attributes, `organization_id` -> company, `created_at` -> `first_seen_at`, `last_login_at` -> `last_seen_at`, `verified` ignored by default (email_trust option), suspended users imported normally (not blocked) with attribute `zendesk_suspended: true`).
- tickets -> conversations: subject, requester -> contact, `collaborator_ids`/CCs -> participants (`cc`), status new/open/pending/hold -> open (preserve) , solved/closed -> closed (`solved_at` from metrics or last update), `tags` -> tags (Zendesk lowercases), `priority` urgent -> `urgent`, high -> `high`, normal -> `medium`, low -> `low`, empty -> `none`, `assignee_id` -> teammate mapping, `group_id` -> team mapping, `brand_id` -> inbox mapping, `via.channel` -> `source.imported.channel` (email, web, api, chat, messaging), custom fields -> `metadata.import.fields`, `deleted` tickets skipped, `spam` (status deleted via spam) skipped unless `include_spam`.
- comments: `public: true` -> message (author end user -> contact, agent -> teammate), `public: false` -> note; `html_body` -> Markdown + sanitized `body_html` for email-origin; attachments `content_url` (may require auth when private attachments on: send basic auth only to `*.zendesk.com` hosts, never on redirect to other host) -> files; voice comments -> text "Call recording not imported (duration)".
- macros (`/api/v2/macros`, active) -> saved replies (title, comment value -> body; actions other than comment ignored; placeholders `{{ticket.requester.first_name}}` -> `{{contact.first_name}}`, unknown placeholders kept as text).
- Guide: `GET /api/v2/help_center/{locale}/categories`, `/sections`, `/articles` (cursor) -> collections (category level 1, section level 2; nested sections flattened), articles (`draft` -> draft, `html_url` -> redirect, `body` HTML, `author_id`, `created_at`, `edited_at`), attachments/images in body rehosted. Redirect paths `/hc/{locale}/articles/{id}-{slug}` + `/hc/{locale}/articles/{id}`, sections `/hc/{locale}/sections/{id}...`, categories `/hc/{locale}/categories/{id}...`.

## 4. Help Scout (`helpscout`)

| Fact | Value |
|---|---|
| Mailbox API 2.0 auth | OAuth2 client credentials: POST `https://api.helpscout.net/v2/oauth2/token` (`grant_type=client_credentials`, app id + secret); token valid 172,800 s (2 days); 401 on expiry -> refresh once. Credentials must belong to active user |
| Mailbox limits | plan-based per minute (shared by account users); writes count 2; headers `X-RateLimit-Limit-Minute`, `X-RateLimit-Remaining-Minute`, `X-RateLimit-Retry-After`; 429 |
| Docs API | `https://docsapi.helpscout.net/v1/`, Basic auth (API key as username, `X` password); limit per 10 min: 2,000 (1 site), 3,000 (2), 4,000 (3+); headers `X-RateLimit-Limit/Remaining/Reset`; collections, categories, articles list/get; 50 per page (collections) |

Mapping:
- users -> teammate mappings; teams -> team mappings; mailboxes -> inbox mappings (suggest `create` with same name).
- customers (`/v2/customers`, embedded emails, phones, organization, properties) -> contacts (primary email first; extra emails -> unverified identities), organization text -> company by name (Help Scout organizations API where available: name, domains).
- conversations (`/v2/conversations?mailbox=&status=all&sortField=createdAt&sortOrder=asc`, delta `modifiedSince`) -> ids; detail `GET /v2/conversations/{id}?embed=threads`.
  - status active/pending -> open (preserve), closed -> closed, spam -> skip unless `include_spam`; `assignee` -> teammate; tags; `cc`/`bcc` -> participants; `subject`; `source.type` (email, chat, phone, beacon) -> channel.
  - threads: `customer` -> contact message, `reply` -> teammate message, `note` -> note, `lineitem` -> event row (assigned, status changed, moved) or skipped, `chat` -> message per chat line, `phone` -> note "Phone call" + body, `forwardparent`/`forwardchild` -> note. Thread `state` hidden/draft skipped. Attachments: `GET /v2/conversations/{id}/attachments/{aid}/data` (base64) streamed decode to Filesystem.
- saved replies (`/v2/mailboxes/{id}/saved-replies`) -> saved replies (shared), Help Scout variables `{%customer.firstName,fallback=there%}` -> `{{contact.first_name}}`.
- Docs (optional Docs API key): sites -> help center mapping; collections -> collection level 1; categories -> level 2 (article in several categories: first category; redirect from other category paths not needed, article URL is category-independent); articles (`status` published/notpublished, `publicUrl` -> redirect path `/article/{number}-{slug}` + `/article/{number}`; collections `/collection/{number}-{slug}`, categories `/category/{number}-{slug}`).

## 5. Featurebase (`featurebase`)

| Fact | Value |
|---|---|
| Auth | `Authorization: Bearer sk_...` (Settings, API) |
| Base | `https://do.featurebase.app/v2` |
| Version | header `Featurebase-Version` pinned (`2026-08-19.orbit` at check; Nova `2026-01-01.nova` added contacts, conversations, companies, post statuses; changelogs at `/v2/changelogs`) |
| Pagination | cursor: `limit` 1-100, `cursor` = previous `nextCursor`; list `{object: "list", data, nextCursor}` |
| Limits | not published in fetched reference; driver default budget 300/min, adapts to 429 `Retry-After` |

Mapping:
- boards -> board mappings (map/create); post statuses -> status mappings (by category: open/planned/in progress/complete/closed -> 14 status categories); categories/tags -> categories/tags.
- posts -> feedback posts (title, content HTML -> Markdown, author contact or member, status, board, `createdAt`, upvotes total -> `imported_vote_count` minus identified voters); voters (`/posts/{id}/voters`) -> votes (contact upsert by email/external id, trust `imported`); comments (threaded) -> 14 comments (private -> internal), replies flattened to parent with quote prefix when 14 lacks depth; merged posts -> skipped with count `merged_at_source` (votes already moved at source).
- changelogs -> 13 entries (title, content, `publishedAt` backdated, labels/categories -> label mappings, featured image -> asset, draft -> draft).
- help center collections/articles -> 08 (same rules as Intercom; redirect from article `url` path).
- contacts, companies -> 03 (external id `userId` when present).
- conversations (Featurebase inbox) -> conversations with messages/notes when scope selected (shape pinned at build against fixtures; conversation without accessible messages: imported with first message only and warning `unsupported_part_type`).

## 6. Productlane (`productlane`)

| Fact | Value |
|---|---|
| Auth | `Authorization: Bearer <key>` |
| Base | `https://productlane.com/api/v2/` |
| Pagination | `limit` default 50, max 200; `cursor` from `page.cursor`; loop on `page.has_more`; sorted `created_at DESC, id DESC`, no sort param; filters only on first request |
| Limits | per key: GET 1,000/min, writes 60/min, burst 2x over 10 s; headers `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After` on 429 |
| Sections | threads, contacts, companies, changelogs, docs, projects, issues, tags, members, portal |

Mapping:
- members -> teammate mappings; tags -> tags.
- contacts, companies -> 03.
- threads -> conversations (state -> status; messages when exposed, else thread text as first contact message; pain level -> `priority` level `high` when high; linked issue/project -> `metadata.import.links` and, when feedback scope selected and issue imported, 14 `feedback_request` link via helper).
- projects/issues on portal with upvotes -> feedback posts (board mapping "Roadmap"), upvotes -> votes / `imported_vote_count`.
- changelogs -> 13 entries; docs -> 08 articles + redirects from doc URL paths.
- Newest-first ordering: driver reads full list (cursor) and import order irrelevant (numbers allocated as encountered; conversations sorted by `created_at` inside page only). Delta: stop paging when item `updated_at < window_since` (list sorted by created, so delta for threads re-scans with checksum skip; bounded by thread count, acceptable at Productlane sizes).

## 7. Canny (`canny`)

| Fact | Value |
|---|---|
| Auth | `apiKey` in JSON POST body (server-side only) |
| Base | `https://canny.io/api/v1/` (boards, posts, votes, categories, tags, entries, statuses), `https://canny.io/api/v2/` (comments, users, companies) |
| Pagination | v1 `skip` + `limit` (`hasMore`); v2 cursor (`cursor`, `hasNextPage`, limit 1-100) |
| Limits | per key per second/minute/hour: Business 20/600/15,000; Core/Starter/Growth/Pro 10/300/5,000; Free 5/100/1,000; also per IP (Business allowance); 429 + `Retry-After` |

Hour cap matters: Pro plan 5,000/h -> preview ETA uses hourly budget (10k posts with votes may take 2+ h; shown).

Mapping: boards -> board mappings; categories; tags; statuses (`open`, `under review`, `planned`, `in progress`, `complete`, `closed`, custom) -> status mappings; posts (title, details, author, `created`, `score` -> votes total, `eta` -> `metadata`, owner -> member mapping) -> 14 posts; votes (`/votes/list` by post, voter user email/userID) -> votes; comments (v2, `internal` -> internal comment, replies flattened) -> comments; `mergeHistory` merged posts -> skipped (`merged_at_source`); users/companies -> contacts/companies (`userID` -> external_id); changelog entries (`/entries/list`, status published/draft/scheduled, `types` new/improved/fixed -> label mappings, `publishedAt`) -> 13 entries (scheduled at source -> draft in Dimmi).

## 8. Public help center by URL (`public_site`)

No credentials. Input URL (`https://help.acme.com` or subpath). Reuses 08 crawl (`app/support/crawl`: robots.txt, sitemap, politeness 2 req/s/host, concurrency 2, SSRF guard per hop).

1. Detect platform from home page + response headers (fingerprints in `publicsite/detect.go`, table-tested on saved HTML): Intercom (`intercom-help-center` markers, `/{locale}/articles/{id}-`), Zendesk Guide (`/hc/{locale}/`), Help Scout Docs (`/article/{n}-`), Crisp (`/{locale}/article/`), Featurebase help center, GitBook, Mintlify (`mintlify` meta / `_next` + docs.json), Document360, Docusaurus (`docusaurus` generator meta), else `generic`.
2. Enumerate: sitemap.xml (and sitemap index) filtered by platform article/collection URL patterns; fallback BFS crawl from home within same host + path prefix, max pages entitlement `imports.public_site_pages`.
3. Extract per page: platform selectors for title, body container, breadcrumb (collection path, max 2 levels), last updated; `generic` uses readability (`framework/dom.Readable`). Body HTML -> Markdown; code blocks language hints kept; images downloaded (same host or CDN, SSRF guarded) -> `help_center_assets`; links to other imported pages rewritten in `article_links` stage.
4. Collections from breadcrumb (or sitemap path segments); article order = sitemap order.
5. Redirect from each page path (exact path as served, query stripped) to new article/collection.
6. Published = everything public (no drafts visible). `articles.publish` still required (draft lets team review first).
7. Delta: re-crawl sitemap; `lastmod` or content checksum change -> update working copy (published article: new version published only when `publish: preserve` and article untouched in Dimmi since import, else working copy marked `has_unpublished_changes`).

Limits: JS-only sites (client rendered, empty body) detected like 08 (`site_requires_js` warning, article skipped); login-protected help centers: nothing found -> 422 `site_not_detected` with hint to use API source.

## 9. CSV (`csv`)

Files `contacts_csv`, `companies_csv` (`import_files`). UTF-8 (BOM stripped), UTF-16 with BOM converted, other encodings rejected (422 at file upload `csv_encoding_unsupported`). Delimiter sniff (`,`, `;`, tab), header row required, max `imports.csv.max_rows`. Column targets: `email`, `name`, `first_name` + `last_name` (joined), `phone`, `external_id`, `company.name`, `company.external_id`, `company.domain`, `first_seen_at`, `last_seen_at`, `custom.<key>`, `skip`. Row key for idempotency: `external_id`, else lowercased email, else `row:<sha256 of row>` (`connection_key csv:<imp id>`).

## 10. Covered elsewhere (catalog kind `external`)

| Need | Where |
|---|---|
| Markdown zip, Notion export (Markdown), GitBook/Mintlify/Docusaurus Git repo | 08 knowledge source `upload` or `git` in `import` mode (suggestions, review queue). Redirects: use `public_site` source instead when old site still live (keeps URLs) |
| Feedback CSV (Canny, Featurebase exports) | 14 `/feedback-imports` |
| Changelog subscribers | 13 `/changelog-subscriber-imports` (consent attestation) |
| Email forwarding continuity | 04 forwarding address setup (wizard next-step checklist) |

## 11. Normalization rules (all sources)

- Times: provider UTC (unix seconds, ms or ISO 8601) -> `timestamptz`; missing `created_at` -> parent's or import time (flag `metadata.import.time_estimated`). Future dates (clock skew at source) clamped to import time.
- IDs: external ids stored as strings; numeric ids normalized without leading zeros only when source documents numeric type.
- Text: NFC; invalid UTF-8 replaced (U+FFFD) and counted; NUL bytes stripped (Postgres text).
- Bodies: HTML -> Markdown (`importkit/htmlmd.go`); over 64 KB message body split into consecutive messages (same timestamp + 1 ms each, `metadata.import.split: {index, of}`); article body over 256 KB -> skipped with `article_too_large` error and listed (team splits manually; never truncated).
- Emails: lowercased domain part, trimmed, validated (stdlib `net/mail`); invalid -> stored in custom attribute `import_invalid_email`, not identity.
- Phones: E.164 normalize when parsable with source country hint; else custom attribute.
- Custom attributes: key normalize (lowercase, non `[a-z0-9_]` -> `_`, collapse, trim to 64); type mismatch with existing key -> mapping conflict; over 100 keys -> extra dropped (error `attribute_limit`, preview warns); values over 1,024 chars truncated with `…` (error `attribute_dropped` counted).
- Attribute trust: every imported custom attribute untrusted for 07 agent actions unless its mapping `trusted = true`; then key added to `contacts.trusted_attribute_keys` for `kind user` contacts where import wrote value. Definitions `client_writable` false at import, protected source attributes included (18). Intercom/Zendesk/Help Scout lead or end-user-editable fields: preview flags them (`external_meta.end_user_writable: true` when source says so) and dashboard warns before trusting.
- Slugs: from source slug (else title) via `support/slug`; conflict in help center -> suffix `-2` (warning `slug_conflict_renamed`), redirect still points right.
- Authors: teammate mapping; deactivated member mapping allowed (history), placeholder name `"{name} (former teammate)"` stored in `metadata.import.author`.

## Sources (verified 2026-11-02)

- Intercom rate limits: https://developers.intercom.com/docs/references/rest-api/errors/rate-limiting
- Intercom pagination: https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/pagination/
- Intercom 500-part limit: https://developers.intercom.com/intercom-api-reference/reference/retrieve-a-conversation , https://community.intercom.com/ideas/allow-api-access-to-all-conversation-parts-beyond-the-500-part-limit-14788
- Intercom Go SDK status: https://github.com/intercom/intercom-go
- Zendesk rate limits: https://developer.zendesk.com/api-reference/introduction/rate-limits/
- Zendesk incremental exports: https://developer.zendesk.com/api-reference/ticketing/ticket-management/incremental_exports/
- go-zendesk latest: https://proxy.golang.org/github.com/nukosuke/go-zendesk/@latest (v0.18.0, 2024-02-06)
- Help Scout auth: https://developer.helpscout.com/mailbox-api/overview/authentication/ ; rate limiting: https://developer.helpscout.com/mailbox-api/overview/rate-limiting/ ; Docs API: https://developer.helpscout.com/docs-api/
- Featurebase API: https://help.featurebase.app/en/api-reference , https://docs.featurebase.app/rest-api/changelog
- Productlane API: https://productlane.com/docs/pagination , https://productlane.com/docs/rate-limits
- Canny API: https://developers.canny.io/api-reference
- Ferndesk migration (22 platforms, redirects): https://ferndesk.com/help/migrate-your-help-center-to-ferndesk-enryy

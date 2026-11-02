# 13 Changelog

Style: caveman. Date 2026-11-02. Owner: 13-changelog. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`, `specs/08-help-center/README.md` (host, rendering, assets, knowledge reused here).

Files in folder:

| File | What |
|---|---|
| README.md | this: scope, data model, code map, API table, public site, events, jobs, config, permissions, budgets, required edits in other specs |
| [api.md](api.md) | request/response JSON for key endpoints, public routes, feeds, widget API, MCP tools, CLI, error codes |
| [email.md](email.md) | subscribers, double opt-in, mailings, deliverability, unsubscribe |
| [edge-cases.md](edge-cases.md) | decided behavior per case (175) |
| [tasks.md](tasks.md) | build checklist, demo script |

## Goal

Public changelog that writes itself from GitHub and coding agents, lives on help center host (`/changelog`), reaches users in widget "What's new" tab, RSS and email, and feeds Dimmi Agent so "what's new?" gets cited answer. Dimmi runs own public changelog on Dimmi (help center host `/changelog`) from day one.

## Why

Product owner moved changelog from post-MVP list (`research/roadmap.md`, old "changelog" item) into launch: "anything we use ourselves as a business is MVP, so we showcase every feature". Changelog is first thing visitors of a dev-tool startup read after landing page. Featurebase ships changelog page + widget + email subscribers (`research/competitors/featurebase.md`); Productlane drafts changelog from Linear projects (`research/competitors/productlane.md` "Release Intelligence"). Our angle: drafts from GitHub releases, merged PRs, or user's own coding agent via MCP; BYOK AI; zero JS; RSS + `.md` + `llms.txt` for agents.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Changelog index and entry page TTFB, cached | p95 under 50 ms (same 08 page cache) |
| Same, cold | p95 under 150 ms (body HTML precomputed at save) |
| Lighthouse mobile, index + entry page | 100 / 100 / 100 / 100 (CI on seeded pages) |
| JavaScript required on public pages | 0 bytes. Reactions and subscribe work without JS (forms); `hc.js` upgrade adds under 1 KB gz |
| Publish to public page, feeds, widget | under 2 s on every replica |
| Publish to agent-searchable (FTS) | same transaction; vectors p95 under 30 s |
| Scheduled publish accuracy | within 60 s of `scheduled_for`, p99 |
| Mailing throughput (cloud) | 10,000 recipients sent in under 10 minutes |
| Email HTML size | under 90 KB (Gmail clips at about 102 KB); over budget: summary + "Read more" |
| Widget "What's new" list | server p95 under 30 ms; lazy widget chunk under 8 KB gz |
| GitHub release published to draft entry | under 10 s (non-AI), under 60 s (AI rewrite, dominated by provider) |
| Double opt-in confirm link | survives corporate link scanners (GET never confirms) |
| Unsubscribe | RFC 8058 one-click, effective before next send, honored instantly (Gmail/Yahoo require within 2 days) |
| Feed | RSS 2.0 validates in CI; stable item ids across slug and host changes |
| Every public entry has `.md` twin, listed in help center `llms.txt` | 100% |
| Nothing AI- or GitHub-drafted publishes without human | enforced in code: generation and GitHub paths create `draft` only; MCP publish behind confirm gate |

## Scope

### In (MVP)

- One changelog per help center (1:1). Every workspace has default help center (08), so every workspace has changelog. Changelog row created by workspace initializer for default help center, lazily for other help centers (`EnsureChangelog`, idempotent).
- Entries: title, slug, summary, Markdown body (08 dialect: GFM, alerts, code, images, internal article links), cover image (08 help center asset), optional version tag, labels, authors (members), related help articles, `announce` (badge + email) and `spotlight` (widget card) flags, SEO overrides.
- States: `draft`, `scheduled`, `published`; trash (soft delete, 30 days). Unpublish returns to draft. Backdated `published_at` for history import.
- No version history: entries edited in place (typo fixes after publish go live on save). Optimistic lock. Decision: changelog posts are short and point-in-time; article-style versions add cost without user value.
- Labels: per changelog, defaults New, Improved, Fixed; max 20 labels, 5 per entry; label filter pages and label feeds.
- Public site on help center host: `/changelog`, entry pages, label pages, pagination, `.md` twins, RSS, generated OG image per entry, JSON-LD, sitemap + `llms.txt` entries via 08 provider hooks. Custom domain = help center domain (08); changelog at its `/changelog`.
- Path mode works too: `{app_url}/hc/{sub}/changelog`.
- Reactions: fixed emoji set (5), per entry counts, page + widget, toggle, no account. Comments out (decision below).
- Shipped feature requests (14): entry editor links feedback posts (14 `feedback_posts.changelog_entry_id`, set via 14 action `feedback.LinkChangelogEntry`); publish event carries `feedback_post_ids`, `notify_voters`; 14 emails voters, 13 newsletter skips them (Feedback seam).
- Email subscribers (separate `changelog_subscribers`, optional link to contact): double opt-in from page and widget, single-click for widget users with verified email, API/import with consent attestation, one email per announced entry, 5-minute undo window before send, RFC 8058 one-click unsubscribe, bounce/complaint handling via 04, GDPR eraser. Details [email.md](email.md).
- Widget "What's new" tab API: list, entry, unread count per contact (`changelog_reads.last_seen_at`), read-state, reactions, subscribe, spotlight card data in widget config.
- GitHub source: points at 08 GitHub repository (`github_repository_id`; shared 08 GitHub layer `08-help-center/docs-sync.md` section 4: one connection token, one repo webhook, one route for 08 knowledge, 08 docs sync, 13). 13 registers `release` handler on 08 dispatcher -> draft entry (raw notes or AI rewrite). Dedupe per release.
- Generations: draft entry from GitHub tag range (merged PRs), single release, or pasted text (CLI sends local `git log`), optional BYOK rewrite via 07, async job resource. Always lands as draft with `needs_review`.
- MCP toolset `changelog` (5 tools + prompt) and CLI `dimmi changelog ...` (09 implements wrappers). User's coding agent drafts entries with zero BYOK spend.
- Import write helper `changelog.ImportEntry` for 16 (Featurebase, Productlane, Canny changelogs): backdated, `source_kind import`, no events, no mailing, no badge (section Import helper).
- Knowledge ingestion: published entries become 08 knowledge documents (source type `changelog`, public, citable, dated title) so Dimmi Agent, Ask AI, help center search and public MCP answer "what changed".
- Webhooks (10) for publish/unpublish/subscriber events; 06 destination trigger posts new entries to Slack channel (Discord later: discord-notifications).
- Analytics, privacy-friendly: daily entry views (page, widget), reactions, subscribers growth, mailing delivery counts (sent, bounced, complained, unsubscribed). No open pixel, no click tracking, no cookies.

### Out (later)

| Item | When | Why |
|---|---|---|
| Comments on entries | later | moderation and spam surface |
| Audience segmentation (entries visible only to plan X, emails to segment) | later | 15 segments available (`segments`, `seg_`); adopt later; schema leaves `audience` jsonb unused |
| Per-label email subscriptions | later | MVP subscribe = all announced entries |
| Notify people who requested feature | ships in 14 (launch) | entry links feedback posts; 14 emails voters on `changelog_entry.published`; 13 newsletter dedupes (Feedback seam) |
| Translations | post-MVP: auto-translation | single locale = help center locale; URL scheme reserves `/{locale}/` like 08 |
| Embeddable `<script>` changelog widget without Messenger, iframe embed | later | Messenger tab covers widget; headless JSON comes with later: atom-json-feeds |
| Popup/banner announcements outside Messenger | ships in 15-announcements-surveys | 15 announcement CTA `open_changelog_entry` links entry; `spotlight` card stays inside Messenger (not counted by 15 throttle) |
| Open and click tracking | not planned | privacy; Apple MPP makes opens noise |
| Changelog import from Featurebase, Productlane, Canny | launch: importers (16 via `changelog.ImportEntry`) | API also supports backdated `published_at` + `source_kind import` |
| Changelog import from Beamer, Headway, LaunchNotes | later: more-import-sources | 16 driver = new registry entry |
| GitHub App install | later: github-app (08 shared GitHub layer) | fine-grained token on 08 `github_connections` at MVP |
| GitLab source | later | GitHub only |
| Linear project completion drafts | post-MVP: Linear | |
| Video cover, Loom/YouTube iframe | later | CSP `frame-src none` (08); link card |
| Entry version history | later if asked | in-place edit decision above |
| Pinned entries, custom templates/CSS | later | 08 theme only |
| Scheduled unpublish | later | |
| Weekly digest cadence (per-subscriber override, digest scheduler) | later: changelog-digest | audit C8: none of IC/PL/FD offer it, FB unclear; second mailing scheduler. MVP: one email per announced entry |
| Atom and JSON Feed (+ CORS headless JSON) | later: atom-json-feeds | audit C9: FB, PL, FD ship RSS only; RSS stays |
| Dedicated changelog custom domain (`purpose = changelog`, `changelog.acme.com`) | later: changelog-custom-domain | audit B12: FB/PL portal domain covers changelog; none has per-purpose domain. MVP: help center domain + `/changelog` |
| Public MCP tool `dimmi_list_updates` | later: public-mcp-changelog | audit B13: none documented. MVP: public `dimmi_search_docs` already returns changelog chunks (knowledge) |
| Changelog email replies + widget "Send feedback" land in inbox tagged with entry (`source.changelog_entry_id`, Reply-To support) | later: changelog-replies-to-inbox | audit B14: none of the 4 document it. MVP: no Reply-To; replies reach From address, handled by 04 like any mail to it, no entry tag |

## User stories

Workspace member:
- Write entry in Markdown, pick labels, cover, version `v1.4.0`, link "Read the guide" article, preview page and email, publish now or schedule Tuesday 09:00 workspace time, undo email within 5 minutes.
- See GitHub release `v1.4.0` arrive as draft rewritten in customer language by our BYOK model; edit; publish.
- See per entry: views, widget views, reactions, emailed to 1,240, 3 bounced, 2 unsubscribed.
- Import 800 existing subscribers with consent attestation; export subscribers CSV.

End customer:
- Land on fast changelog page from tweet, read without JS, react, subscribe with email (confirm in inbox), RSS in Feedly.
- Open widget, see red dot on "What's new", read entry, react.
- Get email per announced entry; one-click unsubscribe from Gmail header.

Developer (solo dev, workspace owner):
- `git tag v1.4.0 && git push --tags` + GitHub release: draft waits in Dimmi.
- In Claude Code: "write changelog for everything since v1.3.0 and publish after I approve" -> MCP prompt + tools; confirmation dialog before publish.
- GitHub Action: `npx dimmi changelog publish --file CHANGELOG/v1.4.0.md --yes` with `changelog:write` token.

AI agent:
- Dimmi Agent asked "can I export CSV yet?" retrieves changelog chunk "Changelog: CSV export (2026-10-30)" and cites entry URL.
- Customer's coding agent reads `llms.txt` or `/changelog.md`.

## Data model

All tables: `id uuid primary key default uuidv7()` unless stated, `workspace_id uuid not null references workspaces(id) on delete cascade`, `created_at`, `updated_at`, text + CHECK enums, indexes lead with `workspace_id`, RLS on all in cloud.

New public ID prefixes (checked against registry in `00-foundation/data-model.md`, no collisions; added to `app/models/public_ids.go` on `framework/ids`): `clg_` changelogs, `cle_` changelog_entries, `cll_` changelog_labels, `csub_` changelog_subscribers, `clm_` changelog_mailings, `cls_` changelog_sources, `cgn_` changelog_generations. Tables without public ids: pivots, reads, reactions, reaction counts, mailing recipients, stats.

### changelogs
| Column | Type | Notes |
|---|---|---|
| help_center_id | uuid not null unique references help_centers on delete cascade | 1:1 |
| enabled | boolean not null default true | false: public routes 404, widget tab hidden, API works |
| title | text not null default 'Changelog' | 1 to 60, page H1, feed title |
| description | text | max 300, meta description, feed description |
| show_in_nav | boolean not null default true | help center header link "Changelog" (only when at least one public published entry) |
| show_authors | boolean not null default true | author names + avatars on public page |
| reactions_enabled | boolean not null default true | |
| subscriptions_enabled | boolean not null default true | email subscribe form and widget subscribe |
| email_delay_minutes | smallint not null default 5 CHECK between 0 and 60 | undo window |
| email_content | text not null default 'full' CHECK in ('full','summary') | |
| email_from_address_id | uuid null references email_addresses on delete set null | 04; null = workspace default sender address |
| og_image_asset_id | uuid null references help_center_assets on delete set null | index page OG; null = help center OG image |
| last_published_at | timestamptz | newest published entry |
| latest_announced_at | timestamptz | newest published entry with `announce`; widget badge |
| subscriber_count | integer not null default 0 | active subscribers, maintained in tx |
| audience | jsonb not null default '{}' | reserved (segmentation later), always `{}` at MVP |
Indexes: unique `(help_center_id)`, `(workspace_id)`.

### changelog_entries
| Column | Type | Notes |
|---|---|---|
| changelog_id | uuid not null references changelogs on delete cascade | |
| help_center_id | uuid not null | denormalized for knowledge and host scoping |
| title | text not null | 1 to 150, NFC |
| slug | citext not null | 1 to 120 `[a-z0-9-]`; unique `(changelog_id, slug) where deleted_at is null`; reserved: `page`, `labels`, `feed`, `rss`, `atom`, `subscriptions`, `subscription-confirmations`, `unsubscriptions`, `llms-full`, `og` |
| previous_slugs | text[] not null default '{}' | max 10, newest first; old URLs 301 |
| summary | text | max 300; excerpt, meta description, email preheader, feed summary. Null: first 200 chars plain text |
| body_markdown | text not null default '' | max 128 KB |
| body_html | text not null default '' | `framework/markdown` with 08 dialect options (`app/support/helpmarkdown`) + `framework/sanitize`, precomputed on every save |
| renderer_version | smallint not null | rerender job on bump (shared constant with 08) |
| content_hash | text not null | sha256 of canonical (title, slug, summary, body, labels, version_tag, cover) |
| word_count | integer not null default 0 | |
| status | text not null CHECK in ('draft','scheduled','published') default 'draft' | |
| announce | boolean not null default true | false: listed on page and feeds, no widget badge, no email |
| notify_subscribers | boolean not null default true | email on publish (needs `announce`); set per publish |
| spotlight | boolean not null default false | widget card on next open (newest spotlight within 14 days wins) |
| version_tag | text | max 40, `^[A-Za-z0-9._+-]+$`, e.g. `v1.4.0` |
| cover_asset_id | uuid null references help_center_assets on delete set null | same help center |
| og_image_asset_id | uuid null references help_center_assets on delete set null | generated card (RenderChangelogOgImage) |
| seo | jsonb not null default '{}' | `{title, description, noindex}` |
| published_at | timestamptz | display date; set on publish (now) or explicit backdate; null for drafts never published |
| first_published_at | timestamptz | |
| scheduled_for | timestamptz | required when `scheduled`; future, max 1 year |
| source_kind | text not null default 'member' CHECK in ('member','api','agent','github_release','generation','import') | |
| source_ref | jsonb not null default '{}' | `{repo, release_id, tag, release_url, pr_numbers[], from_ref, to_ref, client_id, generation_id}` |
| dedupe_key | text | unique `(workspace_id, dedupe_key) where dedupe_key is not null and deleted_at is null` (GitHub release redelivery) |
| notify_voters | boolean not null default true | 14 voter email on publish when posts linked (posts link lives in 14 `feedback_posts.changelog_entry_id`, no 13 column for post ids) |
| needs_review | boolean not null default false | true for agent, github_release, generation; cleared by member edit or publish |
| created_by_type, created_by_id | text, uuid | member, token, oauth_client, system |
| last_edited_by_type, last_edited_by_id | text, uuid | |
| lock_version | integer not null default 1 | `If-Match` |
| edited_after_publish_at | timestamptz | public "Updated" marker when content changed after first publish |
| deleted_at | timestamptz | trash, purge after 30 days |
| title_trgm | text generated `lower(f_unaccent(title))` stored | GIN trgm, dashboard search |
Indexes: `(workspace_id, changelog_id, status, published_at desc, id desc)`, unique slug partial, GIN `(previous_slugs)`, `(scheduled_for) where status = 'scheduled' and deleted_at is null` (system sweeper, documented RLS-bypass scan like 08 domains), GIN trgm `(title_trgm)`, unique dedupe partial.

### changelog_labels
`id, workspace_id, changelog_id references changelogs on delete cascade, name text not null (1 to 30), slug citext not null ([a-z0-9-] 1 to 40), color text not null (#RRGGBB), position integer not null (gaps 1024), created_at, updated_at`. Unique `(changelog_id, lower(name))`, `(changelog_id, slug)`. Hard delete removes pivots (entries keep other labels). Seeded: New `#16a34a`, Improved `#2563eb`, Fixed `#ea580c`.

### changelog_entry_label, changelog_entry_author, changelog_entry_article
- `changelog_entry_label`: `workspace_id, entry_id references changelog_entries on delete cascade, label_id references changelog_labels on delete cascade, created_at`. PK `(entry_id, label_id)`. Max 5 per entry.
- `changelog_entry_author`: `workspace_id, entry_id on delete cascade, workspace_member_id references workspace_members on delete cascade, position smallint, created_at`. PK `(entry_id, workspace_member_id)`. Max 5. Default: creating member (none for token/agent/system).
- `changelog_entry_article`: `workspace_id, entry_id on delete cascade, article_id references articles on delete cascade, position smallint, created_at`. PK `(entry_id, article_id)`. Max 5. Article must be same help center. Rendered only when article public + published.

### changelog_entry_reaction_counts
`workspace_id, entry_id references changelog_entries on delete cascade, emoji text CHECK in ('thumbs_up','heart','tada','eyes','thumbs_down'), count integer not null default 0 CHECK >= 0, updated_at`. PK `(entry_id, emoji)`. Separate rows (not entry column) so hot entries do not contend on entry row or bump `lock_version`.

### changelog_reactions
`workspace_id, entry_id on delete cascade, emoji (same CHECK), reactor_hash bytea not null, contact_id uuid null references contacts on delete set null, surface text CHECK in ('page','widget'), created_at`. PK `(entry_id, reactor_hash, emoji)`. `reactor_hash` = `hashing.HMAC` SHA-256 (key derived from app key via `framework/encryption`, info `changelog-reactor-v1`, `workspace_id || identity`), identity = `contact:<id>` (widget session), or `visitor:<client key>` (128-bit random kept by hc.js / widget in localStorage), or `ip:<ip /24 or /48>|<ua hash>|<utc day>` (no-JS form; not toggleable). Raw IP and client key never stored. Index `(workspace_id, contact_id) where contact_id is not null` (eraser).

### changelog_reads
`workspace_id, changelog_id references changelogs on delete cascade, contact_id references contacts on delete cascade, last_seen_at timestamptz not null, updated_at`. PK `(changelog_id, contact_id)`. Written only for widget sessions (anonymous visitors without session keep cursor in localStorage).

### changelog_subscribers
See [email.md](email.md) for lifecycle.
| Column | Type | Notes |
|---|---|---|
| changelog_id | uuid not null references changelogs on delete cascade | |
| email | citext not null | normalized (trim, lowercase domain, IDNA), max 254; unique `(changelog_id, email)` |
| name | text | max 80 |
| status | text not null CHECK in ('pending','active','unsubscribed','bounced','complained') | |
| source | text not null CHECK in ('page','widget','api','import','dashboard') | |
| consent | text not null CHECK in ('double_opt_in','verified_identity','attested') | |
| consent_evidence | jsonb not null default '{}' | `{ip_prefix, user_agent (max 256), page_url, attested_by: {type,id}, attested_at}`; GDPR proof |
| contact_id | uuid null references contacts on delete set null | widget subscribe or verified email match |
| confirmed_at | timestamptz | active since |
| confirmation_sent_at | timestamptz | |
| confirmation_count | smallint not null default 0 | max 3 per 24 h |
| unsubscribed_at | timestamptz | |
| unsubscribe_reason | text CHECK in ('link','one_click','mailto','bounce','complaint','admin','api') | |
| last_emailed_at | timestamptz | |
| token_nonce | bytea not null | 16 random bytes; part of HMAC tokens; rotate to invalidate all links |
Indexes: unique `(changelog_id, email)`, `(workspace_id, changelog_id, status, id)` (mailing snapshot scan), `(workspace_id, contact_id) where contact_id is not null`, `(workspace_id, email)` (eraser by email, suppression sync).

### changelog_mailings
| Column | Type | Notes |
|---|---|---|
| changelog_id | uuid not null references changelogs on delete cascade | |
| entry_id | uuid null references changelog_entries on delete set null | unique `(entry_id) where status not in ('canceled')`; null after entry purge |
| status | text not null CHECK in ('scheduled','sending','sent','partially_sent','canceled','blocked','failed') | |
| send_after | timestamptz not null | publish time + `email_delay_minutes` |
| blocked_reason | text CHECK in ('sending_domain_required','monthly_cap_reached','email_sending_paused','no_recipients','changelog_disabled') | |
| subject | text not null | entry title |
| from_address_id | uuid null | snapshot at start |
| rendered_html, rendered_text | text | snapshot at start (per-recipient tokens substituted at send) |
| recipient_count, sent_count, failed_count, skipped_count, bounced_count, complained_count, unsubscribed_count | integer not null default 0 | |
| started_at, finished_at | timestamptz | |
| created_by_type, created_by_id | text, uuid | |
Indexes: `(workspace_id, changelog_id, created_at desc)`, `(send_after) where status = 'scheduled'` (system scan).

### changelog_mailing_recipients
`workspace_id, mailing_id references changelog_mailings on delete cascade, subscriber_id uuid null references changelog_subscribers on delete set null, status text CHECK in ('pending','sent','failed','skipped','bounced','complained'), skip_reason text (suppressed, unsubscribed, cap), outbound_email_id uuid null (04 outbound_emails, no FK: different retention), error_code text, sent_at timestamptz, created_at, updated_at`. PK `(mailing_id, subscriber_id)` with surrogate `id uuid` unique (subscriber may be erased, set null keeps counts). Index `(workspace_id, outbound_email_id) where outbound_email_id is not null`. Retention 400 days (match 04 stats), purge job.

### changelog_sources
Repo, token, webhook secret live in 08 shared GitHub layer (`github_connections`, `github_repositories`, `08-help-center/docs-sync.md` section 4). 13 row = changelog settings for one repo.

| Column | Type | Notes |
|---|---|---|
| changelog_id | uuid not null references changelogs on delete cascade | |
| type | text not null CHECK in ('github') | |
| name | text not null | 1 to 80; default repo `full_name` |
| github_repository_id | uuid not null references github_repositories on delete restrict | 08 table; repo must have connection (`connection_id` not null) else 422 `github_connection_required`; 08 repo delete 409 `github_repository_in_use` lists source |
| on_release | text not null default 'draft' CHECK in ('draft','draft_ai','ignore') | |
| include_prereleases | boolean not null default false | |
| default_label_ids | uuid[] not null default '{}' | max 5 |
| status | text not null default 'active' CHECK in ('active','error','paused') | |
| last_error_code, last_error | text | `github_token_invalid`, `github_repo_not_found`, `github_permission_missing`, `github_access_lost`, `rate_limited` (08 layer codes, same names) |
| last_event_at | timestamptz | |
| deleted_at | timestamptz | |
Limit: entitlement `changelog.max_sources` (5 per changelog; over: 422 `entitlement_limit_reached`). Indexes `(workspace_id, changelog_id)`, unique `(changelog_id, github_repository_id) where deleted_at is null` (dup: 422 `changelog_source_exists`), `(workspace_id, github_repository_id) where deleted_at is null` (handler `Wants` lookup).

### changelog_generations
| Column | Type | Notes |
|---|---|---|
| changelog_id | uuid not null | |
| input | jsonb not null | `{kind: github_range|github_release|text, source_id, from_ref, to_ref, release_tag, text (max 64 KB), instructions (max 1,000), label_ids}` |
| use_ai | boolean not null | false: deterministic Markdown assembly only |
| status | text not null CHECK in ('queued','running','succeeded','failed','canceled') | |
| entry_id | uuid null references changelog_entries on delete set null | draft created |
| ai_run_id | uuid null | 07 `ai_runs` (no FK, 07 prunes) |
| stats | jsonb not null default '{}' | `{commits, prs, prs_used, truncated, input_tokens, output_tokens, cost_micros}` |
| error_code, error | text | `github_*`, `ai_unavailable`, `spend_cap_reached`, `provider_error`, `empty_range`, `invalid_output` |
| created_by_type, created_by_id | | |
| started_at, finished_at | timestamptz | |
Index `(workspace_id, changelog_id, created_at desc)`. Pruned after 90 days.

### changelog_entry_stats_daily
`workspace_id, changelog_id, entry_id, day date, views int, widget_views int, markdown_views int`. PK `(workspace_id, entry_id, day)`.

### changelog_stats_daily
`workspace_id, changelog_id, day date, index_views int, feed_fetches int, subscribes int, unsubscribes int`. PK `(workspace_id, changelog_id, day)`. Feeds counted per changelog (feed readers fetch whole feed, not entries).

Both stats tables buffered in memory, flushed every 30 s (08 `FlushHelpCenterStats` ticker extended). Bot user agents excluded (08 list); feed reader UAs counted only as `feed_fetches`.

### Core table changes

None. Columns needed on other features' tables are requested in "Required from other specs" (08 `knowledge_documents`; 01 `widget_settings`). 04 already ships what 13 needs (`outbound_emails.kind changelog` + `metadata.purpose`, suppression scope `changelog`, stats bucket `changelog`). 13 adds no column or migration on 14 tables: 14 declares FK `feedback_posts.changelog_entry_id` inline (14 block runs after 13).

### Migrations

```
database/migrations/20261101001301_create_changelogs_table.sql
database/migrations/20261101001302_create_changelog_labels_table.sql
database/migrations/20261101001303_create_changelog_entries_table.sql
database/migrations/20261101001304_create_changelog_entry_pivot_tables.sql        # label, author, article
database/migrations/20261101001305_create_changelog_reactions_tables.sql          # reactions + reaction_counts
database/migrations/20261101001306_create_changelog_reads_table.sql
database/migrations/20261101001307_create_changelog_subscribers_table.sql
database/migrations/20261101001308_create_changelog_mailings_tables.sql           # mailings + recipients
database/migrations/20261101001309_create_changelog_sources_table.sql
database/migrations/20261101001310_create_changelog_generations_table.sql
database/migrations/20261101001311_create_changelog_stats_tables.sql
database/migrations/20261101001312_add_changelog_rls_policies.sql
database/migrations/20261101001313_backfill_changelogs_for_help_centers.sql      # EnsureChangelog for existing help centers (upgrade path)
database/migrations/20261101001314_add_changelog_entry_id_to_knowledge_documents_table.sql  # 08-owned column, needs 13 table first (agreed with 08, Required section)
```
Numbers after 08 (`0008xx`, incl. `000821-000827` GitHub layer) so FKs to `help_centers`, `help_center_assets`, `articles`, `github_repositories` resolve; after 04 (`email_addresses`). 08/01/04 column changes listed in Required section ship in their owners' migration blocks.

## Code map

```
app/
├── actions/changelog/
│   ensure_changelog.go                       # EnsureChangelog: workspace initializer (default help center) + lazy; seeds labels; idempotent
│   update_changelog.go
│   create_changelog_entry.go, update_changelog_entry.go, delete_changelog_entry.go, restore_changelog_entry.go, purge_changelog_entry.go
│   publish_changelog_entry.go               # publish now or backdated; renders, indexes knowledge (08), schedules mailing, OG job, cache bust; event carries 14 feedback post ids
│   link_changelog_feedback_posts.go         # entry editor `feedback_post_ids` -> 14 `feedback.LinkChangelogEntry(ctx, tx, entryID, postIDs)` (14 owns column)
│   find_published_entry.go                  # changelog.FindPublishedEntry(ctx, workspaceID, entryID): read for 15-announcements-surveys CTA validation + widget deep open
│   schedule_changelog_entry.go, unpublish_changelog_entry.go
│   publish_scheduled_changelog_entry.go     # called by job; re-checks state
│   render_changelog_entry.go                # markdown -> body_html, word_count, warnings (`framework/markdown` + `helpmarkdown` options)
│   bulk_update_changelog_entries.go
│   create_changelog_label.go, update_changelog_label.go, delete_changelog_label.go, reorder_changelog_labels.go
│   react_to_changelog_entry.go, remove_changelog_reaction.go
│   mark_changelog_seen.go                   # changelog_reads upsert (monotonic)
│   unread_changelog_count.go
│   subscribe_to_changelog.go, confirm_changelog_subscription.go, unsubscribe_from_changelog.go
│   update_changelog_subscriber.go, delete_changelog_subscriber.go, import_changelog_subscribers.go
│   create_changelog_mailing.go, cancel_changelog_mailing.go, start_changelog_mailing.go
│   send_changelog_mailing_batch.go, finish_changelog_mailing.go
│   render_changelog_email.go, render_changelog_confirmation_email.go, send_changelog_test_email.go   # templ views; send via 04 SendChangelogEmail / SendContactMail (Kind changelog, Purpose entry|confirmation)
│   sync_subscriber_suppression.go           # 04 email_suppression.created {outbound_kind changelog_*} -> subscriber bounced/complained
│   create_changelog_source.go, update_changelog_source.go, delete_changelog_source.go, test_changelog_source.go   # repo via 08 `github_repositories` (create by id, or `full_name` + `connection_id` -> 08 `github.AddRepository`); delete -> 08 `SyncGithubRepositoryWebhook` drops `release`
│   github_release_handler.go                # implements 08 `github.WebhookHandler`: Name `changelog`, Events `release`, Wants = active source on repo, Handle enqueues HandleGithubChangelogEvent per source; registered at boot via `github.RegisterWebhookHandler`
│   handle_github_release_event.go
│   import_changelog_entry.go                # ImportEntry, EnsureLabel, DeleteImportedEntries: 16 import write helpers (backdated, no events, no mailing)
│   create_changelog_generation.go, run_changelog_generation.go, assemble_changelog_markdown.go (pure)
│   index_changelog_entry_knowledge.go       # calls 08 knowledge.IndexChangelogEntry / RemoveChangelogEntry in tx
│   render_changelog_og_image.go
│   record_changelog_view.go                 # buffered
│   erase_changelog_contact_data.go          # EraseChangelogContactData: contact eraser hook
│   public_sections.go                       # implements 08 helpcenter.PublicSectionProvider (nav, home, sitemap, llms.txt)
├── enums/ changelog_entry_status.go, changelog_source_kind.go, changelog_reaction.go, changelog_subscriber_status.go,
│          changelog_mailing_status.go, changelog_generation_status.go
├── events/ changelog_entry_created.go, changelog_entry_updated.go, changelog_entry_scheduled.go, changelog_entry_published.go,
│           changelog_entry_unpublished.go, changelog_entry_deleted.go, changelog_updated.go, changelog_mailing_updated.go,
│           changelog_mailing_completed.go, changelog_subscriber_subscribed.go, changelog_subscriber_unsubscribed.go,
│           changelog_generation_completed.go, changelog_generation_failed.go, changelog_source_errored.go
├── exceptions/changelog_errors.go
├── http/
│   ├── controllers/api/v1/
│   │   help_center_changelog_controller.go            # Show, Update (singleton)
│   │   help_center_changelog_label_controller.go      # Index, Store, UpdateMany (order)
│   │   changelog_label_controller.go                  # Update, Destroy
│   │   changelog_entry_controller.go                  # Index, Store, Show, Update, Destroy, UpdateMany
│   │   changelog_entry_email_preview_controller.go    # Store (computed)
│   │   changelog_entry_email_test_controller.go       # Store (sync test send to caller)
│   │   changelog_mailing_controller.go                # Index, Store, Show, Update (cancel)
│   │   changelog_subscriber_controller.go             # Index (+csv), Store, Show, Update, Destroy
│   │   changelog_subscriber_import_controller.go      # Store (computed result)
│   │   changelog_source_controller.go                 # Index, Store, Show, Update, Destroy
│   │   changelog_source_test_controller.go            # Store
│   │   changelog_generation_controller.go             # Index, Store, Show, Update (cancel)
│   │   help_center_changelog_stats_controller.go      # Show
│   ├── controllers/web/changelog/
│   │   index_page_controller.go, entry_page_controller.go, label_page_controller.go
│   │   entry_markdown_controller.go, index_markdown_controller.go, llms_full_txt_controller.go
│   │   rss_feed_controller.go
│   │   subscription_controller.go                       # POST form (page) + /api JSON
│   │   subscription_confirmation_controller.go         # GET page, POST confirm (token in path, rest.md exception)
│   │   unsubscription_controller.go                    # GET page, POST one-click (RFC 8058)
│   │   reaction_controller.go                           # POST/DELETE form + /api JSON
│   │   og_image_controller.go                           # serves generated asset by entry (redirects to asset URL)
│   ├── controllers/widget/
│   │   changelog_controller.go (Show), changelog_entry_controller.go (Show), changelog_read_state_controller.go (Update)
│   │   changelog_entry_reaction_controller.go (Store, Destroy), changelog_subscription_controller.go (Store)
│   ├── middleware/resolve_changelog.go             # after 08 resolve_help_center: changelog row, base path `/changelog`, enabled check
│   ├── requests/ store_changelog_entry_request.go, update_changelog_entry_request.go, update_many_changelog_entries_request.go,
│   │   update_changelog_request.go, store_changelog_label_request.go, update_changelog_label_request.go, reorder_request.go (08 shared),
│   │   store_changelog_subscriber_request.go, update_changelog_subscriber_request.go, store_changelog_subscriber_import_request.go,
│   │   store_changelog_mailing_request.go, update_changelog_mailing_request.go, store_changelog_source_request.go,
│   │   update_changelog_source_request.go, store_changelog_generation_request.go, store_changelog_email_test_request.go,
│   │   public_changelog_subscription_request.go, public_changelog_reaction_request.go, widget_changelog_read_state_request.go
│   └── resources/ changelog_resource.go, changelog_entry_resource.go, changelog_label_resource.go, changelog_subscriber_resource.go,
│       changelog_mailing_resource.go, changelog_source_resource.go, changelog_generation_resource.go, changelog_stats_resource.go,
│       changelog_email_preview_resource.go, changelog_subscriber_import_resource.go,
│       widget_changelog_entry_resource.go, public_changelog_entry_resource.go (RSS, `.md`)
├── jobs/ (see Jobs table)
├── listeners/
│   bump_help_center_cache_on_changelog_change.go   # sync after commit: entry published/updated/unpublished/deleted, labels, settings
│   sync_subscriber_suppression_listener.go         # queued mail: 04 email_suppression.created
│   bust_widget_config_on_changelog_publish.go      # sync after commit: bump linked widgets' config_version (01 column) on publish/unpublish of announced entry
│   (no `app/mail` Mailables: recipients are subscribers/contacts, customer stream only via 04 actions; arch test `tests/arch/email_streams_test.go`)
├── mcp/
│   tools/changelog_tools.go                        # dimmi_list_changelog_entries, dimmi_get_changelog_entry, dimmi_draft_changelog_entry,
│                                                   # dimmi_update_changelog_entry, dimmi_publish_changelog_entry
│   prompts/write_changelog_entry.go                # MCP prompt dimmi_write_changelog_entry (text embedded write_changelog_entry.md)
├── policies/ changelog_policy.go, changelog_entry_policy.go, changelog_subscriber_policy.go, changelog_source_policy.go
├── services/github/ releases.go, compare.go, pulls.go       # adapter zone on google/go-github/v93 v93.0.0 (owner 08, shared GitHub layer; 13 adds releases/compare/pulls); client only via 08 `github.ClientFor(ctx, a, ws, githubRepositoryID)`; HTTP via a.HTTP.StdClient() (SSRF guard), host allowlist api.github.com, SDK retries off
├── support/
│   feeds/ items.go                                 # entry -> feed item mapping (ids, absolute URLs, labels); RSS writer on framework `view/feed` (gorilla/feeds v1.2.0)
│   ogimage/ card.go                                # layout only: 1200x630 PNG, embedded Inter subset (OFL), brand color, logo; draw + PNG encode via `framework/image`
│   emailhtml/ inline.go                            # keep custom (no pure-Go MJML): Markdown HTML -> email-safe HTML (inline styles, table layout, absolute URLs, code wrap); parse via `framework/dom`
config/changelog.go
resources/views/changelog/ index.templ, entry.templ, label.templ, subscribe.templ, confirm.templ, unsubscribe.templ, partials/*.templ
resources/views/mail/changelog/ entry.templ, confirmation.templ (text twins *.txt.templ; rendered by 13 actions, not Mailables)
resources/prompts/changelog/2026-11-01/ draft.md, rewrite_release.md
routes/api.go, routes/web.go (help center group), routes/widget.go, routes/mcp.go, routes/console.go (no webhook route: 08 `routes/github.go` shared route + handler registry)
database/queries/ changelogs.sql, changelog_entries.sql, changelog_labels.sql, changelog_entry_pivots.sql, changelog_reactions.sql,
   changelog_reads.sql, changelog_subscribers.sql, changelog_mailings.sql, changelog_mailing_recipients.sql, changelog_sources.sql,
   changelog_generations.sql, changelog_stats.sql
database/factories/ changelog.go, changelog_entry.go (Published(), Scheduled(at), WithLabels(...)), changelog_subscriber.go (Active(), Pending()),
   changelog_mailing.go, changelog_source.go
database/seeders/changelog_seeder.go               # 8 entries over 3 months, labels, 20 subscribers, reactions
app/console/commands/ changelog_rerender_command.go (`dimmi changelog:rerender`), changelog_publish_due_command.go (`dimmi changelog:publish-due`)
tests/
   # integration only (00-conventions/testing.md): real App, real Postgres (pgvector:pg18), real Valkey, real River drained via tc.Queue.Drain;
   # fakes only at process boundaries: tc.Mail (captured MIME), httpclienttest.New(tc.App.HTTP) (GitHub fixtures), llm.Fake(), tc.Clock. Edge ids in names: t.Run("E158 ...")
   feature/changelog/*_test.go (per endpoint), public_site_test.go, feeds_test.go (RSS validator + golden), seo_test.go,
   widget_changelog_test.go, subscriptions_test.go (double opt-in, scanner GET, one-click), mailings_test.go (snapshot, batches, undo, caps, feedback dedupe),
   github_release_handler_test.go (signed delivery through 08 shared route `/webhooks/github/repositories/{ghr}`, GitHub HTTP fixtures), generation_test.go (llm.Fake), mcp_changelog_tools_test.go,
   knowledge_changelog_test.go (agent cites entry), eraser_test.go, leak_test.go (drafts/scheduled never public), feedback_seam_test.go (real 14 posts),
   announcements_seam_test.go (15-announcements-surveys CTA lookup), import_helper_test.go (16 helpers, no side effects), valkey_resilience_test.go (flush/unreachable: E163, E164)
   feature/changelog/render_test.go: assemble_markdown, slug, reactor_hash, ogimage golden, emailhtml golden + size budget through real actions (table-driven, no mocks)
   bench/changelog_bench_test.go (TTFB, widget list, mailing 10k captured by tc.Mail)
```

## HTTP API

Full JSON in [api.md](api.md). All `/api/v1` routes: guard `auth` (letters per `00-foundation/api.md`: `A` = any of session, first-party app, token with ability), `workspace`, `throttle:api`. New abilities `changelog:read`, `changelog:write` (Required section). Subscriber PII uses existing `contacts:read` / `contacts:write`. Settings and GitHub sources use `settings:read` / `settings:write`.

### REST `/api/v1`

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | /help-centers/{hc}/changelog | help-centers.changelog.show | A `changelog:read` | settings, counts, URLs (lazy create) |
| PATCH | /help-centers/{hc}/changelog | help-centers.changelog.update | A `settings:write`, admin | title, toggles, email settings |
| GET | /help-centers/{hc}/changelog/labels | help-centers.changelog.labels.index | A `changelog:read` | ordered |
| POST | /help-centers/{hc}/changelog/labels | help-centers.changelog.labels.store | A `changelog:write` | |
| PATCH | /help-centers/{hc}/changelog/labels | help-centers.changelog.labels.update-many | A `changelog:write` | `{order: [cll_...]}` (rest.md rule 9) |
| PATCH | /changelog-labels/{cll} | changelog-labels.update | A `changelog:write` | name, slug, color |
| DELETE | /changelog-labels/{cll} | changelog-labels.destroy | A `changelog:write` | |
| GET | /help-centers/{hc}/changelog/stats | help-centers.changelog.stats.show | A `changelog:read` | `from`, `to` (max 90 days) |
| GET | /changelog-entries | changelog-entries.index | A `changelog:read` | `filter[help_center,status,label,source_kind,needs_review,trashed]`, `q` (title trigram), `sort=-published_at,-updated_at,title` |
| POST | /changelog-entries | changelog-entries.store | A `changelog:write`, `idempotent` | create draft; optional `status: published|scheduled` in same call |
| GET | /changelog-entries/{cle} | changelog-entries.show | A `changelog:read` | `include=labels,authors,articles,mailing,reactions` |
| PATCH | /changelog-entries/{cle} | changelog-entries.update | A `changelog:write` | fields; `status` transitions (publish, schedule, unpublish); `trashed: false` restore. `If-Match` required from session |
| DELETE | /changelog-entries/{cle} | changelog-entries.destroy | A `changelog:write` | trash; cancels scheduled mailing |
| PATCH | /changelog-entries | changelog-entries.update-many | A `changelog:write` | bulk `{ids, changes}`: `label_ids_add`, `label_ids_remove`, `status: draft`, `trashed` (max 100) |
| POST | /changelog-entries/{cle}/email-previews | changelog-entries.email-previews.store | A `changelog:read` | rendered subject, HTML, text, size, warnings (rest.md rule 11) |
| POST | /changelog-entries/{cle}/email-tests | changelog-entries.email-tests.store | A `changelog:write`, member only (not token) | send to caller's own email; 10/hour (rule 5) |
| GET | /changelog-mailings | changelog-mailings.index | A `changelog:read` | `filter[help_center,entry,kind,status]` |
| POST | /changelog-mailings | changelog-mailings.store | A `changelog:write` | email already-published entry that was not mailed `{entry_id}` (once per entry), or follow-up to skipped recipients `{entry_id, only_skipped_of: clm_}` |
| GET | /changelog-mailings/{clm} | changelog-mailings.show | A `changelog:read` | progress counts |
| PATCH | /changelog-mailings/{clm} | changelog-mailings.update | A `changelog:write` | `{status: "canceled"}` while `scheduled` or `sending` (stops remaining batches) |
| GET | /changelog-subscribers | changelog-subscribers.index | A `contacts:read` | `filter[help_center,status,source]`, `q` email prefix; `?format=csv` (admin, audit logged) |
| POST | /changelog-subscribers | changelog-subscribers.store | A `contacts:write`, `idempotent` | one subscriber; `consent: "attested"` -> active; else `"double_opt_in"` -> pending + email. `?upsert=true` match `email` |
| GET | /changelog-subscribers/{csub} | changelog-subscribers.show | A `contacts:read` | |
| PATCH | /changelog-subscribers/{csub} | changelog-subscribers.update | A `contacts:write` | `name`, `status: "unsubscribed"` (never back to active; see email.md) |
| DELETE | /changelog-subscribers/{csub} | changelog-subscribers.destroy | A `contacts:write`, admin | hard delete (GDPR); recipient rows keep counts |
| POST | /changelog-subscriber-imports | changelog-subscriber-imports.store | A `contacts:write`, admin | up to 1,000 per request, `consent: "attested"` required; computed result (not stored) |
| GET | /changelog-sources | changelog-sources.index | A `settings:read` | |
| POST | /changelog-sources | changelog-sources.store | A `settings:write`, admin | `{help_center_id, github_repository_id}` (08 repo) or `{full_name, connection_id}` (13 calls 08 `github.AddRepository`); no token, no secret: 08 repo owns webhook (`meta.webhook` from 08 only for manual mode) |
| GET | /changelog-sources/{cls} | changelog-sources.show | A `settings:read` | |
| PATCH | /changelog-sources/{cls} | changelog-sources.update | A `settings:write`, admin | `name`, `on_release`, `include_prereleases`, labels, `status: paused|active`. Token rotate: 08 `PATCH /github-connections/{ghc}`; secret rotate: 08 `PATCH /github-repositories/{ghr}` |
| DELETE | /changelog-sources/{cls} | changelog-sources.destroy | A `settings:write`, admin | soft delete; 08 repo hook events resynced (`release` dropped when no other consumer); 08 repo kept |
| POST | /changelog-sources/{cls}/tests | changelog-sources.tests.store | A `settings:write` | sync: 08 connection valid, repo reachable, permissions (08 probe), webhook status, last release |
| GET | /changelog-generations | changelog-generations.index | A `changelog:read` | |
| POST | /changelog-generations | changelog-generations.store | A `changelog:write`, `idempotent` | 202 job resource |
| GET | /changelog-generations/{cgn} | changelog-generations.show | A `changelog:read` | poll; realtime events too |
| PATCH | /changelog-generations/{cgn} | changelog-generations.update | A `changelog:write` | `{status: "canceled"}` while queued/running |

Route lint: no verbs (`publish`, `unsubscribe` never path segments; publish = `PATCH {status: "published"}`, unsubscribe pages = noun `unsubscriptions`).

Status transitions on `PATCH /changelog-entries/{cle}` (rest.md rule 3, illegal: 422 `invalid_transition`):
```
draft      -> published  {status:"published", published_at?, notify_subscribers?}   PublishChangelogEntry
draft      -> scheduled  {status:"scheduled", scheduled_for, notify_subscribers?}     ScheduleChangelogEntry
scheduled  -> scheduled  {scheduled_for}                                             reschedule
scheduled  -> draft      {status:"draft"}                                            unschedule
scheduled  -> published  {status:"published"}                                       publish now
published  -> draft      {status:"draft", reason?}                                   UnpublishChangelogEntry (cancels pending mailing)
published  -> published  content fields                                              live edit, `edited_after_publish_at` when content_hash changes
```
Republish of previously published entry keeps original `published_at` unless body sends `published_at: "now"` (then counts as new for badge and mailing). Workspace `test_mode`: mailings send only to subscribers whose email belongs to a workspace member (others `skipped` reason `test_mode`).

### Help center hosts (web, no auth)

Mounted inside 08 help center group (middleware `resolve_help_center`, `help_center_cache`, `throttle:help_center`) plus `resolve_changelog`. Base path `/changelog` on every help center host (subdomain, custom domain, path mode). Route names prefixed `hc.changelog.`.

| Method | Path (base `/changelog`) | Route name | Purpose |
|---|---|---|---|
| GET | `/changelog` | hc.changelog.index | newest first, 20 per page, labels filter bar, subscribe box |
| GET | `/changelog/page/{n}` | hc.changelog.index.page | n 2..; past last: 404 |
| GET | `/changelog/labels/{label}` | hc.changelog.labels.show | filtered list (+ `/page/{n}`) |
| GET | `/changelog/{slug}` | hc.changelog.entries.show | entry; `Accept: text/markdown` negotiation |
| GET | `/changelog/{slug}.md` | hc.changelog.entries.markdown | |
| GET | `/changelog.md` | hc.changelog.markdown | index of all public entries (title, date, labels, link to `.md`) |
| GET | `/changelog/llms-full.txt` | hc.changelog.llms-full | all entries concatenated, cap 5 MB, newest first |
| GET | `/changelog/feed.xml` | hc.changelog.rss | RSS 2.0, 50 newest, `content:encoded` |
| GET | `/changelog/labels/{label}/feed.xml` | hc.changelog.labels.rss | per-label RSS |
| POST | `/changelog/subscriptions` | hc.changelog.subscriptions.store | no-JS form: email, honeypot, timing; renders "check your inbox" (always same page, no enumeration) |
| GET | `/changelog/subscription-confirmations/{token}` | hc.changelog.subscription-confirmations.show | page with confirm button (link scanners never confirm) |
| POST | `/changelog/subscription-confirmations/{token}` | hc.changelog.subscription-confirmations.store | confirm; renders done page |
| GET | `/changelog/unsubscriptions/{token}` | hc.changelog.unsubscriptions.show | page: unsubscribe button |
| POST | `/changelog/unsubscriptions/{token}` | hc.changelog.unsubscriptions.store | RFC 8058 one-click (`List-Unsubscribe=One-Click` body) or form; 200 |
| POST | `/changelog/{slug}/reactions` | hc.changelog.entries.reactions.store | no-JS form toggle-on (ip-day identity) |
| POST | `/api/changelog/entries/{cle}/reactions` | hc.api.changelog.reactions.store | JSON for hc.js `{emoji, visitor_key}` -> counts |
| DELETE | `/api/changelog/entries/{cle}/reactions/{emoji}` | hc.api.changelog.reactions.destroy | header `Dimmi-Visitor-Key` |
| POST | `/api/changelog/subscriptions` | hc.api.changelog.subscriptions.store | JSON variant, always 202 |
| GET | `/changelog/og/{cle}.png` | hc.changelog.og | 302 to immutable asset URL (stable OG URL across regenerations) |

Exception rows for token-in-path pages added to rest.md (Required section), same shape as 04 `/email-unsubscriptions/{token}`.

### Widget `/widget/v1` (13 owns controllers, 01 route group + middleware)

Auth: `app` (app id + origin) or `widget` (visitor session). Changelog = changelog of widget's `widget_settings.help_center_id` and `widget_settings.changelog_tab_enabled` (01 column, Required). Else 404 `changelog_not_linked`.

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | /changelog | widget.changelog.show | app or widget | `{changelog, entries (20, cursor), unread_count, last_seen_at}`; anonymous: `Cache-Control: public, max-age=60` |
| GET | /changelog/entries/{cle} | widget.changelog.entries.show | app or widget | sanitized HTML, labels, reactions (own reaction state when identity sent), related articles. Deep open for SDK `show('changelog', {entry})` and 15-announcements-surveys CTA `open_changelog_entry`; not published / other help center: 404 `changelog_entry_not_found` |
| PATCH | /changelog/read-state | widget.changelog.read-state.update | widget | `{last_seen_at}` monotonic; returns `unread_count` |
| POST | /changelog/entries/{cle}/reactions | widget.changelog.entries.reactions.store | app or widget | `{emoji, visitor_key?}` (session: contact identity) |
| DELETE | /changelog/entries/{cle}/reactions/{emoji} | widget.changelog.entries.reactions.destroy | app or widget | |
| POST | /changelog/subscriptions | widget.changelog.subscriptions.store | widget | `{email?}`: verified contact email -> active (consent `verified_identity`); else pending + confirmation |

### Webhooks in

None owned by 13. GitHub deliveries arrive on 08 shared route `POST /webhooks/github/repositories/{ghr}` (`08-help-center/docs-sync.md` 4.3): 08 verifies `X-Hub-Signature-256` (`hashing.HMAC` + `hashing.Equal`), checks payload `repository.id`, dedupes `X-GitHub-Delivery` (Valkey `SET NX EX 24h` + River unique), parses compact `release` envelope (13 shape, [api.md](api.md) "GitHub release envelope"), enqueues `HandleGithubDelivery`. 13 handler `changelog` (`Events() = [release]`, actions `published`, `released`, `edited`; `Wants(repo)` = any active non-deleted changelog source on repo) enqueues `HandleGithubChangelogEvent` per source, unique `(source, delivery id)`. Durable dedupe: entry `dedupe_key` unique (Valkey flush never double-creates).

### Error codes (feature)

`changelog_disabled` (404 public, 409 API on publish), `changelog_not_linked` (404 widget), `changelog_entry_not_found` (404 widget entry: not published or other help center), `feedback_post_invalid` (422, `feedback_post_ids` unknown/other workspace/over 50), `slug_taken` (422), `slug_reserved` (422), `stale_resource` (409), `invalid_transition` (422), `scheduled_for_in_past` (422), `scheduled_for_too_far` (422), `published_at_in_future` (422), `nothing_to_publish` (409, empty title/body), `markdown_too_large` (413), `entitlement_limit_reached` (422, `key`, `limit`: `changelog.max_labels`, `changelog.max_sources`), validation 422 for per-entry max 5 labels/authors/articles (field errors), `email_required` (422, widget subscribe without email), `entry_not_published` (409, mailing for unpublished entry), `article_not_in_help_center` (422), `asset_not_in_help_center` (422), `already_mailed` (409), `mailing_not_cancelable` (409), `sending_domain_required` (423, cloud, mailing over 50 recipients from default sender), `email_sending_paused` (423, 04 profile paused), `subscriber_unsubscribed` (409, API resubscribe attempt), `consent_required` (422), `import_too_large` (413), `github_token_invalid` (422), `github_repo_not_found` (422), `github_permission_missing` (422, lists missing permission), `github_connection_required` (422, repo without connection) (all four raised by 08 GitHub layer, same names), `changelog_source_exists` (422, same repo already source of this changelog), `generation_in_progress` (409, same range running), `ai_unavailable` (409, no ready key), `spend_cap_reached` (423, 07), `human_review_required` (403, MCP publish without confirmation path), `reactions_disabled` (404), `subscriptions_disabled` (404), `invalid_emoji` (422), `test_email_rate_limited` (429), `member_required` (403, test email from token). Warnings (`meta.warnings`, non-blocking): 08 Markdown warnings (`missing_alt`, `broken_link`, `heading_skip`, `external_http_image`) plus `missing_summary`, `long_title` (over 70), `email_over_budget` (summary sent instead), `no_labels`, `cover_missing_alt`, `needs_review` (publishing agent/GitHub draft unedited), `possible_internal_link` (GitHub PR/issue links or `@mentions` in body from release notes).

## Public site

Rendering reuses 08 stack: templ layout (header, theme, brand CSS, footer), `framework/markdown` with 08 `helpmarkdown` dialect + `framework/sanitize`, `help_center_assets` URLs, page cache keyed by help center `cache_version` (L1 in-process + cross-replica bust on Valkey pubsub; 13 bumps via 08 `BumpHelpCenterCache` after commit on every public-visible change), ETag/304, CSP unchanged (no new script origins), `hc.js` extended (reactions, subscribe, under 1 KB gz added).

Pages:
- Index: H1 title, description, subscribe form (email + button, honeypot, RSS link), label filter chips (links to label pages), entries newest first by `published_at desc, id desc`: date (`<time datetime>` UTC, displayed date in workspace timezone, absolute format), labels, version tag, title (link), cover (lazy, width/height from asset), full body for first 3 entries then summary + "Read more" (keeps page under 100 KB HTML), reaction bar, authors. Pagination `/page/{n}` with `rel=prev/next`, 20 per page. Empty (no public published entries): 404 (never index empty page).
- Entry: breadcrumb (Help center > Changelog > title), date, "Updated {date}" when `edited_after_publish_at`, labels, version, cover, body, related articles ("Learn more"), reaction bar (forms; hc.js toggles), "Questions? Talk to us" (widget open when help center `show_widget`, else `contact_url`), prev/next entry, subscribe box.
- SEO: `<title>` `seo.title || title` + ` | ` + changelog title; description `seo.description || summary || plaintext 155`; canonical help center primary host + `/changelog`; `og:type article`, `article:published_time`, `article:modified_time`, `og:image` = cover > generated card > changelog OG > help center OG; Twitter `summary_large_image`. JSON-LD: entry `BlogPosting` (headline, datePublished, dateModified, image, author Person names when `show_authors` else Organization, publisher Organization); index `Blog` with `blogPost` list (first page). `<link rel="alternate">` for RSS, `.md`. `noindex` when help center `indexing_enabled = false`, entry `seo.noindex`, label pages beyond page 1 keep index (canonical self).
- Sitemap: index, label pages, entries (`lastmod` = max(published_at, edited_after_publish_at)) appended via 08 provider hook. llms.txt: section `## Changelog` with link to `/changelog.md` ("full list") and 20 newest entries `[title (YYYY-MM-DD)](.../{slug}.md): summary`. Help center home: "Latest updates" block (3 newest announced entries) via provider hook; header nav link "Changelog".
- `.md` twin: H1 title, line `Published: 2026-10-30 · Labels: New, Improved · Version: v1.4.0`, summary blockquote, body with absolute URLs, footer `Source: <canonical>`. Index `.md`: H1, description, list `- 2026-10-30 [Title](.../slug.md) (New): summary`.
- Feeds: 50 newest public published entries. Item id `urn:uuid:<entry uuid>` (stable across slug and domain changes). RSS: `<guid isPermaLink="false">`, `<pubDate>` RFC 822, `<category>` per label, `content:encoded` full sanitized HTML with absolute URLs, `<enclosure>` cover. Headers: `Cache-Control: public, max-age=300`, ETag, `Last-Modified`; 304 support. Feed reader UA counted in `feed_fetches` (not views).
- Generated OG card: PNG 1200x630, brand primary color background (contrast-checked text color), changelog title small, entry title large (max 3 lines, ellipsis), date + version, help center logo when PNG/JPEG. Rendered at publish and on title change by job, stored as help center asset `og-{cle}.png`. Title glyphs missing from embedded font (CJK, Arabic etc.): card without title text (logo + changelog name + date) instead of tofu.

Widget, page and API all render same `body_html`. Related article links resolved at render time to current public published article URL; unpublished articles silently omitted.

## Knowledge ingestion

- `PublishChangelogEntry` (and live edit, unpublish, delete, restore) calls 08 `knowledge.IndexChangelogEntry(ctx, tx, entry)` / `knowledge.RemoveChangelogEntry` in same transaction (FTS consistent at commit, embeddings queued by 08). Requested from 08.
- Document: `changelog_entry_id` set, `help_center_id`, visibility `public`, URL canonical entry URL, title `Changelog: {title} ({YYYY-MM-DD})`, content Markdown = `.md` twin body (labels, version, date line first so date sits in first chunk). `KnowledgeChunk.SourceType = "changelog"`.
- Surfaces: agent (07), Ask AI, help center search, widget Help search, public MCP all include changelog chunks (public). Help center search results show "Changelog" type label and date. `KnowledgeSearchOptions.ExcludeChangelog` lets caller drop them (e.g. 07 article-gap detection later).
- Drafts and scheduled never indexed. Unpublish/delete removes document in same tx. Help center disabled/deleted: 08 removes help center documents incl. changelog docs.
- 07 prompt guidance (requested): changelog chunks state shipped date; agent answers "added on {date}" and never claims unreleased features.

## GitHub source and generations

GitHub REST via 08 `github.ClientFor(ctx, a, ws, source.github_repository_id)` (`app/services/github`, go-github/v93, `a.HTTP.StdClient()` SSRF-safe transport, host allowlist `api.github.com`, token decrypted in memory from 08 `github_connections`, never logged, `X-GitHub-Api-Version` pinned, rate-limit snooze rules of 08 docs-sync.md 4.4). 13 stores no token, no secret.

Release webhook (`HandleGithubReleaseEvent`):
1. 08 dispatcher already verified signature, repo id match, delivery dedupe (section Webhooks in). Job re-checks source active and not deleted, repo still referenced; paused or deleted: no-op.
2. `action = published`: skip `release.draft = true` (never happens on published, guard anyway); skip prerelease unless `include_prereleases`. `action = released` handled only when same release earlier skipped as prerelease (prerelease promoted). `action = edited`: if draft entry for release exists, still `draft`, and `last_edited_by_type` is `system` (no human edit yet): refresh body; else ignore.
3. `on_release = draft`: entry draft with title = release `name` or tag, `version_tag` = tag, body = release body (Markdown passthrough), `source_kind github_release`, `source_ref {repo (full_name snapshot), release_id, tag, release_url}`, `dedupe_key github_release:{source_id}:{repo_id}:{release_id}` (per source: two changelogs on same repo each get draft), labels = `default_label_ids`, `needs_review = true`, `published_at` null. Warning `possible_internal_link` computed.
4. `on_release = draft_ai`: create generation (`github_release`, `use_ai true`) instead; generation creates draft.
5. Emits `changelog_entry.created`; realtime shows "Draft from GitHub release v1.4.0".

Generation pipeline (`RunChangelogGeneration`, queue `ai`):
- `github_range`: `GET /repos/{o}/{r}/compare/{from}...{to}` (commits; over 250 commits truncated, stat `truncated`), then closed PRs `GET /repos/{o}/{r}/pulls?state=closed&sort=updated&direction=desc&per_page=100` (max 3 pages) filtered by `merge_commit_sha` in compare set; PR title, body (first 2,000 chars), labels; max 60 PRs used: all titles, bodies only for first 30 in merge order (token budget). Bot PRs (`dependabot[bot]`, `renovate[bot]`) dropped unless only content.
- `github_release`: release body + PRs referenced by `#123` in body (max 30 fetched).
- `text`: user text (CLI sends `git log --no-merges --format=%s%n%b` output), max 64 KB.
- `use_ai = false`: deterministic `AssembleChangelogMarkdown`: group PR titles by GitHub labels mapped to changelog labels (`feature|enhancement` New, `improvement|perf` Improved, `bug|fix` Fixed, conventional commit prefixes `feat:` `fix:` `perf:` too), bullet list, no LLM.
- `use_ai = true`: 07 `ai.RunTask(ctx, a, RunTaskInput{Kind: changelog, Purpose: generate_entry, ModelRole: answer (classifier when answer key missing), StepKind: generate, System, Messages, ResponseFormat: json, JSONSchema, MaxOutputTokens, Actor})`, JSON output `{title, summary, body_markdown, label_names[], version_tag}`; system prompt `resources/prompts/changelog/<v>/draft.md` (customer-facing language, no internal jargon, no PR numbers or author handles unless instructions say, group by label, short intro, keep facts from input only, never invent features); input wrapped as untrusted data. Output validated (JSON schema, body max 64 KB, links allowed only to repo host, help center hosts, and URLs present in input; others stripped to text via 07 `linkguard`). Invalid twice: generation `failed` `invalid_output`, fallback deterministic draft offered (`stats.fallback_entry_id`).
- Result: draft entry (`source_kind generation`, `needs_review`), labels matched by name (unknown names ignored), generation `succeeded`, event `changelog_generation.completed`. Cost visible in `stats` and 07 runs list.

## Import helper (for 16)

`changelog.ImportEntry(ctx, tx, in ImportEntryInput) (entryID ids.UUID, err error)` in `app/actions/changelog/import_changelog_entry.go`. Called only by 16 `changelog` stage writer (Featurebase, Productlane, Canny), inside 16 page tx; idempotency map stays in 16 `import_records`.
- Input: `ChangelogID`, `Title`, `Slug` (collision: suffix `-2`..`-9`, then random 4 chars), `Summary?` (max 300), `BodyMarkdown` (16 converts HTML), `VersionTag?`, `LabelIDs` (16 mapping `map|create|skip`; create via `changelog.EnsureLabel`, `changelog.max_labels` cap -> mapping conflict), `CoverAssetID?` (08 `ImportAsset`), `PublishedAt?` (null or source scheduled/draft -> `draft`), `SourceRef {import_id, source, external_id, url}`.
- Writes: `source_kind import`, `announce false`, `notify_subscribers false`, `spotlight false`, `needs_review false`, `created_by_type system`, body rendered (same renderer), published rows indexed in same tx (`knowledge.IndexChangelogEntry`). Backdated `published_at` kept; `first_published_at` = `published_at`.
- Never: `changelog_entry.*` events, mailing, widget badge (`latest_announced_at` untouched), widget config bust, 14 voter mail, 06 destination post, webhooks. OG card job enqueued (internal only).
- Update on delta: same entry id, only when `last_edited_by_type = system` (untouched since import); else skipped, 16 reports `kept`.
- Rollback: `changelog.DeleteImportedEntries(ctx, tx, entryIDs)` hard deletes (knowledge documents removed in same tx via `RemoveChangelogEntry`), no events.
- Cache: 16 calls 08 `BumpHelpCenterCache` once per stage end (not per entry). `last_published_at` recomputed in helper.

## MCP tools and CLI

Authenticated MCP (09 runtime), new toolset `changelog` (enable with `?toolsets=changelog`; under 2,000 tokens).

| Tool | Ability | Risk | REST | Does |
|---|---|---|---|---|
| dimmi_list_changelog_entries | changelog:read | read | `GET /changelog-entries` | recent entries + labels list + changelog URL; filters status, label, since |
| dimmi_get_changelog_entry | changelog:read | read | `GET /changelog-entries/{cle}` | Markdown, labels, status, URLs, lock_version |
| dimmi_draft_changelog_entry | changelog:write | write | `POST /changelog-entries` | create draft (`source_kind agent`, `needs_review`); returns review URL |
| dimmi_update_changelog_entry | changelog:write | write (draft) / send (published) | `PATCH /changelog-entries/{cle}` | edit draft freely; editing published entry goes through confirm gate (public content) |
| dimmi_publish_changelog_entry | changelog:write | send | `PATCH /changelog-entries/{cle} {status: published|scheduled}` | publish now or schedule; `notify_subscribers`; always confirm gate (elicitation, then `confirm` param fallback); later `direct` mode (agent-send-modes) must never skip confirmation when mailing would be sent |

Prompt `dimmi_write_changelog_entry` (args `since` tag or date, `until` default HEAD, `audience` default customers): instructs coding agent to read `git log`/merged PRs locally, call `dimmi_list_changelog_entries` (avoid duplicates, match tone and labels), `dimmi_list_articles` to link guides, write customer-facing Markdown, call `dimmi_draft_changelog_entry`, show user review URL, publish only on explicit request via `dimmi_publish_changelog_entry`.

Public MCP (08 server on help center host): no changelog tool at MVP (later: public-mcp-changelog). `dimmi_get_doc` accepts changelog URLs (08 change). `dimmi_search_docs` results include changelog chunks with `kind: "changelog"`.

User CLI (`npx dimmi`, 09 implements, wraps REST):

| Command | REST |
|---|---|
| `dimmi changelog list [--status] [--help-center hc_]` | `GET /changelog-entries` |
| `dimmi changelog new [--title T] [--file PATH\|-] [--label L]... [--version V]` | `POST /changelog-entries` (draft) |
| `dimmi changelog draft --since REF [--until REF] [--ai] [--github cls_]` | local `git log` -> `POST /changelog-generations {kind: text}` or `{kind: github_range}` with `--github`; polls `GET /changelog-generations/{cgn}` |
| `dimmi changelog publish (ENTRY \| --file PATH) [--email\|--no-email] [--at TIME] [--yes]` | `--file`: upsert by front matter `id` or `slug` (`GET /changelog-entries?filter[slug]=`, `POST`/`PATCH`), then `PATCH {status}`; interactive confirm unless `--yes` |
| `dimmi changelog subscribers import FILE.csv --consent attested` | `POST /changelog-subscriber-imports` in chunks of 1,000 |
| `dimmi changelog subscribers export [--status active] > subs.csv` | `GET /changelog-subscribers?format=csv` |

Entry file front matter: `id`, `title`, `slug`, `summary`, `version`, `labels: [New]`, `cover: ./img.png` (uploaded via `POST /help-centers/{hc}/assets`), `authors: [ada@acme.com]`, `publish_at`, `email: true`, `announce`, `spotlight`.

Server commands (artisan style): `dimmi changelog:rerender`, `dimmi changelog:publish-due` (manual sweeper run).

## Events

New events (add to catalog):

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| changelog_entry.created | 13 | yes (workspace) | yes | any source; `source_kind` |
| changelog_entry.updated | 13 | yes | yes | content or flags changed; `published: bool` |
| changelog_entry.scheduled | 13 | yes | no | `scheduled_for` |
| changelog_entry.published | 13 | yes | yes | `{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids[], notify_voters}`; `feedback_post_ids` = 14 posts linked to entry (max 50), `notify_voters` default true when posts linked; feeds 06 destinations, 14 voter mail |
| changelog_entry.unpublished | 13 | yes | yes | reason `reverted_to_draft`, `deleted`, `help_center_deleted` |
| changelog_entry.deleted | 13 | yes | yes | trash `purged: false`; hard purge `purged: true` (14 FK set null on purge) |
| changelog.updated | 13 | yes | no | settings |
| changelog_mailing.updated | 13 | yes | no | progress (coalesced 1/s) |
| changelog_mailing.completed | 13 | yes | yes | final status + counts |
| changelog_subscriber.subscribed | 13 | yes | yes | became active (email in payload, PII; same class as `contact.created`) |
| changelog_subscriber.unsubscribed | 13 | yes | yes | reason |
| changelog_generation.completed | 13 | yes | no | entry id, cost |
| changelog_generation.failed | 13 | yes | no | error code |
| changelog_source.errored | 13 | yes | no | error code; 06 `system_alert` |

Realtime channel `workspace:ws_...` (centrifuge namespace on Valkey broker, history + recovery; `00-foundation/events-and-realtime.md` Channels). Payload Resource JSON (entry resource without `body_markdown`/`body_html` when over 10 KB per foundation rule).

Consumed:

| Event / hook | Listener / handler | Queue |
|---|---|---|
| workspace initializer hook | `EnsureChangelog` for default help center | inside `CreateWorkspace` tx, after 08 `CreateDefaultHelpCenter` |
| 04 `email_suppression.created {outbound_email_id, outbound_kind}` | `SyncSubscriberSuppression` (`outbound_kind = changelog`: complaint scope `changelog` -> `complained`; any hard bounce scope `all` matched by email -> `bounced`; mailing counters via `outbound_email_id`) | `mail` (queued) |
| 08 help center delete/disable hook (not event) | `changelog.OnHelpCenterDeleted` / `OnHelpCenterRestored`: cancel scheduled mailings, past-due scheduled entries to draft on restore (knowledge removal done by 08 bulk removal) | in 08 tx |
| 03 `contact.merged` | `MergeChangelogReads` (GREATEST `last_seen_at` onto target, delete source row; re-point subscriber `contact_id`, reactions `contact_id`) | `default` (queued) |
| `article.unpublished`, `article.deleted` | none (related links resolved at render) | |
| `member.deactivated` / `user.deleted` | none (author shown by name; deleted user: "Former teammate", avatar hidden) | |
| `system.app_url_changed` (11) | none extra (08 rebuild covers help center cache; feeds absolute URLs from cache_version) | |
| contact eraser registry | `EraseChangelogContactData` | inside 03 erase job |

## Jobs

| Job (kind) | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| PublishScheduledChangelogEntry (`publish_scheduled_changelog_entry`) | default | inserted with `ScheduledAt = scheduled_for` | 5 | unique (entry, scheduled_for) |
| SweepScheduledChangelogEntries (`sweep_scheduled_changelog_entries`) | maintenance | periodic 1 min: due `scheduled` entries without job | 0 | periodic |
| StartChangelogMailing (`start_changelog_mailing`) | mail | `ScheduledAt = send_after` | 5 | unique per mailing |
| SendChangelogMailingBatch (`send_changelog_mailing_batch`) | mail | start + chain (100 recipients each, workspace rate `changelog.email.rate_per_second`) | 8, honors `*mail.RateLimitedError.RetryAfter` | unique (mailing, batch cursor) |
| FinishChangelogMailing (`finish_changelog_mailing`) | mail | last batch or watchdog 30 min no progress | 5 | unique per mailing |
| SendChangelogSubscriptionConfirmation (`send_changelog_subscription_confirmation`) | mail | subscribe | 5 | unique per subscriber per 10 min |
| RunChangelogGeneration (`run_changelog_generation`) | ai | generation created | 2 (provider retries inside 07 llmretry) | unique per generation; per workspace concurrency 2; create refuses same `(changelog, input hash)` while queued/running |
| HandleGithubChangelogEvent (`handle_github_changelog_event`) | default | 13 `github.WebhookHandler` `changelog` from 08 `HandleGithubDelivery` (08 route responds 202 under 1 s) | 5 | unique (source, delivery id) |
| PollGithubReleases (`poll_github_releases`) | default | periodic 15 min: active sources whose 08 repo `webhook_status` in (`none`,`pending`,`failing`); `GET /repos/{o}/{r}/releases?per_page=10`, new published releases -> HandleGithubChangelogEvent path | 3 | periodic; unique per source |
| RenderChangelogOgImage (`render_changelog_og_image`) | default | publish, title change, theme change | 3 | unique (entry, content key) |
| RerenderChangelogEntries (`rerender_changelog_entries`) | maintenance | renderer_version bump at boot, command | 3 | unique per workspace |
| PurgeChangelogData (`purge_changelog_data`) | maintenance | daily: trash over 30 days, pending subscribers unconfirmed over 7 days (deleted), mailing recipients over 400 days, generations over 90 days, stats over 400 days, reactions of purged entries | 3 | periodic |
| FlushChangelogStats | in-process ticker 30 s (08 stats buffer) | | keep buffer on failure | |

## Config keys

`config/changelog.go` (env `DIMMI_CHANGELOG__*`):

| Key | Default | Notes |
|---|---|---|
| changelog.entries_per_page | 20 | public index |
| changelog.feed_items | 50 | |
| changelog.entry_max_kb | 128 | |
| changelog.scheduled_max_days | 365 | |
| changelog.unread_window_days | 30 | widget unread counts only entries newer than this |
| changelog.unread_cap | 9 | badge shows `9+` |
| changelog.spotlight_days | 14 | |
| changelog.reactions.per_ip_per_minute | 30 | Valkey limiter |
| changelog.subscribe.per_ip_per_hour | 10 | page + widget; Valkey limiter |
| changelog.subscribe.confirmations_per_day | 3 | per address |
| changelog.subscribe.pending_ttl_days | 7 | unconfirmed rows purged |
| changelog.email.rate_per_second | 20 | per workspace, Valkey limiter; instance provider rate shared (04 driver backs off) |
| changelog.email.batch_size | 100 | |
| changelog.email.html_budget_kb | 90 | |
| changelog.email.test_per_hour | 10 | per member |
| changelog.generation.max_prs | 60 | |
| changelog.generation.max_commits | 250 | GitHub compare limit |
| changelog.generation.max_output_tokens | 2000 | |
| changelog.import.max_per_request | 1000 | |

Entitlement catalog keys (owner 13, `Entitlements.Limit`, 12 cloud driver; over limit 422 `entitlement_limit_reached` with `key`, `limit`): `changelog.max_labels` (20 per changelog, both editions), `changelog.max_sources` (5 per changelog, both editions), `changelog.email.monthly_recipient_cap` (cloud 10,000 recipients/month; cost guard, staff raise per customer), `changelog.email.default_sender_max_recipients` (cloud 50, self-host -1; over: mailing `blocked` `sending_domain_required`). Not config: cloud caps live in entitlement catalog (`extensibility.md` section 6).

## Permissions matrix

| Action | Owner | Admin | Member | Token ability |
|---|---|---|---|---|
| View changelog settings, entries, labels, mailings, generations, stats | yes | yes | yes | changelog:read |
| Create/edit drafts, labels, generations | yes | yes | yes | changelog:write |
| Publish, schedule, unpublish, delete, restore, send/cancel mailing, test email | yes | yes | yes | changelog:write (test email: session/app only) |
| Changelog settings (title, toggles, email from, delay) | yes | yes | no | settings:write |
| GitHub sources create/update/delete/test | yes | yes | no | settings:write (create by `full_name` also needs 08 `github:manage`; connections/repos managed in 08) |
| View subscribers | yes | yes | yes | contacts:read |
| Add/update/unsubscribe subscriber | yes | yes | yes | contacts:write |
| Import, export CSV, hard delete subscribers | yes | yes | no | contacts:write (+ admin role ceiling) |
| MCP publish | yes | yes | yes | changelog:write + confirm gate |

Deactivated member: none. Token capped by owner role (00-accounts).

## Performance budgets and mechanics

- Body HTML precomputed on save; index renders from cached list per `(help_center_id, cache_version, page, label)` in 08 page cache. Hit path: zero DB queries except host map.
- Widget list: query `(workspace_id, changelog_id, status, published_at desc, id desc)` limit 21; anonymous responses `max-age=60` CDN-cacheable; unread count = `count(*) ... where published_at > greatest(last_seen_at, @now - window) and announce limit 10` (bounded; `@now` from `a.Clock`, never SQL `now()`).
- Reactions: upsert counts row + reaction row in one short tx; no entry row lock. p95 under 20 ms. Rate limit check on Valkey first (one round trip); dedupe = Postgres PK (durable).
- Mailing: recipients snapshot `INSERT ... SELECT` in chunks of 5,000; batches of 100 via 04 `SendChangelogEmail`; counters `UPDATE ... SET sent_count = sent_count + n` per batch.
- Feeds: built from same cached list; 50 items under 1 MB.
- Benchmarks (`tests/bench`): 2,000 entries changelog index cached/cold, widget list p95, 10,000-recipient mailing captured by `tc.Mail` under 60 s wall (excluding provider rate), reaction burst 200/s single entry.

## Observability

Metrics: `dimmi_changelog_requests_total{route,status,cache}`, `dimmi_changelog_publish_total{source_kind,scheduled}`, `dimmi_changelog_schedule_lag_seconds` (histogram publish time minus scheduled_for), `dimmi_changelog_mailing_recipients_total{result}` (sent, failed, skipped, bounced, complained), `dimmi_changelog_mailing_duration_seconds`, `dimmi_changelog_subscriptions_total{source,result}` (pending, confirmed, rejected_bot), `dimmi_changelog_unsubscribes_total{reason}`, `dimmi_changelog_generations_total{kind,ai,result}`, `dimmi_changelog_github_events_total{action,result}`, `dimmi_changelog_reactions_total{surface}`, `dimmi_changelog_feed_fetches_total{format}`.
Logs: publish (entry id, source, scheduled lag), mailing summary (counts, duration, blocked reason), generation summary (tokens, cost, PR count), GitHub event result. Never log subscriber emails at info (debug redacted `a***@acme.com`), tokens never, entry bodies never.
`dimmi doctor` adds: overdue scheduled entries (should be 0), mailings stuck in `sending` over 30 min, sending domain verified when subscribers over 50 (cloud).

## Security and privacy

- Drafts, scheduled, trashed entries never public: HTML, `.md`, feeds, sitemap, llms.txt, widget, public MCP, knowledge, OG endpoint. 404 (not 410: may return). Leak test with marker strings (extends 08 leak suite).
- Scheduled publish atomic: page cache bust after commit; feeds and widget read only `status = published`.
- Markdown same `framework/sanitize` policy as 08; GitHub release bodies and AI output are untrusted (prompt injection in PR bodies): only drafts, human publishes; link guard on AI output.
- Subscribe abuse (list bombing: attacker subscribes victims to flood confirmations): per IP limit, honeypot, minimum 2 s form timing, max 3 confirmations per address per 24 h, confirmation email minimal (no user-supplied text except changelog name), pending rows expire 7 days. Confirmation and subscribe responses identical for existing/new addresses (no enumeration).
- Double opt-in confirm needs POST (GET page only): link scanners (Microsoft Safe Links, Mimecast) cannot confirm or unsubscribe. Exception: RFC 8058 one-click POST from mailbox provider.
- Tokens: HMAC-SHA256 over `{v, workspace_id, subscriber_id, purpose (confirm|unsubscribe), token_nonce}` via `hashing.HMAC`, key derived from app key (`framework/encryption` HKDF info `changelog-subscriber-v1`), base64url; no expiry for unsubscribe (law: must work), confirm token valid 7 days. Path tokens redacted in access logs (route-level redaction like 04).
- Reactions: hashed identities only, no IP or client key stored; counts are approximate by design (no login); bot UAs rejected; per-IP limits.
- Email: customer stream only via 04 `SendContactMail` (wrapper `SendChangelogEmail`), instance provider; List-Unsubscribe + One-Click headers, `List-Id: <{clg}.changelog.{host}>`, `Auto-Submitted: auto-generated`, `Precedence: bulk`; From verified domain (cloud above 50 recipients); suppressions (`all`, `changelog`) checked per send by 04.
- GitHub tokens and webhook secrets: never in 13 rows. Owned by 08 GitHub layer (`github_connections` `encryption.Sealed`, last 4 shown; per-repo webhook secret, constant-time compare, `repository.id` check). 13 calls `github.ClientFor` (workspace-scoped; other workspace or deleted repo `ErrNotFound`, 08 E203).
- AI generation only with workspace BYOK key, counted in 07 spend cap; never Dimmi-paid.
- Privacy: no cookies on public pages (reaction client key in localStorage only after user reacts), no IP stored, no open pixels, no click redirects. Subscriber `consent_evidence` stores /24 IP prefix + UA (GDPR proof) and is erased with subscriber.
- GDPR: eraser `EraseChangelogContactData(workspace, contact)`: deletes `changelog_subscribers` where `contact_id` = contact OR email in contact's emails (contact email + email identities); deletes `changelog_reads`; deletes `changelog_reactions` with `contact_id` (counts decremented); recipient rows keep `subscriber_id` null. Subscriber hard delete via API does same for one row. Subscribers export via contact export (03) includes changelog subscriptions (Required: 03 export hook, optional).
- Audit log: changelog settings, source create/update/delete (token changes audited by 08), subscriber import/export/delete, mailing cancel, publish/unpublish by token or MCP.

## Metering impact

None billable (`research/pricing.md` section 2, `12-cloud-billing/metering-rules.md`):
- Changelog pages, feeds, widget tab, reactions, subscriptions, mailings: no conversation, D1 never activates.
- Reply to changelog email: no 13 handling (later: changelog-replies-to-inbox); reaches From address, 04 treats like any inbound mail there (D1 applies if it becomes conversation). Auto-replies to changelog mail discarded (04), never conversations.
- AI generations: BYOK tokens via 07 (`ai_runs.kind changelog`), counted in 07 monthly AI spend cap, D6 spirit (no Dimmi AI fee).
- Cloud cost guard (not billing): entitlement `changelog.email.monthly_recipient_cap` 10,000 recipients/month, staff raise per customer; enforced by 13 at mailing start (mailing `blocked` `monthly_cap_reached`, not an HTTP error), not in ledger.
- 12 replay fixture `changelog_reply_billing`: publish + mailing + reactions + subscribe = zero ledger lines.

## Dependencies on other specs

| Spec | Need |
|---|---|
| 00-foundation / framework | `framework/ids`, `filesystem` (`a.Storage`), `cache` + `ratelimit` + `pubsub` on Valkey (required in every deployment), `httpclient` (`a.HTTP`), `sanitize`, `markdown`, `hashing`, `encryption`, `routing`, `clock`, `broadcasting` (centrifuge), framework `view/feed` + `image` (catalog additions), RLS, idempotency, entitlements, workspace initializer hook, contact eraser registry, event catalog |
| 00-accounts | roles, tokens + abilities, audit log, member names/avatars |
| 01 | widget route group + middleware, `widget_settings.help_center_id`, new `changelog_tab_enabled`, config + session payload additions, SDK `show('changelog', {entry})`, `changelogUnread`, `changelog_unread_changed` (sdk-contract.md, defined) |
| 03 | contact eraser registry, contact export hook (optional) |
| 04 | `SendContactMail` (`Kind: changelog`, `Purpose: entry|confirmation`) + wrapper `SendChangelogEmail` (outbound.md 16, defined), suppression scope `changelog`, `HasVerifiedSendingDomain`, `IsSuppressed(ctx, ws, email, scopes...)`, `email_suppression.created {outbound_kind}`, auto-reply discard for kind `changelog` (pipeline.md 7b), caller `MailtoToken` routing (open) |
| 06 | destination trigger for published entries, `system_alert` |
| 07 | `ai.RunTask` (kind `changelog`, purpose `generate_entry`; defined), run kind `changelog`, prompt guidance for dated changelog chunks, linkguard |
| 08 | host, page cache, renderer, assets, knowledge indexing of entries, provider hooks (nav, home, sitemap, llms.txt), public MCP `dimmi_get_doc` changelog URLs, shared GitHub layer (`github_repositories` FK, `github.ClientFor`, `RegisterWebhookHandler`, shared route `/webhooks/github/repositories/{ghr}`, `github.AddRepository`), `ImportAsset` for 16 covers |
| 09 | MCP toolset + prompt, confirm gate rule, CLI commands, OAuth scope mapping |
| 10 | webhook catalog registry entries |
| 11 | none beyond config; `dimmi doctor` checks registered via provider |
| 12 | confirm not metered (replay fixture `changelog_reply_billing`); entitlement keys in cloud driver catalog |
| 16 | calls `changelog.ImportEntry` (Featurebase, Productlane, Canny changelogs) |
| 14 | `feedback.LinkChangelogEntry`, `feedback.ShippedRecipientContactIDs` (read), voter mail on `changelog_entry.published`, FK `feedback_posts.changelog_entry_id` (14 migration) |
| 15-announcements-surveys | consumes `changelog.FindPublishedEntry` + widget entry route for CTA `open_changelog_entry`; segments (adopt later) |

## Required from other specs

Exact edits for owners to apply. 13 is read-only on these folders.

### 00-foundation

1. `data-model.md` ownership map row: `changelogs, changelog_labels, changelog_entries, changelog_entry_label, changelog_entry_author, changelog_entry_article, changelog_entry_reaction_counts, changelog_reactions, changelog_reads, changelog_subscribers, changelog_mailings, changelog_mailing_recipients, changelog_sources, changelog_generations, changelog_entry_stats_daily, changelog_stats_daily | 13-changelog`.
2. `data-model.md` prefix registry rows (code: `app/models/public_ids.go`): `clg_` changelogs, `cle_` changelog_entries, `cll_` changelog_labels, `csub_` changelog_subscribers, `clm_` changelog_mailings, `cls_` changelog_sources, `cgn_` changelog_generations.
3. `data-model.md` migration ranges: 13 uses `20261101001301-001399`.
4. `events-and-realtime.md` "Events added by feature specs" row 13: `changelog_entry.created, changelog_entry.updated, changelog_entry.scheduled, changelog_entry.published, changelog_entry.unpublished, changelog_entry.deleted, changelog.updated, changelog_mailing.updated, changelog_mailing.completed, changelog_subscriber.subscribed, changelog_subscriber.unsubscribed, changelog_generation.completed, changelog_generation.failed, changelog_source.errored` (flags in 13 README Events).
5. `api.md` abilities list: add `changelog:read`, `changelog:write`. Endpoint ownership row: `changelog, changelog entries, labels, subscribers, mailings, sources, generations | 13-changelog`.
6. `rest.md` exceptions table: web pages `/changelog/subscription-confirmations/{token}` and `/changelog/unsubscriptions/{token}` (help center host): token in path allowed (double opt-in link and RFC 8058 one-click), GET renders page only, POST acts. No 13 webhook route: old `/webhooks/github/changelog/{cls}` replaced by 08 shared `/webhooks/github/repositories/{ghr}` (08 docs-sync.md 16 item 5).
7. `contracts.md` workspace initializer list: append `13 changelog for default help center (EnsureChangelog, after 08)`.
8. `contracts.md` contact eraser hooks: append `13: changelog subscribers (by contact_id and contact emails), reads, reactions (EraseChangelogContactData)`.
9. `contracts.md` Knowledge: `KnowledgeChunk.SourceType` adds `changelog`; `KnowledgeChunk.ChangelogEntryID *ids.UUID`; `KnowledgeSearchOptions.ExcludeChangelog bool`; `Surface` unchanged. `KnowledgeDocument` gets `ChangelogEntryID *ids.UUID` (`framework/ids`).
10. `security.md` "Privacy and GDPR": add line "Bulk email (changelog 13): double opt-in, RFC 8058 one-click, no tracking pixels".

### 01-messenger-widget

1. `widget_settings` column `changelog_tab_enabled boolean not null default false` (requires `help_center_id`), migration in 01 block (e.g. `20261101000107_add_changelog_tab_enabled_to_widget_settings_table.sql`). Settings API accepts it.
2. Widget config `features.changelog`: `{enabled, title, latest_announced_at, spotlight: {id, title, summary, cover_url, published_at} | null}` (from 13 action `changelog.WidgetConfigFragment`; config cache busted on `changelog_entry.published/unpublished` via `config_version` bump or changelog `latest_announced_at` in ETag).
3. `GET /widget/v1/session` resume payload: `changelog: {last_seen_at, unread_count}` when tab enabled.
4. Route table row: `/widget/v1/changelog*` owned by 13 (shares group + middleware, like Help routes).
5. SDK contract (sdk-contract.md, already defined there; listed for completeness): `show('changelog', {entry?: 'cle_...'})` (unknown/unpublished entry: open list + `console.warn` `view_unavailable` reason `changelog_entry_not_found`, sdk-contract console codes), property `changelogUnread`, event `changelog_unread_changed {count}`, launcher badge rule (dot when unread > 0), deep link `#dimmi-open=changelog`, anonymous unread cursor in localStorage `dimmi:{app}:changelog_seen`.
6. Lazy chunk "What's new" under 8 KB gz.
7. Out table: remove row "News / changelog space in widget" (old post-MVP changelog item; ships in 13).
8. Withdrawn (later: changelog-replies-to-inbox): `conversations.source.changelog_entry_id` widget key and `startConversation(..., {context: {changelog_entry_id}})` "Send feedback" use. 01 drops if added.

### 03-shared-inbox

1. Contact eraser registry: 13 eraser registered (no code change in 03 besides registration order note).
2. Withdrawn (later: changelog-replies-to-inbox): `conversations.source.changelog_entry_id` and sidebar "About changelog: {title}" link. 03 drops if added.
3. Optional: contact export includes changelog subscriptions via export hook (`ExportContributor`), else skip.

### 04-email-channel

Done by 04 (final contact mail model, `04-email-channel/outbound.md` sections 4, 10, 13, 15, 16; pipeline.md 7b; README `outbound_emails.kind` 223, `email_suppressions.scope` 244, `email_sending_stats` 260): `email.SendContactMail(ctx, a, SendContactMailInput{Kind: changelog, Purpose: entry|confirmation (free text, 04 metadata only), Unsubscribe{URL, Post, MailtoToken}, ListID, Tags (max 5), ReplyTo, IdempotencyKey, Automated, ...})` -> `SendContactMailResult{Status, SkipReason, OutboundEmailID, ProviderMessageID}`; wrapper `email.SendChangelogEmail` (skip `suppressed` -> `email.ErrSuppressed`, `paused` -> `email.ErrSendingPaused`); `Precedence: bulk` on kind `changelog`; `outbound_emails.kind changelog` + `metadata.purpose` / `metadata.tags`; suppression scope `changelog` (complaint blocks changelog only; hard bounce `all`); pause + caps per kind; stats bucket `changelog`; `email_suppression.created {outbound_email_id, outbound_kind}`; contact-mail reply handling (auto-replies to kind `changelog` discarded; human replies: 13 no longer asks for `source.changelog_entry_id` tagging, see asks below); cloud default sender allowed, 13 enforces 50-recipient entitlement via `HasVerifiedSendingDomain`; `IsSuppressed(ctx, ws, email, scopes...)`; Out row "Outbound campaigns, broadcasts" updated.

Also done by 04: mailto unsubscribe prefix registry `email.RegisterMailtoUnsubscribe(prefix, handler)` (outbound.md 10, pipeline.md). 13 registers prefix `cunsub` at boot -> `UnsubscribeFromChangelog` (reason `mailto`); `MailtoToken cunsub-<token>` in `List-Unsubscribe` mailto recognized before classification, message discarded.

Remaining asks:
1. No Purpose `digest` (later: changelog-digest; 04 Purpose is free text, nothing to drop) and no human-reply tagging `source.changelog_entry_id` from `metadata.tags.entry` (later: changelog-replies-to-inbox). 13 sends no `ReplyTo`. Auto-reply discard stays.

### 06-notifications

1. Workspace destination triggers (`notification_destinations.triggers` allowed list) add `changelog_entry_published`: posts title, labels, summary, link to Slack channel (community announcements; Discord destination later: discord-notifications). Source event `changelog_entry.published`, condition `announce = true`. Not a member trigger.
2. `system_alert` sources: `changelog_source.errored`, mailing `blocked`/`failed`, scheduled publish failed (to owners/admins).

### 07-dimmi-agent

Done by 07 (`07-dimmi-agent/README.md` task action, `ai_runs.kind`): `ai.RunTask(ctx, a, RunTaskInput{WorkspaceID, Kind: changelog, Purpose: generate_entry, ModelRole, StepKind, System, Messages, ResponseFormat, JSONSchema, MaxOutputTokens, Redact, Actor})` replaces requested `ai.CompleteTask` (key resolution incl. classifier fallback, readiness of key only, `PreflightSpend`, retries, `ai_runs` kind `changelog` + step, `RecordAiSpend`; errors `ai_unavailable`, `spend_cap_reached`, `provider_error`, `invalid_output`); `ai_runs.kind` CHECK has `changelog`; `ai.WrapUntrusted` for release/PR text; `linkguard` exported; changelog chunk date guidance; readiness `HasContent` via 08 status. No open asks.

### 08-help-center

Applied in 08 sweep (2026-11-02); listed for traceability. Column 1 lands via 13 migration `20261101001314_add_changelog_entry_id_to_knowledge_documents_table.sql` (documented in 08). Item 13 added: shared `app/services/github` adapter (go-github/v93) owned by 08, 13 adds releases/compare/pulls files.

1. `knowledge_documents.changelog_entry_id uuid null references changelog_entries on delete cascade`, unique where not null; CHECK exactly one of `source_id`, `article_id`, `changelog_entry_id`. Migration in 08 block after 13 table exists, or 13 migration `001314` adds it (owners decide; FK needs 13 table first: recommend 13 migration `20261101001314_add_changelog_entry_id_to_knowledge_documents_table.sql` documented in 08).
2. Knowledge actions exported: `knowledge.IndexChangelogEntry(ctx, tx, entry)`, `knowledge.RemoveChangelogEntry(ctx, tx, entryID)` (same tx as publish; embeddings queued). Chunk `SourceType changelog`, title `Changelog: {title} ({date})`. Search option `ExcludeChangelog`. Public scope (`ExcludeSources`) keeps changelog. Help center delete/disable removes changelog documents with article documents.
3. Withdrawn (later: changelog-custom-domain): `help_center_domains.purpose`, changelog-purpose host routing, `POST /help-centers/{hc}/domains` `purpose`. 08 drops them; changelog always at help center host `/changelog`.
4. Provider registry `helpcenter.PublicSectionProvider` (`NavLinks`, `HomeSections`, `SitemapURLs`, `LlmsTxtSection`, `SearchResultKind`) iterated by 08 templates/builders; 13 registers one. llms.txt gains `## Changelog` section; sitemap includes changelog URLs; home shows "Latest updates".
5. `help_center_assets`: in-use check (`asset_in_use` 409) also checks `changelog_entries.cover_asset_id`, `og_image_asset_id`, changelog body references; `referenced_at` set by 13. Generated OG PNGs stored as assets (`uploaded_by_type = system`).
6. Public MCP: no 13 tool (`dimmi_list_updates` withdrawn, later: public-mcp-changelog); `dimmi_get_doc` resolves changelog entry URLs/slugs (`changelog/{slug}`); `dimmi_search_docs` result items include `kind` (`article` | `changelog`).
7. `hc.js`: reactions + subscribe handlers (13 provides module, 08 bundles; budget 5 KB total holds).
8. Withdrawn: `knowledge.ResolveGitCredentials` (removed from 08 exports). Replaced by 08 shared GitHub layer (docs-sync.md section 4, 16 items for 13 applied here): `changelog_sources.github_repository_id` FK `on delete restrict`, `github.ClientFor`, `github.RegisterWebhookHandler` (13 handler `changelog`, events `release`), shared route `/webhooks/github/repositories/{ghr}`, envelope `release` in 13 shape ([api.md](api.md) "GitHub release envelope"), repo delete 409 `github_repository_in_use` lists changelog sources, token permissions row for 13 (Metadata, Contents, Pull requests read; Webhooks write optional). Edge cases on token/secret: 08 E209-E216.
9. Search page: changelog results labeled with date. Ask AI unchanged (gets changelog via knowledge).
10. Edge cases: help center subdomain change, disable, delete apply to changelog routes identically (no 08 text change beyond mention).
11. `DeleteHelpCenter` / restore call `changelog.OnHelpCenterDeleted(ctx, tx, hc)` / `OnHelpCenterRestored` in same tx (registered hook, 08 does not import 13 types).
12. Sitemap index: changelog URLs in child sitemap `/sitemaps/changelog-{n}.xml` when split.

### 09-agent-native

1. Toolset `changelog` with 5 tools (13 README table) and prompt `dimmi_write_changelog_entry`; add to toolset list in `mcp.md` "Feature toolsets".
2. Confirm gate: `dimmi_publish_changelog_entry` and `dimmi_update_changelog_entry` on published entries declared risk `send`; gate rule addition: "changelog publish with `notify_subscribers` always confirms" (MVP confirms every send; holds once later `direct` mode ships: agent-send-modes) (mass email egress, same class as webhook URL change).
3. CLI `dimmi changelog list|new|draft|publish|subscribers import|subscribers export` (13 README map) in `cli.md` command list + REST map; front matter format.
4. OAuth scope mapping: scopes `changelog:read`, `changelog:write` mirror abilities; consent screen strings.
5. `llms.txt` for Dimmi docs (cli.md section 5): add "Changelog" line.

### 10-webhooks

1. `webhook_catalog.go` registry entries (type, object, description, fixture) for webhook=yes 13 events: `changelog_entry.created|updated|published|unpublished|deleted` (object `changelog_entry`, body Markdown excluded over 10 KB, `links.self`), `changelog_mailing.completed` (`changelog_mailing`), `changelog_subscriber.subscribed|unsubscribed` (`changelog_subscriber`).
2. Event types table row in README: `changelog_entry.*`, `changelog_mailing.completed`, `changelog_subscriber.*`.

### 12-cloud-billing

1. Metering rules: no change to D1-D15; `metering-rules.md` "Not activity": changelog mailings, reactions, subscriptions (no conversation). Replay fixture `changelog_reply_billing` (zero lines; reply clause dropped with later: changelog-replies-to-inbox).
2. Entitlement catalog (cloud driver) references 13 keys: `changelog.max_labels` (20), `changelog.max_sources` (5), `changelog.email.monthly_recipient_cap` (10,000/month, cost guard, raise on request; not ledgered), `changelog.email.default_sender_max_recipients` (50). Pricing page line: "Changelog, feedback board and roadmap, announcements and surveys included".

### 14-feedback-roadmap

1. Action `feedback.LinkChangelogEntry(ctx, tx, workspaceID, entryID, postIDs)`: sets/clears `feedback_posts.changelog_entry_id` (14 column, FK inline in 14 migration `20261101001406`, `on delete set null`); validates posts same workspace; max 50.
2. Read action `feedback.ShippedRecipientContactIDs(ctx, workspaceID, entryID) ([]ids.UUID, error)`: contacts 14 notifies (or notified) for posts linked to entry; empty when `notify_voters = false`.
3. 14 consumes `changelog_entry.published {feedback_post_ids, notify_voters}` (voter mail, status move to shipped per 14 rules) and `changelog_entry.deleted {purged}`.
4. Entry editor candidate list: `GET /feedback-posts?filter[status_category]=active,done` (14 REST).

### 15-announcements-surveys

1. 13 provides (no 15 edit needed beyond reference): `changelog.FindPublishedEntry(ctx, workspaceID, entryID) (Entry, bool, error)` for CTA `open_changelog_entry` validation; widget route `GET /widget/v1/changelog/entries/{cle}`; SDK `show('changelog', {entry})` opens it.
2. `spotlight` card stays inside Messenger; not counted by 15 throttle.
3. Segments (`seg_`) available for entry/email audience later (`changelogs.audience` reserved).

### 16-importers

1. 13 provides (no 16 edit needed beyond reference): `changelog.ImportEntry`, `changelog.EnsureLabel`, `changelog.DeleteImportedEntries` (section Import helper). Mirrors 16 README Required 13 item 2.

### research/roadmap.md, specs/README.md

Applied by 13 (allowed): launch item 13, post-MVP list renumbered, folder row, build order. Spec refs use named post-MVP items ("post-MVP: Linear"), never numbers.

## Open questions (recommended default)

1. Monthly email recipient cap on cloud: default 10,000/month free (SES cost about $1), raise on request; revisit paid overage after launch data.
2. Auto-subscribe identified widget users (Featurebase default)? Default: no. Explicit consent only; API import with attestation for owners who have consent.
3. Comments on entries? Default: no at MVP.
4. Should help center search show changelog results by default? Default: yes, labeled; owner cannot toggle at MVP.
5. Emoji set fixed (`thumbs_up, heart, tada, eyes, thumbs_down`)? Default: yes, not configurable.

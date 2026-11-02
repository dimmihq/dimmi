# 14 Feedback board and public roadmap

Style: caveman. Date 2026-11-02. Owner: 14-feedback-roadmap. Backend only. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*` (incl. `extensibility.md`), `specs/00-foundation/rest.md` (binding).

Files in folder:

| File | What |
|---|---|
| README.md | this: scope, data model, code map, API table, events, jobs, config, permissions, budgets, required edits in other specs |
| [api.md](api.md) | request/response JSON for key endpoints, portal and widget routes, error codes, webhook payloads |
| [portal.md](portal.md) | portal on help center host: identity model (contacts, portal sessions, SSO, email sign-in, widget hand-off), SSR pages, caching, SEO, widget tab |
| [loop.md](loop.md) | subscriptions, post updates (undo window), voter notifications (email + widget feed), unsubscribe, changelog link, notify-on-ship |
| [ai.md](ai.md) | BYOK AI: duplicate detection, request capture from conversations, prompts, cost guards |
| [edge-cases.md](edge-cases.md) | decided behavior per case (152) |
| [tasks.md](tasks.md) | build checklist, demo script |

## Goal

Customers ask for things in chat, email, widget and portal. Every ask lands on one post with votes, quotes and revenue behind it. Team sees what matters, plans it on public roadmap, ships it, and every requester hears back automatically. Loop closes without spreadsheet.

## Why

- Product owner rule: "anything we use ourselves as business is MVP, so we can showcase features." Dimmi runs its own feature board and roadmap on Dimmi from day one (`feedback.trydimmi.com`, `/roadmap`).
- `research/roadmap.md` post-MVP item "feature request capture and notify-on-ship" ("Feature request capture and notify-on-ship. Agent detects request in conversation, links to list, notifies when shipped. Public board and roadmap optional") and "Not at launch: feedback portal, roadmap". Pulled into MVP by product owner decision 2026-11-02 (orchestrator updates roadmap).
- Competitors: Featurebase moat is feedback loop (conversations auto-linked to requests, MRR-weighted prioritization, auto email when shipped; `research/competitors/featurebase.md`). Productlane: requests portal, upvote with written context and importance, roadmap mirrors real statuses, critics say "votes plus volume, no revenue weighting" (`research/competitors/productlane.md`). Canny: statuses Under review > Planned > Shipped, voters emailed on release, MRR weighting. Linear Customer Requests: demand filtered by revenue, size, tier. Featurebase anonymous voters never notified (help.featurebase.app). We ship all of it free, self-hostable, inside the support tool, so capture from conversations is native, not an integration.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Inbox message to linked request (member clicks "Link to feedback", picks post) | 1 API call, p95 under 80 ms, vote + subscription + quote in same tx |
| Similar posts while typing title (portal, widget, dashboard) | p95 under 50 ms server, 100k posts per workspace (FTS + trigram, no AI) |
| Vote write | p95 under 30 ms; counts visible in next read (same tx) |
| Board list API | p95 under 50 ms at 100k posts, 2M votes per workspace, any sort |
| Portal page TTFB | anonymous cached p95 under 30 ms; signed-in personalized p95 under 120 ms |
| Portal Lighthouse (mobile) | 100/100/100/100 on `/feedback`, post page, `/roadmap` (CI on seeded portal) |
| JavaScript required on portal | 0 bytes to read and to vote/post/comment (forms work, PRG). Optional `fb.js` under 6 KB gz |
| Widget feedback tab chunk | at most 15 KB gz lazy chunk (01 budget rules) |
| Status change to all voter emails queued | undo window (default 10 min) + under 60 s for 10,000 subscribers |
| Duplicate notifications per contact per post change | 0 (unique per update per contact, coalescing rules in loop.md) |
| Every email one-click unsubscribe | 100% (RFC 8058 header + footer per-post link) |
| AI writes without human confirm | 0. AI only creates `feedback_suggestions`; human accepts |
| Voter identity on public surfaces | hidden by default; never email or company publicly |
| Feedback billable conversations | 0 (no `message.created` from feedback; 12 test) |
| Sign in to vote from widget (identified user) | 0 extra steps (widget JWT identity) |
| Portal sign in from widget | 1 click (widget hand-off code), no email round trip |

## Scope

### In (MVP)

- Boards: many per workspace, `public` or `private` (members only), submissions open or members only, approval mode, categories, widget visibility, order.
- Statuses: workspace-level list (max 12) with category `open|active|done|closed`, color, roadmap flag, notify flag, order. Six defaults seeded.
- Posts: per-workspace number (`#12`), title, Markdown body, board, category, status, internal tags (core `tags`), owner (member), author (contact or member, or member on behalf of contact), image attachments (max 5), pinned, comments locked, external links (manual GitHub/Linear URLs, seam), moderation state, soft delete 30 days.
- Votes: one per contact per post; member adds vote on behalf of contact; vote trust level recorded; imported vote count for migrated posts.
- Revenue weighting: workspace picks one numeric custom attribute (company or contact, e.g. `mrr`) and currency; post shows summed value of distinct voter companies. Value read from attributes, never computed from Dimmi billing. Sort by revenue.
- Customer requests: link post to conversation, message, quote (max 1,000 chars), contact, company snapshot, priority. From inbox, API, MCP, AI suggestion, manual (call notes).
- Comments: public and internal, one reply level, status-update comments, edit window, moderation, @mentions of members in internal comments.
- Merge duplicates: votes, requests, subscriptions, comments carried to target (deduped); source 301s; unmerge restores.
- Sorting: trending (decayed votes, O(1) update), top, new, revenue, requests, recent activity. Filters by board, status, category, tag, owner, company, contact.
- Search: Postgres FTS (`dimmi_search` config, title A + body B) + trigram on title for similar-as-you-type.
- Public roadmap: columns = statuses with roadmap flag, posts from public boards, sort `votes|manual`, board filter. Dashboard roadmap includes private boards.
- Public portal on help center host (`/feedback`, `/feedback/boards/{slug}`, `/feedback/posts/{number}-{slug}`, `/roadmap`), SSR templ, works without JS, access `public` or `signed_in`.
- Portal identity: voters are `contacts`. Portal sessions (`portal_sessions`, contact-bound, scope `feedback`). Sign in: SSO with 02 JWT, email magic link + 6-digit code, one-click hand-off from widget, optional anonymous voting.
- Widget feedback tab API (`/widget/v1/feedback/*`): identified widget users vote with zero extra steps.
- Moderation: approval queue, spam state, heuristics (links, banned words, rate), ban contact from feedback (optional hide content, revoke sessions).
- Close the loop: subscriptions (auto on vote, post, comment, request), post updates with undo window and coalescing, email + widget/portal notification feed, notify-on-ship with changelog link (13 seam), one-click unsubscribe, per-contact daily cap.
- AI (BYOK via 07, optional, human confirms): duplicate suggestions on new posts, request capture from conversations (on close and on 07 intent `feedback`).
- Privacy: voter names hidden publicly by default, author display policy, GDPR eraser, contact merge repoint.
- Member notifications via 06: new post, post pending approval, comment on subscribed post, mention.
- REST API, MCP toolset `feedback` (8 tools), CLI via 09 (generated + `dimmi feedback import`), webhooks (10), realtime for dashboard.
- CSV import (posts + optional votes file, column mapping, dry run): covers Canny and Featurebase CSV exports.

### Out (later)

| Item | When |
|---|---|
| GitHub Issues link with automatic status sync (closed issue moves post, deploy hook ships) | post-MVP: GitHub loop. MVP seam: `external_links` jsonb, manual URLs |
| Linear two-way sync | post-MVP: Linear. Same seam |
| Native API importers Canny, Featurebase, Productlane (voters, comments) | launch: importers (16 via 14 import write helpers below; CSV stays in 14) |
| Productboard, Frill, UserVoice importers | later: more-import-sources (16) |
| Fold 14 CSV import into 16 framework | later: feedback-csv-unify |
| Changelog itself (entries, widget News, subscriber email) | 13-changelog. 14 owns only shipped link + voter notify |
| Custom post fields, segments (15-announcements-surveys `segments` reusable later), per-company private boards, multiple portals per brand, per-board host | later (extensibility #9 brands) |
| Value/effort scoring, RICE, roadmap swimlanes by quarter | later |
| Comment reactions, polls, anonymous posting and commenting | later (anonymous voting only) |
| Translations of portal content | post-MVP: auto-translation |
| Agent answering "is X on roadmap?" from posts | later: posts as knowledge source type (08 seam) |
| Realtime on portal and widget feedback tab | later; dashboard only realtime at MVP |
| Embeddable standalone board widget (no Messenger) | later; portal embeddable via iframe allowed (`frame-ancestors` setting) |
| Survey/NPS | not planned here |
| `.md` twins of portal pages (`/feedback/posts/{n}-{slug}.md`, `/roadmap.md`), portal sections in llms.txt | later: portal-md-twins. None of IC/PL/FB/FD ship `.md` for feedback boards (docs only) |
| ETA on posts (month/quarter, public toggle), roadmap sort by ETA | later: feedback-eta. FB/PL coverage unclear; status columns carry timing at MVP |
| On-demand AI summary of requests per post (themes, counts) | later: feedback-ai-summary. FB partial only (Insights quotes); MVP keeps AI duplicates + capture |

## User stories

Workspace member:
- As founder, I see top requests by votes, revenue and trend, filter by company, and plan from that.
- As support teammate, I select customer sentence in conversation, click "Link to feedback", pick existing post (or create one), customer gets vote and future updates. No copy paste.
- As founder, I move post to "Shipped", type short note, link changelog entry; every voter, requester and commenter gets email and widget notice. I can undo within 10 minutes.
- As moderator, I approve or reject pending posts, merge duplicates, ban spammer.
- As founder, I turn on AI capture; closed conversations with feature asks show up as suggestions ("Link 'export to CSV' quote to #12?"); I accept or dismiss in one click.
- As admin, I import my Canny CSV and keep vote counts.

End customer:
- As logged-in user of host app, I open widget Feedback tab, see roadmap, vote with one tap, no new login.
- As visitor on public portal, I sign in with SSO link from app or email code, vote, post idea, comment.
- As voter, I get one email when my request ships, with changelog link; I unsubscribe in one click.
- As customer, nobody sees my name or company next to my vote unless workspace enables it.

Developer:
- As developer, I subscribe webhook to `feedback_post.status_changed` and post shipped items to my Discord.
- As developer, I mount `/api/dimmi/sso` (also my `sso_login_url`) and link to it with `createPortalLink` / `portalLink`; route signs portal token with same identity secret as widget (`aud dimmi-portal`, 300 s, nonce, URL fragment), user lands signed in.

AI agent (external via MCP):
- As Claude Code session, I search feedback, link conversation quote to post, draft status update; customer-visible changes ask human to confirm.

## Data model

All tables: `id uuid primary key default uuidv7()` unless stated, `workspace_id uuid not null references workspaces(id) on delete cascade` (P8: join tables too), `created_at`, `updated_at timestamptz not null default now()`. Every index leads with `workspace_id`. Text + CHECK for state enums; registry enums noted. RLS on all (cloud) via foundation `dimmi_visible_workspaces()` policy.

### Public ID prefixes (new rows for `00-foundation/data-model.md` registry)

| Prefix | Table | Prefix | Table |
|---|---|---|---|
| fbd_ | feedback_boards | fbct_ | feedback_categories |
| fbs_ | feedback_statuses | fbp_ | feedback_posts |
| fbv_ | feedback_votes | fbc_ | feedback_comments |
| fbr_ | feedback_requests | fbm_ | feedback_post_merges |
| fbsb_ | feedback_subscriptions | fbu_ | feedback_post_updates |
| fbn_ | feedback_notifications | fbsg_ | feedback_suggestions |
| fbi_ | feedback_imports | pts_ | portal_sessions |
| psi_ | portal_sign_ins | | |

Checked against registry 2026-11-02 (incl. `org_`, `fac_`, `ent_`): no collision. Avoided 13 prefixes (`clg_`, `cle_`, `cll_`, `csub_`, `clm_`, `cls_`, `cgn_`) and 15-announcements-surveys prefixes (`seg_`, `ann_`, `svy_`, `svq_`, `svr_`, `erc_`, `eas_`). Token secret prefix `dmi_pts_` (portal session bearer, not an ID). No prefix: `feedback_settings` (singleton per workspace), `feedback_post_tag` (pivot), `feedback_contact_states` (keyed by contact), `feedback_post_embeddings` (keyed by post).

### feedback_settings (one row per workspace)

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid pk references workspaces on delete cascade | created by initializer `EnsureFeedbackDefaults` |
| portal_enabled | boolean not null default false | public portal routes live only when true AND help center active |
| help_center_id | uuid null references help_centers on delete set null | host for portal; null = default help center (resolved at read) |
| portal_title | text not null default 'Feedback' | 1 to 60 |
| access | text not null default 'public' CHECK in ('public','signed_in') | `signed_in`: every portal page needs portal session (private B2B portal) |
| email_sign_in | boolean not null default true | |
| email_sign_in_domains | citext[] not null default '{}' | non-empty: email sign-in only for these domains (max 20) |
| sso_sign_in | boolean not null default true | effective only when active 02 identity secret exists (02 enforcement modes later: identity-validations) |
| sso_login_url | text | https URL on host app; portal "Sign in" sends user there with `return_to` (portal.md 3.1). Recommended: SDK route `https://{host app}/api/dimmi/sso` (mints portal token, redirects back) |
| anonymous_voting | boolean not null default false | anonymous visitors and anonymous widget leads may vote (never post or comment) |
| show_voter_names | boolean not null default false | public portal/widget show voter first names + avatars; never emails, never companies |
| author_display | text not null default 'first_name' CHECK in ('full_name','first_name','anonymous') | public author label for contact authors |
| show_vote_counts | boolean not null default true | false: public shows no numbers (members still see) |
| roadmap_enabled | boolean not null default true | |
| roadmap_sort | text not null default 'votes' CHECK in ('votes','manual') | |
| default_board_id | uuid null references feedback_boards on delete set null | |
| revenue_source | text null CHECK in ('company','contact') | null = revenue weighting off |
| revenue_attribute | text null | custom attribute key `[a-z0-9_]{1,64}` (e.g. `mrr`) |
| revenue_currency | text not null default 'USD' | ISO 4217 |
| revenue_unit | text not null default 'major' CHECK in ('major','minor') | attribute holds dollars (`major`) or cents (`minor`) |
| notify | jsonb not null default '{}' | typed `models.FeedbackNotifySettings`: `undo_seconds` (0 to 3600, default 600), `email_enabled` (true), `kinds` (subset of `status_changed`,`shipped`,`merged`,`team_comment`,`reply`; default all), `from_name` (max 60, default "{workspace} Product Updates") |
| ai | jsonb not null default '{}' | typed `models.FeedbackAiSettings`: `duplicate_detection` (true), `capture` (`off`\|`suggest`, default `off`), `capture_min_confidence` (0.5 to 0.95, default 0.7) |
| moderation | jsonb not null default '{}' | typed `models.FeedbackModerationSettings`: `banned_words` (max 200, each max 40 chars), `link_limit` (default 2, links above put post/comment `pending`), `new_contact_review` (bool default false: first post of email-verified contact pending) |
| frame_ancestors | text[] not null default '{}' | origins allowed to iframe portal (max 10); empty = `frame-ancestors 'none'` |
| next_post_number | bigint not null default 1 | allocator `UPDATE ... RETURNING` in create tx |
| cache_version | bigint not null default 1 | bumped on any public-visible change (portal.md caching) |
| created_at, updated_at | timestamptz | |

### feedback_boards

| Column | Type | Notes |
|---|---|---|
| name | text not null | 1 to 60, NFC |
| slug | citext not null | `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, 1 to 60; unique `(workspace_id, slug) where deleted_at is null`; reserved: `new`, `sign-in`, `sso`, `account`, `posts`, `boards`, `files`, `roadmap`, `api` |
| description | text not null default '' | max 500, inline Markdown |
| visibility | text not null default 'public' CHECK in ('public','private') | `private`: members only, never on portal, widget, public roadmap, public search, sitemap |
| submissions | text not null default 'open' CHECK in ('open','members_only') | `members_only`: customers vote and comment only |
| approval | text not null default 'off' CHECK in ('off','untrusted','all') | `untrusted` = everyone except identified contacts and members needs approval; `all` = every customer post |
| widget_visible | boolean not null default true | public board shown in widget tab |
| position | int not null default 0 | |
| is_default | boolean not null default false | unique `(workspace_id) where is_default and deleted_at is null` |
| post_count | int not null default 0 | approved, not merged, not trashed |
| deleted_at | timestamptz | soft delete; posts trashed with board, restored with board within 30 days |

Index `(workspace_id, position)`. Cloud cap via entitlement `feedback.boards` (25); self-host unlimited. Over: 422 `entitlement_limit_reached` (`key: feedback.boards`, `limit`).

### feedback_categories

`id, workspace_id, board_id uuid not null references feedback_boards on delete cascade, name text not null (1 to 40), slug citext not null, position int not null default 0, created_at, updated_at`. Unique `(workspace_id, board_id, slug)`. Max 30 per board. Public (shown on portal).

### feedback_statuses

| Column | Type | Notes |
|---|---|---|
| name | text not null | 1 to 40; unique `(workspace_id, lower(name))` |
| slug | citext not null | used in portal filters `?status=planned` |
| color | text not null | `#RRGGBB` |
| category | text not null CHECK in ('open','active','done','closed') | semantics: `done` = shipped (notify kind `shipped`, sets `shipped_at`); `closed` = declined/won't do (hidden from default lists, voting closed); `open`/`active` normal |
| position | int not null default 0 | list order and roadmap column order |
| on_roadmap | boolean not null default false | column on roadmap |
| notify | boolean not null default true | moving post into status creates post update (loop.md) |
| is_default | boolean not null default false | unique `(workspace_id) where is_default`; must be category `open` |
| created_at, updated_at | timestamptz | |

Seeded (initializer): Open (`open`, default, notify false), Under review (`open`), Planned (`active`, roadmap), In progress (`active`, roadmap), Shipped (`done`, roadmap), Declined (`closed`). Rules: max 12; at least one `open` and one `done`; delete needs `replacement_id`.

### feedback_posts

| Column | Type | Notes |
|---|---|---|
| number | bigint not null | unique `(workspace_id, number)`; from `feedback_settings.next_post_number` |
| board_id | uuid not null references feedback_boards | |
| category_id | uuid null references feedback_categories on delete set null | must belong to board |
| status_id | uuid not null references feedback_statuses | |
| status_changed_at | timestamptz | |
| title | text not null | 3 to 150 chars, NFC, single line |
| slug | text not null | from title, transliterated, max 80; not unique (URL carries number) |
| body | text not null default '' | Markdown (08 dialect, raw HTML off), max `feedback.post.body_max_kb` (20 KB) |
| body_html | text not null default '' | sanitized render cache, `renderer_version` bump re-renders |
| renderer_version | int not null default 1 | |
| author_type | text not null CHECK in ('contact','member') | |
| author_contact_id | uuid null references contacts on delete set null | |
| author_member_id | uuid null references workspace_members on delete set null | |
| created_by_member_id | uuid null references workspace_members on delete set null | member created on behalf of contact |
| owner_member_id | uuid null references workspace_members on delete set null | internal owner |
| source | text not null CHECK in ('portal','widget','dashboard','api','mcp','ai_capture','import') | |
| moderation_state | text not null default 'approved' CHECK in ('approved','pending','rejected','spam') | |
| moderation_reason | text | code: `approval_required`, `links`, `banned_word`, `new_contact`, `rate`, member note |
| moderated_by_member_id | uuid null references workspace_members on delete set null | |
| moderated_at | timestamptz | |
| pinned_at | timestamptz | pinned first on board (max 3 per board) |
| comments_locked | boolean not null default false | |
| vote_count | int not null default 0 | live votes + `imported_vote_count` |
| imported_vote_count | int not null default 0 | votes without voter identity (CSV `score`) |
| comment_count | int not null default 0 | public approved comments |
| request_count | int not null default 0 | |
| vote_revenue_minor | bigint not null default 0 | summed revenue in minor units of `revenue_currency` (async) |
| trending_log | double precision not null default 0 | log2 sum of decayed vote weights (Trending below) |
| last_activity_at | timestamptz not null default now() | vote, comment, status, request |
| merged_into_id | uuid null references feedback_posts on delete set null | set when merged away |
| merged_at | timestamptz | |
| changelog_entry_id | uuid null references changelog_entries on delete set null | 13 entry; FK inline in 14 migration 001406 (13 block `0013NN` runs first, table exists) |
| shipped_at | timestamptz | first entry into `done` category (cleared when leaving `done`) |
| external_links | jsonb not null default '[]' | `[{provider: github\|linear\|url, url, title?}]`, max 10, internal only |
| edited_at | timestamptz | author or member edit of title/body |
| import_id | uuid null references feedback_imports on delete set null | |
| import_key | text null | source row key; unique `(workspace_id, import_key) where import_key is not null` |
| search | tsvector generated always as (setweight(to_tsvector('dimmi_search', title), 'A') \|\| setweight(to_tsvector('dimmi_search', left(body, 20000)), 'B')) stored | 03 `search.md` config |
| deleted_at | timestamptz | trash, purge after 30 days |

CHECK `NOT (author_contact_id IS NOT NULL AND author_member_id IS NOT NULL)`. Both null allowed (author erased or member deleted: "Deleted contact" / "Former teammate"). Indexes:
- unique `(workspace_id, number)`
- `(workspace_id, board_id, vote_count desc, id desc) where deleted_at is null and merged_into_id is null`
- `(workspace_id, board_id, trending_log desc, id desc)` same predicate
- `(workspace_id, board_id, created_at desc, id desc)` same predicate
- `(workspace_id, status_id, vote_count desc, id desc)` same predicate (roadmap)
- `(workspace_id, vote_revenue_minor desc, id desc)` same predicate
- `(workspace_id, moderation_state, created_at)` where `moderation_state = 'pending'`
- `(workspace_id, author_contact_id)`, `(workspace_id, owner_member_id)`, `(workspace_id, merged_into_id)`, `(workspace_id, changelog_entry_id)`
- GIN `(workspace_id, search)` (btree_gin), GIN `(workspace_id, title gin_trgm_ops)`

Cloud cap entitlement `feedback.posts` (100,000 per workspace). Over: 422 `entitlement_limit_reached` (`key: feedback.posts`, `limit`) on every create surface (API, portal, widget, MCP, import rows skipped with same code).

### feedback_post_tag

`workspace_id, post_id uuid references feedback_posts on delete cascade, tag_id uuid references tags on delete cascade, created_by_type, created_by_id, created_at`. Primary key `(post_id, tag_id)`. Index `(workspace_id, tag_id)`. Tags are core `tags` (03): internal only, never public. Max 20 per post.

### feedback_votes

| Column | Type | Notes |
|---|---|---|
| post_id | uuid not null references feedback_posts on delete cascade | |
| contact_id | uuid null references contacts on delete set null | null only after GDPR erase (anonymized, still counted) |
| trust | text not null CHECK in ('identified','email_verified','anonymous','member_added','imported') | snapshot at vote |
| source | text not null CHECK in ('portal','widget','dashboard','api','mcp','request','import','merge') | |
| created_by_member_id | uuid null references workspace_members on delete set null | vote on behalf |
| merged_from_post_id | uuid null references feedback_posts on delete set null | moved by merge (unmerge restores) |
| created_at | timestamptz | `updated_at` omitted (immutable row, deleted on unvote) |

Unique `(workspace_id, post_id, contact_id) where contact_id is not null`. Indexes `(workspace_id, contact_id, created_at desc)`, `(workspace_id, post_id, created_at desc)`.

### feedback_comments

| Column | Type | Notes |
|---|---|---|
| post_id | uuid not null references feedback_posts on delete cascade | |
| parent_id | uuid null references feedback_comments on delete cascade | one level; parent must have null `parent_id` |
| kind | text not null default 'comment' CHECK in ('comment','status_update','merge_notice') | `status_update`: written with status change, `merge_notice`: system "merged #8 into this" |
| visibility | text not null CHECK in ('public','internal') | internal: members only; never portal, widget, notifications to contacts, webhooks without `include_notes` |
| author_type | text not null CHECK in ('contact','member','system') | |
| author_contact_id, author_member_id | uuid null | on delete set null |
| body | text not null | Markdown, max 10 KB; empty allowed only for `status_update` |
| body_html | text not null | |
| status_from_id, status_to_id | uuid null references feedback_statuses on delete set null | status_update only |
| moderation_state | text not null default 'approved' CHECK in ('approved','pending','rejected','spam') | |
| merged_from_post_id | uuid null | moved by merge |
| edited_at, deleted_at | timestamptz | delete: body cleared, row kept when it has replies ("Comment deleted") |

Indexes `(workspace_id, post_id, created_at)`, `(workspace_id, author_contact_id)`, `(workspace_id, moderation_state, created_at) where moderation_state = 'pending'`. Mentions in internal comments stored in `feedback_comment_mentions (workspace_id, comment_id, member_id, primary key (comment_id, member_id))`.

### feedback_requests (customer requests: evidence)

| Column | Type | Notes |
|---|---|---|
| post_id | uuid not null references feedback_posts on delete cascade | |
| contact_id | uuid null references contacts on delete set null | |
| company_id | uuid null references companies on delete set null | snapshot of contact company at link |
| conversation_id | uuid null references conversations on delete set null | |
| message_id | uuid null references messages on delete set null | must belong to conversation |
| quote | text not null default '' | max 1,000 chars plain text, NFC |
| priority | text null CHECK in ('low','medium','high','critical') | how much customer cares |
| source | text not null CHECK in ('inbox','ai_capture','manual','api','mcp','import') | |
| created_by_member_id | uuid null references workspace_members on delete set null | null for import |
| suggestion_id | uuid null references feedback_suggestions on delete set null | |
| vote_id | uuid null references feedback_votes on delete set null | vote created by this request |
| merged_from_post_id | uuid null | |
| created_at, updated_at | timestamptz | |

Unique `(workspace_id, post_id, conversation_id) where conversation_id is not null`. Indexes `(workspace_id, conversation_id)`, `(workspace_id, contact_id)`, `(workspace_id, company_id)`, `(workspace_id, post_id, created_at desc)`.

### feedback_post_merges

`id, workspace_id, target_post_id, source_post_id (both references feedback_posts on delete cascade), moved_votes int, deduped_vote_contact_ids uuid[] not null default '{}' (source votes dropped as duplicates; recreated on unmerge), moved_requests int, moved_comments int, moved_subscriptions int, source_status_id uuid, merged_by_type, merged_by_id, unmerged_at timestamptz, unmerged_by_member_id, created_at, updated_at`. Unique `(workspace_id, source_post_id) where unmerged_at is null`.

### feedback_subscriptions

`id, workspace_id, post_id references feedback_posts on delete cascade, contact_id uuid null references contacts on delete cascade, member_id uuid null references workspace_members on delete cascade, reason text not null CHECK in ('author','voted','commented','request','manual','import','merge','mentioned'), unsubscribed_at timestamptz, created_at, updated_at`. CHECK exactly one of contact_id/member_id. Unique `(workspace_id, post_id, contact_id) where contact_id is not null`, `(workspace_id, post_id, member_id) where member_id is not null`. Index `(workspace_id, post_id) where unsubscribed_at is null`. Unsubscribed row kept (re-vote never silently re-subscribes, loop.md).

### feedback_post_updates (announcement with undo)

| Column | Type | Notes |
|---|---|---|
| post_id | uuid not null references feedback_posts on delete cascade | |
| kind | text not null CHECK in ('status_changed','shipped','merged','team_comment','reply') | |
| from_status_id, to_status_id | uuid null | |
| comment_id | uuid null references feedback_comments on delete set null | status note or team comment |
| target_post_id | uuid null | merged: post it merged into |
| changelog_entry_id | uuid null references changelog_entries on delete set null | shipped via 13 (FK inline, migration 001414) |
| recipient_contact_id | uuid null | `reply`: single recipient |
| state | text not null CHECK in ('scheduled','sending','sent','cancelled','superseded','skipped') | |
| skip_reason | text | `no_subscribers`, `status_silent`, `reverted`, `post_hidden`, `notify_disabled` |
| scheduled_for | timestamptz not null | now + undo window (status) or comment delay |
| recipients_count, emailed_count | int not null default 0 | |
| created_by_type, created_by_id | text, uuid | actor |
| created_at, updated_at | timestamptz | |

Index `(workspace_id, post_id, created_at desc)`, `(state, scheduled_for) where state = 'scheduled'` (scheduler scan; non-tenant leading column allowed for maintenance job, documented exception like River tables).

### feedback_notifications (contact feed + email outbox)

| Column | Type | Notes |
|---|---|---|
| contact_id | uuid not null references contacts on delete cascade | |
| post_id | uuid not null references feedback_posts on delete cascade | |
| update_id | uuid not null references feedback_post_updates on delete cascade | |
| kind | text not null | same values as update kind |
| data | jsonb not null default '{}' | render snapshot: post number, title, status names/colors, note excerpt (280), changelog url/title, target post. Max 4 KB |
| email_status | text not null default 'pending' CHECK in ('pending','sent','skipped','failed','coalesced') | |
| email_skip_reason | text | `opted_out`, `unsubscribed_post`, `suppressed`, `no_email`, `unverified_email`, `daily_cap`, `email_disabled`, `banned`, `anonymous`, `limited` |
| email_sent_at | timestamptz | |
| outbound_email_id | uuid null | 04 `outbound_emails` row (`kind = feedback_update`, `metadata.purpose` = update kind: `update`, `shipped`, `merged`, `comment`, `digest`) |
| read_at | timestamptz | widget/portal feed |
| created_at, updated_at | timestamptz | |

Unique `(workspace_id, update_id, contact_id)`. Indexes `(workspace_id, contact_id, created_at desc)`, partial `(workspace_id, contact_id) where read_at is null`. Retention 180 days (prune job).

### feedback_contact_states

`workspace_id, contact_id uuid references contacts on delete cascade, email_opt_out_at timestamptz (all feedback email off), banned_at timestamptz, banned_by_member_id, ban_reason text (max 500), last_email_at timestamptz, emails_today int not null default 0, emails_day date, created_at, updated_at`. Primary key `(workspace_id, contact_id)`. Row created lazily.

### portal_sessions (contact sessions, reusable by later support portal)

Implements `extensibility.md` section 10 ("portal later = contact authentication on contact side ... session row like widget_sessions"). Generic: `scopes` lets later support portal reuse rows.

| Column | Type | Notes |
|---|---|---|
| id | uuid | public `pts_` |
| contact_id | uuid not null references contacts on delete cascade | |
| token_hash | bytea not null | SHA-256 of `dmi_pts_` + 43 base64url chars; unique global (pre-tenant lookup via `SECURITY DEFINER portal_resolve_session(bytea)` returning `(workspace_id, id)` only) |
| surface | text not null CHECK in ('portal','widget') | `widget`: token held by widget in header `Dimmi-Feedback-Session` |
| help_center_id | uuid null references help_centers on delete cascade | portal surface |
| widget_settings_id | uuid null references widget_settings on delete cascade | widget surface |
| grant | text not null | registry enum (P1): `sso`, `magic_link`, `email_code`, `widget_handoff`, `anonymous` |
| trust | text not null CHECK in ('identified','email_verified','anonymous') | |
| auth_methods | text[] not null default '{}' | P3: how identity proven (`jwt`, `email_link`, `email_code`, `widget_session`, `none`) |
| authenticated_at | timestamptz not null | |
| scopes | text[] not null default '{feedback}' | |
| external_id | text | SSO `sub` snapshot |
| identity_secret_id | uuid null references identity_secrets on delete set null | |
| expires_at | timestamptz not null | sliding; see config |
| revoked_at | timestamptz | |
| revoke_reason | text CHECK in ('logout','contact_merged','contact_erased','banned','secret_revoked','expired','admin','identity_switch','access_changed') | |
| ip_prefix | inet | /24 v4, /48 v6 |
| user_agent | text | 512 |
| last_seen_at | timestamptz not null default now() | throttled 60 s |
| created_at, updated_at | timestamptz | |

Indexes: unique `(token_hash)`, `(workspace_id, contact_id) where revoked_at is null`, `(workspace_id, identity_secret_id) where revoked_at is null`, `(workspace_id, expires_at)`.

### portal_sign_ins

`id (psi_), workspace_id, kind text not null CHECK in ('email','widget_handoff'), email citext null, contact_id uuid null references contacts on delete cascade (handoff), token_hash bytea not null, code_hash bytea null (email kind: HMAC-SHA256 of 6-digit code keyed by app key + sign-in id; 6 digits safe because 5 attempts then locked), locked_at timestamptz null (durable lockout, written when Valkey attempt counter reaches `sign_in_code_attempts`), surface text CHECK in ('portal','widget'), help_center_id, widget_settings_id, return_to text (relative path, max 512), expires_at timestamptz not null, consumed_at timestamptz, ip_prefix inet, created_at, updated_at`. Index `(workspace_id, email, created_at desc)`, unique `(token_hash)`. Pruned 24 h after expiry. Wrong-code attempt counter lives in Valkey (`fb:signin:{psi}:attempts`, `INCR` + `EXPIRE` = sign-in TTL; rebuildable state, D11): flush resets only an in-progress count below 5 (attacker gains at most 4 tries per flush, still capped by per-IP and per-email redeem limits); reaching 5 writes `locked_at` in Postgres, so a lock survives flush.

### feedback_suggestions (AI triage queue)

| Column | Type | Notes |
|---|---|---|
| kind | text not null CHECK in ('link_request','new_post','merge_posts') | |
| status | text not null default 'pending' CHECK in ('pending','accepted','dismissed','superseded','expired') | |
| post_id | uuid null references feedback_posts on delete cascade | link target, or merge target |
| source_post_id | uuid null references feedback_posts on delete cascade | merge: duplicate to merge away |
| conversation_id | uuid null references conversations on delete cascade | |
| message_id | uuid null references messages on delete set null | |
| contact_id | uuid null references contacts on delete set null | |
| quote | text not null default '' | max 1,000 |
| proposed_title | text | new_post |
| proposed_body | text | new_post, max 4 KB |
| proposed_board_id | uuid null references feedback_boards on delete set null | |
| confidence | numeric(4,3) not null | 0 to 1 |
| reasoning | text | max 500, shown to reviewer, untrusted |
| ai_run_id | uuid null references ai_runs on delete set null | 07 trace |
| reviewed_by_member_id | uuid null | |
| reviewed_at | timestamptz | |
| result_request_id, result_post_id | uuid null | |
| expires_at | timestamptz not null | created + 30 days |

Unique pending: `(workspace_id, conversation_id, post_id) where status = 'pending' and kind = 'link_request'`, `(workspace_id, post_id, source_post_id) where status = 'pending' and kind = 'merge_posts'`. Index `(workspace_id, status, created_at desc)`.

### feedback_post_embeddings (optional)

`workspace_id, post_id uuid primary key references feedback_posts on delete cascade, model_key text not null (provider:model:dims), embedding vector not null, content_hash text not null, updated_at`. Exact KNN scan (no HNSW; posts per workspace small). Only filled when 08 knowledge embedding configured (ai.md).

### feedback_imports

`id (fbi_), workspace_id, status text CHECK in ('pending','validating','validated','running','completed','failed','cancelled'), source text not null default 'csv' CHECK in ('csv'), board_id, mapping jsonb (column map, status map), posts_attachment_id, votes_attachment_id (core attachments, uploaded via 03), dry_run boolean, subscribe_voters boolean default true, totals jsonb (rows, posts_created, posts_skipped, votes_created, contacts_created, errors), errors jsonb (first 100 `{file, row, code, message}`), created_by_member_id, started_at, completed_at, created_at, updated_at`.

### Import write helpers (for 16)

`app/actions/feedback/imports/imported.go`, called only by 16 `feedback` stage writer (Featurebase, Canny, Productlane) inside 16 page tx. Never through `feedback_imports`. Idempotency map stays in 16 `import_records`; post rows also carry `import_key`.
- `feedback.UpsertImportedPost(ctx, tx, in)`: `import_key = '<connection_key>:<external_id>'`, `import_id` null, `source import`, board/status/category from 16 mappings, author contact or member (placeholder: null author), `created_at`/`shipped_at` from source, `imported_vote_count` = source total minus identified voters (never negative). Status creation respects `feedback.statuses` cap (16 mapping conflict `status_cap_reached` otherwise); `feedback.posts` cap: row skipped, `entitlement_limit_reached`.
- `feedback.UpsertImportedVote(ctx, tx, in)`: trust `imported`, source `import`, original vote time when valid; unique `(post, contact)` dedupes; `subscribe_voters` creates subscriptions silently (reason `import`).
- `feedback.UpsertImportedComment(ctx, tx, in)`: public comments, replies deeper than one level flattened with `> quoted parent` prefix (16 E108), author contact/member/null.
- `feedback.UpsertImportedRequest(ctx, tx, in)`: links imported conversation to imported post (Productlane issue on thread, 16 E112), source `import`, `created_by_member_id` null.
- Never: `feedback_post.*`, `feedback_vote.*`, `feedback_comment.*`, `feedback_request.*` events, notifications, voter email, AI duplicate/capture jobs (ai.md trigger excludes source `import`), webhooks. Counts recomputed once per page (`recompute_feedback_post_counts` by post ids, no events), trending from source vote times.
- Rollback (16): `feedback.DeleteImportedPosts(ctx, tx, postIDs)` hard deletes posts with votes, comments, requests, subscriptions; untouched only (16 keeps edited rows).
- CSV sniffing helper `app/support/csvsniff` (BOM strip, delimiter sniff) owned by 14, shared with 16 `csv` driver.

### Core table changes

| Table | Change | Migration |
|---|---|---|
| attachments | `feedback_post_id uuid null references feedback_posts on delete cascade`, `feedback_comment_id uuid null references feedback_comments on delete cascade`; CHECK at most one of `message_id`, `feedback_post_id`, `feedback_comment_id`; index `(workspace_id, feedback_post_id)` | `20261101001420_add_feedback_columns_to_attachments_table.sql` |
| widget_settings (01 table) | `feedback_tab_enabled boolean not null default false` | `20261101001421_add_feedback_tab_to_widget_settings_table.sql` |
| notifications (06 table) | `feedback_post_id uuid null references feedback_posts on delete cascade`; index `(workspace_id, member_id, feedback_post_id, trigger) where read_at is null` (06 aggregation key) | `20261101001423_add_feedback_post_id_to_notifications_table.sql` |

No other core changes. `contact_identities.type` gains registry value `portal_visitor` (anonymous portal session lead, value = `pts_` id; registry enum, no migration). Tags reuse core `tags`.

### Migrations

```
database/migrations/20261101001401_create_feedback_settings_table.sql
database/migrations/20261101001402_create_feedback_boards_table.sql      # then FK feedback_settings.default_board_id
database/migrations/20261101001403_create_feedback_categories_table.sql
database/migrations/20261101001404_create_feedback_statuses_table.sql
database/migrations/20261101001405_create_feedback_imports_table.sql
database/migrations/20261101001406_create_feedback_posts_table.sql
database/migrations/20261101001407_create_feedback_post_tag_table.sql
database/migrations/20261101001408_create_feedback_votes_table.sql
database/migrations/20261101001409_create_feedback_comments_table.sql      # + feedback_comment_mentions
database/migrations/20261101001410_create_feedback_suggestions_table.sql
database/migrations/20261101001411_create_feedback_requests_table.sql
database/migrations/20261101001412_create_feedback_post_merges_table.sql
database/migrations/20261101001413_create_feedback_subscriptions_table.sql
database/migrations/20261101001414_create_feedback_post_updates_table.sql
database/migrations/20261101001415_create_feedback_notifications_table.sql
database/migrations/20261101001416_create_feedback_contact_states_table.sql
database/migrations/20261101001417_create_portal_sessions_table.sql       # + SECURITY DEFINER portal_resolve_session
database/migrations/20261101001418_create_portal_sign_ins_table.sql       # + SECURITY DEFINER portal_resolve_sign_in
database/migrations/20261101001419_create_feedback_post_embeddings_table.sql
database/migrations/20261101001420_add_feedback_columns_to_attachments_table.sql
database/migrations/20261101001421_add_feedback_tab_to_widget_settings_table.sql
database/migrations/20261101001422_add_feedback_rls_policies.sql
database/migrations/20261101001423_add_feedback_post_id_to_notifications_table.sql
```

Range `0014NN`. Depends on 08 (`help_centers`), 01 (`widget_settings`), 06 (`notifications`), 02 (`identity_secrets`), 07 (`ai_runs`), 13 (`changelog_entries`, FK targets in 001406 and 001414) migrations being earlier (they are: lower numbers). 13 adds no migration on 14 tables.

### Trending score

Hacker-News-style decay without periodic recompute: each vote weight `2^((t_vote - E) / H)` with fixed epoch `E = 2026-01-01T00:00:00Z` and half-life `H = feedback.trending.half_life` (7 days). Ordering by sum at any time equals ordering by decayed score (common factor `2^(-t/H)`). Store `trending_log = log2(sum)`. Add vote: `trending_log = max(a, b) + log2(1 + 2^-(|a - b|))` where `b = (t - E)/H`, done in SQL in vote tx. Remove vote, merge, unmerge, import: async `RecomputeFeedbackPostCounts` recomputes from rows (exact). Imported votes count at post `created_at`. Exponent grows 52 per year; double safe for centuries.

## Code map

```
app/enums/
  feedback_board_visibility.go, feedback_board_submissions.go, feedback_board_approval.go
  feedback_status_category.go, feedback_moderation_state.go, feedback_post_source.go
  feedback_vote_trust.go, feedback_vote_source.go, feedback_comment_kind.go, feedback_comment_visibility.go
  feedback_request_source.go, feedback_request_priority.go, feedback_subscription_reason.go
  feedback_update_kind.go, feedback_update_state.go, feedback_suggestion_kind.go, feedback_suggestion_status.go
  portal_session_grant.go (registry), portal_session_trust.go, portal_surface.go

app/actions/feedback/
  ensure_feedback_defaults.go           # workspace initializer: settings row, 6 statuses, default board; idempotent
  update_feedback_settings.go
  boards/{create_board.go, update_board.go, delete_board.go, restore_board.go, reorder_boards.go}
  categories/{create_category.go, update_category.go, delete_category.go, reorder_categories.go}
  statuses/{create_status.go, update_status.go, delete_status.go, reorder_statuses.go}
  posts/
    create_post.go                      # number alloc, moderation evaluation, author vote + subscription, request, attachments, embed + duplicate jobs
    update_post.go                      # title, body, category, owner, pinned, lock, links, tags (edit window for authors)
    change_post_status.go               # status + optional note comment + post update (undo) + shipped_at + events
    moderate_post.go                    # approve/reject/spam (+ ban author)
    move_post.go                        # board change (category reset, private/public transitions)
    trash_post.go, restore_post.go, purge_trashed_posts.go
    merge_posts.go, unmerge_posts.go
    reorder_roadmap_posts.go            # manual roadmap order
    link_changelog_entry.go             # called by 14 listener on 13 `changelog_entry.published`
    shipped_recipient_contact_ids.go    # ShippedRecipientContactIDs(ctx, entryID) []ids.UUID: read only, contacts 14 emails for entry's posts (13 newsletter dedupe)
    evaluate_moderation.go              # heuristics: approval mode, links, banned words, new contact, rate
    render_post_body.go                 # `framework/markdown` with 08 dialect options (`app/support/helpmarkdown`), raw HTML off, `framework/sanitize` UGC policy
  votes/{cast_vote.go, retract_vote.go, add_vote_on_behalf.go, recompute_post_counts.go, recompute_post_revenue.go}
  comments/{create_comment.go, update_comment.go, delete_comment.go, moderate_comment.go}
  requests/{link_request.go, update_request.go, unlink_request.go}
  subscriptions/{subscribe.go, unsubscribe.go, unsubscribe_by_signed_link.go}
  updates/{schedule_post_update.go, cancel_post_update.go, release_post_update.go, fan_out_post_update.go, coalesce_updates.go}
  notifications/{send_feedback_notification_emails.go, render_feedback_email.go, mark_feedback_notifications_read.go, sync_feedback_suppression.go}   # render = templ subject/HTML/text; send = 04 `email.SendContactNotification` (customer stream); sync = 04 `email_suppression.created{outbound_kind: feedback_update}` -> notification row
  suggestions/{create_suggestion.go, accept_suggestion.go, dismiss_suggestion.go, expire_suggestions.go}
  ai/{detect_duplicates.go, capture_from_conversation.go, embed_post.go, queue_capture.go}
  moderation/{ban_contact.go, unban_contact.go}
  imports/{create_import.go, validate_import.go, run_import_chunk.go, cancel_import.go}
  imports/imported.go                  # UpsertImportedPost/Vote/Comment/Request, DeleteImportedPosts: 16 import write helpers (no events, no notifications)
  contacts/{repoint_feedback_contact.go, erase_feedback_contact_data.go}   # contact.merged listener action; eraser hook
  search/{search_posts.go, similar_posts.go}                              # FTS + trigram + optional vector, RRF k=60

app/actions/portal/
  start_sign_in.go                      # email: rate limits, token + code, 04 `SendContactNotification{Kind: portal_sign_in}`; no enumeration
  create_portal_session.go              # grants: sso (02 VerifyIdentityToken + IdentifyContact), magic_link, email_code, widget_handoff, anonymous
  resolve_portal_email_contact.go       # 03 ResolveInboundContact with verified email claim
  create_widget_handoff.go              # widget session -> one-time code (60 s)
  revoke_portal_sessions.go             # by contact, secret, ban, access change
  touch_portal_session.go

app/http/controllers/api/v1/
  feedback_settings_controller.go       # Show, Update  (/feedback/settings)
  feedback_roadmap_controller.go        # Show (/feedback/roadmap)
  feedback_stats_controller.go          # Show (/feedback/stats)
  feedback_board_controller.go          # Index, Store, Show, Update, Destroy, UpdateMany (order)
  feedback_board_category_controller.go # Index, Store, UpdateMany (order)
  feedback_category_controller.go       # Update, Destroy
  feedback_status_controller.go         # Index, Store, Update, Destroy, UpdateMany (order)
  feedback_status_post_controller.go    # UpdateMany (manual roadmap order)
  feedback_post_controller.go           # Index, Store, Show, Update (field dispatch), Destroy, UpdateMany (bulk)
  feedback_post_merge_controller.go     # Index, Store, Destroy (unmerge)
  feedback_post_vote_controller.go      # Index, Store, Destroy
  feedback_post_comment_controller.go   # Index, Store
  feedback_comment_controller.go        # Update, Destroy
  feedback_post_request_controller.go   # Index, Store
  feedback_request_controller.go        # Index (global filters), Update, Destroy
  feedback_post_subscription_controller.go # Index, Store
  feedback_subscription_controller.go   # Destroy
  feedback_post_update_controller.go    # Index (/feedback-posts/{fbp}/updates)
  feedback_post_update_item_controller.go # Update (/feedback-post-updates/{fbu}: cancel, send now)
  feedback_post_similar_controller.go   # Index (/feedback-posts/{fbp}/similar-posts)
  feedback_vote_controller.go           # Index (/feedback-votes?filter[contact])
  feedback_suggestion_controller.go     # Index, Show, Update, UpdateMany
  feedback_capture_controller.go        # Store (/feedback-captures: run AI capture on one conversation, 202)
  feedback_contact_state_controller.go  # Index, Show, Update (/feedback-contact-states/{ctc})
  feedback_import_controller.go         # Index, Store, Show, Update (cancel, run after dry run)

app/http/controllers/widget/feedback/
  feedback_home_controller.go           # Show (/feedback)
  feedback_board_controller.go          # Index
  feedback_post_controller.go           # Index, Store, Show
  feedback_post_vote_controller.go      # Store, Destroy
  feedback_post_comment_controller.go   # Index, Store
  feedback_comment_controller.go        # Update, Destroy (own)
  feedback_post_subscription_controller.go # Update (singleton viewer subscription)
  feedback_roadmap_controller.go        # Show
  feedback_notification_controller.go   # Index, UpdateMany
  feedback_sign_in_controller.go        # Store
  feedback_session_controller.go        # Store, Show (current), Destroy (current)
  feedback_portal_handoff_controller.go # Store

app/http/controllers/web/portal/
  portal_page_controller.go             # Home (/feedback), Board, Post, Roadmap, New, Account
  portal_sign_in_page_controller.go     # Show form, Landing (/feedback/sign-in/{psi}), Sso (/feedback/sso)
  portal_file_controller.go             # Show (/feedback/files/{att}/{filename})
  portal_api/                           # /api/feedback/* JSON + form (PRG) controllers: post, vote, comment, subscription, roadmap, sign-in, session, notification, preference, attachment
  feedback_unsubscription_controller.go # Create (GET confirm page), Store (RFC 8058 POST)  (app host, routes/web.go)

app/http/middleware/
  resolve_portal.go                     # after 08 resolve_help_center: portal enabled, settings, access mode
  portal_session.go                     # cookie or Dimmi-Feedback-Session header -> contact, trust
  portal_csrf.go                        # `framework/http/middleware` CSRF (Go `http.CrossOriginProtection`: Origin / Sec-Fetch-Site) on unsafe portal requests; `grant = sso` exempt (portal.md 3.1)
  feedback_actor.go                     # widget: header session > identified widget session > anonymous lead

app/http/requests/feedback/             # store_/update_ per resource, bulk_update_feedback_posts_request.go, store_feedback_import_request.go (multipart), store_portal_session_request.go (grant-discriminated) ...
app/http/resources/
  feedback_board_resource.go, feedback_category_resource.go, feedback_status_resource.go
  feedback_post_resource.go             # member view (full)
  public_feedback_post_resource.go      # portal/widget view (no tags, owner, internal counts, voter emails)
  feedback_vote_resource.go, feedback_comment_resource.go, public_feedback_comment_resource.go
  feedback_request_resource.go, feedback_subscription_resource.go, feedback_post_update_resource.go
  feedback_notification_resource.go, feedback_suggestion_resource.go, feedback_import_resource.go
  feedback_roadmap_resource.go, feedback_settings_resource.go, portal_session_resource.go

app/policies/
  feedback_post_policy.go (UpdateField per field), feedback_board_policy.go, feedback_status_policy.go
  feedback_comment_policy.go, feedback_request_policy.go, feedback_suggestion_policy.go, feedback_settings_policy.go
  portal_actor_policy.go                # contact-side: can vote/post/comment by trust, board, ban, lock, access

app/events/                             # one struct per new event (Events section)
app/listeners/
  broadcast_feedback_counts.go          # coalesced feedback_post.counts_changed (5 s per post; cross-replica gate Valkey `SET NX EX 5` on `fb:counts:{post}`)
  schedule_feedback_capture.go          # conversation.closed -> capture job (when enabled)
  queue_feedback_duplicate_detection.go # feedback_post.created -> duplicate + embed jobs
  repoint_feedback_on_contact_merge.go  # contact.merged (queued)
  revoke_portal_sessions_on_contact_event.go # contact.merged, contact.deleted, contact.blocked
  revoke_portal_sessions_on_secret_revoke.go # identity_secret.revoked (02)
  recompute_revenue_on_company_change.go # company.updated / contact.updated when revenue attribute changed
  bust_portal_cache.go                  # public-visible events bump feedback_settings.cache_version
  link_feedback_on_changelog_publish.go # changelog_entry.published (13)
  unlink_feedback_on_changelog_delete.go # changelog_entry.deleted (13)
  disable_portal_on_help_center_delete.go # help_center.updated (deleted/disabled) (08)
  release_feedback_ownership.go         # member.deactivated (00-accounts)
  repoint_feedback_requests_on_conversation_merge.go # conversation.merged (03)
  reembed_feedback_posts.go             # knowledge.embeddings_switched (08)

app/jobs/
  fan_out_feedback_post_update.go, send_feedback_notification_emails.go, release_scheduled_post_updates.go (periodic 15 s)
  recompute_feedback_post_counts.go, recompute_feedback_post_revenue.go, recompute_feedback_revenue_for_company.go
  detect_feedback_duplicates.go, capture_feedback_from_conversation.go, embed_feedback_post.go
  run_feedback_import.go, repoint_feedback_contact.go, purge_trashed_feedback.go
  prune_portal_sessions.go, prune_feedback_notifications.go, expire_feedback_suggestions.go, rerender_feedback_bodies.go

resources/views/mail/feedback/{update,shipped,merged,comment,sign_in}.templ   # rendered by render_feedback_email.go; NOT `app/mail` Mailables (customer mail goes through 04 `SendContactMail`, 04 outbound.md 15-16; arch test `tests/arch/email_streams_test.go`)
app/notifications/{feedback_post_submitted_notification.go, feedback_post_pending_notification.go, feedback_comment_created_notification.go, feedback_mentioned_notification.go}   # 06 classes
app/mcp/tools/feedback/{search_feedback.go, get_feedback_post.go, create_feedback_post.go, link_feedback_request.go, update_feedback_status.go, comment_on_feedback.go, merge_feedback_posts.go, list_feedback_suggestions.go}
app/console/commands/{feedback_recount_command.go, feedback_reembed_command.go, feedback_portal_sessions_revoke_command.go, feedback_rerender_command.go}
app/support/feedback/{trending.go, revenue.go, csv_mapping.go, heuristics.go}   # pure, no deps
app/support/csvsniff/sniff.go             # BOM strip, delimiter sniff (~30 lines); pure; shared with 16 `csv` driver
# slugs (board, category): `framework/support/str.Slug` (gosimple/slug v1.15.0) + collision suffix in action; no own slug.go
# unsubscribe links: `framework/routing` signed routes (`a.URL.SignedRoute`, middleware `signed`); no own signer

resources/views/portal/{layout,home,board,post,roadmap,new,sign_in,sign_in_landing,sso,account,unsubscription}.templ
resources/views/portal/static/fb.js          # optional progressive enhancement (similar posts, vote without reload)
resources/prompts/feedback/2026-11-01/{extract_requests.md, judge_duplicates.md, CHANGELOG.md}

routes/api.go (feedback group), routes/widget.go (feedback group), routes/web.go (portal group on help center hosts, unsubscribe), routes/console.go (periodic jobs), routes/mcp.go (toolset feedback), routes/channels.go (none new)

database/queries/{feedback_settings,feedback_boards,feedback_categories,feedback_statuses,feedback_posts,feedback_post_tag,feedback_votes,feedback_comments,feedback_requests,feedback_post_merges,feedback_subscriptions,feedback_post_updates,feedback_notifications,feedback_contact_states,portal_sessions,portal_sign_ins,feedback_suggestions,feedback_post_embeddings,feedback_imports}.sql
database/factories/{feedback_board,feedback_status,feedback_post,feedback_vote,feedback_comment,feedback_request,portal_session}_factory.go
database/seeders/feedback_seeder.go         # demo: 2 boards, 40 posts, 600 votes, roadmap filled, 3 suggestions

tests/feature/feedback/                    # per endpoint: happy, 422, policy, ability, cross-tenant 404, idempotency
tests/feature/portal/                      # SSR pages, forms PRG, sign-in flows, access modes, leak test (private boards, internal comments)
tests/feature/widget/feedback/             # actor resolution matrix
tests/feature/feedback/merge_test.go, coalescing_test.go   # merge/unmerge, update coalescing: integration (real Postgres, testkit.New)
tests/feature/feedback/support_test.go     # pure `app/support/feedback` funcs (trending math, revenue, heuristics, csv mapping): table-driven, no mocks, no I/O
tests/bench/feedback_bench_test.go         # 100k posts, 2M votes
```

## HTTP API

Full JSON in [api.md](api.md). REST `/api/v1`: middleware `auth` (`A`: session, first-party app, token), `workspace`, `throttle:api`; POST also `idempotent`. Abilities new: `feedback:read`, `feedback:write`. Permissions (P2) in Permissions matrix.

### REST `/api/v1`

| Method | Path | Route name | Auth + ability (permission) | Purpose |
|---|---|---|---|---|
| GET | /feedback/settings | feedback.settings.show | A feedback:read | settings + effective portal URL, sign-in methods availability |
| PATCH | /feedback/settings | feedback.settings.update | A settings:write (feedback:manage) | portal, access, sign-in, privacy, revenue, notify, ai, moderation |
| GET | /feedback/roadmap | feedback.roadmap.show | A feedback:read | columns with posts; `filter[board]`, `include_private=true` |
| GET | /feedback/stats | feedback.stats.show | A feedback:read | counts by status/board, top posts by votes/revenue/trend, new requests per week (90 d max) |
| GET | /feedback-boards | feedback-boards.index | A feedback:read | `filter[trashed]` |
| POST | /feedback-boards | feedback-boards.store | A settings:write (feedback:manage) | entitlement cap (422 `entitlement_limit_reached`) |
| GET | /feedback-boards/{fbd} | feedback-boards.show | A feedback:read | `include=categories` |
| PATCH | /feedback-boards/{fbd} | feedback-boards.update | A settings:write (feedback:manage) | attributes; `trashed: false` restore |
| DELETE | /feedback-boards/{fbd} | feedback-boards.destroy | A settings:write (feedback:manage) | trash board + its posts; not last board |
| PATCH | /feedback-boards | feedback-boards.update-many | A settings:write (feedback:manage) | `{order: [fbd_...]}` exact list |
| GET | /feedback-boards/{fbd}/categories | feedback-boards.categories.index | A feedback:read | |
| POST | /feedback-boards/{fbd}/categories | feedback-boards.categories.store | A settings:write (feedback:manage) | |
| PATCH | /feedback-boards/{fbd}/categories | feedback-boards.categories.update-many | A settings:write (feedback:manage) | `{order}` |
| PATCH | /feedback-categories/{fbct} | feedback-categories.update | A settings:write (feedback:manage) | name, slug |
| DELETE | /feedback-categories/{fbct} | feedback-categories.destroy | A settings:write (feedback:manage) | posts uncategorized |
| GET | /feedback-statuses | feedback-statuses.index | A feedback:read | |
| POST | /feedback-statuses | feedback-statuses.store | A settings:write (feedback:manage) | max 12 |
| PATCH | /feedback-statuses | feedback-statuses.update-many | A settings:write (feedback:manage) | `{order}` |
| PATCH | /feedback-statuses/{fbs} | feedback-statuses.update | A settings:write (feedback:manage) | name, color, category, on_roadmap, notify, is_default |
| DELETE | /feedback-statuses/{fbs} | feedback-statuses.destroy | A settings:write (feedback:manage) | `?replacement_id=fbs_` required when posts use it; moves posts silently (no notifications) |
| PATCH | /feedback-statuses/{fbs}/posts | feedback-statuses.posts.update-many | A feedback:write | manual roadmap `{order: [fbp_...]}` (top 200 of column) |
| GET | /feedback-posts | feedback-posts.index | A feedback:read | filters, `q`, sort, include (api.md) |
| POST | /feedback-posts | feedback-posts.store | A feedback:write | create; optional `on_behalf_of`, `request`, `attachment_ids`, `tags` |
| GET | /feedback-posts/{fbp} | feedback-posts.show | A feedback:read | merged post returns 200 with `merged_into` (no redirect in API) |
| PATCH | /feedback-posts/{fbp} | feedback-posts.update | A feedback:write (per field) | attributes; `status_id` (+`status_note`, `notify`); `moderation_state` (+`reason`, `ban_author`); `board_id`; `owner_id`; `add_tags`/`remove_tags`; `trashed: false` (feedback:delete) |
| PATCH | /feedback-posts | feedback-posts.update-many | A feedback:write | bulk `{ids, changes}` max 100: `status_id`, `notify`, `moderation_state`, `board_id`, `category_id`, `owner_id`, `add_tags`, `remove_tags`, `trashed` |
| DELETE | /feedback-posts/{fbp} | feedback-posts.destroy | A feedback:write (feedback:delete) | trash |
| GET | /feedback-posts/{fbp}/merges | feedback-posts.merges.index | A feedback:read | merges into this post |
| POST | /feedback-posts/{fbp}/merges | feedback-posts.merges.store | A feedback:write | `{from: [fbp_...], notify?}` target in path, max 20 |
| DELETE | /feedback-posts/{fbp}/merges/{fbm} | feedback-posts.merges.destroy | A feedback:write (feedback:delete) | unmerge |
| GET | /feedback-posts/{fbp}/votes | feedback-posts.votes.index | A feedback:read + contacts:read for identity | voters with contact, company, revenue value, trust, source |
| POST | /feedback-posts/{fbp}/votes | feedback-posts.votes.store | A feedback:write | on behalf: `{contact_id}` or `{contact: {email?, external_id?, name?}}` (03 upsert match), `subscribe?` |
| DELETE | /feedback-posts/{fbp}/votes/{fbv} | feedback-posts.votes.destroy | A feedback:write | remove vote |
| DELETE | /feedback-posts/{fbp}/votes | feedback-posts.votes.destroy-many | A feedback:write (feedback:delete) | bulk remove by filter, `filter[trust]=anonymous` required (vote-stuffing cleanup); returns `meta.deleted` |
| GET | /feedback-posts/{fbp}/comments | feedback-posts.comments.index | A feedback:read | public + internal, moderation filter |
| POST | /feedback-posts/{fbp}/comments | feedback-posts.comments.store | A feedback:write | `{body, visibility, parent_id?, attachment_ids?, notify?}` |
| PATCH | /feedback-comments/{fbc} | feedback-comments.update | A feedback:write | own body; `moderation_state`; others' body needs feedback:delete |
| DELETE | /feedback-comments/{fbc} | feedback-comments.destroy | A feedback:write (own) / feedback:delete (others) | |
| GET | /feedback-posts/{fbp}/requests | feedback-posts.requests.index | A feedback:read | quotes, contacts, companies, conversations |
| POST | /feedback-posts/{fbp}/requests | feedback-posts.requests.store | A feedback:write + conversations:read when `conversation_id` | link evidence; `add_vote` (default true), `subscribe` (default true) |
| GET | /feedback-requests | feedback-requests.index | A feedback:read | `filter[conversation\|contact\|company\|post\|source]` (inbox side panel) |
| PATCH | /feedback-requests/{fbr} | feedback-requests.update | A feedback:write | quote, priority, `post_id` (move) |
| DELETE | /feedback-requests/{fbr} | feedback-requests.destroy | A feedback:write | unlink; `remove_vote` (default false) |
| GET | /feedback-votes | feedback-votes.index | A feedback:read | `filter[contact\|company]` (contact sidebar) |
| GET | /feedback-posts/{fbp}/subscriptions | feedback-posts.subscriptions.index | A feedback:read | subscribers (contacts + members), counts |
| POST | /feedback-posts/{fbp}/subscriptions | feedback-posts.subscriptions.store | A feedback:write | `{contact_id}` or `{member_id: "me"}` |
| DELETE | /feedback-subscriptions/{fbsb} | feedback-subscriptions.destroy | A feedback:write | |
| GET | /feedback-posts/{fbp}/updates | feedback-posts.updates.index | A feedback:read | announcements incl. scheduled (undo UI) |
| PATCH | /feedback-post-updates/{fbu} | feedback-post-updates.update | A feedback:write | `{state: "cancelled"}` undo; `{scheduled_for}` send sooner (>= now) |
| GET | /feedback-posts/{fbp}/similar-posts | feedback-posts.similar-posts.index | A feedback:read | duplicate candidates (no AI) |
| GET | /feedback-suggestions | feedback-suggestions.index | A feedback:read | `filter[status,kind,conversation,post]` |
| GET | /feedback-suggestions/{fbsg} | feedback-suggestions.show | A feedback:read | |
| PATCH | /feedback-suggestions/{fbsg} | feedback-suggestions.update | A feedback:write | `{status: "accepted", overrides?}` or `{status: "dismissed"}` |
| PATCH | /feedback-suggestions | feedback-suggestions.update-many | A feedback:write | bulk dismiss/accept `{ids, changes: {status}}` |
| POST | /feedback-captures | feedback-captures.store | A feedback:write + conversations:read | `{conversation_id}` run AI capture now; 202 `feedback_capture` |
| GET | /feedback-contact-states | feedback-contact-states.index | A feedback:read | `filter[banned]=true`, `filter[opted_out]=true` |
| GET | /feedback-contact-states/{ctc} | feedback-contact-states.show | A feedback:read | contact feedback profile: votes, posts, requests counts, ban, opt-out |
| PATCH | /feedback-contact-states/{ctc} | feedback-contact-states.update | A settings:write (feedback:manage) for `banned`; feedback:write for `email_opt_out` | `{banned, reason, hide_content, revoke_sessions}` |
| GET | /feedback-imports | feedback-imports.index | A feedback:read | |
| POST | /feedback-imports | feedback-imports.store | A settings:write (feedback:manage) | multipart `posts`, `votes?`, `mapping`, `board_id`, `dry_run` (default true); 202 |
| GET | /feedback-imports/{fbi} | feedback-imports.show | A feedback:read | progress, errors |
| PATCH | /feedback-imports/{fbi} | feedback-imports.update | A settings:write (feedback:manage) | `{status: "running"}` after validated dry run; `{status: "cancelled"}` |

Post `PATCH` field dispatch (rest.md rule 3, one tx, one response):

| Field | Action | Event | Permission |
|---|---|---|---|
| title, body, category_id, pinned, comments_locked, external_links | `UpdatePost` | `feedback_post.updated` (`changes`) | feedback:write |
| status_id (+ status_note Markdown, notify bool default true) | `ChangePostStatus` | `feedback_post.status_changed` | feedback:write |
| moderation_state `approved\|rejected\|spam` (+ reason, ban_author) | `ModeratePost` | `feedback_post.moderated` | feedback:write; ban_author needs feedback:manage |
| board_id | `MovePost` | `feedback_post.updated` | feedback:write |
| owner_id (`mem_`, `me`, null) | `UpdatePost` | `feedback_post.updated` | feedback:write |
| add_tags, remove_tags | `TagPost` | `feedback_post.updated` | feedback:write |
| trashed false | `RestorePost` | `feedback_post.restored` | feedback:delete |

Same status: no-op 200, no event. `status_id` on trashed or merged post: 409 `post_trashed` / `post_merged`.

### Portal host routes (web, help center hosts), widget routes

Listed in [portal.md](portal.md) and [api.md](api.md). Summary:
- HTML (`routes/web.go`, help center group, after 08 `resolve_help_center`, middleware `resolve_portal`, `portal_session`, `throttle:portal`): `/feedback`, `/feedback/boards/{slug}`, `/feedback/posts/{number}-{slug}`, `/feedback/new`, `/feedback/sign-in`, `/feedback/sign-in/{psi}`, `/feedback/sso`, `/feedback/account`, `/feedback/files/{att}/{filename}`, `/roadmap`.
- JSON + form (`/api/feedback/*` on help center host; route lint applies): posts, votes, comments, subscription, roadmap, sign-ins, sessions, notifications, preferences, attachments.
- Widget `/widget/v1/feedback/*` (01 widget group, owned here): home, boards, posts, votes, comments, subscription, roadmap, notifications, sign-ins, sessions, portal-handoffs.
- App host web: `GET/POST /feedback-unsubscriptions` (signed, RFC 8058), rest.md exceptions row required.

### Error codes (feature)

`feedback_disabled` (404 portal/widget when portal off or widget tab off), `board_not_found` (404), `board_private` (404 on public surfaces, never 403), `entitlement_limit_reached` (`key`, `limit`; 422 count caps `feedback.boards`, `feedback.posts`, `feedback.statuses`; 429 + `Retry-After` per-period cap `feedback.import_rows_per_month`; replaces feature limit codes), `last_board` (409), `slug_taken` (422), `slug_reserved` (422), `status_in_use` (409, delete without replacement), `status_category_required` (422, would remove last open/done), `invalid_transition` (422, `from`, `to`: moderation from rejected to pending etc.), `post_trashed` (409), `post_merged` (409, with `merged_into`), `post_locked` (403 comments locked), `voting_closed` (409, status category closed), `submissions_closed` (403, board members only), `merge_cycle` (422), `merge_limit` (422, over 20 sources), `merge_cross_workspace` (404), `unmerge_conflict` (409, target trashed), `request_exists` (409 with existing id), `message_not_in_conversation` (422), `vote_exists` (200 returns existing; never error), `sign_in_required` (401 with `sign_in_methods`), `trust_insufficient` (403, e.g. anonymous tries to post; `required_trust`), `feedback_banned` (403), `email_domain_not_allowed` (422, email sign-in domains), `sign_in_expired` (410), `sign_in_invalid` (422, wrong code; attempts left), `sign_in_locked` (429, 5 bad codes), `sso_not_configured` (409), plus 02 token codes (`token_expired`, `token_signature_invalid`, ...) on SSO grant, `handoff_invalid` (422), `session_invalid` (401), `ai_unavailable` (409), `capture_disabled` (409), `suggestion_not_pending` (409), `import_invalid` (422 with row errors), `import_too_large` (413), `import_not_validated` (409), `body_too_large` (413), `attachment_not_image` (422), `attachment_limit` (422), `attachment_not_owned` (422), `pin_limit` (422), `category_cleared` (warning in `meta.warnings`), `edit_window_closed` (403), `rate_limited` (429 with `Retry-After`), `csrf_mismatch` (419-style 403 on portal forms), `precondition_failed` (409 `stale_resource` on `If-Unmodified-Since`).

## MCP tools and CLI

Toolset `feedback` (09 registry rules: feature toolset, under 2,000 tokens, filtered by abilities, `app/mcp/confirm` gate for customer-visible effects). Tools keep task names (rest.md rule 15).

| Tool | Risk | Ability | REST equivalent | Gate |
|---|---|---|---|---|
| `dimmi_search_feedback` | read | feedback:read | `GET /feedback-posts?q=&filter...` | none |
| `dimmi_get_feedback_post` | read | feedback:read | `GET /feedback-posts/{fbp}?include=requests,status,board` + top 10 quotes | none |
| `dimmi_create_feedback_post` | write | feedback:write | `POST /feedback-posts` | private board: none; public board: confirm (public content) |
| `dimmi_link_feedback_request` | write | feedback:write + conversations:read | `POST /feedback-posts/{fbp}/requests` (or create post + request when `title` given instead of post) | none when target post exists (internal + vote); creating public post: confirm |
| `dimmi_update_feedback_status` | send | feedback:write | `PATCH /feedback-posts/{fbp} {status_id, status_note, notify}` | confirm when `notify` and subscribers > 0 (customers emailed); preview shows recipient count |
| `dimmi_comment_on_feedback` | write/send | feedback:write | `POST /feedback-posts/{fbp}/comments` | internal: none; public: confirm |
| `dimmi_merge_feedback_posts` | destructive | feedback:write | `POST /feedback-posts/{target}/merges` | confirm |
| `dimmi_list_feedback_suggestions` | read | feedback:read | `GET /feedback-suggestions?filter[status]=pending` | none |

Post identifiers accept `fbp_...` or number (`12`, `"#12"`). Status accepts `fbs_` or name (case-insensitive). Untrusted envelope (09) wraps post bodies, comments, quotes (customer text). No MCP tool bans, deletes, accepts suggestions creating public posts without gate, or changes settings.

CLI (09 generates from OpenAPI, space style): `dimmi feedback posts list|show|create`, `dimmi feedback status <post> <status> [--note] [--no-notify]`, `dimmi feedback link <post> --conversation 1042 --quote "..."`, `dimmi feedback import posts.csv [--votes votes.csv] --board fbd_ [--map title=Title,...] [--run]` (dry run first, prints errors, `--run` confirms), `dimmi feedback suggestions [--accept fbsg_]`.

Server commands (artisan style, owned here): `dimmi feedback:recount [--workspace ws_]` (counts, trending, revenue from rows), `dimmi feedback:reembed [--workspace]`, `dimmi feedback:rerender` (body_html after renderer bump), `dimmi feedback:portal-sessions:revoke --contact ctc_ | --workspace ws_`.

## Events

Emitted (all new; add to `00-foundation/events-and-realtime.md` "Events added by feature specs"):

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| feedback_post.created | yes (workspace) | yes | includes `source`, `moderation_state` |
| feedback_post.updated | yes | yes | `changes: {field: [from, to]}` (title, body hash, category, owner, board, tags, pinned, lock, links) |
| feedback_post.status_changed | yes | yes | `from`/`to` status ids + categories, `note_comment_id`, `notify`; `to.category = done` means shipped |
| feedback_post.moderated | yes | yes | `from`/`to` moderation state, reason |
| feedback_post.merged | yes | yes | `source_id`, `target_id`, moved counts |
| feedback_post.unmerged | yes | yes | |
| feedback_post.deleted / feedback_post.restored | yes | yes | trash / restore |
| feedback_post.counts_changed | yes (workspace, coalesced 5 s per post) | no | vote/comment/request counts, trending, revenue |
| feedback_vote.created / feedback_vote.deleted | no | yes | `contact_id`, `trust`, `source`, `created_by_member_id`; realtime via counts_changed |
| feedback_comment.created / .updated / .deleted | yes | yes (internal only to endpoints with `include_notes = true`) | |
| feedback_request.created / feedback_request.deleted | yes | yes | conversation, message, contact, company |
| feedback_suggestion.created / feedback_suggestion.reviewed | yes | no | triage queue |
| feedback_board.created / .updated / .deleted | yes | no | |
| feedback_status.created / .updated / .deleted | yes | no | |
| feedback_settings.updated | yes | no | audit |
| feedback_post_update.sent | no | yes | announcement delivered: `kind`, `post_id`, `recipients_count`, `emailed_count` (developer can mirror to Discord) |
| feedback_import.completed | yes (`member:{mem}` of creator) | no | |

Realtime: centrifuge via `framework/broadcasting` (Redis broker + presence on Valkey, D11), channel `workspace:{ws}` for all above (dashboard; channel names per `00-foundation/events-and-realtime.md`). No new channels, no new authorizers in `routes/channels.go`. Reconnect gaps: centrifuge history + recovery (no own replay). Portal and widget not realtime at MVP. Payload max 10 KB rule (foundation) applies; `feedback_post.updated` with large body sends `{id, object, updated_at}`.

Consumed:

| Event | Listener | Effect |
|---|---|---|
| conversation.closed (03) | `ScheduleFeedbackCapture` | when `ai.capture = suggest` and conversation has customer messages: job `capture_feedback_from_conversation` delayed `feedback.ai.capture_delay` (10 min), unique per conversation |
| ai intent `feedback` (07 calls `feedback.QueueCapture`) | direct action call, not event | same job, delay 0 (dedupe by unique key) |
| contact.merged (03) | `RepointFeedbackOnContactMerge` (queued) | votes (dedupe), subscriptions, requests, authors, notifications, contact state (ban wins, opt-out wins), sessions revoked `contact_merged` |
| contact.deleted (03, erase) | eraser hook (not listener) | `EraseFeedbackContactData` |
| contact.blocked (03) | `RevokePortalSessionsOnContactEvent` | revoke sessions; blocked contact treated as banned for feedback |
| company.updated / contact.updated (03) | `RecomputeRevenueOnCompanyChange` | only when `changes` touch configured revenue attribute or `company_id`; unique job per company 60 s |
| identity_secret.revoked (02) | `RevokePortalSessionsOnSecretRevoke` | revoke SSO sessions with that secret when `revoke_sessions` |
| help_center.updated (08) | `DisablePortalOnHelpCenterChange` | help center deleted/disabled: portal routes 404; settings keep `portal_enabled` (restore works) |
| tag.deleted (03) | none (FK cascade on pivot) | |
| changelog_entry.published (13) | `LinkFeedbackOnChangelogPublish` | payload `{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids[], notify_voters}`: `LinkChangelogEntry` for listed post ids (loop.md) |
| changelog_entry.deleted (13) | `UnlinkFeedbackOnChangelogDelete` (queued) | `changelog_entry_id` set null by FK already; listener bumps portal cache + clears `changelog` snapshot on scheduled updates (no email) |
| member.deactivated (00-accounts) | `ReleaseFeedbackOwnership` (queued) | `owner_member_id` null on posts; member subscriptions removed |
| conversation.merged (03) | `RepointFeedbackRequestsOnConversationMerge` (queued) | requests on source conversation move to target; unique conflict: duplicate removed, quotes concatenated (max 1,000) |
| email_suppression.created (04) | `SyncFeedbackSuppression` (queued) | only `outbound_kind` `feedback_update` / `portal_sign_in`: notification row by `outbound_email_id` -> `email_status failed`, `email_skip_reason suppressed` (bounce/complaint after send); contact state resource shows `email_suppressed` (via `email.IsSuppressed(ctx, ws, email, "all", "feedback")`). No opt-out written (suppression is 04 state) |
| knowledge.embeddings_switched (08) | `ReembedFeedbackPosts` (queued) | vector arm off until `embed_feedback_post` re-run per workspace (chunked 100) |

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| release_scheduled_post_updates | maintenance | periodic 15 s | 0 (next tick) | singleton (leader) |
| fan_out_feedback_post_update | notifications | released update | 10 backoff | `ByArgs` update_id; chunks 500 subscribers, cursor in args |
| send_feedback_notification_emails | mail | fan-out chunk | 10 | `ByArgs` (update_id, chunk) |
| recompute_feedback_post_counts | default | unvote, merge, unmerge, import, moderation, comment delete | 5 | `ByArgs` post_id, `ByPeriod` 5 s |
| recompute_feedback_post_revenue | default | vote/unvote/request, settings change | 5 | `ByArgs` post_id, `ByPeriod` 30 s |
| recompute_feedback_revenue_for_company | default | company/contact attribute change | 5 | `ByArgs` company_id or contact_id, `ByPeriod` 60 s |
| detect_feedback_duplicates | ai | post created (customer or member), when AI ready and `duplicate_detection` | 3 | `ByArgs` post_id |
| embed_feedback_post | ai | post created/title-body changed, when embeddings configured | 5 | `ByArgs` post_id, `ByPeriod` 60 s |
| capture_feedback_from_conversation | ai | conversation.closed (delayed), 07 feedback intent, `POST /feedback-captures` | 3 | `ByArgs` conversation_id + `ByPeriod` 1 h (manual run bypasses period) |
| run_feedback_import | ingest | import running | 3 per chunk | `ByArgs` (import_id, chunk) |
| repoint_feedback_contact | default | contact.merged | 10 | `ByArgs` (from, into) |
| purge_trashed_feedback | maintenance | daily | 3 | singleton; `RetentionGuard` check |
| prune_portal_sessions | maintenance | daily | 3 | singleton; expired + 30 d, sign-ins 24 h after expiry |
| prune_feedback_notifications | maintenance | daily | 3 | singleton; older than 180 days; `RetentionGuard` |
| expire_feedback_suggestions | maintenance | daily | 3 | singleton |
| rerender_feedback_bodies | default | boot when renderer version bumped | 3 | `ByArgs` workspace_id |

## Config keys

`config/feedback.go` (env `DIMMI_FEEDBACK__*`). Cloud caps are entitlement catalog keys (`extensibility.md` section 6), not config.

| Key | Default | Notes |
|---|---|---|
| feedback.post.title_max | 150 | |
| feedback.post.body_max_kb | 20 | |
| feedback.post.attachments_max | 5 | images only, 10 MB each |
| feedback.comment.body_max_kb | 10 | |
| feedback.author_edit_window | 60m | contact edits own post/comment |
| feedback.notify.undo_window_default | 10m | per-workspace `notify.undo_seconds` overrides |
| feedback.notify.comment_delay | 2m | team comment / reply notifications (edit/delete grace) |
| feedback.notify.post_cooldown | 30m | at most 1 email per contact per post per window; later updates coalesce |
| feedback.notify.contact_daily_max | 10 | feedback emails per contact per day |
| feedback.notify.fanout_chunk | 500 | |
| feedback.portal.session_ttl | 30d | sliding, identified + email_verified |
| feedback.portal.anonymous_session_ttl | 180d | sliding |
| feedback.portal.sign_in_ttl | 15m | magic link + code |
| feedback.portal.sign_in_code_attempts | 5 | Valkey counter; then `locked_at` (Postgres) + `sign_in_locked` |
| feedback.portal.handoff_ttl | 60s | widget to portal code |
| feedback.portal.anonymous_cache_ttl | 15s | anonymous HTML page cache |
| feedback.rate.votes_per_contact_per_minute | 30 | all `feedback.rate.*` enforced by `ratelimit.Limiter` on Valkey (`a.Limiter`, keys `fb:rl:{rule}:{subject}`); rebuildable, never durable state |
| feedback.rate.posts_per_contact_per_hour | 5 | |
| feedback.rate.comments_per_contact_per_hour | 20 | |
| feedback.rate.sign_ins_per_email_per_hour | 5 | |
| feedback.rate.sign_ins_per_ip_per_hour | 20 | |
| feedback.rate.anonymous_votes_per_ip_per_day | 100 | |
| feedback.rate.portal_per_ip_per_minute | 300 | page + API |
| feedback.trending.half_life | 168h | trending stays Postgres (`trending_log` on post row, ordered by index) |
| feedback.ai.capture_delay | 10m | after close |
| feedback.ai.capture_daily_max | 200 | per workspace, protects BYOK spend |
| feedback.ai.capture_max_messages | 40 | last N customer + team messages sent to model |
| feedback.ai.duplicate_candidates | 10 | candidates sent to judge |
| feedback.ai.duplicate_min_confidence | 0.75 | below: no suggestion |
| feedback.ai.suggestion_ttl | 720h | |
| feedback.import.max_rows | 20000 | per file |
| feedback.import.max_file_mb | 10 | |
| feedback.search.similar_limit | 5 | |

Entitlement catalog keys (owner 14): `feedback.boards` (cloud 25, self-host unlimited), `feedback.posts` (cloud 100,000, self-host unlimited), `feedback.statuses` (12 both), `feedback.import_rows_per_month` (cloud 100,000).

## Permissions matrix

Policies check permissions (P2). Permission to ability group mapping and role sets live in `app/auth/permissions.go` / `roles.go` (00-accounts; was `app/support/auth`, framework.md 10.1).

| Permission | Ability group | Owner | Admin | Member | Covers |
|---|---|---|---|---|---|
| feedback:read | feedback:read | yes | yes | yes | read boards, posts, votes, requests, comments incl. internal, suggestions, roadmap, stats, imports |
| feedback:write | feedback:write | yes | yes | yes | create/edit posts, status changes (+notify), moderation approve/reject/spam, votes on behalf, requests, comments, merges, subscriptions, suggestion review, captures, cancel post updates, manual roadmap order |
| feedback:delete | feedback:write | yes | yes | no | trash/restore posts, unmerge, edit/delete others' comments |
| feedback:manage | settings:write | yes | yes | no | settings, boards, categories, statuses, bans, imports |

Deactivated member: none. Token: effective = role permissions intersect abilities (no `feedback:*` ability: 403 `missing_ability`). Votes list exposes contact emails only with `contacts:read` too (else contact compact without email). Contact-side permissions (portal and widget): portal.md "Actions by trust".

## Performance budgets

See World-class bar. Mechanics:
- Every list sort index-backed with `(workspace_id, board_id, key desc, id desc)` partial indexes; keyset cursor. Cross-board list (all boards) uses `(workspace_id, ...)` variants planned via `UNION ALL` per public board when board count under 20, else seq on workspace index (documented bench).
- Counts denormalized, updated in vote tx (`UPDATE feedback_posts SET vote_count = vote_count + 1, trending_log = ..., last_activity_at = @now`; `@now` from `a.Clock`, testing.md time rule). Hot post (viral): row lock per vote under 1 ms; tested 200 votes/s on one post with p95 under 30 ms.
- Vote dedupe by Postgres unique index + `ON CONFLICT DO NOTHING RETURNING`; no read-then-write race. Never Valkey: a vote is a durable record (counts, revenue, notify-on-ship recipients), Valkey flush must never allow a second vote or lose one.
- Valkey (D11, always deployed; holds only rebuildable state): rate limits (`ratelimit.Limiter`), anonymous vote throttles, sign-in attempt counters, cross-replica cache bust (`pubsub.PubSub` topic `feedback.cache`), realtime (centrifuge Redis broker), coalescing gates (`SET NX EX`). Postgres: posts, votes, trending, counts, per-contact daily email counter (`feedback_contact_states.emails_today`: over-mailing after a flush would be visible to customers, so durable), sign-in lock.
- Valkey unavailable: limiter fails closed on unauthenticated abuse surfaces (start sign-in, redeem code, anonymous vote/session create: 503 `service_unavailable` + `Retry-After: 5`; code redeem cannot count attempts so never runs unlimited); fails open on member API and on identified/email_verified contact writes (metric `dimmi_feedback_ratelimit_bypass_total`, warn log; Postgres unique vote + per-contact daily email cap still hold). Reads unaffected (L1 cache serves; lost busts bounded by TTL). Realtime pauses; dashboard refetches on reconnect (centrifuge recovery fails -> client refetch).
- Revenue: async, never on vote path.
- Portal: anonymous HTML cache in-process L1 (otter, 16 MB budget shared with 08 page cache config, key `(workspace, cache_version, path, query, variant)`, TTL 15 s), busted on every replica via Valkey pubsub (`feedback.cache`); lost bust (Valkey down) bounded by 15 s TTL; signed-in pages render personalized bits (voted, subscribed) from 1 query per page (`SELECT post_id FROM feedback_votes WHERE contact_id = $1 AND post_id = ANY($2)`).
- Similar-as-you-type: `title % $q` trigram + FTS top 5, `SET LOCAL pg_trgm.similarity_threshold = 0.3`, debounced client side 250 ms, server in-process LRU 10 s per (workspace, normalized prefix) (L1 only, no Valkey hop).
- Fan-out: chunk 500 per tx, notifications insert with `INSERT ... SELECT` from subscriptions; email job per 100.
- Bench in CI (`tests/bench/feedback_bench_test.go`): 100k posts, 2M votes, 50k requests; regression over 10% fails.

## Observability

Metrics: `dimmi_feedback_votes_total{source,trust}`, `dimmi_feedback_posts_total{source,moderation}`, `dimmi_feedback_requests_total{source}`, `dimmi_feedback_updates_total{kind,state}`, `dimmi_feedback_notifications_total{channel,status,skip_reason}`, `dimmi_feedback_fanout_seconds`, `dimmi_feedback_portal_requests_total{route,status,cache}`, `dimmi_feedback_portal_ttfb_seconds{route,cache}`, `dimmi_portal_sign_ins_total{kind,result}`, `dimmi_portal_sessions_active` (gauge, sampled), `dimmi_feedback_ai_runs_total{kind,outcome}`, `dimmi_feedback_suggestions_total{kind,status}`, `dimmi_feedback_imports_total{status}`, `dimmi_feedback_similar_seconds`.
Logs: post update fan-out summary (update id, recipients, emailed, skipped by reason), import summary, AI run outcome (ids only). Never log emails, quotes, post bodies at info; sign-in tokens and codes never.
`dimmi doctor`: portal reachable on help center host, Valkey reachable (foundation check; 14 adds none), instance mail provider set for customer stream (04 outbound.md 15; sign-in and updates need it; `log` driver = warning), pg_trgm present.

## Security and privacy

- Tenancy: every query by `workspace_id`; RLS; portal and widget resolve workspace from host or app id only, never from body. Other workspace ids 404.
- Private boards never leave members, except one deliberate case: `shipped` email to contacts linked by request or vote on behalf (title + note, no link; loop.md). Public resources built only from `public_feedback_post_resource` after visibility predicate `board.visibility = 'public' AND moderation_state = 'approved' AND merged_into_id IS NULL AND deleted_at IS NULL`; leak test crawls every portal, widget, sitemap route for marker text from private posts and internal comments.
- Internal comments, tags, owner, requests, quotes, revenue, external links, voter emails: never on portal, widget, contact emails, or webhooks without `include_notes` (internal comments). Requests and quotes are customer PII: members only.
- Voter privacy: `show_voter_names` false by default; even when true only first name + avatar; never email, company, revenue publicly. Counts can be hidden.
- Author display: contact author shown per `author_display`; member authors show name + workspace badge.
- Portal sessions: token 32 random bytes, SHA-256 stored, cookie `__Host-dimmi_portal` (custom domain or subdomain host) or `dimmi_portal_{hc}` with `Path=/hc/{sub}` (path mode), `Secure`, `HttpOnly`, `SameSite=Lax`. Widget surface token only in header, never cookie, never URL.
- CSRF: portal unsafe methods pass `framework/http/middleware` CSRF (Go `http.CrossOriginProtection`: `Sec-Fetch-Site` / `Origin` must be same origin as portal host; no token cookie, no `/csrf` endpoint). JSON API and no-JS form posts (PRG) same rule. Exempt: `grant = sso` session POST (portal.md 3.1), RFC 8058 unsubscribe POST. Reject: 403 `csrf_mismatch`.
- Sign-in: no account enumeration (always 202, same body, timing padded to 150 ms); magic link token in URL fragment (never sent to server in GET, never in logs); redemption by POST only (link scanners safe); code 6 digits, 5 attempts, 15 min; tokens HMAC/SHA-256 stored.
- SSO: 02 verification (alg allowlist, kid, exp, nonce single-use) with `Surface: portal`: `aud` must contain `dimmi-portal` (widget tokens from identity endpoint rejected `token_audience_mismatch`). Host SSO route (`@dimmi/nextjs/portal-sso` or `createPortalSsoHandler` in `@dimmi/server`, at `/api/dimmi/sso`; link helpers `createPortalLink` / `portalLink` only build `/api/dimmi/sso?return_to=...`, never mint; 01 sdk-contract.md; other stacks raw snippet from 02 snippets.md) signs `aud: dimmi-portal`, `exp = now + 300 s`, random `nonce`. JWT carried in fragment `#token=` then POST body; never query string.
- Widget hand-off: one-time code 60 s, bound to widget session contact and help center, single use.
- `return_to` only relative portal paths (open redirect guard).
- Markdown: `framework/markdown` with 08 dialect options (raw HTML off) + `framework/sanitize` UGC policy (link rel `nofollow noopener ugc`); images only from own `/feedback/files` or attachments; external images rendered as links.
- Attachments: images only (png, jpeg, gif, webp; SVG rejected), sniffed, dimension guard; served on portal only when parent post/comment publicly visible, else 404; `Content-Disposition: inline` for allowlisted images only.
- Anonymous voting abuse: per-IP daily cap (Valkey limiter), PoW reuse from 01 (`widget.bot` rules) for widget anonymous leads, portal anonymous session creation requires `fb.js` PoW or form honeypot + IP rate; anonymous votes shown separately in member UI (`trust`) and excludable from sort (`filter[trust]`).
- Moderation heuristics never send content to third parties (local rules). AI spam classification not at MVP.
- Prompt injection: conversation text, post bodies, quotes are untrusted in AI prompts (07 wrapping rules); AI output only creates suggestions; reasoning text rendered as plain text.
- GDPR: eraser hook `EraseFeedbackContactData` (edge-cases section K). Export: 03 contact export includes `feedback` section (votes, posts, comments, requests) via export hook (required from 03).
- Audit log: settings change, board/status create/delete, ban/unban, merge/unmerge, import, post trash/restore, bulk status change, portal access mode change.
- `frame-ancestors` from settings only (clickjacking on vote buttons otherwise).

## Metering impact

None billable (`research/pricing.md` section 2). Verified against 12 `metering-rules.md`: activation only from `message.created` substantive replies in conversations with inbound message (D1/D2), InTx gate on `message.created` and `conversation.created`.
- Feedback writes (posts, votes, comments, requests, notifications) create no `messages` or `conversations` rows. Never `message.created`. Nothing reaches `RecordBillableActivity`.
- Linking a conversation to a post writes only `feedback_requests`; conversation unchanged; no event that 12 meters.
- Notify-on-ship is email via 04 `SendContactNotification` (wrapper over `SendContactMail`, customer stream, `outbound_emails.kind = feedback_update`, sub-kind in `metadata.purpose`; not a conversation reply): D7 spirit (broadcast never billable). Counts toward 04 sending limits (reputation), not billing, not D7 fair-use (12 counts `conversation.created` only).
- 12 replay fixture `feedback_activity_not_billable` (vote, post, comment, request, status change with notify, AI capture on fake LLM: zero ledger lines).
- Customer replies to feedback emails land as new email conversation (04 inbound); billed only if team replies (normal D1). Documented in email footer copy.
- AI capture, duplicate judge: BYOK tokens recorded in 07 spend (`ai_runs.kind = feedback`), count toward AI spend cap; never Dimmi money.
- Import: free (D11 spirit).

## Dependencies on other specs

| Spec | Needs from it |
|---|---|
| 00-foundation | `framework/ids` (prefixes in `app/models/public_ids.go`), events, realtime (`framework/broadcasting`, Valkey broker), Valkey-backed `cache.Store` / `ratelimit.Limiter` / `pubsub.PubSub`, RLS, idempotency, querybuilder, `framework/httpclient` (none outbound here), `filesystem.Filesystem`, signed URLs (`framework/routing`), `framework/markdown` + `sanitize`, `framework/support/str.Slug`, CSRF middleware, entitlements, RetentionGuard, initializer + eraser registries |
| 00-accounts | permissions catalog + roles, abilities `feedback:*`, audit log, member deactivation event |
| 01-messenger-widget | widget route group/middleware, widget sessions, PoW/bot score, attachments upload flow, Feedback tab, SDK `show('feedback'\|'roadmap')`, server helpers `createPortalLink` / `portalLink` / `/api/dimmi/sso` (sdk-contract.md) |
| 02-widget-identity | `VerifyIdentityToken`, `IdentifyContact` (no widget session), secrets revoke event |
| 03-shared-inbox | contacts, companies, conversations, messages, tags, `ResolveInboundContact` (verified email claim), contact merge/erase/export hooks, attachments store |
| 04-email-channel | `SendContactNotification` wrapper over `SendContactMail` (outbound.md 16: `Kind` `feedback_update` \| `portal_sign_in`, `Purpose` sub-kind, sender, suppression scope `all` + `feedback`, limits, List-Unsubscribe passthrough, customer stream), `email_suppression.created{outbound_kind}` |
| 06-notifications | 4 member triggers |
| 07-dimmi-agent | `ai.RunTask` (final: `Kind feedback`, `Purpose capture\|duplicate`, structured output, spend cap, `ai_runs.kind feedback`), `ai.WrapUntrusted`, readiness, intent `feedback` hook |
| 08-help-center | host resolution, routes, nav, sitemap, Markdown pipeline, embeddings helper |
| 09-agent-native | MCP toolset, confirm gate, CLI generation |
| 10-webhooks | catalog entries, `include_notes` for internal comments |
| 12-cloud-billing | entitlements catalog values; metering note |
| 13-changelog | `changelog_entry.published` with `feedback_post_ids`, `notify_voters`; `changelog_entries` table (FK target, earlier migration block); newsletter dedupe calls 14 `ShippedRecipientContactIDs` |

## Required from other specs (exact edits; orchestrator applies or forwards)

### 00-foundation
1. `data-model.md` ownership map: row `feedback_settings, feedback_boards, feedback_categories, feedback_statuses, feedback_posts, feedback_post_tag, feedback_votes, feedback_comments, feedback_comment_mentions, feedback_requests, feedback_post_merges, feedback_subscriptions, feedback_post_updates, feedback_notifications, feedback_contact_states, feedback_suggestions, feedback_post_embeddings, feedback_imports, portal_sessions, portal_sign_ins | 14-feedback-roadmap`.
2. `data-model.md` prefix registry: add `fbd_`, `fbct_`, `fbs_`, `fbp_`, `fbv_`, `fbc_`, `fbr_`, `fbm_`, `fbsb_`, `fbu_`, `fbn_`, `fbsg_`, `fbi_`, `pts_`, `psi_` (table above); code: add to `app/models/public_ids.go` (`framework/ids` registry). Token secret note: `dmi_pts_` not an ID.
3. `data-model.md` "Core table changes": `attachments | feedback_post_id, feedback_comment_id, one-target CHECK | 14`; `widget_settings | feedback_tab_enabled | 14` (01 table, listed for completeness); migration range note `0014NN`.
4. `data-model.md` `contact_identities` note: registry value `portal_visitor` (14).
5. `events-and-realtime.md` "Events added by feature specs": row 14 with event list above (flags in this README).
6. `api.md`: abilities list add `feedback:read`, `feedback:write`; endpoint ownership row `feedback boards, posts, votes, comments, requests, suggestions, imports, portal, /widget/v1/feedback | 14-feedback-roadmap`; Surfaces table: help center hosts also serve feedback portal (`/feedback`, `/roadmap`, `/api/feedback/*`).
7. `rest.md` exceptions table: row `/feedback-unsubscriptions` (app host, signed HMAC, RFC 8058 one-click; same shape as `/notification-unsubscriptions`); web pages list: help center hosts incl. portal pages `/feedback/*`, `/roadmap`.
8. `contracts.md` workspace initializer list: append `14 feedback defaults (settings, statuses, default board; after 08 default help center)`; contact eraser list: `14: votes anonymized, posts/comments/requests scrubbed, subscriptions, notifications, portal sessions/sign-ins, contact state`; new short section "Feedback shipping" (13 to 14 contract: `feedback.LinkChangelogEntry` + event `changelog_entry.published{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids, notify_voters}` + read action `feedback.ShippedRecipientContactIDs` for 13 newsletter dedupe).
9. `security.md`: add "Contact sessions (portal)" paragraph pointing to 14 portal.md (cookie names, CSRF via `http.CrossOriginProtection` same as dashboard, fragment tokens, portal JWT `aud dimmi-portal`); abuse list adds anonymous voting limits.
10. `extensibility.md` section 10: note "contact portal sessions implemented by 14 (`portal_sessions`, `scopes`), later support portal reuses"; entitlement catalog keys from 14.

### 00-accounts
11. Permission catalog: `feedback:read` (group feedback:read), `feedback:write`, `feedback:delete` (group feedback:write), `feedback:manage` (group settings:write); `roles.go`: member gets read/write, admin/owner all. Abilities enum adds `feedback:read`, `feedback:write` (09 OAuth scopes pick up automatically). Audit action names `feedback.*` (settings_updated, board_created, board_deleted, status_created, status_deleted, contact_banned, contact_unbanned, posts_merged, post_unmerged, import_started, post_trashed, post_restored, portal_access_changed, bulk_status_changed).

### 01-messenger-widget
12. Document `widget_settings.feedback_tab_enabled` (column added by 14 migration 001421); `PATCH /api/v1/widget-settings/{widget}` accepts it (requires portal settings row; works even when public portal disabled: widget tab independent of `portal_enabled`).
13. Config response `features.feedback: {enabled, boards_count, roadmap}` and texts `feedback_tab_label` (max 24) in `models.WidgetTexts`.
14. Session resume/start response adds `feedback_unread_count` (int, from `feedback_notifications`, capped 99) when tab enabled.
15. Widget route group mounts `/widget/v1/feedback/*` (controllers owned by 14, like 08 Help); app-only auth allowed for reads; CORS allowed request header `Dimmi-Feedback-Session`.
16. Attachments: widget upload flow (`POST /widget/v1/attachments` + complete) usable for feedback post/comment images; 01 cleanup job (unattached 24 h) treats rows with `feedback_post_id`/`feedback_comment_id` as attached.
17. SDK (sdk-contract.md, already there; keep in sync): `show('feedback', {post?: number, board?: string})`, `show('roadmap')`; deep links `#dimmi-open=feedback`, `#dimmi-open=roadmap`; storage key `dimmi:{app}:feedback` (feedback session token, sent as `Dimmi-Feedback-Session`); lazy chunk `feedback` at most 15 KB gz in size budget CI; server helpers `createPortalLink()` (`@dimmi/server`), `portalLink()` (`@dimmi/nextjs/server`), route `/api/dimmi/sso` (`@dimmi/nextjs/portal-sso`, or `createPortalSsoHandler()` from `@dimmi/server` on any Fetch runtime; other stacks raw snippet; framework engines later: identity-helper-packages) = recommended `sso_login_url`; env `DIMMI_PORTAL_URL`. Helpers sign `aud: dimmi-portal`, `exp` 300 s, random `nonce`, pass token in fragment only.
18. Session revoke (logout, identity_switch, contact merge/erase) also revokes `portal_sessions` with `surface = widget` for that widget + contact (14 action `RevokePortalSessions`, called by 01).

### 02-widget-identity
19. `VerifyIdentityToken` accepts `Surface` (`widget` | `portal`) for errors and hints (no verification stats per surface: `identity_verification_stats` later: identity-validations); same rules except step 9: `Surface: portal` requires `aud` containing `dimmi-portal` (missing or widget audience: `token_audience_mismatch`, hint "Sign portal links with createPortalLink / portalLink"); `Surface: widget` rejects `dimmi-portal`-only tokens (portal link token cannot open widget session).
20. New action `IdentifyContact{WorkspaceID, Claims, Surface}` (no widget, no session): steps 1, 2, 4, 5, 6 of `IdentifyVisitor` (find/create by `sub`, apply claims, verified email collision handling, test detection) without lead upgrade/merge. `ApplyIdentityClaims` reused.
21. Snippets (`snippets.md`): per stack raw "Portal SSO link" example for stacks without SDK helper: JWT `aud dimmi-portal`, `exp` 300 s, `nonce`; `https://{portal}/feedback/sso#token=<jwt>&return_to=/feedback/posts/12`; redirect-back route `/api/dimmi/sso` used as `sso_login_url`.
22. `identity_secret.revoked` with `revoke_sessions: true` also revokes `portal_sessions` by `identity_secret_id` (14 listener; event already exists, document consumer).

### 03-shared-inbox
23. `ResolveInboundContact` rule addition (channel-contract.md section 4): verified claim (type, value) whose identity row is unverified and held by other contact: row moves to claimant, `verified = true`, `contact.updated` on both (old holder keeps `contacts.email` text). Used by 14 portal email sign-in and correct for 04 DMARC-aligned mail.
24. Attachments: `LinkAttachments` accepts targets `feedback_post` and `feedback_comment` (ownership, limits, image-only check passed by caller); orphan purge treats rows with feedback columns as attached.
25. Contact export (`POST /contacts/{ctc}/exports`) includes `feedback` section via export hook registry (14 registers `ExportFeedbackContactData`).
26. Eraser registry: register 14 `EraseFeedbackContactData`; CI table list includes 14 tables.
27. Events: confirm `company.updated` and `contact.updated` carry `changes` keys including changed `custom_attributes` keys (14 revenue recompute filter). If not, add `changed_attribute_keys []string`.
28. Tag delete confirmation count (`tag.deleted`) may add `feedback_post_count` (optional, display only); pivot cascades.
29. Conversation resource `include=feedback_requests` (compact `{id, post: {id, number, title, status}}`) for inbox side panel without extra round trip (14 provides query function `feedback.RequestsForConversation`).

### 04-email-channel
30. Done in 04 (outbound.md 16, README 223/244/718): `email.SendContactNotification` = thin wrapper over `email.SendContactMail(ctx, a, SendContactMailInput{WorkspaceID, ContactID, To, Kind: feedback_update | portal_sign_in, Purpose (`update`, `shipped`, `merged`, `comment`, `digest`, `sign_in`), FromName: notify.from_name, Subject, HTML, Text, Unsubscribe (feedback_update only), IdempotencyKey: fbn_/psi_ id, Automated: true})`; result `{Status sent|skipped, SkipReason suppressed|limited|paused|no_sender|disabled, OutboundEmailID}` returned as-is. `outbound_emails.kind` = `feedback_update` / `portal_sign_in` directly (CHECK values in 04), sub-kind in `metadata.purpose`. Customer stream (never system Mailables). Suppression: scope `all` blocks both kinds; scope `feedback` (complaint on voter mail) stops only `feedback_update` (sign-in still sent); `notifications` blocks neither. `portal_sign_in` transactional and exempt from daily caps (covers former "sign-in priority" ask). `email_suppression.created` carries `outbound_kind` (04 README 718); 14 listener `SyncFeedbackSuppression` consumes it. Reply-To = workspace default address without thread token (replies become new conversation). No open 04 asks.
31. Done in 04: `SendContactMail.Unsubscribe` sets List-Unsubscribe + RFC 8058 headers with caller URL (14 signed `/feedback-unsubscriptions` link).

### 06-notifications
32. Triggers: `feedback_post_submitted` (customer created approved post; recipients all active members with `feedback:read`; defaults in_app on, push off, email off, slack on; destination-eligible; Discord later: discord-notifications), `feedback_post_pending` (post or comment awaiting approval; recipients members with `feedback:write`; in_app on, email on (delayed 5 min, skipped if moderated meanwhile), push off), `feedback_comment_created` (comment on post member subscribed to; in_app on, others off), `feedback_mentioned` (internal comment mention; in_app, push, email on). Registered as entries in 06 `notifications.trigger` registry enum (no CHECK lists; preferences and destination `triggers` validate against registry). Notification classes owned by 14 (`app/notifications/feedback_*_notification.go`).
33. Column `notifications.feedback_post_id uuid null references feedback_posts on delete cascade` added by 14 migration `20261101001423_add_feedback_post_id_to_notifications_table.sql` (06 documents it, no 06 migration); 06 aggregation key `(member, feedback_post_id, trigger)` unread.
34. Source events: `feedback_post.created` (approved, source customer), `feedback_post.moderated`/created pending, `feedback_comment.created`, mentions from `feedback_comment_mentions`.

### 07-dimmi-agent
35. Done in 07: `ai_runs.kind` includes `feedback`; `ai_run_steps.kind` uses existing `classify`, `generate`.
36. Done in 07 (final): `ai.RunTask(ctx, a, RunTaskInput{WorkspaceID, Kind: feedback, Purpose: capture|duplicate, ModelRole: classifier|answer, StepKind, System, Messages, ResponseFormat, JSONSchema, MaxOutputTokens, Redact, Actor})`: readiness check, spend cap (skip with `spend_cap`), retries/fallback, redaction, `ai_runs.kind = feedback` + `ai_run_steps` (`classify`, `generate`), spend recording. Customer text wrapped with `ai.WrapUntrusted`. Prompts owned by 14 under `resources/prompts/feedback/`. No open 07 ask.
37. Pipeline: when classifier intent `feedback` on customer message and workspace feedback `ai.capture = suggest`, call `feedback.QueueCapture(conversationID, messageID)` after commit (no extra LLM call in 07).

### 08-help-center
38. Help center route group registers 14 portal routes (`/feedback*`, `/roadmap*`, `/api/feedback/*`) before redirect/404 fallback; same host resolution, path mode, canonical 301 rules (portal pages redirect to primary host like articles; `/api/feedback/*` not redirected).
39. Redirect paths `/feedback*`, `/roadmap*` reserved (422 `redirect_shadows_portal`). Portal adds no hostnames: custom domains stay 08 `help_center_domains` (verification + TLS on `framework/customdomain`, claims + shared reserved subdomain list in `app/support/customdomains`); no 14 domain table.
40. Layout header: "Feedback" and "Roadmap" nav links when portal enabled and access public (or signed-in session present); shared theme CSS and brand vars; footer unchanged. Delivered through 08 provider registry `helpcenter.PublicSectionProvider` (14 implements `NavLinks`, `Routes`, `ReservedPaths`, `SitemapURLs`, `RobotsRules`, `CSP`; same registry 13 uses; no llms.txt provider: `LlmsTxtSection` returns nil, no `HomeSections`, no `SearchResultKind` at MVP; llms.txt sections later: portal-md-twins): 14 registers `feedback.PortalSectionProvider`; 08 never imports 14 types.
41. Via same provider: sitemap adds `/feedback`, `/roadmap`, public board pages, public approved posts (`lastmod` = `last_activity_at`) when portal enabled, access `public`, indexing enabled. No llms.txt portal sections (later: portal-md-twins). robots.txt adds `Disallow: /feedback/sign-in`, `/feedback/sso`, `/feedback/account`, `/feedback/new`.
42. CSP: `form-action 'self'`, `frame-ancestors` from `feedback_settings.frame_ancestors` on portal pages only.
43. Export embeddings helper `knowledge.EmbedTexts(ctx, workspaceID, texts) ([][]float32, modelKey, usage, error)` using configured embedding key/model and 07 spend recording; returns `ErrNoEmbeddings` when not configured.
44. `help_center.updated` payload includes `deleted`/`status` change (14 listener).

### 09-agent-native
45. Toolset `feedback` (8 tools above) in registry list; confirm gate rules per table; resource identifiers `fbp_`/number; untrusted envelope for bodies and quotes.
46. CLI: `dimmi feedback import` helper (multipart upload + dry run then `PATCH {status: running}`), status shortcut; others generated.

### 10-webhooks
47. Registry entries for webhook events above with fixtures; `feedback_comment.*` with `visibility = internal` delivered only to `include_notes = true` endpoints (same rule as notes); catalog API flag `notes_opt_in` for them.

### 12-cloud-billing
48. `metering-rules.md`: note "Feedback (14) creates no messages or conversations; never billable; feedback emails are not conversation outbound (D7 fair-use unaffected)". Replay fixture `feedback_activity_not_billable`: vote, post, comment, request, status change with notify, AI capture on fake LLM produce zero ledger lines.
49. Cloud driver catalog references 14 keys: `feedback.boards` (cloud 25), `feedback.posts` (cloud 100,000), `feedback.statuses` (12), `feedback.import_rows_per_month` (cloud 100,000). Pricing page line: "Changelog, feedback board and roadmap, announcements and surveys included".

### 13-changelog
50. `changelog_entry.published` payload: `{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids[], notify_voters bool}` (`feedback_post_ids` max 50, from entry editor; `notify_voters` default true); entry editor lists candidate posts via `GET /feedback-posts?filter[status_category]=active,done`; 13 API stores linked post ids on entry (owner chooses column or pivot). 13 also emits `changelog_entry.deleted` (14 consumer).
51. No 13 migration on 14 tables: FK `feedback_posts.changelog_entry_id` and `feedback_post_updates.changelog_entry_id` (`references changelog_entries on delete set null`) declared inline by 14 migrations 001406 / 001414 (13 block `0013NN` runs first).
52. Newsletter dedupe: 14 sends voter emails (`feedback_update` kind `shipped`); 13 mailing snapshot excludes contacts from `feedback.ShippedRecipientContactIDs(ctx, entryID)` (14 read action: contacts with `feedback_notifications` for updates linked to entry, email not `opted_out`/`suppressed`). Subscribers without contact row unaffected.
53. Prefix check done: 13 uses `clg_ cle_ cll_ csub_ clm_ cls_ cgn_`; no collision with `fb*`, `pts_`, `psi_`.

### 11-self-host
54. `dimmi doctor` check list adds 14 checks (portal reachable, instance customer-stream mail provider not `log`); seeders demo data; no packaging change.

## Open questions (recommended default in bold)

1. Anonymous voting default. **Off** (Featurebase defaults on; we keep loop closable: anonymous voters get no notifications). Toggle per workspace.
2. Voter names on public portal. **Hidden** by default (privacy-first); first name + avatar when enabled.
3. Portal host when workspace has several help centers. **One portal per workspace on chosen help center**; per-brand portals later.
4. AI capture default. **Off** (spend); one toggle; dashboard shows estimated monthly tokens from last 30 days closed conversations.
5. Status-change undo window. **10 minutes**, per-workspace 0 to 60 min.
6. Status change notifies by default. **Yes** when status `notify` true; dashboard shows recipient count before save (`meta.notification_estimate` on dry run `?preview=true`).
7. CSV import in MVP. **Yes** (posts + votes), stays in 14 (decision in `16-importers/README.md`; fold into 16 later: feedback-csv-unify). API importers (Canny, Featurebase, Productlane) ship at launch in 16 through 14 import write helpers.
8. Revenue weighting source. **Company attribute** (`mrr`), contacts without company use contact attribute fallback when `revenue_source = contact`.

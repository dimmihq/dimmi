# 14 Portal, identity, widget tab

Style: caveman. Owner: 14-feedback-roadmap. Read with README.md.

## 1. Identity model (decision)

Every customer who votes, posts or comments is a `contacts` row (03 core). No portal users, no passwords, never `users` (extensibility.md section 10). Proof of identity is attached to contact-bound session rows:

| Surface | Credential | Contact source | Trust |
|---|---|---|---|
| Widget, identified | 01 widget session `kind = identified` (02 JWT) | session contact (`kind = user`) | `identified` |
| Widget, anonymous lead | 01 widget session `kind = anonymous` | session lead | `anonymous` |
| Widget, lead after email code | `portal_sessions` `surface = widget` (header `Dimmi-Feedback-Session`) | verified-email contact (3.2) | `email_verified` |
| Portal, SSO | `portal_sessions` grant `sso` (cookie) | 02 `IdentifyContact` by JWT `sub` | `identified` |
| Portal, widget hand-off | `portal_sessions` grant `widget_handoff` | widget session contact | trust of widget session (identified or email_verified via header session) |
| Portal, email link or code | `portal_sessions` grant `magic_link` / `email_code` | verified-email contact (3.2) | `email_verified` |
| Portal, anonymous vote | `portal_sessions` grant `anonymous` | new lead with identity `portal_visitor` | `anonymous` |
| Dashboard/API | member session or token | member, or contact via `on_behalf_of` | `member_added` (vote trust) |

Widget actor resolution (`feedback_actor` middleware) for `/widget/v1/feedback/*`: valid `Dimmi-Feedback-Session` header bound to same widget > widget session identified > widget session anonymous > none (app-only, read). Header session for other widget or revoked: ignored, `meta.feedback_session: "invalid"` so SDK drops it.

Why not merge the email-proved contact into the widget lead: 02 rule R4/R7 and foundation merge rules keep widget conversation history bound to JWT identity. Feedback session grants feedback scope only, so a person proving email ownership acts as that email's contact for votes, posts, comments, subscriptions and notification feed, never sees conversations.

### Actions by trust (contact side)

| Action | none | anonymous | email_verified | identified |
|---|---|---|---|---|
| Read public boards, posts, roadmap (`access = public`) | yes | yes | yes | yes |
| Read when `access = signed_in` | no (sign-in page) | no | yes | yes |
| Vote | `anonymous_voting` on: creates anonymous session then votes; else 401 `sign_in_required` | only if `anonymous_voting` | yes | yes |
| Create post | 401 | 403 `trust_insufficient` | yes (board `submissions = open`; approval per board mode) | yes |
| Comment (public) | 401 | 403 | yes | yes |
| Edit/delete own post, comment | n/a | n/a | yes within `author_edit_window` (post edit also requires no votes from others, else members only) | same |
| Follow/unfollow post | 401 | 403 (no email) | yes | yes |
| Notification feed | no | yes (widget lead: in-widget only) | yes | yes |
| Email notifications | no | no | yes | yes when contact email present (verified or from JWT) |

Banned contact (`feedback_contact_states.banned_at` or `contacts.blocked_at`): read only; writes 403 `feedback_banned`. Member-authored state changes untouched.

## 2. Portal sessions

- Token `dmi_pts_` + 43 base64url (32 bytes). SHA-256 stored. Lookup before tenant known via `SECURITY DEFINER portal_resolve_session(bytea)` returning `(workspace_id, id)`; then workspace into context, `SET LOCAL app.workspace_id` at start of every tx. Host must resolve to same workspace (cookie from other workspace host never sent anyway; header session checked against widget workspace): mismatch = invalid.
- Cookie (portal surface): custom domain or `{sub}.{base_domain}` host: `__Host-dimmi_portal` (`Path=/`, `Secure`, `HttpOnly`, `SameSite=Lax`). Path mode `{app_url}/hc/{sub}`: `dimmi_portal_{hc_public_id}` with `Path=/hc/{sub}`, `Secure`, `HttpOnly`, `SameSite=Lax` (`__Host-` needs `Path=/`; dashboard cookie `__Host-dimmi_session` untouched). Plain-http self-host: `Secure` dropped with boot warning (foundation rule).
- TTL: identified and email_verified 30 days sliding (`last_seen_at` write throttled 60 s via Valkey gate `SET NX EX 60` per session, `expires_at` slide throttled 1 h); anonymous 180 days sliding. SSO session does not require fresh JWT to slide (portal has no host app context); revoked when secret revoked with `revoke_sessions`, contact merged/erased/blocked, banned, access changes to `signed_in` with domain restriction excluding contact email (`access_changed`), logout.
- One contact may hold many sessions (devices). List and revoke: member `PATCH /feedback-contact-states/{ctc} {revoke_sessions: true}`; contact `/feedback/account` "Sign out everywhere" (`DELETE /api/feedback/sessions` collection = all own sessions).
- `GET /api/feedback/sessions/current`: `{object: "portal_session", trust, contact: {name, email_masked, avatar_url}, expires_at, sign_in_methods}`. No session: 200 `{object: "portal_session", trust: "none", sign_in_methods: [...]}` (not 401: lets `fb.js` render without error noise).

## 3. Sign-in flows

`sign_in_methods` (array of objects, P6): `[{type: "sso", url: "https://app.acme.com/login?return_to=..."} , {type: "email"}, {type: "anonymous"}]`, only enabled ones.

### 3.1 SSO (02 JWT)

1. Host SSO route (`/api/dimmi/sso` on host app) signs JWT with widget identity secret and 02 claims plus `aud: "dimmi-portal"` (required on portal surface; widget tokens rejected `token_audience_mismatch`), `exp` 300 s, random `nonce` (single use), then 302 to portal fragment URL (step 2). Link helpers never mint: `createPortalLink({returnTo})` (`@dimmi/server`) and `portalLink({returnTo})` (`@dimmi/nextjs/server`) return host-route link `/api/dimmi/sso?return_to=<path>`; route = `@dimmi/nextjs/portal-sso` or `createPortalSsoHandler()` (`@dimmi/server`); env `DIMMI_PORTAL_URL` (01 sdk-contract.md, 02 snippets.md). Stacks without helper: own route minting with raw snippet (02 snippets.md).
2. Host route redirects user to `https://{portal}/feedback/sso#token=<jwt>&return_to=/feedback/posts/12`. Token only in URL fragment: never hits server logs.
3. `/feedback/sso` page: tiny nonce'd inline script reads fragment, `history.replaceState` strips it, POSTs `POST /api/feedback/sessions {grant: "sso", token, return_to}`; no JS: page shows "Enable JavaScript or sign in with email" (SSO needs fragment read). Host may instead render auto-submitting form `POST /api/feedback/sessions` (`application/x-www-form-urlencoded`, fields `grant=sso&token=...`) from own page (cross-origin check bypassed for `grant = sso` only: token itself is proof (`aud dimmi-portal`, `exp` 300 s, single-use `nonce`); `SameSite=Lax` cookie set on 303 response works for top-level POST navigation).
4. Server: `settings.sso_sign_in` and active identity secret exists, else 409 `sso_not_configured`. 02 `VerifyIdentityToken{Surface: portal}`: errors return 401 with 02 code and hint (same language). Then 02 `IdentifyContact` (find/create user contact by `sub`, apply claims). Contact banned/blocked: 403 `feedback_banned`. Create session (`grant sso`, `trust identified`, `external_id`, `identity_secret_id`, `auth_methods [jwt]`), set cookie, 201 JSON or 303 to `return_to` (relative path under portal prefix, else `/feedback`).
5. Portal "Sign in" button with `sso_login_url` set: link `sso_login_url?return_to=<urlencoded portal absolute url>`; host app authenticates user and redirects back to `/feedback/sso#token=...&return_to=...`. Recommended `sso_login_url` = `https://{host app}/api/dimmi/sso`: route shipped by SDK server packages (`@dimmi/nextjs/portal-sso`; `createPortalSsoHandler()` in `@dimmi/server` for any Fetch runtime incl. Express/Hono subpaths; other stacks copy raw snippet from 02 snippets.md): signed out -> host login with return; signed in -> mint token (step 1), 302 to portal fragment URL. `return_to` must be on portal host (helper checks against `DIMMI_PORTAL_URL`). Without `sso_login_url`: SSO option hidden on portal (only reachable by host-generated links).

### 3.2 Email (magic link + code)

1. `POST /api/feedback/sign-ins {email, return_to?}` (portal) or `POST /widget/v1/feedback/sign-ins {email}` (widget, needs widget session). Always 202 `{object: "portal_sign_in", id: "psi_...", expires_at}` (same body, padded timing) even when email domain not allowed, contact banned, or rate limited per email (IP rate limit 429 is the only visible error).
2. Checks (silent skip): `email_sign_in` true; domain in `email_sign_in_domains` when set; suppression scope `all`; per-email and per-IP rate. Passing: token (32 bytes) + 6-digit code; 04 `SendContactNotification{Kind: portal_sign_in}` with link `https://{portal}/feedback/sign-in/{psi}#token=<token>` and code. Widget surface: code only (link opens portal, also works and signs into portal).
3. Redeem: `POST /api/feedback/sessions {grant: "magic_link", sign_in_id, token}` (landing page script reads fragment, POSTs) or `{grant: "email_code", sign_in_id, code}` (form, works without JS; widget uses this). Wrong code: 422 `sign_in_invalid` with `attempts_left` (Valkey counter); 5 wrong: `portal_sign_ins.locked_at` set (Postgres), 429 `sign_in_locked`, new sign-in needed. Expired: 410 `sign_in_expired`. Consumed: 410. Token redemption is POST-only so mail link scanners (GET) never consume.
4. Contact resolution (`ResolvePortalEmailContact`): 03 `ResolveInboundContact` with `IdentityClaim{type: email, value, verified: true}`, kind lead, name empty. Outcomes: verified identity row exists: that contact (follow merge chain, skip deleted). Unverified row held by other contact: row moves to new or existing claimant and becomes verified (03 rule addition, README required edit 23); old holder keeps typed email text. No row: create lead with `email`, `email_verified = true`, verified email identity. Never merges contacts. Contact banned/blocked: 403 `feedback_banned` at redeem.
5. Session: `grant magic_link|email_code`, `trust email_verified`, `auth_methods [email_link]` or `[email_code]`. Widget surface returns `{token: "dmi_pts_..."}` once in body (widget stores in its storage key `dimmi:{app}:feedback`); portal surface sets cookie.

### 3.3 Widget hand-off (one click from widget to portal)

1. Widget (any session with trust identified or email_verified) calls `POST /widget/v1/feedback/portal-handoffs {return_to?}` -> 201 `{url: "https://{portal}/feedback/sso#handoff=<code>&return_to=..."}`. Code 32 bytes, `portal_sign_ins` kind `widget_handoff`, contact bound, 60 s, single use. Requires `portal_enabled`.
2. Portal `/feedback/sso` script sees `handoff`, POSTs `{grant: "widget_handoff", code}`. Session trust = trust of originating actor; `auth_methods [widget_session]` (+ underlying).
3. Anonymous widget lead: 403 `trust_insufficient` (no hand-off; anonymous portal vote flow instead).

### 3.4 Anonymous voting

Only when `anonymous_voting` true. First vote without session: `POST /api/feedback/posts/{fbp}/votes` with `{anonymous: true}` plus anti-bot proof:
- `fb.js` present: PoW (01 algorithm, difficulty from `widget.bot` config) in `X-Dimmi-Pow`; no JS form: honeypot field empty + per-IP limit stricter (`anonymous_votes_per_ip_per_day` / 4). All anonymous throttles on Valkey limiter.
- Creates lead contact (no email) + identity `portal_visitor` (value `pts_` id) + session `grant anonymous`, `trust anonymous`; cookie set; vote created with trust `anonymous`.
- Anonymous contact may later sign in by email or SSO: anonymous votes move to signed-in contact (`RepointFeedbackContact` from anon lead to signed-in contact, dedupe), anon session revoked `identity_switch`, lead kept (no conversations) and purged by 03 empty-lead rules if any. Same for widget anonymous lead when header session created: votes cast by lead in that widget session move to email-verified contact.

## 4. Portal pages (SSR, templ)

Routes registered in 08 help center route group (path mode prefix `/hc/{sub}` applies). Middleware: 08 `resolve_help_center` -> `resolve_portal` (settings, `portal_enabled`, help center match = `feedback_settings.help_center_id` or default help center; mismatch 404) -> `portal_session` -> `portal_csrf` (unsafe) -> `throttle:portal`.

| Method | Path | Route name | Page |
|---|---|---|---|
| GET | /feedback | portal.home | boards tabs, posts list (sort trending default, `?sort=top\|new\|trending`, `?status=`, `?category=`, `?q=`), "Submit idea" button, sign-in state |
| GET | /feedback/boards/{slug} | portal.boards.show | same list filtered to board, board description |
| GET | /feedback/posts/{number}-{slug} | portal.posts.show | post, status badge, vote button (form), comments (public, approved, threaded one level), subscribe toggle, merged posts list ("Also requested as #8"), status timeline (status_update comments) |
| GET | /feedback/posts/{number} | portal.posts.short | 301 to canonical slug URL; slug mismatch 301 too |
| GET | /feedback/new | portal.posts.create | submit form (board, title, body, category), similar posts appear via `fb.js`; no JS: server shows similar posts after first submit step (`?title=` preview step) |
| GET | /roadmap | portal.roadmap | columns by `on_roadmap` statuses, cards (title, votes, category); `?board=` |
| GET | /feedback/sign-in | portal.sign-in | methods; email form |
| GET | /feedback/sign-in/{psi} | portal.sign-in.landing | magic link landing (script POSTs token from fragment) + code form |
| GET | /feedback/sso | portal.sso | fragment reader (`token` or `handoff`) |
| GET | /feedback/account | portal.account | own votes, posts, notifications feed, email preference, sign out (all) |
| GET | /feedback/files/{att}/{filename} | portal.files.show | public post/comment images; 404 unless parent publicly visible; `Cache-Control: public, max-age=300` |

Merged post URL: 301 to target post URL. Trashed or rejected or private: 404 page (never 410 for posts: may be restored). Post in `pending`: visible only to its author (session) with banner "Awaiting review"; everyone else 404.

Board slug for private board on portal: 404 (same as unknown).

### JSON + form API on help center host (`/api/feedback/*`)

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | /api/feedback/posts | portal.api.posts.index | portal (public or session per access) | list; `filter[board]`, `filter[status]`, `filter[category]`, `filter[mine]=voted\|authored`, `q`, `sort`, cursor |
| GET | /api/feedback/posts/{fbp} | portal.api.posts.show | portal | |
| GET | /api/feedback/similar-posts | portal.api.similar-posts.index | portal | `?title=` (3 to 150 chars), max 5, public posts only |
| POST | /api/feedback/posts | portal.api.posts.store | session email_verified+ | create (form or JSON) |
| PATCH | /api/feedback/posts/{fbp} | portal.api.posts.update | own post, edit window | title, body |
| DELETE | /api/feedback/posts/{fbp} | portal.api.posts.destroy | own post, no other votes | trash |
| POST | /api/feedback/posts/{fbp}/votes | portal.api.posts.votes.store | session (or anonymous flow) | idempotent: existing vote 200 |
| DELETE | /api/feedback/posts/{fbp}/votes/{fbv} | portal.api.posts.votes.destroy | own vote | |
| GET | /api/feedback/posts/{fbp}/comments | portal.api.posts.comments.index | portal | public approved (+ own pending) |
| POST | /api/feedback/posts/{fbp}/comments | portal.api.posts.comments.store | session email_verified+ | `{body, parent_id?, attachment_ids?}` |
| PATCH | /api/feedback/comments/{fbc} | portal.api.comments.update | own, edit window | |
| DELETE | /api/feedback/comments/{fbc} | portal.api.comments.destroy | own | |
| PATCH | /api/feedback/posts/{fbp}/subscription | portal.api.posts.subscription.update | session email_verified+ | `{subscribed: bool}` singleton (viewer) |
| GET | /api/feedback/roadmap | portal.api.roadmap.show | portal | |
| POST | /api/feedback/sign-ins | portal.api.sign-ins.store | guest | 202 always |
| POST | /api/feedback/sessions | portal.api.sessions.store | guest | grant `sso`, `magic_link`, `email_code`, `widget_handoff` (anonymous created by vote) |
| GET | /api/feedback/sessions/current | portal.api.sessions.current.show | any | trust none allowed |
| DELETE | /api/feedback/sessions/current | portal.api.sessions.current.destroy | session | logout |
| DELETE | /api/feedback/sessions | portal.api.sessions.destroy-many | session | sign out everywhere (own sessions) |
| GET | /api/feedback/notifications | portal.api.notifications.index | session | feed |
| PATCH | /api/feedback/notifications | portal.api.notifications.update-many | session | `{filter: {before?}, changes: {read: true}}` or `{ids, changes}` |
| PATCH | /api/feedback/preferences | portal.api.preferences.update | session email_verified+ | `{email_updates: bool}` (sets/clears `email_opt_out_at`) |
| POST | /api/feedback/attachments | portal.api.attachments.store | session email_verified+ | multipart image, max 10 MB; unattached purged 24 h |

Form posts (`application/x-www-form-urlencoded` or multipart) on any unsafe route: same controller, same validation; success 303 to `return_to` (relative), validation error re-renders page with errors (422 status, HTML). JSON requests get JSON. Content negotiation by `Content-Type`/`Accept`.

## 5. Rendering, caching, SEO

- templ views share 08 theme (brand color vars, light/dark, system fonts, CSS from 08 `/_static` hashed bundle + `portal.css` under 6 KB gz). No web fonts. Optional `fb.js` (under 6 KB gz): similar posts as you type, vote/subscribe without reload, fragment readers, PoW for anonymous vote, relative time.
- Lists: 20 posts per page, cursor links (`?cursor=`), `rel=next/prev`.
- Vote button: `<form method=post action=/api/feedback/posts/{fbp}/votes>` (no CSRF field: `http.CrossOriginProtection` checks `Sec-Fetch-Site`/`Origin`, README Security); voted state `aria-pressed`.
- Anonymous (no portal cookie) GET pages cached in otter: key `(workspace_id, feedback cache_version, help center cache_version, host kind, path, normalized query)`, TTL 15 s, `Cache-Control: public, max-age=0, must-revalidate`, ETag. Signed-in: `private, no-store`, never cached server side.
- `feedback_settings.cache_version` bumped (after commit, `pubsub.PubSub` on Valkey topic `feedback.cache`; each replica drops L1 entries) on: post create/approve/reject/status/merge/trash/restore/update of public fields, board/category/status changes, settings changes. Vote count changes do not bump (15 s TTL bounds staleness; avoids cache churn on hot posts).
- SEO: post page `<title>` = `{title} | {portal_title} | {help center name}`, description first 155 chars of body plaintext, canonical primary host, OG/Twitter, JSON-LD `DiscussionForumPosting` (headline, text excerpt, datePublished, interactionStatistic LikeAction count when counts shown, comment count). Roadmap `ItemList`. `noindex` when help center indexing off, access `signed_in`, post pending, search query pages, sign-in/sso/account/new pages.
- No `.md` twins or llms.txt portal sections at MVP (later: portal-md-twins).
- Access `signed_in`: every portal page except sign-in/sso/landing redirects (302) to `/feedback/sign-in?return_to=...`; sitemap entries omitted; JSON API 401 `sign_in_required`.
- CSP: 08 policy + `form-action 'self'`; portal inline scripts only with nonce (fragment readers); `frame-ancestors` from settings.

## 6. Widget feedback tab (`/widget/v1/feedback/*`)

Owned here; mounted in 01 widget route group (origin check, `throttle:widget`). Enabled only when `widget_settings.feedback_tab_enabled` (else 404 `feedback_disabled`). Independent of `portal_enabled` (widget-only feedback allowed). Boards = public boards with `widget_visible`.

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | /widget/v1/feedback | widget.feedback.show | app or widget | home: boards, viewer `{trust, can_vote, can_post, sign_in_methods}`, unread count, portal URL |
| GET | /widget/v1/feedback/boards | widget.feedback.boards.index | app or widget | |
| GET | /widget/v1/feedback/posts | widget.feedback.posts.index | app or widget | list, `q`, sort, filters, `filter[mine]` (session) |
| GET | /widget/v1/feedback/similar-posts | widget.feedback.similar-posts.index | app or widget | `?title=` |
| POST | /widget/v1/feedback/posts | widget.feedback.posts.store | widget (trust email_verified+) + `idempotent` | create; `attachment_ids` from 01 widget uploads |
| GET | /widget/v1/feedback/posts/{fbp} | widget.feedback.posts.show | app or widget | |
| POST | /widget/v1/feedback/posts/{fbp}/votes | widget.feedback.posts.votes.store | widget | idempotent |
| DELETE | /widget/v1/feedback/posts/{fbp}/votes/{fbv} | widget.feedback.posts.votes.destroy | widget | |
| GET | /widget/v1/feedback/posts/{fbp}/comments | widget.feedback.posts.comments.index | app or widget | |
| POST | /widget/v1/feedback/posts/{fbp}/comments | widget.feedback.posts.comments.store | widget (email_verified+) | |
| PATCH | /widget/v1/feedback/comments/{fbc} | widget.feedback.comments.update | widget (own) | |
| DELETE | /widget/v1/feedback/comments/{fbc} | widget.feedback.comments.destroy | widget (own) | |
| PATCH | /widget/v1/feedback/posts/{fbp}/subscription | widget.feedback.posts.subscription.update | widget (email_verified+) | `{subscribed}` |
| GET | /widget/v1/feedback/roadmap | widget.feedback.roadmap.show | app or widget | |
| GET | /widget/v1/feedback/notifications | widget.feedback.notifications.index | widget | contact feed |
| PATCH | /widget/v1/feedback/notifications | widget.feedback.notifications.update-many | widget | mark read |
| POST | /widget/v1/feedback/sign-ins | widget.feedback.sign-ins.store | widget | email code (202) |
| POST | /widget/v1/feedback/sessions | widget.feedback.sessions.store | widget | `{grant: "email_code", sign_in_id, code}` -> `{token}` |
| DELETE | /widget/v1/feedback/sessions/current | widget.feedback.sessions.current.destroy | header session | drop email-verified session |
| POST | /widget/v1/feedback/portal-handoffs | widget.feedback.portal-handoffs.store | widget (identified or email_verified) | one-click portal URL |

Widget-safe resources = public resources (same as portal). App-only reads: `Cache-Control: public, max-age=30`. Session reads: `private`.

SDK surface (01 sdk-contract.md, binding): `show('feedback', {post?: number, board?: string})` opens tab, post or board; `show('roadmap')` opens roadmap inside tab; deep links `#dimmi-open=feedback` / `#dimmi-open=roadmap`; `data-dimmi="show" data-dimmi-view="feedback"`. Feedback session token in localStorage `dimmi:{app}:feedback`, sent as header `Dimmi-Feedback-Session` (CORS allowed header); `meta.feedback_session: "invalid"` makes SDK drop it. Lazy chunk `feedback` at most 15 KB gz. Wire bodies snake_case; SDK payloads camelCase. No realtime on widget tab at MVP (unread via session payload `feedback_unread_count` + refetch on open).

## 7. Portal for Dimmi itself (dogfood)

Dimmi cloud workspace `dimmi` runs portal on `feedback.trydimmi.com` (custom domain on its help center), SSO from dashboard ("Feedback" menu item: dashboard backend mints portal JWT for logged-in user, `aud dimmi-portal`, `exp` 300 s, `nonce`, `sub = usr_` id, `email_verified: true`; same token shape as SDK `/api/dimmi/sso` route), widget tab in dashboard Messenger, changelog links from 13. Demo script in tasks.md uses same setup.

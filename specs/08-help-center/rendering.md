# 08 Rendering, SEO, AI-readable output, domains, public MCP

Style: caveman. Server-side only. Scope trim 2026-11-02: changelog-purpose domain (B12), public MCP provider tools (B13), 14 llms.txt sections (C10) out. Templates templ (`resources/views/helpcenter`, `framework/view`), Markdown `framework/markdown` (goldmark) with help center options `app/support/helpmarkdown`.

## 1. Markdown dialect (canonical storage)

Stored `body_markdown` = CommonMark 0.31 + GFM (tables, strikethrough, task lists, autolinks) + extensions:

| Feature | Syntax | Render |
|---|---|---|
| Alerts | `> [!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]`, `[!CAUTION]` (GitHub syntax) | `<aside class="alert alert-note" role="note">` |
| Heading anchors | auto (goldmark `WithAutoHeadingID` inside `framework/markdown`, unicode-safe, dedupe `-1`); TOC via go.abhg.dev/goldmark/toc | `<h2 id="...">` + anchor link |
| Code | fenced with lang | server-side highlight (`markdown.Options.Highlight`: chroma v2 via goldmark-highlighting, CSS classes, no inline style) |
| Images | `![alt](url "title")` | `<img loading="lazy" decoding="async" width height>` (dims from asset row); asset URLs rewritten to current host |
| Internal links | `[x](/articles/slug)` or `[x](art_...)` | resolved to current host path; broken internal link -> render text + publish warning `broken_link` |
| Embeds | bare YouTube/Loom/Vimeo URL on own line | MVP: render as link card (no iframe; CSP frame-src none). Iframe embeds later |
| Footnotes | GFM footnotes | yes |
| Raw HTML | any | escaped as text, never rendered |
| Math, Mermaid | | rendered as code blocks (later) |

Why Markdown canonical: AI drafts, Git import, diffs, `.md` twins, llms.txt all native; Tiptap 3 (frontend later) round-trips via its Markdown extension. Frontend sends Markdown in API, never HTML/JSON.

Sanitizer after render (defense in depth): `framework/sanitize` UGC policy (bluemonday) + allow `id` on headings, `class` from fixed allowlist (`alert*`, `chroma*`, `task-list*`), `loading`, `decoding`, `width`, `height`. Renderer version constant `RendererVersion` bump -> boot enqueues `RerenderArticleVersions` (body_html regenerated, cache bust).

Publish-time warnings (non-blocking, returned in publish response and preview): `missing_alt` (image without alt text), `broken_link`, `heading_skip` (H2 to H4), `empty_body`, `long_title` (over 70 chars SEO), `missing_description`, `external_http_image` (http not https: blocked at render).

## 2. URL scheme

| Page | Path | Notes |
|---|---|---|
| Home | `/` | |
| Collection | `/collections/{slug}` | sub-collections same scheme (slug unique per help center) |
| Article | `/articles/{slug}` | no numeric id; redirects table preserves history |
| Markdown twins | `/index.md`, `/collections/{slug}.md`, `/articles/{slug}.md` | llmstxt.org convention (`.md` appended to extensionless path) |
| Search | `/search?q=` | `noindex` |
| Ask | `/answers?q=` | `noindex` |
| Locale (post-MVP: auto-translation) | `/{locale}/articles/{slug}` | default locale unprefixed forever, so MVP URLs never change |

Trailing slash: 301 to without. Uppercase slug: 301 to lowercase. Unknown path: redirect table lookup, else 404 page. Archived/trashed article without redirect: 410 page (search engines drop). `agent_only` article: 404 (never reveal existence).

Path mode (`{app_url}/hc/{subdomain}/...`): identical tree under prefix; all generated links include prefix. Canonical always primary host.

## 3. Host resolution and canonical

Middleware `resolve_help_center`:
1. Host equals active custom domain -> help center.
2. Host `{sub}.{base_domain}` -> help center by subdomain.
3. App host with `/hc/{sub}` prefix -> help center (path mode).
4. Else 404 (plain, no help center info).

Primary host = custom domain if `primary_domain_id` set and active, else `{sub}.{base_domain}` if base_domain configured, else app host path mode. Requests on non-primary hosts for HTML/MD pages -> 301 to primary (same path). Exceptions not redirected: `/robots.txt` (serves Disallow for non-primary, prevents duplicate indexing), `/mcp`, `/api/*` (avoid breaking clients mid-switch), `/_static`, `/assets` (served everywhere).

Host map cached in-process (L1; hostname -> help_center_id, primary host), TTL 60 s, busted on `help_center.updated` and domain status change via `pubsub.PubSub` (Valkey).

13 changelog lives under `/changelog` on every help center host (no own domain at MVP; `changelog-custom-domain` later).

Public section providers (`helpcenter.PublicSectionProvider`, README HTTP API): routes mounted inside help center group after 08 routes, before redirect/404 fallback; host resolution, path mode and canonical rules identical (provider pages redirect to primary host like articles; `/api/*` never redirected).

## Audiences (18 M7)

Article and collection audience: `access` gate, verified values only. Rules binding in [18](../18-audiences/README.md) "Help center behaviour", "Help center identity".

- Reader identity: widget session in Help tab; else `__Host-dimmi_hc` from SSO fragment, 02 surface `help_center`, `aud: dimmi-help`, needs `help_centers.login_url`.
- Filter every output: HTML, `.md` twin, `Accept: text/markdown`, sitemap, `llms.txt`, `llms-full.txt`, search, suggestions, Ask AI, public MCP. Canary title grepped in anonymous response of every path.
- Page cache `variant` gains hash of reader's allowed set. Public responses unchanged. Restricted responses `Cache-Control: private, no-store`; public cache never sees restricted content.
- Unknown and restricted slug, anonymous: same 404. `login_url` set: same page with sign-in button, status 404. Signed in, no match: 404. Restricted branch does no extra rendering. `noindex`, old sitemap entry removed.
- Collection with no visible article disappears for that reader. All restricted: home shows empty state.

## 4. Pages and SEO

Per page `<head>`:
- `<title>`: article `seo.title || title` + ` | ` + help center name (max 60 chars total target, never truncated server side).
- `<meta name="description">`: `seo description || description || first 155 chars of plaintext body` (word boundary).
- `<link rel="canonical" href="https://{primary}{path}">`.
- `<link rel="alternate" type="text/markdown" href="{path}.md">` and HTTP `Link: <{path}.md>; rel="alternate"; type="text/markdown"`.
- `<link rel="describedby" href="/llms.txt">` (llmstxt.org discovery hint).
- OpenGraph: `og:type` (article/website), `og:title`, `og:description`, `og:url` (canonical), `og:image` (article og image > help center og image > none), `og:site_name`, `article:modified_time`. Twitter: `summary_large_image` when image else `summary`.
- JSON-LD: `Article` (headline, dateModified, datePublished, author = help center name as Organization, image), `BreadcrumbList` (home > collection > sub > article), home: `WebSite` with `SearchAction` (`/search?q={search_term_string}`).
- `<html lang="{locale}">`, `<meta name="viewport">`, theme-color from brand.
- `noindex` cases: help center `indexing_enabled=false`, article `seo.noindex`, search/ask pages, non-primary hosts. Sends both meta and `X-Robots-Tag`.

Theme: one built-in template. CSS custom properties from brand color (contrast checked: text on primary auto black/white by WCAG 4.5:1). Light/dark via `prefers-color-scheme` + `settings.theme.appearance` force. System font stack (no web font download). Article page: breadcrumb, title, updated date ("Updated 3 days ago" rendered as absolute date server side, `<time datetime>`), TOC (H2/H3, sticky on wide screens, CSS only), body, "Was this helpful?" (two `<form method=post>` buttons, works without JS; hc.js upgrades to fetch), prev/next in collection, "Still need help?" with Ask AI box and contact link (`settings.contact_url` or widget open when `show_widget`).

Accessibility: skip link, landmarks, focus styles, heading order, alt text required warnings, color contrast checked at settings save (warn).

`/sitemap.xml`: public published articles + collections with ≥1 public published article + home; `<lastmod>` = published_at. Plus provider URLs (`PublicSectionProvider.SitemapURLs`: 13 changelog entries + label pages, 14 `/feedback`, `/roadmap`, public boards, public approved posts with `lastmod` = `last_activity_at`) when that section is enabled and public. Over 50,000 URLs (or any provider present with many URLs) -> sitemap index `/sitemap.xml` + `/sitemaps/{n}.xml` for articles and `/sitemaps/{section}-{n}.xml` per provider (protocol limit 50,000 URLs / 50 MB per file). Not served when indexing disabled (404).

`/robots.txt`:
```
User-agent: *
Allow: /
Disallow: /search
Disallow: /answers
Disallow: /api/
# + provider rules (14: Disallow /feedback/sign-in, /feedback/sso, /feedback/account, /feedback/new)
Sitemap: https://{primary}/sitemap.xml
```
Indexing disabled or non-primary host: `User-agent: *` + `Disallow: /`. AI crawlers treated same as others (owner controls via indexing toggle; per-bot rules later).

## 5. llms.txt, llms-full.txt, `.md`

`/llms.txt` (format per llmstxt.org: H1 required, blockquote summary, H2 file lists `[name](url): notes`):
```markdown
# Acme Help Center

> Help articles for Acme, the invoicing app. Markdown versions of every article are linked below. Full text in /llms-full.txt.

Ask questions programmatically via MCP at https://help.acme.com/mcp (read-only).

## Getting started

- [Create your account](https://help.acme.com/articles/create-account.md): Sign up and verify email.
- [Invite teammates](https://help.acme.com/articles/invite-teammates.md): Add people to your workspace.

## Billing

- [Change your plan](https://help.acme.com/articles/change-plan.md): Upgrade or downgrade.

## Billing / Refunds

- [Request a refund](https://help.acme.com/articles/request-refund.md): How refunds work and how long they take.

## Optional

- [Uncategorized article](https://help.acme.com/articles/x.md): ...
```
- Note per link = article description (or first sentence), single line, max 200 chars.
- Sub-collections flattened into own H2 section `## Billing / Refunds` (llmstxt.org allows only H2 file-list sections; no H3).
- Uncollected public articles under `## Optional`.
- Provider sections appended before `## Optional` (`PublicSectionProvider.LlmsTxtSection`): 13 `## Changelog` (latest entries `.md`). Omitted when section disabled, not public, or empty. 14 `## Roadmap` / `## Feedback` later (`portal-md-twins`).
- MCP line omitted when `public_mcp_enabled=false`.

`/llms-full.txt`: every public published article concatenated in nav order:
```
# {Article title}
Source: https://{primary}/articles/{slug}
Updated: 2026-10-30

{markdown body, images absolute URLs, internal links absolute}

---
```
Cap 5 MB. Over cap: `/llms-full.txt` contains header + per-collection index pointing to `/collections/{slug}/llms-full.txt` (each capped 5 MB, further overflow truncated with final line `[truncated: see /llms.txt for per-article Markdown]`). Generated on demand, cached under cache_version, `Content-Type: text/plain; charset=utf-8`.

`.md` twins: `Content-Type: text/markdown; charset=utf-8`, body = title as H1 + description blockquote + Markdown body with absolute URLs + footer `Source: <canonical URL>`. Same visibility rules as HTML (agent_only 404). `Accept: text/markdown` (q higher than text/html) on HTML path returns same with `Vary: Accept`.

Collection `.md`: H1 collection name, description, list of article links (`.md` URLs). Home `.md`: same as llms.txt minus Optional section.

## 6. Caching

| Layer | Key | Invalidation |
|---|---|---|
| Version body HTML | `article_versions.body_html` | immutable; rerender job on renderer bump |
| Nav tree per help center | `(hc_id, cache_version)` | version bump |
| Page cache (otter, 32 MB LRU, cost = bytes) | `(hc_id, cache_version, host_kind, path, variant)` | version bump makes old keys unreachable (evicted LRU) |
| Host map | hostname | 60 s TTL + Valkey pubsub bust |
| HTTP | strong `ETag` = hash(cache_version, path, variant); `Cache-Control: public, max-age=0, must-revalidate` for HTML/MD; `public, max-age=31536000, immutable` for `/_static` and `/assets` | 304 on match |

`cache_version` bump (action `BumpHelpCenterCache`, after commit) on: publish, unpublish, archive, delete, restore-with-publish, collection change, slug change, redirect change, help center settings, domain primary change, asset alt text change on referenced asset. Bump publishes `pubsub.PubSub` (Valkey) message `help_center.cache {hc_id, cache_version}`; replicas update local version map immediately (no wait for 60 s TTL). Missed message (Valkey down or flushed): page served from stale cache max `help_center.cache.ttl` (10 min) because each cache entry also validates cache_version from host map refreshed every 60 s. Effective worst case 60 s staleness, normal under 1 s.

Stampede: singleflight per cache key on miss.

No CDN purge at MVP. Cloud may front with CDN honoring `max-age=0` + ETag revalidation (origin 304 under 5 ms).

Search, ask, API, MCP responses: not page-cached (search results cached 30 s per normalized query per help center in otter L1, invalidated by version bump; query embeddings L2 on Valkey, knowledge.md section 5).

## 7. Ask AI

Endpoint `POST /api/answers` on help center host (SSE; noun for computed output, rest.md rule 11) and `POST /answers` form (no-JS, waits for full answer, renders page; 30 s timeout then "Try search" message).

Request `{ "question": "How do I get a refund?", "article_id": "art_..."? }` (article context when asked under article). Question 3 to 500 chars.

Flow:
1. Rate limits on `ratelimit.Limiter` (Valkey): per IP minute/day, per help center day. Exceeded -> 429 `ask_rate_limited`. Valkey unavailable: per-help-center day cap falls back to Postgres count of today's `ai_runs` kind `help_center` (BYOK spend guard); per-IP limits fail open.
2. `help_center.ask_ai_enabled` and 07 readiness (`ai.CheckAgentReadiness`, cached) else 409 `ai_unavailable` (hc.js hides box when `/api/search` response field `ask_ai: false`; status embedded in page at render so box not shown).
3. Calls 07 `ai.AnswerQuestion(ctx, AnswerQuestionInput{WorkspaceID, HelpCenterID, Question, ContextArticleID, Kind: "help_center"})`; 07 forces `Surface: ask_ai`, `Visibility: [public]`, `HelpCenterIDs: [hc]`, `ExcludeSources` (anonymous visitors never get agent_only content). Returns stream of events: `delta{text}`, `citations[{title,url}]` (public article URLs only), `done{answered: bool, run_id}`, `error{code}`. 07 owns prompt, grounding, "I don't know", spend cap.
4. SSE events forwarded as `event: delta|citations|done|error`. Server sends `delta` text while streaming and, on `done`, sanitized `html` of full answer (same `framework/markdown` + `framework/sanitize` pipeline). hc.js shows plain text while streaming, swaps to sanitized HTML on done. No client Markdown library.
5. `done.answered=false` -> UI shows "Talk to us" (widget opens composer via SDK `startConversation(question)` when widget on page (`show_widget`), else `#dimmi-open=messages` deep link, or `contact_url`).
6. Logged: help_center_searches row surface `ask_ai` (query, result_count = citations count). 07 records ai_run (no conversation).

Not a conversation, never billable. Token cost to workspace provider, counted in 07 spend cap.

## 8. Custom domains and TLS

### Add

`POST /help-centers/{hc}/domains {hostname}`. Mechanics below live in `framework/customdomain` (generic); claims, reserved names and authorization in `app/support/customdomains`:
- Normalize: trim, lowercase, strip trailing dot, IDNA to A-label (UTS 46), max 253, at least one dot, no IP literal, no port, no wildcard.
- Reject: base_domain or its subdomains (`domain_is_platform`), app host, `localhost`, public suffixes themselves (`co.uk`), hostname claimed by any `help_center_domains` or `widget_domains` row (`customdomains.Claimed`, `domain_taken`; if pending elsewhere > 7 days without ownership, stale claim deleted and new claim allowed).
- Limit 1 per help center (config `help_center.domains.max_per_help_center`; 422 `domain_limit_reached`).
- Response: DNS records to create:
```json
{
  "records": [
    { "type": "TXT", "name": "_dimmi-verify.docs.acme.com", "value": "dimmi-verify=k3j9...", "purpose": "ownership" },
    { "type": "CNAME", "name": "docs.acme.com", "value": "domains.dimmi.help", "purpose": "routing" }
  ],
  "apex": false
}
```
Apex hostname (no subdomain label beyond registrable domain): routing record `A`/`AAAA` to `help_center.domains.a_records` (if configured) or ALIAS/ANAME to cname_target with note; if neither configured -> 422 `apex_not_supported` with hint "use docs.yourdomain.com".

### Verify

`VerifyHelpCenterDomain` job (on add, manual check `POST /help-centers/{hc}/domains/{hcd}/checks`, schedule):
1. TXT lookup `_dimmi-verify.{hostname}` via `framework/dns` resolver (fakeable in tests) (system resolver + fallback to 1.1.1.1/8.8.8.8 over DoH not used; plain DNS UDP/TCP to configured resolvers `help_center.domains.resolvers` default system). Must contain exact value. Ownership stays verified while TXT present (rechecked daily).
2. Routing: CNAME chain resolves to cname_target, or A/AAAA set subset of configured IPs. Cloudflare proxied (orange cloud) records resolve to Cloudflare IPs: detected (ASN list not used; detect by HTTP probe below) -> accept if HTTP probe succeeds.
3. HTTP probe: `GET http://{hostname}/.well-known/dimmi-domain-check/{token}` via `a.HTTP` (SSRF guard, public resolution) must return token from this server (proves traffic reaches Dimmi, covers proxies). Probe passing alone (without CNAME match) counts as routing OK.
4. CAA check: if CAA records exist and exclude `letsencrypt.org` (or configured CA) -> `caa_blocks` error with fix text.
5. Ownership + routing OK -> `tls_status=pending`, trigger TLS (certmagic: `Ensure` (certmagic `ManageAsync`) for hostname; caddyask: nothing, Caddy issues on first handshake, Dimmi pre-warms by making one HTTPS request to itself through public DNS). Issuance success -> `active`, `tls_issued_at`, `primary_domain_id` set if none, event `help_center.domain_status_changed`, cache bust.
6. Failures: `customdomain` backoff schedule 1, 2, 5, 10, 30 min, then hourly for 72 h, then `failed` (manual `POST /help-centers/{hc}/domains/{hcd}/checks` restarts). `last_error_code` shown with exact fix instructions.

### Ongoing

- Daily recheck of active domains. TXT removed or routing broken 3 consecutive daily checks -> `failed`, stop serving (host resolves to 404), primary falls back to subdomain, email owner/admins (system stream Mailable `HelpCenterDomainFailedMail` to members only, `a.Mail`). Prevents serving tenant content on domain they no longer control and frees hostname for legit owner.
- Remove: status `removed`, certificate deleted from storage, primary falls back, hostname free immediately.

### TLS (`framework/customdomain`)

Generic TLS manager in framework (built; no product words; app passes `customdomain.Authorizer func(ctx, host) error` = `AuthorizeTLSHostname` via `foundation.Options.Domains`, nil refuses all). Drivers (config `custom_domains.default` naming an instance under `custom_domains.tls.<name>.driver`): `certmagic` | `caddyask` | `none`; `customdomain/drivertest` suite (pebble) runs against all. Replaces former app `TLSProvisioner` contract and `app/drivers/domaintls/*`.

```go
// framework/customdomain
type TLS interface {
    Ensure(ctx context.Context, host string) error          // start/confirm issuance (async ok)
    Remove(ctx context.Context, host string) error
    TLSConfig() *tls.Config                                 // for a TLS listener (certmagic); nil otherwise
    HTTPHandler(next http.Handler) http.Handler             // wraps :80 for HTTP-01 challenges
}
// a.Domains (*customdomain.Manager): Default(), Setup(name), Ensure, Remove, TLSConfig, HTTPHandler
```
- `certmagic` driver (caddyserver/certmagic v0.25.6, embedded; config `email`, `ca`; certmagic logs bridged through go.uber.org/zap into framework log): Dimmi terminates TLS on :443 (cloud behind NLB TCP passthrough; self-host when `http.tls.mode=auto`). HTTPS listener wiring in foundation is a framework follow-up (driver + `TLSConfig()` built, listener not). On-demand issuance with `DecisionFunc(ctx, name)` -> app authorizer (`AuthorizeTLSHostname`: ownership verified `help_center_domains` of any purpose, status pending/active, plus app host; base_domain wildcard via DNS-01 configured by 11/ops). Storage: Postgres `certmagic_storage` (`key`, `value`, `modified`; framework migration set, `framework/customdomain/storage`) implementing certmagic `Storage` + `Locker` (Postgres advisory locks, no lock rows/`lock_expires_at`; ~150 lines custom: only unlicensed community impl exists) so all replicas share certs and ACME account; certs durable, never Valkey. HTTP-01 on :80 via `HTTPChallengeHandler`; TLS-ALPN-01 on :443. Let's Encrypt limits: certmagic internal throttle; `Ensure` max 20 new hostnames per hour per instance (well under 300 orders / 3 h per account); duplicate-cert limit avoided by storage reuse (never re-issue on restart).
- `caddyask` driver: Caddy in front (compose profile from 11) with `on_demand_tls { ask http://dimmi:{admin_port}/internal/tls/ask }`. Ask endpoint `GET /internal/tls/ask?domain=` (`caddyask.Driver.AskHandler()`) returns 200 when authorizer allows, else 404. Admin port only. TLS status (app) derived from HTTPS probe.
- `none`: domains work only with user-managed TLS proxy in front; dashboard shows "TLS handled by your proxy" and skips issuance (status active after routing check over HTTP probe).
- Later: `cloudfront` driver (CloudFront SaaS Manager tenants, ACM certs) for cloud edge caching.

Base domain wildcard `*.{base_domain}`: one wildcard cert via DNS-01 (ops/11 config; certmagic DNS provider for Route 53 on cloud). Not per help center.

## 9. Public MCP (decision: MVP, minimal)

Decision: ship in MVP. Reasons:
- Audience builds dev tools; their customers code in Cursor/Claude Code. "Your docs are an MCP server" is unique in this set (roadmap standout "personal-agent channel"; Productlane, Ferndesk lack it).
- Cheap: 3 read-only tools over existing `contracts.Knowledge.Search` (`[public]`, this help center only, `ExcludeSources`) and article reads; MCP 2026-07-28 revision is stateless (no sessions), so endpoint is plain POST handler; reuse 09 MCP runtime (`framework/mcp`).
- Low risk: exposes only content already public; no auth, no writes, no LLM token spend (no ask tool).

Endpoint `POST https://{help host}/mcp` (streamable HTTP). No auth. Disabled -> 404. Rate limit per IP 60/min (`ratelimit.Limiter` on Valkey).

| Tool | Input | Output |
|---|---|---|
| dimmi_search_docs | `query` (1..300), `limit` (1..10, default 5) | `[{kind (article \| changelog), title, url, markdown_url, snippet (max 600 chars), section, published_at?}]` (changelog chunks are public; `ExcludeSources` keeps them) |
| dimmi_get_doc | `url` or `slug` | full Markdown + title, url, updated_at; max 256 KB. Resolves article URLs/slugs and changelog entry URLs/slugs (`changelog/{slug}`) via provider lookup |
| dimmi_list_docs | `collection` slug optional, `cursor` | `[{title, url, description, collection}]`, 100 per page |

Provider MCP tools (13 `dimmi_list_updates`): later (`public-mcp-changelog`).

Server info name = help center name, instructions = "Read-only documentation for {name}. Use dimmi_search_docs first." Resources/prompts none. Tool results tagged as untrusted content in descriptions (customer docs). Search uses `ModeAuto` with 800 ms embed timeout (hybrid when available, workspace BYOK embedding cost small; per-IP limit bounds it) and logs searches surface `public_mcp`.

Personal Agent Protocol / PACT support: later (landscape notes), not MVP.

## 10. Widget Help tab payloads (01 consumer)

- `GET /widget/v1/help`: `{help_center: {name, url, ask_ai: bool}, collections: [{id, name, icon, description, article_count}], popular: [{id, title, description}] (top 5 by 30-day views, fallback first 5 by position)}`.
- `GET /widget/v1/help/articles/{art}` (deep-open target of SDK `show('help', {article})` and 15-announcements-surveys CTA `open_article`; not found: 404, widget shows Help home + toast): `{id, title, description, html (sanitized body_html with absolute URLs, links target _blank for external), toc, url, updated_at, collection: {id, name}}`.
- `GET /widget/v1/help/search?q=&instant=1`: keyword mode, public only, max 10 `{id, title, snippet_html (ts_headline with <mark>), url}`.
- Widget-only views recorded as `widget_views`.
- Help center = widget's `widget_settings.help_center_id` (01), resolved from `app_id`; no visitor session needed. Help tab hidden by 01 when that help center has zero public published articles (`collections` empty and `popular` empty).

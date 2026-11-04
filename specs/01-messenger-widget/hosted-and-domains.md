# 01 Messenger widget: hosted page, native clients, custom domain

Style: caveman. Three ways to reach same widget beyond a script tag on customer's site.

## Clients without Origin (trusted domains rule)

`allowed_origins` = trusted domains: browser defense, not authentication. Replaces earlier "mobile app keys" idea.

| `Origin` header | `allowed_origins` | `native_apps_enabled` | Result |
|---|---|---|---|
| present | empty | any | allowed |
| present | matches, or equals this install's own host | any | allowed |
| present | no match | any | 403 `origin_not_allowed` |
| absent | empty | any | allowed |
| absent | non-empty | true | allowed, recorded as origin `app` in install tracker |
| absent | non-empty | false | 403 `origin_not_allowed` |

No separate mobile key. Why: key ships inside app binary, Origin is trivially spoofed by non-browsers, so key adds rotation work and zero security. Identity security comes from JWT (02). Bot checks, PoW and rate limits apply equally.

## Hosted page

Migration `20261201010008_add_transferred_from_to_widget_sessions_table.sql` adds nullable self-reference to `widget_sessions`.

Full-page messenger URL: an app opens it in a WebView, a site links to it, an email can point to it.

`GET /messenger/{app}` route `widget.hosted.show`, web group, no cookies, `routes/widget_hosted.go`, controller `app/http/controllers/web/hosted_messenger_controller.go`, templ `resources/views/widget/hosted.templ`. Not REST (rest.md web page exception). `hosted_page_enabled = false` or widget disabled or deleted: 404 page, no widget detail.

- Page: minimal HTML, `<script src=/widget/v1/loader.js data-app-id data-mode="page">`, `lang` and `dir` from locale, `theme-color` = header colour, title = company name, viewport fit cover.
- `data-mode="page"`: launcher never renders, panel fills viewport, close button hidden, no animation, safe-area insets. Same core, same budgets, no extra chunk.
- Query params (non-secret): `view=messages|help|changelog|feedback`, `conversation=conv_...`, `article=art_...`, `locale=fr`, `theme=light|dark`. Unknown ignored.
- Headers: `X-Robots-Tag: noindex`, `Referrer-Policy: no-referrer`, CSP strict same as dashboard pages, `Cache-Control: public, max-age=60`. `frame-ancestors`: `*` when `allowed_origins` empty, else that list.
- Origin: page runs on install host, always allowed by table above. Blocked site cannot frame it when allowlist set.
- Works on custom domain.

Identity in page. Page cannot call a same-origin identity route on the customer's domain, so:

| Case | How |
|---|---|
| Anonymous | normal lazy session, storage on install origin, one per WebView profile |
| App has Dimmi SDK | `POST /widget/v1/session-transfers` (bearer session) -> `{token: "dmi_wst_...", expires_at, url}`; SDK opens `url` = `/messenger/{app}#dimmi-transfer=dmi_wst_...`. Page: `POST /widget/v1/sessions {app_id, transfer_token}` creates new session row same contact, same kind, same `external_id`, `verified_at`, `expires_at`, with `transferred_from_session_id` (revoking parent revokes child). 60 s, single use (Valkey `SET NX EX`), bound to widget |
| App has no SDK, own backend | backend mints identity JWT (02), app opens `/messenger/{app}#dimmi-identity=<jwt>`. Page reads fragment, removes with `history.replaceState`, treats as identity source answer once. On expiry: `identity_refresh_missing`, then anonymous. Same fragment rule as 14 portal SSO: fragment never reaches server or logs |

Sign out: app closes WebView and clears its web data, or opens `/messenger/{app}#dimmi-logout` (handshake `DELETE /session`, wipes storage).

`postMessage` bridge for host WebView: page posts `{type: "dimmi:event", name, payload}` (sdk-contract events) to `window.parent` and `window.ReactNativeWebView` when present. No inbound commands: keeps attack surface zero.

Install id `webview` gives URL and notes.

## Custom widget API domain

Customer serves loader, API and stream from own hostname (`chat.acme.com`), which beats most blockers without proxy. First-party proxy (`api_base`, sdk-contract) stays documented.

`widget_domains`, prefix `wdm_`. Migration `20261201010005_create_widget_domains_table.sql`. Same columns and check machinery as 08 `help_center_domains` with `widget_settings_id` (unique: one per widget, because snippet carries one base). Hostname claim check through shared `customdomains.Claimed`: extended to include `widget_domains` so one host cannot be both.

Reuse: `framework/customdomain` (normalize, verify, TLS on demand). `AuthorizeTLSHostname` authorizer adds `widget_domains` lookup; ask endpoint unchanged. Delta for 08.

API (`widgets:manage`, reads `widgets:read`):

| Method | Path | Purpose |
|---|---|---|
| GET, POST | `/api/v1/widget-settings/{widget}/domains` | list, add. 422 `domain_limit_reached` on second; `hostname_claimed` |
| GET, DELETE | `/api/v1/widget-settings/{widget}/domains/{wdm}` | detail with DNS rows, remove |
| POST | `/api/v1/widget-settings/{widget}/domains/{wdm}/checks` | re-verify now (rest rule 5) |

Status machine and job same as 08 (`CheckWidgetDomain` on maintenance queue, `next_check_at`). Event `widget_domain.status_changed` workspace channel only.

Serving. Host middleware `widget.host`, first in chain: request `Host` is active widget domain -> resolve via `widget_resolve_domain(hostname)` SECURITY DEFINER (cached otter 60 s, Valkey bust). Allowed paths only: `/widget/v1/*`, `/widget/assets/*`, `/connection/uni_sse`, `/messenger/{app}`. Everything else 404 (no dashboard, no API, no sign-in on that host). `{app}` must equal that domain's widget, except `/widget/assets/*`. Session routes: session's widget must equal domain's widget else 401 `session_invalid`.

Config `realtime.url`, `token_url` and loader chunk URLs use request host when it is active domain. CDN `assets_base_url` unaffected unless domain request: domain wins for same-host promise. Install `api_base` = domain when active. Removing domain: sites using old host fail with DNS or TLS errors; dashboard warns before delete. CORS rules unchanged: `Origin` still checked against `allowed_origins`.

Self-host: works when instance terminates TLS through framework path; behind own proxy operator handles TLS and only needs hostname added (`dimmi doctor` line). Cloud: same. No entitlement: one per widget for everyone, like 08 fixed rule.

## Per-widget help center

Each widget links own `help_center_id`; `help_tab_enabled` and `changelog_tab_enabled` read it. Two widgets, two help centers, two brands in one workspace: logo, name, colours, inbox and help all per widget. No `brands` table: widget already carries brand fields, and `brd_` stays reserved. 08 `help_center.max_per_workspace` still caps help centers. 08 Ask AI, article links stay on that help center's host.

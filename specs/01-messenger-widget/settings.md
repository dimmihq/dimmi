# 01 Messenger widget: settings model

Style: caveman. Date 2026-11-04. Every customer-changeable field: storage, validation, config output, who reads it. Base table in [README](README.md) "widget_settings". Dashboard screens in [settings-page.md](settings-page.md). Client behavior in [client.md](client.md). Edge cases E152 onward in [edge-cases.md](edge-cases.md).

## Rules

- Several widgets per workspace, up to 10 (entitlement `widget.max_per_workspace`), each with own settings, install snippet, inbox. Mobile app = one more widget, same fields, same API; native SDKs ignore web-only fields (table below).
- Every setting lives per widget, never per workspace. Fallbacks to workspace only where named (`company_name`, logo).
- Every write goes through `UpdateWidgetSettings` (or translation, asset actions): validate, bump `config_version`, emit `widget.settings_updated`, bust config cache. Live pages pick change up within 60 s.
- Budgets hold: no setting adds a request on page load, config stays one response under 4 KB gz.

## Audience seam

audience rules arrive with the audiences spec. Nothing here decides who sees the messenger or who may start a conversation by person, segment, attribute or event. Builders add no audience field, no interface, no placeholder code. When that spec lands it plugs in at two named places only: `WidgetConfigResource` (what config says to show) and `StartWidgetConversation` (whether a start is accepted), and 15 segments are reusable there.

These stay plain settings, not audience rules: `enabled`, `allow_anonymous` (identity switch, existing), `allowed_origins` (trusted domains), `page_rules` (URL display), `open_to`, `collect_email`.

## New widget_settings columns

Migration `20261201010001_add_settings_fields_to_widget_settings_table.sql`.

| Column | Type | Notes |
|---|---|---|
| company_name | text null | customer-facing name, 1 to 60. Chain: this -> `workspaces.settings.customer_facing_name` (00-accounts) -> workspace name |
| default_locale | text not null default 'en' | language `texts` is written in. Must be BCP 47 |
| open_to | text not null default 'home' CHECK in ('home','conversation') | `conversation` = no home screen (below) |
| tab_order | text[] not null default '{home,messages,help,changelog,feedback}' | any order; exactly known tabs, no duplicates; missing known tabs appended by action |
| privacy_notice | jsonb not null default '{}' | `models.WidgetPrivacyNotice`: `enabled` bool, `url` https url or null. Text in `texts.privacy_notice` |
| home_links | jsonb not null default '[]' | max 6 `{title (1..40), url (https or mailto), icon: calendar\|book\|link}`. Static home "Get in touch" list. Titles translated via `texts`-style key `home_link_{index}` |
| page_rules | jsonb not null default '{"mode":"all","patterns":[]}' | below |
| dev_context | jsonb not null default '{}' | `{console: bool, network: bool}`, both default false ([client.md](client.md) "Developer context") |
| customer_status_enabled | boolean not null default true | show status chip in thread and list |
| delay_notice_enabled | boolean not null default true | show `texts.delay_notice` when team away |
| native_apps_enabled | boolean not null default false | own migration `20261201010007_add_native_apps_enabled_to_widget_settings_table.sql`, built in M1. Requests with no `Origin` accepted despite non-empty `allowed_origins` ([hosted-and-domains.md](hosted-and-domains.md)) |
| hosted_page_enabled | boolean not null default true | full-page URL on |
| schedule_id | uuid null | 17 `schedules`; no FK here, 17 adds FK in own migration. Null: workspace default schedule once 17 is live, else presence only |

`appearance` jsonb gains (max grows to 8 KB, `models.WidgetAppearance`):

| Key | Type | Notes |
|---|---|---|
| primary_color | #RRGGBB | existing: buttons, accents, visitor bubbles |
| header_color | #RRGGBB or null | null = primary_color |
| button_color | #RRGGBB or null | launcher button; null = primary_color |
| logo_asset_id | `wga_` or null | header tile. Null: workspace logo, then first letter of company name |
| background | object | `{type: solid\|gradient\|image, gradient: {from, to, angle 0..359}, image_asset_id, fade: bool}`; default `solid`. Applies to home hero and header. `fade` softens image to page surface at bottom |
| launcher.style | bubble, tab, hidden | existing. `tab` = vertical edge tab with `label` (static widget gains it) |
| launcher.shape | circle, squircle | from static widget |
| launcher.label | 0..24 | translated: key `launcher_label` |
| launcher.icon | chat, help, wave | existing |
| launcher.image_asset_id | `wga_` or null | custom image replaces icon; chevron still shows while open |
| sound | bool, default true | new message sound |
| theme, position, padding_x, padding_y, font | | existing |

Text colour on any brand surface comes from `isLight` of that surface (static `options.ts`). Image background: `WidgetAsset.luminance` computed at upload. Gradient: mean of both ends.

Config output resolves ids to URLs: `logo_url`, `launcher.image_url`, `background.image_url`, plus `header_ink`, `button_ink` (`light`, `dark`). Asset ids never reach config.

## Web-only fields

Native SDKs (20) read same config, ignore: `appearance.position`, `padding_x`, `padding_y`, `launcher.*`, `font`, `page_rules`, `dev_context`, `allowed_origins`. Hosted page ignores launcher and position, keeps the rest. Colours, logo, background, texts, tabs, `open_to`, privacy notice, sound, team intro apply everywhere.

## Texts: keys and limits

`texts` jsonb holds customer text in `default_locale`. Empty key = built-in default. Plain text, NFC, no HTML. Static `options.ts` key names win over earlier spec names (`greeting_title` is now `greeting_intro`, `greeting_body` is now `greeting`).

| Key | Max | Where | Key | Max | Where |
|---|---|---|---|---|---|
| greeting_intro | 60 | home hero line 1 | greeting | 280 | home hero line 2 |
| welcome_message | 280 | client-side first bubble of new conversation; empty = none | team_intro | 140 | beside team faces on home and new thread |
| delay_notice | 280 | below visitor's first message when team away | offline_message | 280 | no team online, email capture |
| office_hours_text | 140 | availability row | email_prompt | 140 | email capture |
| privacy_notice | 280 | above composer until first send | composer_placeholder | 60 | |
| chat_title, chat_body | 60, 140 | home card | recent_messages, links, show_all, send_message | 40 each | |
| messages_empty_title, messages_empty_body | 60, 140 | | search_placeholder, popular, helpful, update_feedback, still_need_help | 80 each | help, changelog |
| home, messages, changelog, help, feedback | 24 each | tab labels | launcher_label, home_link_0..5 | 24, 40 | |
| status_with_team, status_waiting_on_you, status_closed | 40 each | status chip | action_confirm, action_cancel | 24 each | 07 card |
| seen, reopen_hint, deleted_message, new_conversation, talk_to_human, ai_answering | 40 each | thread | | | |

`welcome_message`, `delay_notice`, `privacy_notice`, `team_intro` render client side, never stored as messages (metering D1, D2). A 17 opener that matches replaces `welcome_message`.

Built-in defaults: `resources/widget/lang/<locale>.json`, embedded, read through `app/support/widgetstrings`. Kept apart from `a.Lang` because dashboard languages are a separate list. Shipped locales: `en es fr de it pt pt-BR nl pl sv da nb fi cs ro tr ru uk ar he hi id vi th ja ko zh-CN zh-TW` (28). Test fails when a file misses a key or adds unknown one. Text written once in repo, no runtime translation service, no per-string auto-translation dependency: not needed for fixed interface strings.

## widget_translations

Decision: own table, not jsonb on `widget_settings`. Why: config reads one locale, 30 locales times 40 strings would bloat hot row and every settings response, and independent per-locale edits avoid lost updates.

Migration `20261201010002_create_widget_translations_table.sql`. No public id (keyed by parent).

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid not null | |
| widget_settings_id | uuid not null references widget_settings on delete cascade | |
| locale | text not null | BCP 47, canonical casing; never equals `default_locale` |
| strings | jsonb not null | subset of text keys, same limits; unknown key 422 |
| enabled | boolean not null default true | off: visitors fall through chain as if row missing |
| created_at, updated_at | timestamptz | |

PK `(workspace_id, widget_settings_id, locale)`. Max 60 rows per widget (config `widget.max_translations`). RLS.

Resolve one key for locale `L` (`ResolveWidgetTexts`, pure, otter cached with config):
1. Widget translation row `L`, then row for base language of `L` (`pt-BR` then `pt`).
2. If `texts[key]` is empty: built-in `L`, then built-in base language.
3. `texts[key]`, then built-in `default_locale`, then built-in `en`.

Why step 2 is conditional: customer's own wording in their language beats generic string that loses its meaning. Dashboard shows missing count per language.

Requested locale: config `?locale=` (SDK sends `Dimmi` option, else `navigator.language`, when widget `locale` is `auto`; forced `locale` ignores it). Normalized to nearest of widget rows + shipped locales + `default_locale` before cache lookup, so cache keys stay bounded. Response has `locale` (resolved), `dir` (`rtl` for `ar he fa ur`), `locales` (available). `update({locale})` refetches config.

Offline email, push and availability text use `widgetstrings` with contact `locale`.

## Page rules (show or hide on specific pages)

Plain URL display setting. Not an audience rule: knows nothing about the person, server never enforces it, API stays open.

```json
{ "mode": "all | only | except", "patterns": ["/pricing", "/docs/*", "https://app.acme.com/*"] }
```

- Max 50 patterns, each 1 to 500 chars. Starts `/` = matches path on any host; starts `https://` or `http://` = host + path. `*` = any run of characters. Trailing slash ignored, query and fragment never compared, case-sensitive path, case-insensitive host.
- `all`: show. `only`: show when any matches. `except`: hide when any matches.
- Evaluated in core on load and each route change (sdk-contract "SPA route detection"). Hidden = launcher hidden, panel closes unless visitor is mid-conversation with panel open (then stays until closed). Unread badge on `data-dimmi` elements keeps working.
- `Dimmi.open()`, `show()`, `data-dimmi` buttons still work on hidden pages: host asked explicitly. Engage items (15) follow own URL triggers. Realtime and session unaffected.
- No match logic on server. 422 only for shape errors. Dashboard tester shows yes or no for typed URL using same matcher.

## Open straight into conversation

`open_to = conversation`: no home screen. `home` dropped from config `tabs`. Launcher opens latest open conversation of session; none: new conversation view with `welcome_message`, `team_intro`, `privacy_notice`. Other enabled tabs stay in tab bar; only one tab left: tab bar hidden. No session is created until first write (lazy rule unchanged). `show('home')` then opens default view, no warn.

## Tabs

`help_tab_enabled`, `changelog_tab_enabled`, `feedback_tab_enabled` stay on or off switches; `tab_order` is only order. Config `tabs` = `tab_order` filtered by what is on and linked. PATCH field `tab_order`, per rest.md rule 9 spirit (array of names), reorder only.

## Assets: logo, launcher image, background

`widget_assets`, prefix `wga_`. Migration `20261201010003_create_widget_assets_table.sql`. Not core `attachments` (signed URLs, 24 h orphan purge). Not `engage_assets` (bound to 15 lifecycle). Public, long-lived.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `wga_` |
| workspace_id, widget_settings_id | uuid not null | cascade |
| kind | text not null CHECK in ('logo','launcher','background') | |
| disk, path | text not null | `filesystems.default` disk, path `widget-assets/{ws}/{sha256}.{ext}` |
| content_type | text not null CHECK in ('image/png','image/jpeg','image/webp','image/gif') | |
| size_bytes, width, height | int not null | |
| sha256 | text not null | unique per widget + kind (re-upload reuses row) |
| luminance | smallint not null | 0..100 mean, `framework/image` decode downscaled |
| source | text not null default 'upload' CHECK in ('upload','site_import') | |
| created_at | timestamptz | |

Upload `POST /api/v1/widget-settings/{widget}/assets` multipart `file`, `kind`. `image.Sniff`, `DecodeConfig` with limits, `Reencode` strips metadata and active content. Limits: logo, launcher 1 MB, 2048 px; background 5 MB, 4096 px. SVG refused (`asset_type_not_allowed`): needs sanitizer, and PNG covers it. Output writes per `Reencode` rule (WebP in, PNG out); background over 500 KB after reencode rewritten JPEG.

Serve: `GET /widget/v1/apps/{app}/assets/{wga}/{sha256}.{ext}`, no auth, no origin check, `public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`, CORP cross-origin. Content hash in path, so replacing never serves stale.

Delete: `DELETE .../assets/{wga}` refuses in use with 409 `asset_in_use`. Unreferenced rows older than 30 days removed by `PruneWidgetAssets` (daily, `RetentionGuard` per workspace), since cached config may still point at them.

## Site import: logo and colours from their website

Customer types URL. Dimmi proposes logo, colours, company name; customer sees it in preview and applies with normal Save. Never auto-applied.

`widget_site_imports`, prefix `wsi_`. Migration `20261201010004_create_widget_site_imports_table.sql`. Columns: `id`, `workspace_id`, `widget_settings_id`, `url text`, `status` CHECK in (`queued`,`running`,`succeeded`,`failed`), `result jsonb` (`{company_name, primary_color, header_color, button_color, colors[], logo_asset_id, found[]}`), `error_code text`, `requested_by_member_id`, timestamps. Rows pruned at 30 days.

API: `POST /api/v1/widget-settings/{widget}/site-imports {url}` 202 (`widgets:manage`, 10 per hour per widget, one in flight per widget: second returns running one). `GET .../site-imports/{wsi}`. Event `widget.site_import_finished` on workspace channel, page also refetches on 5 s timer as fallback.

Job `ImportWidgetBrandFromSite` (queue `default`, 1 retry, 60 s deadline, unique per widget). All network via `a.HTTP` (`framework/httpclient`, safedial guard on every hop and redirect). `Timeout(10 s)`, `MaxBody`, at most 3 redirects, user agent `dimmi-site-import/1.0`. No JavaScript runs. One page requested by owner of that site, so robots not consulted.

Reads, in order, parse with `framework/dom`:

| Signal | Used for |
|---|---|
| `<meta name="theme-color">` | primary_color, first choice |
| `<link rel="manifest">` then `theme_color`, `icons` (max 256 KB) | primary_color, logo |
| CSS custom props named like `--primary`, `--brand`, `--color-primary`, `--accent` in inline `<style>` and first same-origin stylesheet (max 256 KB, regex scan, hex, rgb, hsl only) | primary_color |
| `background-color` of `button`, `.btn`, `header`, `nav` rules in same CSS | button_color, header_color |
| JSON-LD `Organization.logo`, `<link rel="apple-touch-icon">`, `<link rel="icon">` png, `<meta property="og:image">` | logo, in this order; largest square-ish wins; SVG, ICO under 128 px skipped |
| `og:site_name`, then `<title>` cut at first `|`, `-`, `:` | company_name |
| dominant non-grey colour of chosen logo (histogram in `app/support/brandimport`, no new library) | primary_color, last resort |

Colour filter: drop near-white, near-black, greys for primary; keep up to 6 in `colors[]` as swatches.

Logo fetched max 5 MB, runs asset pipeline above, stored `source = site_import`. Partial result = `succeeded`; `found[]` lists what was found.

Failure codes (plain sentence each, `lang/en/errors.json`):

| Code | When | Page says |
|---|---|---|
| `site_invalid_url` (422 at POST) | not http or https, has credentials | "Enter a full website address." |
| `site_unreachable` | DNS, connect, 5xx, safedial refusal (blocked and unreachable same code, no network oracle) | "We could not reach that site." |
| `site_timeout` | deadline | "That site took too long to answer." |
| `site_blocked` | 401, 403, 429 | "That site refused us. Upload your logo and pick colours here." |
| `site_not_html` | content type not HTML | "That address is not a web page." |
| `site_too_large` | HTML over 2 MB | same as unreachable with size named |
| `nothing_found` | no signal (script-only page) | "We found no logo or colours there. Upload them here." |

Failed row changes nothing on widget. Tests fake third-party HTTP with `httpclienttest` and exercise SSRF through real safedial.

## Config response

Replaces `workspace`, `appearance`, `texts`, adds keys in [api.md](api.md) "GET config". Cache key: `wgt`, `config_version`, resolved locale, `engage_settings.version`. Web-only keys are sent to every client (no per-platform variants: keeps one CDN object).

## Lazy chunks and budgets

Loader at most 3 KB gz, core at most 35 KB gz: both unchanged. Named chunk budgets unchanged, three added.

| Feature | Where | Budget |
|---|---|---|
| colours, gradient, background image, logo, launcher image, company name, tab order, `open_to`, privacy notice, team intro, delay notice, welcome message, translated texts (arrive in config), status chip, seen, typing, AI deltas, page rule matcher (under 300 B) | core | counted in 35 KB. Today's core 18.8 KB gz includes sample content that moves out |
| error capture listeners for dev context (`error`, `unhandledrejection`, ring of 10) | loader | under 250 B; loader stays under 3 KB |
| console and network capture | new chunk `dev-context` | 3 KB. Loaded only when `dev_context` on |
| sample content and `simulate` | new chunk `preview` | 12 KB. Loaded only by preview page, never in customer traffic |
| voice note recording | new chunk `voice` | 3 KB |
| attachments, markdown, emoji | existing chunks | unchanged |

`scripts/build-widget.ts` checks gzip per named chunk (today raw bytes for loader and core only) and publishes all budgets in manifest.

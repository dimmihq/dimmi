# 01 Messenger widget: dashboard settings page

Style: caveman. Connects static `MessengerSettings` in `resources/js/features/settings/pages/channels.tsx` to real data. Field detail in [settings.md](settings.md). Route stays settings view `messenger` (`features/settings/views.tsx`).

## Files

`MessengerSettings` moves out of `channels.tsx` (`ConnectionsSettings` stays). Each file under 400 lines.

```
resources/js/features/settings/messenger/
  page.tsx               # MessengerSettings: list + selected widget + preview layout
  widget-list.tsx        # short list, New widget
  draft.ts               # useWidgetDraft: draft, diff, save, discard
  api.ts                 # TanStack Query: widgets, translations, assets, site imports, domains, install
  preview.tsx            # iframe + postMessage
  sections/install.tsx, general.tsx, appearance.tsx, texts.tsx, languages.tsx,
           visitors.tsx, pages.tsx, tabs.tsx, home-links.tsx, apps.tsx, domain.tsx, danger.tsx
  site-import-dialog.tsx, new-widget-dialog.tsx, translation-dialog.tsx, image-field.tsx, sortable-tabs.tsx
```

Controls stay `settings/controls.tsx` and `layout.tsx` parts, extended to controlled values (`value` + `onChange`; `Colour` gains `onChange`). `SettingsPage` gains `aside` prop: wider container with sticky preview column at 1200 px and up. BoardUI only, semantic tokens only.

## Several widgets

Short list on top of the page.

- One widget: single quiet row (name, install state) + "New widget" button. Barely noticed; rest of page is that widget.
- Several: one row each (name, inbox, install state dot, on or off). Selected row drives page; selection in URL search param `?widget=wgt_...`.
- "New widget" dialog: name, inbox (`Select` of inboxes). `POST /api/v1/widget-settings`. At limit: button disabled with "10 messengers is the limit", server 422 `entitlement_limit_reached` shown same way.
- Members with only `widgets:read`: controls disabled, install and test visible, no save bar.

## Draft and save

Single draft per selected widget. Preview shows draft live; customers see nothing until Save. Sticky bar "Unsaved changes" with Discard, Save. Save sends `PATCH` with changed keys only, stores response, resets draft. 422 field errors map to rows. Leaving with unsaved changes: BoardUI confirm dialog. Translations, assets, site import, domains save on their own (dialogs), since each has own endpoint; assets only attach to widget through draft.

`409` is not used; last write wins per key, draft diff keeps untouched keys safe.

## Binding: every static control

| Section | Static control | Field | Notes |
|---|---|---|---|
| Install | description "Not seen on a site yet." | `install.status` | see Installed status |
| Install | Framework `Choice` | `?framework=` on install endpoint | full id list below |
| Install | Copy snippet `Action` | `blocks[]` from install endpoint | shows blocks, copy each (`use-copy.ts`) |
| General | Name | `name` | |
| General | On | `enabled` | |
| General | Language | `locale` | Auto + list of `locales` |
| Appearance | Colour | `appearance.primary_color` | |
| Appearance | Theme | `appearance.theme` | Auto, Light, Dark = auto, light, dark |
| Appearance | Position | `appearance.position` | |
| Appearance | Launcher | `appearance.launcher.style` | Bubble, Tab, Hidden |
| Texts | Greeting | `texts.greeting_intro` | 60 |
| Texts | Away message | `texts.offline_message` | |
| Visitors | Ask for an email | `collect_email` | When offline, Always, Never = offline, always, never |
| Visitors | Anonymous visitors | `allow_anonymous` | |
| Visitors | Allowed sites | `allowed_origins` | one field becomes tag list; validation hint from server; origins seen and blocked listed with one-click add |
| Tabs | Help | `help_tab_enabled` + `help_center_id` | new Select of help centers; disabled until 08 lands |
| Tabs | What's new | `changelog_tab_enabled` | needs help center |
| Tabs | Feedback | `feedback_tab_enabled` | needs 14 |
| Danger zone | Delete messenger | `DELETE` | BoardUI confirm typing name; last widget allowed, page shows empty state with New widget |

## New controls

| Section | Control | Field | Component |
|---|---|---|---|
| General | Inbox | `inbox_id` | Select; hint "new conversations only" |
| General | Company name | `company_name` | Field, placeholder = chain value |
| General | Default language | `default_locale` | Select |
| Appearance | Import from website | site import | Action opens dialog: URL input, progress, proposal with swatches, logo, name; "Use these" writes to draft |
| Appearance | Logo | `appearance.logo_asset_id` | `image-field.tsx`: upload, replace, remove |
| Appearance | Header colour, Button colour | `appearance.header_color`, `button_color` | Colour + "Same as main" toggle |
| Appearance | Background | `appearance.background` | Choice Solid, Gradient, Image; gradient two colours + angle; image upload + fade toggle |
| Appearance | Launcher shape, label, icon, image | `appearance.launcher.*` | Choice, Field, Choice, image-field. Label shown only for bubble and tab |
| Appearance | Sounds | `appearance.sound` | Toggle |
| Appearance | Spacing from edge | `padding_x`, `padding_y` | number inputs 0..200 |
| Texts | Second greeting line | `texts.greeting` | |
| Texts | Welcome message | `texts.welcome_message` | |
| Texts | Team introduction | `texts.team_intro`, `show_team_avatars` | Field + Toggle |
| Texts | Delay notice | `texts.delay_notice`, `delay_notice_enabled` | |
| Texts | Reply time | `reply_time` | Auto, Minutes, Hours, A day |
| Texts | Privacy notice | `privacy_notice.enabled`, `url`, `texts.privacy_notice` | |
| Texts | All other strings | `texts.*` | disclosure "All text", grouped by view, placeholder = built-in |
| Languages | List | translations API | row per language: missing count, on or off, Edit, Remove; Add language |
| Languages | Edit | `translation-dialog.tsx` | two columns: default text beside translation; per-key, scrollable (`ScrollFade`) |
| Visitors | Open to | `open_to` | Home screen, Conversation |
| Visitors | Require email when away | `require_email_when_offline` | Toggle |
| Visitors | Attachments, max size | `attachments_enabled`, `max_attachment_mb` | |
| Visitors | Conversation status | `customer_status_enabled` | Toggle |
| Pages | Show on | `page_rules.mode` (writes `audiences.launcher`) | Every page, Only these, All except these; "Custom rule" + builder link when richer |
| Pages | Patterns | `page_rules.patterns` (writes `audiences.launcher`) | list editor + URL tester on `lib/rules.ts` |
| Tabs | Order | `tab_order` | `sortable-tabs.tsx`, `GridList` with react-aria `useDragAndDrop` (keyboard works) |
| Tabs | Home links | `home_links` | list editor, max 6 |
| Developers | Capture console errors, Capture failed requests | `dev_context.console`, `.network` | Toggles; warning line on what is sent |
| Developers | Intercom snippet support | `intercom_shim` | Toggle |
| Apps | Accept mobile apps | `native_apps_enabled` | Toggle, shown when allowed sites non-empty |
| Apps | Push keys | 18 push credentials | built in track 10 |
| Apps | Hosted page | `hosted_page_enabled` | Toggle + URL + copy |
| Domain | Custom domain | widget domains API | hostname, DNS rows, status, Check now (same layout as help center domains) |

Who sees messenger, who may start, tabs: shared builder `resources/js/features/audiences/` added here by 18 M5 (track 13). Until then only Pages control.

## Live preview: real widget code

Preview runs real loader and core in an iframe, never a lookalike.

- `/widget-preview/{widget}?embed=1` (web group, session auth, `frame-ancestors 'self'`, `no-store`). Page: `<script src="/widget/v1/loader.js" data-app-id=... data-preview="on">` on bare neutral background.
- `data-preview` makes core load chunk `preview` (sample team, conversations, thread, articles from `content.ts`, `simulate`) and skip network entirely except config for the saved state. Sample content stays out of production core.
- Parent sends `iframe.contentWindow.postMessage({type: "dimmi:preview", update: OptionsUpdate, view}, origin)` on each draft change, debounced 60 ms. Preview host script checks `event.origin === location.origin` and calls `Dimmi("update", update)`, which already merges and re-renders keeping open tab. Drafts never leave the browser.
- Images upload immediately and return URL, so draft holds real `wga_` + URL, and preview loads real file.
- Draft to `OptionsUpdate`: `preview.tsx` maps field names to config shape using same `ResolveWidgetTexts` order client side only for shown locale.
- Toolbar above preview: view (Home, Messages, Thread, Help, Launcher only), Light or Dark (`theme`), width (Desktop, Phone), language (preview translation: draft strings merged). Panel open by default.
- "Try it for real" button: `POST .../test-sessions`, opens `preview_url` in new tab with real session (test conversation, `is_test`). This path uses real API.
- Page rules not simulated in preview (preview always shows); tester row covers them.
- Phones: preview behind "Preview" button in sheet; narrow layouts keep form usable.
- Preview iframe fails to load: section says "Preview did not load" and form still works.

## Installed status

Install section header reads `GET .../install`:

| State | Text |
|---|---|
| never seen | "Not seen on a site yet." + spinner polling 10 s while page open |
| seen within 7 days | "Installed. Last seen on https://app.acme.com 2 minutes ago." |
| seen, over 7 days ago | "Last seen 12 days ago on https://app.acme.com." |
| blocked origins exist | warning row "Blocked on https://x.com" + Add to allowed sites |

Data: tracker (README "Install tracking"), `last_boot_at` about 2 minutes behind worst case. Row in widget list shows same dot. Mobile install counts: any widget request, `Origin` absent recorded as `app`.

## Snippet per framework

`Choice` options (ids equal install endpoint): Next.js App Router `nextjs-app`, Next.js Pages `nextjs-pages`, React `react`, React Router `react-router`, SvelteKit `sveltekit`, Nuxt `nuxt`, Vue `vue`, Astro `astro`, Rails `rails`, Laravel `laravel`, Django `django`, FastAPI `fastapi`, Go `go`, Express `express`, Hono `hono`, Plain HTML `html`, Proxy `proxy`, React Native `react-native`, Expo `expo`, iOS `ios`, Android `android`, WebView `webview`. Default guessed from nothing: `html`. Selection kept per widget in localStorage (try/catch). `blocks` rendered: command, file with path and code. Below: "Or run npx dimmi init". Custom domain active: `api_base` in blocks is that domain.

## e2e (tests/e2e/messenger-settings.spec.ts, light and dark)

Plain-word tests: changing colour updates launcher inside preview iframe without saving; save persists after reload; discard restores; new widget appears in list with own snippet; eleventh widget refused; member sees read-only; upload logo shows in preview; reorder tabs by keyboard changes preview order; add French translation shows in preview at French; site import proposal applied (job result seeded with factory row, network path covered by feature tests); snippet copy per framework; installed status flips after real loader visit on second origin; no console or CSP error (Playwright rule).

# 20 Native SDKs: API mapping

Style: caveman. Same verbs as web (`sdk-contract` Design rule 5). Same wire. JS names camelCase, Swift and Kotlin idiomatic.

| Action | JS (`client`) | Swift `Dimmi` | Kotlin `Dimmi` | Wire |
|---|---|---|---|---|
| Configure | `createDimmiClient(opts)` | `configure(appId:apiBase:identity:)` | `configure(context, appId, apiBase, identity)` | `GET /widget/v1/apps/{app}/config?locale=` |
| Availability | `availability()` | `availability()` | `availability()` | `GET /widget/v1/apps/{app}/availability` |
| Resume or sign in | automatic, `identify()` | same | same | handshake table: `POST /sessions`, `GET`/`PATCH`/`DELETE /session` |
| Sign out | `logout()` | `logout()` | `logout()` | `DELETE /session` + wipe |
| Visitor details | `updateContact({name, email})` | same | same | `PATCH /contact`, lead only |
| List | `conversations.list()` | `conversations()` | `conversations()` | `GET /conversations` |
| Start | `conversations.start(text, {attachments, context})` | `startConversation` | `startConversation` | `POST /conversations` |
| Thread | `messages.list(id, {before})` | `messages(in:before:)` | `messages(id, before)` | `GET /conversations/{conv}/messages` |
| Send | `messages.send(id, text, {attachments})` | `send` | `send` | `POST .../messages`, `client_id` |
| Typing | `typing(id, state)` | same | same | `POST .../typing-indicators` |
| Read | `markRead(id, messageId)` | same | same | `PATCH .../read-state` |
| Human | `requestHuman(id)` | same | same | `POST .../handoffs` |
| Agent card | `actionCall(id, callId, status)` | same | same | `PATCH .../action-calls/{call}` |
| Quick reply | `messages.send(id, label, {workflowAnswer})` | same | same | `workflow_answer` (17) |
| Attach | `uploads.create(file)` | same | same | `POST /attachments`, PUT, `PATCH /attachments/{att}` |
| Page or screen | `screen(name)` | `screen(_:)` | `screen(name)` | `POST /pings` with `page.url = app://{bundle}/{name}` |
| Context | `update({context, locale})` | same | same | `POST /pings`, config refetch on locale |
| Track | `track(name, props)` | same | same | `POST /contact-events` only for `surface.writable_events`; else no-op (18) |
| Surface | `useConfig`, `useSession` | same | same | `GET /surface`, session `surface`; `surface.changed` refetch |
| Live stream | automatic | automatic | automatic | `POST /realtime-tokens`, `GET /connection/uni_sse`, fallback `GET /updates` |
| Secondary tab | `show('help', opts)` | `show(.help)` | `show(View.Help)` | `POST /session-transfers`, then `/messenger/{app}?view=` |
| Help data headless | `help.search(q)` | no | no | 08 routes |
| Push | `setPushToken` | `setPushToken(_:)` | `setPushToken(_:)` | `POST`, `PATCH`, `DELETE /devices` |
| Unread | `useUnreadCount()`, `on('unread_count_changed')` | `unreadCount` publisher | `unreadCount` Flow | session + stream |

Events same names as web `DimmiEvents` where they apply: `ready`, `open`, `close`, `unread_count_changed`, `message_received`, `message_sent`, `conversation_started`, `identity_changed`, `error`. Swift `AsyncStream`, Kotlin `SharedFlow`.

## Request differences from web

| Item | Web | Native |
|---|---|---|
| `Origin` | browser sets | none; widget table in 01 hosted-and-domains.md |
| `page` | URL | `app://{bundle}/{screen}`; server query scrub applies; stored as-is in `current_page` |
| `sdk` | `dimmi-browser` | `dimmi-ios` etc + version |
| `user-agent` | browser | `dimmi-ios/1.2.0 (Acme 3.1; iOS 18.1; iPhone16,1)`; bot score UA markers skip SDK names only with verified PoW |
| PoW | Web Worker | background thread, same ALTCHA payload |
| Honeypot, timing | present | `website` empty, min send time applies: SDK sends only after human action, tested |
| Storage | localStorage | Keychain, Keystore file |
| Hosted transfer | n/a | `dmi_wst_` fragment |

## Config fields used

Read: `company`, `appearance.primary_color`, `header_color`, `button_color`, `header_ink`, `button_ink`, `background`, `theme`, `sound`, `texts`, `locale`, `dir`, `locales`, `tabs`, `open_to`, `show_team_avatars`, `show_availability`, `customer_status`, `delay_notice`, `privacy`, `home_links`, `features`, `branding`, `realtime`, `workflow_openers`.

Ignored: `position`, `padding_*`, `launcher`, `font`, `surface.anonymous.launcher`, `features.dev_context`, `intercom_shim`, `engage`.

Branding: same rule as web, cloud never-paid shows "Powered by dimmi" row.

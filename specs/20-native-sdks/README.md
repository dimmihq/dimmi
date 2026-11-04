# 18 Native and headless SDKs

Style: caveman. Date 2026-11-04. Owner decision: moved from later to now. Track 10 native-sdks, wave 2, after 01 session and messaging API frozen ([01 tasks](../01-messenger-widget/tasks.md) "Freeze"). Files: [api-mapping.md](api-mapping.md), [edge-cases.md](edge-cases.md), [tasks.md](tasks.md).

## Goal

Mobile app talks to dimmi exactly like website: another widget, same settings, same `/widget/v1`, same inbox. Customer gets one dashboard, one identity model. Developer gets working support screen in under 10 minutes, and own-UI hooks when wanted.

## World-class bar

| Bar | Target |
|---|---|
| Install to first message | under 10 minutes on Expo, RN, iOS, Android |
| No private API | every SDK call is a documented `/widget/v1` route. New server routes only for push devices |
| Teammate reply to foreground app | p95 under 500 ms over SSE; under 6 s in polling fallback |
| Reply to backgrounded app | push accepted by APNs or FCM p95 under 10 s after message commit |
| Message loss | zero. Persisted outbox + `client_id`, survives app kill |
| Size added | `@dimmi/headless` at most 15 KB gz, `@dimmi/react-native` at most 60 KB gz JS, iOS and Android core at most 1 MB each, UI at most 2 MB each |
| Third-party dependencies | Swift none. Android OkHttp, kotlinx (coroutines, serialization), Compose for UI. RN listed below |
| Silent failures | zero. Same error codes as web, logged once with fix |

## Scope

### Packages (names decided)

| Package | Registry | Runtime | Contents |
|---|---|---|---|
| `@dimmi/headless` | npm | browser, RN, Node 18+ | `createDimmiClient`, stores, protocol. Subpath `@dimmi/headless/react`: hooks |
| `@dimmi/react-native` | npm | RN 0.76+, Expo SDK 52+ (development build for push; Expo Go works without push) | `<DimmiProvider>`, `<DimmiMessenger>`, components, Expo config plugin `app.plugin.js`. Built on headless |
| `Dimmi`, `DimmiUI` | Swift Package Manager, `https://github.com/dimmihq/dimmi-ios` | iOS 15+, Swift 5.9+ | core, SwiftUI screens |
| `com.trydimmi:dimmi-android`, `com.trydimmi:dimmi-android-ui` | Maven Central | minSdk 24, Kotlin 2 | core, Compose screens |

Rejected names: `@dimmi/mobile`, `@dimmi/expo` (one package, config plugin inside). CocoaPods not offered: owner chose Swift Package Manager. `sdk.name`: `dimmi-headless`, `dimmi-react-native`, `dimmi-ios`, `dimmi-android`.

Repo: `sdk/packages/headless` (builds from `resources/widget/client/`, so web and headless share one protocol), `sdk/packages/react-native`, `sdk/ios` (mirrored by release job to `dimmihq/dimmi-ios`, since Swift Package Manager needs `Package.swift` at a tagged repo root), `sdk/android`.

### Native UI scope

Prebuilt screen on each platform, drawn natively:
- Home: logo, company name, greeting, team faces, team intro, start conversation, recent conversations, home links. Skipped when `open_to = conversation`.
- Conversations list. Thread: messages, Markdown subset, status chip, Seen, typing, AI streaming, action card, quick replies (17), delay notice, welcome, privacy notice, email capture.
- Composer: text, photos, camera, files, sending and failed states.
- Theme from config colours, background, logo, `theme`; texts from config `texts`, `dir` respected. Web-only fields ignored (01 settings.md).
- Tabs Help, What's new, Feedback, CSAT survey card: open hosted page in in-app web view (`WKWebView`, Android `WebView`, `react-native-webview` optional peer, else system browser) through session transfer + `view=`. Why: three feature UIs per platform is far more code than value; hosted page is same real widget and always current.

Automatic announcements and survey triggers (15) do not fire in native SDKs: triggers are URL and `track` based on web core. `show('announcement' | 'survey')` still opens hosted page; headless exposes `/widget/v1/engage-items` to build own.

### Headless hooks

`createDimmiClient({appId, apiBase, identity, storage?, fetch?, eventSource?})`. Hooks under `<DimmiClientProvider>`:

`useDimmiClient()`, `useSession()`, `useConfig()`, `useAvailability()`, `useConversations({status})`, `useConversation(id)`, `useMessages(id)` (paged), `useSendMessage(id?)`, `useStartConversation()`, `useTyping(id)` (`typing`, `report`), `useReadState(id)`, `useUnreadCount()`, `useAttachmentUpload()`, `useRealtimeStatus()`, `useIdentity()`, `useLogout()`. Built with `useSyncExternalStore`. No DOM needed. Web use: same app can run this with no widget core, and loader not loaded. Example: `sdk/examples/headless-react`.

## Data model

Owned by 18. Conventions apply. Migrations track 10.

### widget_devices (`wdv_`)

`20261201100001_create_widget_devices_table.sql`

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | |
| workspace_id, widget_settings_id | uuid not null | cascade |
| widget_session_id | uuid not null references widget_sessions on delete cascade | logout and prune remove device |
| contact_id | uuid not null references contacts on delete cascade | repointed by 01 `RepointWidgetSessions` |
| platform | text not null CHECK in ('ios','android') | |
| push_token | text not null | |
| push_token_hash | bytea not null | unique `(widget_settings_id, push_token_hash)`; re-register moves row to new session |
| environment | text not null default 'production' CHECK in ('production','sandbox') | iOS only |
| app_version, sdk_version, locale | text | |
| last_seen_at | timestamptz not null | |
| disabled_at | timestamptz | |
| disabled_reason | text CHECK in ('unregistered','bad_token','topic_mismatch','admin') | |
| created_at, updated_at | timestamptz | |

Index `(workspace_id, contact_id) WHERE disabled_at IS NULL`. RLS.

### widget_push_credentials (`wpc_`)

`20261201100002_create_widget_push_credentials_table.sql`. Per widget, because each app has own keys.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | |
| workspace_id, widget_settings_id | uuid not null | |
| platform | text CHECK in ('ios','android') | unique per widget |
| secret_encrypted | bytea not null | `encryption.Sealed`. iOS: `.p8` text. Android: service account JSON |
| apns_key_id, apns_team_id, apns_bundle_id | text | iOS |
| fcm_project_id, fcm_client_email | text | Android |
| status | text CHECK in ('untested','ok','failing') | |
| last_ok_at, last_error_code | | |
| created_at, updated_at | timestamptz | |

`20261201100003_add_push_preview_to_widget_settings_table.sql`: `push_preview boolean not null default true`.

## HTTP API

| Method | Path | Route | Auth | Purpose |
|---|---|---|---|---|
| POST | `/widget/v1/devices` | widget.devices.store | widget | `{platform, push_token, environment?, app_version?, locale?}`; upsert by token; 201 or 200 `widget_device` |
| PATCH | `/widget/v1/devices/{wdv}` | widget.devices.update | widget, own session | `{push_token?, locale?, app_version?}` |
| DELETE | `/widget/v1/devices/{wdv}` | widget.devices.destroy | widget, own session | 204, user disabled notifications |
| GET, POST | `/api/v1/widget-settings/{widget}/push-credentials` | widget-settings.push-credentials.* | `widgets:read` / `widgets:manage` | list without secret; create `{platform, p8, key_id, team_id, bundle_id}` or `{platform, service_account_json}`; validates shape, 422 |
| PATCH, DELETE | `.../push-credentials/{wpc}` | same | `widgets:manage` | replace or remove |
| POST | `.../push-credentials/{wpc}/tests` | widget-settings.push-credentials.tests.store | `widgets:manage` | rest rule 5, synchronous check, returns outcome |

Secrets never returned. Audit logged. All other SDK calls: existing routes, see [api-mapping.md](api-mapping.md).

## Push notifications to customers

Server: `framework/push`, product-free package: `push.Sender` with `apns` and `fcm` drivers, `Fake()` for tests, `drivertest` suite. Message `{Token, Title, Body, Data, Badge, CollapseID, Environment}`. Errors `ErrTokenInvalid`, `ErrCredentials`, `ErrRateLimited`.

| | APNs | FCM |
|---|---|---|
| Endpoint | `https://api.push.apple.com/3/device/{token}`, `https://api.sandbox.push.apple.com/...` | `https://fcm.googleapis.com/v1/projects/{id}/messages:send` |
| Auth | ES256 provider JWT from `.p8`, reused, renewed under 60 min, never per send | OAuth2 service account JWT grant, scope `https://www.googleapis.com/auth/firebase.messaging`, token cached until expiry |
| Headers | `authorization: bearer`, `apns-topic` = bundle id, `apns-push-type: alert`, `apns-collapse-id` | `android.collapse_key`, priority high |
| Dead token | 410 `Unregistered`, 400 `BadDeviceToken`: disable device; 400 `DeviceTokenNotForTopic`: disable + credential warn | 404 `UNREGISTERED`, 400 `INVALID_ARGUMENT` naming token: disable |
| Bad credentials | 403 `InvalidProviderToken`, `ExpiredProviderToken` (renew once) | 401 `THIRD_PARTY_AUTH_ERROR`, 403 `SENDER_ID_MISMATCH` |

Reuse check: `firebase-admin-go` (Apache) refused: pulls GCP and gRPC stack, against binary size rule. `sideshow/apns2` (MIT) refused: second JWT library against "one JOSE lib", saves little. Both protocols are one HTTP call each: thin drivers on `framework/httpclient` (HTTP/2 forced on transport, `httpclient/safedial` fine for public hosts) and `framework/jwt` signing via go-jose. `golang.org/x/oauth2` already in tree used for FCM token only.

Sending:
- Listener `ScheduleWidgetPush` (queued, `default`): member or agent `message.created`, `type=message`, conversation visible on widget per same-person rule. Job `SendWidgetPush` delay 5 s, unique per `(conversation, contact)`, later replies join.
- Skip: all read; conversation spam or deleted; contact blocked; no credentials; session of that device has live realtime connection (centrifuge presence, foreground app). Test sessions send only to minting member's device.
- Devices: contact's enabled devices on conversation's widget only (a conversation stays on its widget).
- Content: title = agent or member name + company name; body = excerpt 180 graphemes, or `texts` "New message" when `push_preview = false`, in device locale. Badge = unread count for that widget. Data: `{dimmi: {app_id, conversation_id, message_id}}`. Collapse id = conversation id.
- Failure: retries 3 with backoff on 5xx, 429 honors `Retry-After`; `ErrCredentials` sets credential `failing`, event `widget_push_credential.failing` (workspace channel, banner on settings page), no more retries that hour.
- Offline email (01) unchanged: still after its delay, only when unread. Metrics `dimmi_widget_push_total{platform,result}`.
- Erase: 01 `EraseWidgetContactData` deletes devices. Logout, identity switch, merge into user, secret revoke: session revoked, device row deleted in same flow (never pushes to signed-out phone). Natural expiry keeps device up to prune (30 days after expiry): customer who has not opened app still gets replies.

Client:
- iOS: host app forwards token: `Dimmi.setPushToken(deviceToken)`. Environment from `aps-environment` of embedded provisioning profile; override `environment:`. `Dimmi.handleNotification(userInfo) -> Bool` opens thread; false when not ours.
- Android: `Dimmi.setPushToken(token)` from host's `FirebaseMessagingService.onNewToken`; `Dimmi.handleMessage(remoteMessage)`. Helper `DimmiMessagingService` for apps with no own service. Default notification channel `dimmi_messages`.
- RN: `Dimmi.setPushToken({platform, token})`. Expo: helper `registerForDimmiPush()` calls `expo-notifications` `getDevicePushTokenAsync` (native token, not Expo push token; dimmi talks to APNs and FCM directly, customer owns keys, no Expo service in path). Expo Go has no push: warn `push_unavailable`.
- Prompt timing belongs to host app; SDK never asks permission itself except optional `Dimmi.requestPushPermission()`.
- Settings page "Apps" section: upload keys, Test button, status.

Reference sources: [Apple provider auth and errors](https://developer.apple.com/library/archive/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/CommunicatingwithAPNs.html), [FCM send](https://firebase.google.com/docs/cloud-messaging/send/v1-api), [FCM errors](https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode), [Expo device tokens](https://docs.expo.dev/push-notifications/sending-notifications-custom.md).

## Offline queue

Same rules as web outbox (sdk-contract "Storage and tabs", "Network behavior").
- Persisted: iOS file in Application Support (`NSFileProtectionCompleteUntilFirstUserAuthentication`); Android app-private file; RN adapter (`@react-native-async-storage/async-storage` optional peer, else memory with warn); headless injected.
- Max 20 messages, attachments held by local file copy up to 100 MB total.
- Send sequentially per conversation, same `client_id` on retry, backoff 1 s to 60 s, on connectivity change (`NWPathMonitor`, `ConnectivityManager`, NetInfo optional) and app foreground. iOS `beginBackgroundTask`, Android WorkManager for one flush after going background. Not guaranteed by OS: next foreground finishes.
- 4xx except 401, 408, 429 marks failed, shows retry. Sign-out or identity switch discards queue.
- History cache: last 50 conversations, 100 messages each per session, for offline reading; wiped on logout.

## Attachments

Three-step upload on existing routes. Picker: `PHPickerViewController`, camera, document picker; Android Photo Picker, `ActivityResultContracts`; RN `onPickAttachment` prop (host supplies file via `expo-image-picker` or own; SDK no hard picker dependency). Client downscales images over 2048 px JPEG unless host disables. Limit `features.max_attachment_mb` checked before upload. Upload in background session on iOS and foreground service-free `OkHttp` on Android; failure retries via outbox. Signed read URLs expire (1 h): refetch message on 403 (builds on F14 download route: assume same signed URL shape).

## Identity

Same handshake table as web (01 README "Boot handshake"), client decides row. Source = async closure provided by host, which calls host's own backend (signs with 02 helper or snippet). Secret never in app, documented loudly; SDK refuses string starting `dmi_idsec_` at init (`identity_secret_public`).

```swift
Dimmi.configure(appId: "wgt_...", apiBase: nil, identity: { try await api.dimmiToken() })
```
```kotlin
Dimmi.configure(context, appId = "wgt_...", identity = { api.dimmiToken() })
```
```tsx
<DimmiProvider appId={process.env.EXPO_PUBLIC_DIMMI_APP_ID} identity={() => api.dimmiToken()}>
```

- Asked on launch, foreground, `identify()`, 401 `session_expired`. `null` = signed out: `DELETE /session`.
- `dmi_ws_` stored in Keychain (`kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`), Android Keystore-wrapped AES file, RN `expo-secure-store` or `react-native-keychain` optional peer (memory + warn when neither).
- Identity token memory only. `Dimmi.logout()` same semantics as web.
- Same-person rule holds: verified customer on app and website = one contact, threads stay on own widget.
- Requires `native_apps_enabled` when widget has allowed sites; else `native_not_allowed`.

## Realtime

One-way SSE with polling fallback, same as web (owner decision).
- iOS: `URLSession.bytes(for:)` + small line parser, no dependency. Android: `okhttp-sse`. RN: `react-native-sse` (MIT, XMLHttpRequest based, no native code, works in Expo) as dependency. Headless web: `EventSource`.
- Foreground only. Background: stream closed, push takes over. Foreground: new connect token, recover from `offset`, then `GET /conversations` refresh.

## Developer context

No patching of URL loading or console. `Dimmi.addDeveloperContext(entry)` / `setContext` send through same `dev_context`, `context`. Automatic `app_version`, `os`, `device_model` in `context`.

## Distribution

| Target | Publish | Human step (publishing accounts) |
|---|---|---|
| npm | `npm publish --provenance` from `sdk-release` job | create npm org `@dimmi`, automation token |
| Swift Package Manager | tag `x.y.z` on `dimmihq/dimmi-ios`; no registry | create public repo, deploy key |
| Maven Central | Gradle `vanniktech` maven-publish plugin (Apache-2.0) to Central Portal | verify namespace `com.trydimmi` by DNS TXT on `trydimmi.com`, Portal token, GPG key |

Semver per package, independent of server. Each declares min server version; config reports `api_version`. Licenses MIT. Reuse check new libs: OkHttp, kotlinx (Apache-2.0), react-native-sse (MIT), vanniktech plugin (Apache-2.0), Maestro (Apache-2.0, test only).

## Example apps

`sdk/examples/expo`, `react-native`, `ios-swiftui`, `android-compose`, `headless-react`, `webview-ios`, `webview-android`. Each: anonymous chat, sign in with fake backend route, push wiring, own-UI variant where relevant. Install endpoint ids `react-native`, `expo`, `ios`, `android` return text from these.

## Tests

| Layer | How |
|---|---|
| Contract | every SDK decodes `tests/contract/widget_v1_fixtures` |
| Headless | vitest on injected boundary fakes; plus integration against real `bin/dimmi-cloud` + compose in `sdk-ci` (no mock server) |
| Server (devices, push) | Go feature tests, real DB, `framework/push` `Fake()`; driver tests `httpclienttest` |
| iOS | XCTest on simulator against real server (macOS runner); `xcrun simctl push` for tap handling |
| Android | instrumented tests on emulator against real server; injected `RemoteMessage` for tap handling |
| End to end | Maestro flows per example: install, send, teammate reply via API appears, kill app offline send survives, sign in merges, sign out wipes, dark mode, RTL |

Real APNs and FCM never called in CI. Test button documented for manual check. No LLM in any test: AI stream via provider fake on server.

## Dependencies

01 frozen API, config, session transfer, hosted page, install endpoint. 02 helpers. 03 eraser hook via 01. 07 stream events. 17 quick replies. 00 prefix registry, `framework/jwt`, `framework/httpclient`, `encryption`.

## Decisions

1. Native UI, not WebView wrapper, for chat itself: push, offline queue and keyboard feel need it. Secondary tabs via hosted page.
2. Customer owns APNs and FCM keys per widget. Why: app's identity, no relay, works on self-host. No dimmi relay service.
3. No mobile key (01 hosted-and-domains.md).
4. Push on foreground skipped by presence. Host decides foreground banner for own notifications.
5. `@dimmi/headless` compiled from widget client source: one protocol.

# 20 Native SDKs: tasks

Style: caveman. Track 10 native-sdks, wave 2. **Starts only after 01 M3 merged and `/widget/v1` freeze committed** (01 tasks "Freeze" lists exactly what is frozen). Needs 01 M11 for hosted page tabs (M4 on) and 01 M8, M9 for theme and texts config (additive, stubbed by fixtures until landed). New migrations `20261201 10 NNNN`.

CI: `sdk-ci` job runs against real binary + compose. No real APNs, FCM or LLM. Each milestone adds Maestro or Playwright flows.

## M1 Headless core (track 10 native-sdks)

- [ ] Package `sdk/packages/headless` builds from `resources/widget/client/`; `createDimmiClient`; adapters for fetch, storage, event source.
  - Tests: contract fixtures; integration against real server: every handshake row, send, stream, recover, polling.
- [ ] Read `surface` when present (booleans, tab residual hidden, no evaluator); `track` posts only `surface.writable_events` names, else no-op. Additive: works with server before and after 18 M5.
  - Tests: fixtures with and without `surface`; 403 `conversation_start_restricted` hides start; bundle holds no rule code.
- [ ] Hooks `@dimmi/headless/react`, example `headless-react`.
  - Tests: Playwright: own UI sends and receives; E43 to E46; size budget 15 KB gz.

## M2 Server: devices and push (track 10 native-sdks)

- [ ] `framework/push` with `apns`, `fcm` drivers, `Fake()`, `drivertest`, 90% coverage. No product words.
  - Tests: `httpclienttest` per error table row; provider JWT reused under 60 min; OAuth token cached; HTTP/2 used for APNs.
- [ ] Migrations `20261201100001` to `003`; devices routes; credentials routes + tests collection; events; policy.
  - Tests: upsert moves token; other session 404; secret never returned; audit; E13 to E15, cross-tenant 404.
- [ ] `ScheduleWidgetPush` + `SendWidgetPush`; hook device deletion into `RevokeWidgetSession`, `EraseWidgetContactData`, `RepointWidgetSessions`.
  - Tests: E17 to E19, E21; other widget's devices never pushed; spam, blocked, test session rules; logout stops push; locale text.
- [ ] Settings "Apps" section: upload, status, Test (`resources/js/features/settings/messenger/sections/apps.tsx`), install id blocks.
  - Tests: Playwright light and dark with `Fake()`.

## M3 React Native and Expo (track 10 native-sdks)

- [ ] `@dimmi/react-native` on headless: `react-native-sse`, storage adapters, config plugin.
- [ ] Screens: home, list, thread, composer, status, seen, typing, streaming, cards, theme, RTL.
- [ ] Offline queue + cache; attachments via `onPickAttachment`; push helpers.
- [ ] Examples `expo`, `react-native`; Maestro flows both platforms.
  - Tests: E23, E25 to E27, E30, E31, E35 to E37, E40. Size 60 KB gz.

## M4 iOS (track 10 native-sdks)

- [ ] `Dimmi` core: Codable models from fixtures, session state machine, Keychain, outbox, SSE parser, polling, PoW, device registration.
  - Tests: XCTest against real server; E1 to E5, E9, E25, E28, E29, E32; size 1 MB.
- [ ] `DimmiUI` SwiftUI screens; attachments; hosted web sheet via transfer.
  - Tests: Maestro on simulator; dark, RTL, Dynamic Type; `simctl push` tap (E20); E41, E42.
- [ ] Example `ios-swiftui`, `webview-ios`; mirror release job dry run.

## M5 Android (track 10 native-sdks)

- [ ] `dimmi-android`: models, session, Keystore store, outbox with WorkManager flush, okhttp-sse, polling, PoW, device registration, `DimmiMessagingService`.
  - Tests: instrumented on emulator against real server; E1, E2, E5, E24, E25, E28, E29; size 1 MB.
- [ ] `dimmi-android-ui` Compose screens; Photo Picker; hosted web view.
  - Tests: Maestro; dark, RTL, font scale; push tap via injected message.
- [ ] Example `android-compose`, `webview-android`; Gradle publish to local repo in CI, signed dry run.

## M6 Release (track 10 native-sdks)

- [ ] `sdk-release` workflow: npm provenance, Swift tag mirror, Central Portal upload; versions; changelogs; install templates final.
  - Accept: dry run green. Real publish waits on human steps (npm org, GitHub repo, Central namespace DNS TXT, GPG key).
- [ ] Docs pages per platform. `dimmi init` picks up `react-native`, `expo`, `ios`, `android` ids (09 delta).

## Done when

- Expo example: signed in, sends message, teammate replies: appears live; background app: push arrives on real device (manual step documented), tap opens thread; offline send survives kill; sign out wipes.
- Same on iOS and Android examples. Own-UI headless example works on web.

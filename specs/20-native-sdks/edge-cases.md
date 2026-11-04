# 20 Native SDKs: edge cases

Style: caveman. Each: case. decided behavior. Session and message rules from 01 edge-cases apply unchanged.

## Identity

1. Developer passes `dmi_idsec_` into SDK. Init throws `identity_secret_public`. Never sends.
2. Identity closure throws at launch. Treated signed out for this launch (fail closed) with warn; retried on foreground. Stored identified session not shown meanwhile.
3. App reinstalled on iOS: Keychain survives, app files do not. `ThisDeviceOnly` item read; first launch after reinstall detected by missing marker file: wipe Keychain items, start clean.
4. Device restored to new phone. Keychain item `ThisDeviceOnly` not restored. New anonymous; identified restores via token.
5. User A signs out, B signs in. `DELETE /session`, wipe outbox, cache, device row; `POST /sessions`; `POST /devices` for B.
6. Lead on app signs in. `previous_token` flow; device row follows new session (upsert by token).
7. Same user on app and website. One contact; threads stay per widget; web thread never lists in app (different widget) unless same widget id used.
8. App and website share one widget id. Allowed: threads visible on both. `native_apps_enabled` needed if allowed sites set.
9. Session revoked server side while app backgrounded. Next foreground 401 `session_revoked`: wipe, anonymous. Device row already deleted.

## Push

10. Token rotates. `onNewToken` -> `setPushToken`; old row replaced by `PATCH`, or dead by provider later.
11. Same phone token used by two signed-in apps with same widget. Token unique per widget; last session wins.
12. Build run from Xcode (sandbox token) against production host. Environment detected from profile; explicit mismatch 400 `BadDeviceToken`: row disabled; dashboard test explains sandbox and production.
13. Wrong bundle id in credentials. `DeviceTokenNotForTopic`: credential `failing`, row disabled, banner.
14. `.p8` revoked by customer in Apple portal. 403: `failing`, one event, send paused 1 hour. Test button clears status on success.
15. Service account lacks role. 401 or 403 mapped; same.
16. Provider 410 lag after uninstall. Sends until 410, then disabled. Accepted (Apple documents delay).
17. Ten replies in 5 s. One push, collapse id, unique job. Badge latest.
18. App open on phone A (stream live), phone B locked. Session of A skipped by presence, B pushed.
19. Conversation read on A then push job fires for B. Skipped when all read; already-delivered banner stays until opened (no silent pushes, reason: needs background modes in every host app).
20. Push tap for conversation contact no longer owns (logged out meanwhile). `handleNotification` true, opens list, toast "not found".
21. `push_preview = false`. No message text leaves dimmi to Apple or Google.
22. Self-host behind firewall. APNs, FCM hosts must be reachable; `dimmi doctor` line when credentials exist. Failure surfaces via status.
23. Expo Go. `push_unavailable` warn; chat works.
24. Android 13+, user denies notification permission. Token may exist, display silently blocked. `setPushEnabled(false)` from host sends `DELETE /devices`; SDK checks on foreground and does it itself.

## Offline, network

25. App killed with queued message. Persisted, sent next launch, same `client_id`, server dedupes.
26. Attachment file deleted by OS before send. Marked failed with retry hidden; text kept.
27. Queue full (20). New send refused with visible error; never drops oldest silently.
28. Clock wrong. Server decides; display uses `server_time` offset.
29. Corporate proxy buffers SSE on phone. Same fallback rule: polling in foreground only.
30. App foreground after 3 hours. `recovered: false`; REST refresh; dedupe.
31. Airplane mode at launch. Cached config and history shown; composer queues; banner.
32. Config cached with old `config_version`. Session response newer: refetch.

## Config

33. Widget disabled. 403: messenger screen shows closed state, launcher APIs no-op, log `widget_disabled`.
34. Allowed sites set, `native_apps_enabled` false. 403 `origin_not_allowed`; SDK logs `native_not_allowed` naming setting.
35. `open_to = conversation`. Native home skipped, same rule as web.
36. Tab Help on, no web view peer on RN. System browser with transfer URL.
37. Tabs list only `messages`. No tab bar.
38. Old SDK meets new config key. Ignored. Old SDK meets removed frozen key: cannot happen without `Dimmi-Version` entry.
39. Background image large. Cached on disk with size cap; failure falls back to `header_color`.
40. RTL locale. Layout mirrors by `dir`, independent of device locale.

## Hosted page in native

41. Transfer token expires before web view loads. Page anonymous + "Open again"; SDK retries once with fresh token on web view error event.
42. Web view session separate row. Logout in app revokes both: 01 `TransferWidgetSession` stores `transferred_from_session_id`; `RevokeWidgetSession` revokes children too.

## Headless

43. Hooks used outside provider. Throws clear message in dev; no-ops in production plus `error` event.
44. StrictMode double effect. One stream, ref counted.
45. Two clients same app id. Second shares singleton per `appId`; warn.
46. Server render. No network; hooks return defaults.

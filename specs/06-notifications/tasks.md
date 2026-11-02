# 06 Notifications: tasks

Style: caveman. Ordered. Each about 1 day, merged with tests. Requires foundation + framework done (events, River, broadcasting on centrifuge, `mail` named `system` mailer, `httpclient`, `routing` signed URLs, `encryption.KeyManager`, Valkey `cache`/`ratelimit`, `notifications/channels/webpush` on webpush-go).

Test rule (00-conventions/testing.md): business logic tested only through integration tests (real Postgres, real Valkey, real App via `testkit`); fakes only at process boundary (`tc.Mail`, `httpclienttest.New(tc.App.HTTP)`, push service stubbed via `httpclienttest` (webpush channel sends through `a.HTTP`), `tc.Clock`). No app unit tests; "unit test" items below run as feature/action tests. Test names carry edge ids (`E06-96`).

## M1: Schema and in-app

- [ ] Migrations 20261101000601..607 (0601 ALTERs framework `notifications` base table, never CREATE) + sqlc queries (`notifications.sql`, `notification_preferences.sql`, `notification_settings.sql`, `notification_deliveries.sql`) + factories. Add `notifications` queue to `config/queue.go`, prefixes `ntf_ psub_ ndst_ ndlv_ mem_` to `app/models/public_ids.go` (`framework/ids` registry), abilities `notifications:read|write`.
  - Accept: `dimmi migrate` clean up/down; tenant lint passes (push_subscriptions allowlisted); ids round-trip tests.
- [ ] Enums + `defaults.go` matrix + locked cells. `notifications.trigger` (and preferences/deliveries/destination trigger columns) as registry enum: format CHECK + `NotificationTrigger` Go registry with per-trigger defaults; other specs register at boot.
  - Accept: integration test every trigger x channel default via `GET /notification-preferences`; locked cells listed; registering fake trigger in test adds matrix row with no migration; unknown trigger 422 (edge case 115).
- [ ] `audience.go` resolver (assignee team, inbox default team, all members, minus actor, active only).
  - Accept: integration tests (real teams/inboxes rows) cases 6-9 from edge-cases.
- [ ] `DispatchNotifications` job + `SendMemberNotifications` listener for `conversation.created`, `conversation.assigned`; database channel upsert with aggregation + `notification.created` broadcast.
  - Accept: feature test: create conversation, member gets row + realtime event on `member:{mem}` (`tc.AssertBroadcast`); self-assign no row; retry job twice no duplicates.
- [ ] Remaining triggers: `customer_replied` (first-message skip, reopen audience, `notify_unassigned_replies`), `mentioned` (03 `message_mentions` rows via `message.created.MentionedMemberIDs` + `message.updated.AddedMentionedMemberIDs`), `ai_handoff_requested`, `snooze_expired`, `system_alert` `Notify` API.
  - Accept: `dispatch_triggers_test.go` covers each trigger table row + actor exclusion + spam/blocked skip.
- [ ] Notifications API: index (filters, cursor, counts), show, `PATCH /notifications/{id}` (`read` bool, `seen` true; field-dispatch to mark actions), NotificationResource, policy.
  - Accept: feature tests happy path, same value no-op no broadcast, `seen: false` 422, other member 404, other workspace 404, token ability 403, `member_actor_required`, first-party app token allowed like session, unread cap 100.
- [ ] Bulk `PATCH /notifications` (`NotificationController.UpdateMany`, `BulkUpdateNotifications`): ids (max 100, rule 8 per-id results) or filter `{conversation_id, trigger, before}` (meta only, batched 5,000, one broadcast).
  - Accept: mark all read with `{filter: {}}` over 50,000 rows (edge 88); seen up to `before`; ids from other member `failed not_found`; ids and filter together 422.
- [ ] `conversation.read` listener marks conversation notifications read + `notifications.read` broadcast. Spam/deleted listener cleanup.
  - Accept: read conversation via 03 API, bell count drops, broadcast received.
- [ ] `member.deactivated` cleanup + 03 contact eraser `EraseNotificationContactData`.
  - Accept: deactivated member gets no delivery queued before deactivation (skipped `member_inactive`); erased contact name/excerpt gone from `notifications.data` and pending emails canceled; other workspace notifications untouched.
- [ ] MCP tools `dimmi_list_notifications`, `dimmi_mark_notifications_read`.
  - Accept: MCP tool tests via in-process MCP client, schema from OpenAPI types.

## M2: Preferences, settings, suppression

- [ ] Preferences API (matrix, PATCH sparse, `DELETE /notification-preferences` reset 204) + settings API (quiet hours, pause, previews, include_ai_handled, delay, timezone).
  - Accept: tests for locked 422, unknown trigger 422, default-equal deletes row, timezone validation, start==end 422.
- [ ] `quiet_hours.go` with DST handling.
  - Accept: dispatch integration tests with `tc.Clock.Freeze` in Europe/Berlin and America/New_York spring/fall transitions, overnight window, day mask.
- [ ] Focus heartbeat endpoint + Valkey storage (`ntf:focus:{mem}:{device}`, `SET EX 75`).
  - Accept: feature test heartbeat then dispatch: push `suppressed focused`; expired after clock advance.
- [ ] Suppression pipeline (`suppression.go`) ordered rules + `notification_deliveries` rows (outcome + reason; no read API at MVP).
  - Accept: `suppression_test.go` one test per reason; `preference_off` writes no row.
- [ ] Burst (`a.Limiter` sliding windows, Valkey) + cooldown (`SET NX EX 30`) + `SendNotificationDigest`.
  - Accept: 20 conversations in 1 minute (clock frozen): 5 push sent, 15 batched, 1 digest; items read before digest dropped.

## M3: Email

- [ ] `MemberNotificationMail` + templ templates per trigger + layout footer (open, manage, unsubscribe) + headers (List-Unsubscribe, One-Click, Auto-Submitted, Message-ID threading).
  - Accept: `tc.Mail` asserts system mailer used (tag `stream=system`), headers, subject format, preview off hides body, HTML escaping of `<script>` in excerpt.
- [ ] Delayed email (customer_replied 120 s) with read and condition re-check; stale 2 h.
  - Accept: tests with `tc.Clock.Freeze(t)`: read before delay skips; teammate reply skips.
- [ ] Unsubscribe web routes `GET`/`POST /notification-unsubscriptions` (signed GET confirm page, POST RFC 8058 one-click, CSRF exempt), `UnsubscribeFromEmail` action.
  - Accept: GET changes nothing; POST disables cell; tampered signature 403; removed member 200 page.
- [ ] `SendQuietHoursSummary` + email cap digest fallback (cap from `Entitlements.Limit(ws, "notifications.email_daily_per_workspace")`, catalog entry self-host -1, cloud 5000; remove `notifications.cloud.*` config key).
  - Accept: fake entitlements driver limit 3: 4th email goes to digest; limit -1 never caps; override change honored after cache bust (edge case 116).
  - Accept: suppressed items during window produce one summary email at end; nothing if all read.

## M4: Web Push

- [ ] Push channel on framework `notifications/channels/webpush` (wraps SherClockHolmes/webpush-go v1.4.x; RFC 8291 + VAPID in library, own crypto removed): options (`HTTPClient` = `a.HTTP.StdClient()`, TTL, Urgency, Topic, `VapidExpiration` 12 h, Subscriber).
  - Accept: `httpclienttest` stub captures push request; `VapidExpiration` under 24 h; Topic length 32 and URL-safe. RFC 8291 vectors live in framework tests.
- [ ] VAPID key bootstrap: config or auto-generate into `system_settings`, `dimmi push:vapid`, `push-config` endpoint (`key_id`, insecure origin flag, iOS hint).
  - Accept: first boot generates once under advisory lock (two replicas race test); `--force` rotates.
- [ ] Push subscriptions API (`U` guard: session or first-party app; register upsert, list, delete, `old_endpoint` replace, host allowlist, key validation, 20 limit eviction, reassignment across users) + `kind` column (`web`; `apns`/`fcm` 422 `push_kind_unsupported`) + `session_id`/`oauth_grant_id` exactly-one CHECK + `device_id` column (session rows copy `sessions.device_id`; app rows body value checked against grant `device_id`, 422 `device_id_mismatch`) + partial index `(user_id, device_id)` + logout and grant-revoke hooks (hard dependency on 09 `oauth_grants` + `oauth_grant.revoked`; migration order test).
  - Accept: tests edge cases 50-56, 97-101; SSRF host rejected; PAT 403 `first_party_only`; logout, session revoke, `oauth_grant.revoked` and `user.deleted` remove subscriptions (FK cascade + listener); no push sent to subscription after its session or grant gone; CHECK rejects web row missing keys; edge cases 110 to 112, 118.
- [ ] Push channel delivery + response mapping (201, 400, 403 variants, 404/410 delete, 413 minimal retry, 429, 5xx) + stale guard. Shared `PushPayload` builder enforcing payload rule (ids + optional title/body, under 3,000 bytes plaintext) reused by later APNs/FCM.
  - Accept: `httpclienttest` push stub per status; subscription deleted on 410; metrics incremented; builder test every trigger with max inputs under limit, no contact email/attachments (edge cases 113, 114).
- [ ] Test notification endpoint (`POST /notification-tests`, 201 `notification_test` outcome) sync result + `dimmi notifications:test`.
  - Accept: returns per-device outcome within 10 s with fake push server.

## M5: Slack

- [ ] Destinations CRUD API (workspace + member scope), URL shape validation, encryption, `url_hint`, policy (`authz.Can(p, "notification_destinations:manage")` for workspace scope; no role names), limit 25, audit log. `system_alert` recipients resolved by `notification_alerts:receive` permission (both 06 permissions registered in 00-accounts catalog).
  - Accept: member creating workspace destination 403; URL with `hooks.slack.com.evil.com` rejected; secrets never in response.
- [ ] Slack incoming webhook channel: Block Kit builder with escaping, error mapping, destination health transitions, `system_alert` once per transition.
  - Accept: tests for 403/404/410/429/5xx; `<!channel>` in customer text stays literal.
- [ ] Slack app OAuth install (start, callback, state), `slack_app` + auto `slack_webhook`, member `slack_dm` link via `users.lookupByEmail`, DM send via `conversations.open` + `chat.postMessage`, uninstall hook from 05.
  - Accept: fake Slack API tests; expired state 410; token revoked marks error.
- [ ] Workspace destination fan-out (`DeliverDestinationNotification`): triggers, inbox filter, AI-handled filter, burst digest per destination.
  - Accept: "post every new conversation to #support" test; 15 conversations in a minute: 10 posts + 1 digest.
- [ ] Destination test endpoint `POST /notification-destinations/{id}/tests`.
  - Accept: sync outcome resource (201) returned, error surfaced with Slack error string; throttle 6/min.

## M6: Triggers from 05, 07, 08, 13, 14, 15-announcements-surveys, 16 guard

- [ ] Registry entries + defaults for 14 `feedback_post_submitted`, `feedback_post_pending` (email delay 300 s, `condition_changed` when moderated), `feedback_comment_created`, `feedback_mentioned`; 15-announcements-surveys `csat_negative`; destination-only `changelog_entry_published` (13, `announce = true`) with `destination_eligible` flag; owner specs register at boot, 06 ships fan-out + aggregation on `feedback_post_id`.
  - Accept: feature tests per trigger with real source events (factories from 13/14/15 or stubs until merged); aggregation per feedback post; `csat_negative` assignee then team fallback, `is_test` skip; changelog post appears only on destinations listing it; edges 119-123.
- [ ] `system_alert` sources from 13 (source errored, mailing blocked/failed, scheduled publish failed) via `Notify`.
  - Accept: one alert per transition; recipients by `notification_alerts:receive`.
- [ ] 07/05/08 sources: registry entry + defaults `ai_action_approval_requested` (07 class; audience assignee -> team holders of `ai_action_calls:approve` -> admins; `condition_changed` when no longer pending); `system_alert` sources `ai.action_connection_failed` (07), `channel.errored`/`channel.disconnected` + recovered resolve (05), optional `github_connection.errored` / repo webhook `failing` (08) with per-subject 24 h dedupe; destination-only `docs_suggestions_ready` (08, `suggestion_count > 0`).
  - Accept: feature tests with real source events (factories until owners merge); edges 129-134.
- [ ] Imported guard (16): `SendMemberNotifications` skips `metadata.imported`; metric reason `imported`. Accept: edges 135, 136.

## M7: System stream feedback

- [ ] Migration `20261101000608_create_system_mail_suppressions_table.sql` + `RecordSystemMailFeedback` (called by 04 router for `stream=system` events; event-id dedupe Valkey + unique) + `system_suppression_guard.go` on system mailer (critical auth bypass 3/day) + email channel `email_suppressed` + `system_alert` `system_mail.suppressed` + lift listener (`user.email_verified`, email change) + `dimmi mail:suppressions --stream system` + ops alert gauges.
  - Accept: SES and Postmark fixtures routed through 04 webhooks reach 06; member notification email skipped after hard bounce; magic link still sent (critical) up to 3/day; complaint never touches 04 `email_suppressions`; edges 124-128.

## M8: Hardening

- [ ] Metrics, logs (redaction of endpoints/URLs/tokens), traces; `dimmi doctor` checks (VAPID, subject, From vs inbox, push host reachability).
  - Accept: doctor output tests; log redaction test with Slack URL and `xoxb-`.
- [ ] Prune jobs (notifications 90 d, deliveries 14 d, push subscriptions rules) in `routes/console.go`.
  - Accept: tests with frozen clock.
- [ ] Latency benchmark: 1,000 events with 10 members each, fake providers with 200 ms latency; assert p95 commit-to-accept under 5 s, p99 under 10 s on CI runner.
  - Accept: CI benchmark job, regression over 10% fails.
- [ ] Docs: notifications page (channels, iOS Home Screen steps, Slack setup, quiet hours, troubleshooting via `notification-tests` outcomes), changelog policy for default changes.

## Done when

- `go test ./...` green; feature and contract tests above pass with real Postgres + Valkey and only process-boundary fakes.
- Demo script (local, `docker compose up`, real browser + real Slack test channels):
  1. Fresh workspace, owner only. Open widget, send "checkout broken". Owner email (Mailpit in compose) arrives under 10 s with subject `[Acme] New conversation from Visitor…`, working "Open conversation" link and one-click unsubscribe.
  2. Dashboard: enable push (Chrome desktop). `POST /notification-tests {channel: push}` shows `sent`. Customer sends second conversation: desktop push under 5 s; click opens conversation.
  3. Keep conversation open and focused, customer replies: no push; delivery row (and log line) shows `suppressed focused`; bell count updates live.
  4. Admin pastes Slack incoming webhook for `#support` (triggers new_conversation, ai_handoff_requested) and second Slack webhook for `#support-alerts` (handoff only). New conversation posts to `#support`; AI handoff posts to both. Customer text `<!channel>` does not ping.
  5. Revoke Slack webhook in Slack. Next post: destination `error`, admin gets in-app + email `system_alert`.
  6. Set quiet hours to cover now, bypass `ai_handoff_requested`. New conversation: no push; handoff: push arrives. At window end: summary email.
  7. Fire 20 conversations in 1 minute via API: 5 pushes then 1 digest push.
  8. iPhone (iOS 18.4+), dashboard added to Home Screen, enable push: lock-screen notification arrives, tap opens conversation. Delete PWA: next send 410 and subscription row gone.
  9. Logout on browser: next notification sends no push to that browser.
  10. `PATCH /notifications {filter: {}, changes: {read: true}}`: bell count 0 on every open tab and first-party app via `notifications.read`.

# 15 Engage: tasks

Style: caveman. Ordered. Each task about 1 day, with acceptance and tests. Depends on M3 (01 + 02 + 03) and 18 M1 to M4 done. Runs parallel to M4 tracks. Tests per `00-conventions/testing.md`: integration only for app logic (real App via `testkit.New(t)`, real Postgres `pgvector/pgvector:pg18`, real dispatcher, River drained with `tc.Queue.Drain(ctx)`, real RLS, real Valkey). No unit tests with mocks, no sqlmock/pgxmock/embedded-postgres. Fakes only at process boundaries: `tc.Mail` (captured MIME), `httpclienttest.New(tc.App.HTTP)`, `tc.Clock`. Pure helpers (`urlmatch` vectors, rule parser) may use table-driven tests without mocks or I/O. Tests named in plain words; every edge case covered. No real LLM, no network.

## E0 Foundation hooks (needs owner edits in "Required from other specs")

- [ ] 1. Migrations `20261101001501`..`001510`, RLS, sqlc queries, factories. Accept: `dimmi migrate` clean up/down locally; cross-tenant query lint passes. Tests: factory create per table; RLS isolation test (other workspace sees nothing).
- [ ] 2. Prefixes `ann_ svy_ svq_ svr_ erc_ eas_` in `app/models/public_ids.go` (`framework/ids` registry); abilities `engage:read`, `engage:write` in ability registry, permission `engage:manage` (00-accounts catalog); enums. Accept: wrong prefix 404; token without ability 403 `missing_ability`. Tests: ids round trip, ability matrix.
- [ ] 3. `engage_settings` lazy ensure + `BumpEngageVersion` + `live_summary` compute; `GET/PATCH /engage-settings`; 01 config `engage` block + cache bust listener. Accept: publish item flips config `active` within one cache cycle; workspace without items returns `active:false`. Tests: version bump in same tx, rollback leaves version.

## E1 Consume 18 (15 builds no evaluator)

- [ ] 4. Item `audience` and `trigger.page` parsed, validated, linted through `app/support/audience`; surfaces `engage.announcement`, `engage.survey` registered; item save rewrites `audience_references`. Tests: 18 error codes returned with JSON path; page tree refuses non-page fields; references written and removed.
- [ ] 5. Eligibility and anonymous list read `audience.Partial`; widget item carries residual; canary rule test. Accept: response holds no attribute, event or audience names; country-dependent rule absent from CDN list. Tests: integration on real Postgres, fake clock for hour bucket in ETag.
- [ ] 6. Editors embed shared builder `resources/js/features/audiences/`; `filter[audience]` on `/announcements` and survey results; `meta.affected_items` on saved audience patch. Tests: feature per filter; e2e light and dark.

## E2 Assets and rendering

- [ ] 7. Engage assets: upload (shared `app/support/images` from 08 on `framework/image`), disk `a.Storage.Disk("public")`, dedupe, scan (`a.Scanner`, `malware` fake with EICAR verdict), alt, public route, purge orphans. Tests: SVG refused, bomb guard, infected 404, immutable headers.
- [ ] 8. `RenderEngageContent`: `framework/markdown` with 08 `helpmarkdown.Engage()` options + `framework/sanitize`, variables with fallbacks, per-locale cache, contrast check, URL validation. Tests: XSS corpus, `javascript:` refused, variables, RTL `dir`.

## E3 Announcements

- [ ] 9. Announcement CRUD, typed content per format, translations, soft delete/restore, duplicate. Tests: validation per format, `format_locked`, trashed filter.
- [ ] 10. Status machine + publish validation + live caps from entitlements (row lock, 422 `entitlement_limit_reached` with `key`, `limit`) + audit + events. Tests: every transition, `invalid_transition`, concurrent cap race, member cannot publish, member can pause.
- [ ] 11. `TransitionEngageSchedules` job + `live_summary.next_transition_at`. Tests: `tc.Clock.Freeze`, end at `ends_at`, DST absolute times.
- [ ] 12. Stats tracker (`app/support/engagestats`) + flush + `/stats` + `/receipts`. Tests: flush batching, shutdown flush, rates math.

## E4 Widget delivery

- [ ] 13. Eligibility engine `ResolveEligibleItems` (audience, schedule, frequency, throttle, cooldown, priority, size cap, ETag). Accept: p95 under 20 ms at 1M contacts, 70 live items (bench). Tests: integration, table-driven per rule over factory data, throttle window, recurring cycles.
- [ ] 14. `GET /widget/v1/engage-items` (session, ETag 304, `preview` for test sessions) and `GET /widget/v1/apps/{app}/engage-items` (partial evaluation, CDN headers). Tests: anonymous never gets contact-targeted items, fallback variables, 304.
- [ ] 15. `POST /widget/v1/engage-events` (batch, receipts upsert, `next_eligible_at`, clamp times, local receipt upload, rate limits on Valkey `a.Limiter`). Tests: dismissal permanent, `until_engaged` counting, anonymous counters only, idempotent replay.
- [ ] 16. Manual routes `GET /widget/v1/announcements/{ann}`, `/surveys/{svy}`; deep link contract; chat reply via 01 `announcement_id` + 03 event row. Tests: not eligible 404, reply creates contact-initiated conversation with `announcement.replied` row, no fair-use count.
- [ ] 17. Widget scheduler checks residual through 18 `resources/widget/lib/rules.ts` on each navigation. Tests: shared corpus from 18; SPA navigation cancels delay timer.
- [ ] 18. E2E protocol test (Node, `tests/e2e/widget_engage_protocol_test.go`): config flag, anonymous list, session list, events, preview. Accept: green in CI.

## E5 Surveys

- [ ] 19. Survey CRUD, templates endpoint, inline `questions` with `ref`, questions child collection, reorder, archive-on-delete. Tests: refs mapping, `question_type_locked`, `order_mismatch`.
- [ ] 20. Branching validator (forward-only, archived targets, path computation shared by widget, API and CSAT rating page). Tests: graph cases, `answer_not_on_path`.
- [ ] 21. Widget responses `POST/PATCH /widget/v1/surveys/{svy}/responses`: partial save, resume state, revision compatibility, completion, redaction, write-back, receipt completed, events. Tests: idempotency, cross-device resume, `survey_changed`, card redaction, write-back limit.
- [ ] 22. API-submitted responses `POST /surveys/{svy}/responses` (contact by id/external_id/email). Tests: replay by `client_id`, draft refused.
- [ ] 23. Responses list + filters + FTS `q` + CSV stream (formula guard, BOM, 1M cap) + delete + audit. Tests: CSV injection fixtures, large export streaming memory under 50 MB.
- [ ] 24. Results computation (headline per kind, funnel, distributions, trend, confidence, Valkey cache 60 s, timeout 503). Accept: bench 100k under 150 ms, 1M under 400 ms. Tests: NPS math fixtures.

## E6 CSAT

- [ ] 25. 03 `SendSystemMessage` integration (owner 03 ships action) + `ScheduleCsatRequest` listener + `SendCsatRequest` job with all skip reasons + metrics. Tests: each skip reason (incl. imported conversation, E134), reopen cancel, cooldown, unique per close; 16 import fixture with closed conversations + source ratings creates zero requests and zero responses (E134, E135, E136).
- [ ] 26. Email rendering through 04 email `Channel.Send` (`outbound_emails.kind system`, thread headers, `Auto-Submitted`, customer stream, `email.IsSuppressed(ctx, ws, email)` pre-check, default scope `all`), widget card metadata (realtime `widget_contact:{ctc}`), `ExpireSurveyRequests`. Tests: `tc.Mail` asserts links, headers, stream tag `customer`; suppressed skip; `tc.AssertBroadcast("conversation:"+conv.PublicID, "survey_response.completed")`; expiry.
- [ ] 27. CSAT email rating page `GET/POST /csat-ratings/{svr}`: templ views, signed URL (`svr` + `expires`), GET never writes, POST answers first question (+ text comment) and completes, rate limit (Valkey), CSP. Not a general hosted survey page (later: hosted-survey-page). Tests: scanner GET, tampered signature 403, expired request closed page, completed thanks, no-JS flow, axe-core accessibility check on rendered HTML.
- [ ] 28. Conversation and contact response endpoints, 03 `include=csat` data provider. Tests: ownership 404s, ability gates.

## E7 Agent-native, integrations

- [ ] 29. MCP toolset `engage` (8 tools), confirm gate on publish, token budget under 2,000. Tests: MCP integration (real tool handler): tools/list budget, confirm path (elicitation + `confirm` param), untrusted envelope.
- [ ] 30. CLI `announcements`, `surveys` with front matter parser, `--dry-run`, `--csv`. Tests: CLI integration against real App HTTP server (`testkit`), golden outputs.
- [ ] 31. Webhook catalog entry `survey_response.completed` + fixture; 06 `csat_negative` trigger wiring. Tests: catalog CI check, real dispatcher creates 06 `csat_negative` notification rows.

## E8 Privacy, ops, hardening

- [ ] 32. Eraser `EraseEngageContactData`, contact export hook, merge repoint listener, prune and purge jobs. Tests: eraser idempotent in batches, merge conflict rules, retention prune.
- [ ] 33. Benchmarks + observability: metrics, `dimmi doctor` table size line, load test 200 rps engage-items with 1M contacts seed; seeder `engage_seeder.go`. Accept: budgets in README met; CI bench regression gate 10%.

## E9 Cross-feature alignment (08, 13, SDK)

- [ ] 34. CTA targets + SDK mappings: `open_article` -> `show('help', {article})` with per-widget Help tab check (warning `help_tab_disabled_cta`), `open_changelog_entry` validated by 13 `changelog.FindPublishedEntry` -> `show('changelog', {entry})`, `open_messenger` -> `startConversation(prefill, {context: {announcement_id}})`, `sdk_event` -> `engage_action {id, ctaId, eventName}`. Tests: E62, E63, E129-E131; camelCase payload keys in e2e test.
- [ ] 35. Coverage review: every edge case in edge-cases.md has a test (review check).

- [ ] 36. Valkey wiring per D11: otter L1 + Valkey pubsub version bust, results cache on `cache.Store` (Valkey), limiters on Valkey, receipts stay Postgres. Tests on real Valkey (compose `valkey/valkey:8` via `REDIS_URL`, per-test prefix): E132 (test's keyspace flushed mid-test: no item reappears), E133 (Valkey unreachable via in-test TCP proxy cut: lists served, limiter fails open).

## Done when (demo script)

1. `docker compose up`, sign in, seed demo app with widget (01) and 1,000 contacts (`dimmi db:seed --class=EngageSeeder`).
2. Open demo site anonymous: DevTools shows only config request (no engage requests, `engage.active=false`).
3. `dimmi audiences preview` on rule `contact.custom.plan in [pro]`: count 312.
4. `dimmi announcements create --file launch.md --publish --yes` (modal, audience Pro, `/dashboard*`). Reload demo site as Pro user (JWT): modal appears after 3 s, keyboard Tab cycles inside, Esc closes. Reload: not shown again. Same user on second browser: not shown.
5. Free user: no modal. Anonymous visitor: no modal (identified audience).
6. Create critical banner via API to `everyone`, `bypass_throttle`: appears for anonymous visitor within 60 s; `PATCH {status: "ended"}`: gone on next load.
7. Publish NPS popover (template `nps`, recurring 90 days). As Pro user answer 5, branch shows "What should we fix?", close tab mid-way, reopen on other browser: resumes at question 2, submit. `GET /surveys/{svy}/results`: NPS -100, 1 response. Webhook test endpoint (request bin) receives `survey_response.completed`.
8. `Dimmi('track', 'export_completed')` in console: export-feedback survey shows; no network call for `track` itself.
9. Turn on CSAT (delay 0 for demo). Close widget conversation in inbox: CSAT card appears in widget thread; rate 2: inbox sidebar shows rating, assignee gets `csat_negative` notification. Close email conversation: CSAT email arrives (Mailpit) in same thread; link opens CSAT rating page with 4 preselected; no-JS submit works.
10. `dimmi surveys responses svy_... --csv > nps.csv` opens in spreadsheet with BOM, no formula execution.
11. Billing (cloud mode demo): usage page shows 0 ledger lines from announcements, surveys, CSAT; chat-format reply conversation answered by teammate shows 1 normal charge.
12. Erase demo contact: receipts and responses gone, results counts drop, stats daily unchanged.

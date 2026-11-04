# Build tracks

Style: caveman. Date 2026-11-04. Split of all work from "everything now" round into parallel tracks for agents in separate git worktrees. Read `CLAUDE.md`, `specs/00-conventions/*`, `specs/00-foundation/*`, then track's specs. Where track table and a spec disagree on order or number, this file wins and disagreement gets reported.

## Waves at a glance

| Wave | Track | Goal | Start |
|---|---|---|---|
| 1 | 01 widget-platform | Real messenger: API, sessions, SSE, static widget connected, settings page, install, abuse, offline email, all settings | now |
| 1 | 02 inbox-access | Team-based conversation access, assignment, away, capacity, real inbox settings page | now, first merge fast |
| 1 | 03 inbox-workbench | Views, sort, keyboard, Cmd+K, composer, drafts, snooze, undo send, bulk, mentions, start conversation, search UI | now |
| 1 | 04 customer-records | Tags, panel, directory, companies, merges, To/Cc/Bcc, spam, block, malware, redaction, erase, export, retention | now |
| 1 | 05 email-sending | Custom domains, own address, signatures, bounces, suppression, limits, sender rules, staging infrastructure | now, M6, M7 wait fixes |
| 1 | 06 email-inbound | SES route, setup guides, test button, mistake detection, providers, IMAP, Google + Microsoft mailboxes | now, M1 to M5 wait fixes |
| 1 | 07 accounts-settings | Profile, password, sign-in page, 2FA help, members, preferences, audit, rate limits, language | now |
| 1 | 08 enterprise-identity | Sign-in rules, session length, IP allowlists, SAML, OIDC, SCIM, custom roles | now |
| 2 | 09 widget-identity | Signed-in customers: JWT, secrets, modes, validations, per-stack packages | after 01 M6 |
| 2 | 10 native-sdks | Headless, React Native + Expo, iOS, Android | after 01 M3 + freeze |
| 2 | 11 notifications | In-app, email, push, Slack, mention and assignment triggers, settings page | after 03 W9 write side, 02 A1 |
| 2 | 13 audiences | One rule language: M1 to M4 core, then M5 widget, M6 events, M7 help center, M8 lists | after 02 A1, 03 W1 to W2, 04 R1, R2 |
| 3 | 12 inbox-extras | Table layout, folders, side conversations, similar, attributes, translation, scheduled export, away reasons | after 03 + 04 main milestones |
| 3 | 14 engage (reserved) | 15 announcements + surveys consume audiences | after 13 M4 |
| 3 | 15 workflows (reserved) | 17 engine consumes audiences and 02 picker | after 13 M4, 02 A3 |

Everything is launch scope. Waves are order, not priority cut. 05, 07, 08, 09, 10, 11, 12, 13, 14, 16 folders not listed stay on [README.md](README.md) build order; a later track takes next free number from 20 up.

## Rules for every track

1. **One worktree, one branch, one track.** Branch `track/NN-name`. Merge to `main` small and often, each merge green on `make lint test`, Biome included, no warnings. Attribution rules per repo owner.
2. **Migrations.** Every new migration takes next number in track range `20261201TTNNNN` (TT = track). Old numbers still printed in specs (`20261101000101`, `001801`) are labels of unbuilt work: renumber into own range, keep plain name. Never reuse, never edit merged migration. Rule of independence: wave 1 migration depends only on what is on `main` plus own track; dependency on another wave 1 track is written below and waits for that merge. Additive only (expand, migrate, contract).
3. **Prerequisite commit H0, before first worktree opens.** `framework/database/migrate/session.go` adds `goose.WithAllowOutofOrder(true)` to provider options, with runner test. Why: goose refuses pending version older than latest applied, and parallel merges land out of number order; a developer database migrated past track 05 would refuse track 02's next merge. Without allow-out-of-order, every shared database needs a rebuild per merge. Coordinator makes H0 on `main`. Order safety stays with rule 2.
4. **Own your files.** Touch only owned paths, plus shared files below additively (append a line, a route call, a catalog row). Never reformat a shared file. Prefer one new file per action, one route file per area, one query file per table, one feature folder per screen.
5. **Generated files never hand-merged.** `app/models/*.sql.go` per `make sqlc`; `resources/js/route-tree.gen.ts` by Vite plugin; `pnpm-lock.yaml`, `go.sum` by tool. On conflict: take main's, rerun generator.
6. **Dependencies.** New Go module: only those approved in specs, added to depguard `framework` allowlist in `.golangci.yml` as sole change there, never to quiet lint: `codeberg.org/go-pdf/fpdf` (track 04), `github.com/emersion/go-imap/v2` (06), `github.com/russellhaering/gosaml2`, `github.com/russellhaering/goxmldsig` at 1.6.1 or later, `scim2/filter-parser/v2` (08). Browser libs per owner spec, exact pins. Licenses per CLAUDE.md.
7. **Tests.** `make lint test` green. Business logic: integration tests on real Postgres, Valkey, S3, Mailpit. Each new page or flow: Playwright in `tests/e2e`, light and dark. Coverage gates 90% framework, 95% app. New endpoints: extend `docs/testing/manual-e2e.md` flow table. Fakes only at process boundaries. **Never call real LLMs, real mail providers or real IdPs from tests.** Real-model and real-mailbox runs need owner approval per run.
8. **Foundation is shared truth.** New event, prefix, table: already recorded in `00-foundation`; if a build forces a change, edit owning spec and foundation in same merge.
9. **Public repo.** No secrets, account ids, or exploit details of unfixed bugs in code comments, specs, tests or commit messages. Code never mentions specs, gaps, or tickets.
10. **Worktrees lack untracked files.** `research/competitors/intercom-gaps-in-built-features.md` and `edge-case-audit-chatwoot-zammad.md` may be untracked. Specs carry what builder needs. `F14` style names in specs mean "assumes this bug fix landed": check code on `main`; if behaviour missing, wait or ask, never redo fix inside track.

## Shared files (additive only, all tracks)

| File | Rule |
|---|---|
| `app/auth/permissions.go`, `app/auth/roles.go` | append catalog rows; 08 alone changes role resolution |
| `routes/routes.go` | one line calling new route file. `routes/api.go`, `routes/inbox.go`, `routes/web.go`, `routes/webhooks.go`: append group or route only; prefer new file |
| `app/providers/*.go` | one registration line (listener, eraser, driver, initializer) |
| `app/events/`, `app/listeners/`, `app/jobs/`, `app/enums/`, `app/mcp/tools/` | new files; registry append |
| `app/models/public_ids.go` | append prefix from foundation registry |
| `config/*.go`, `.env.testing` | new keys in own block; new file when area is new (`config/inbox.go`) |
| `lang/en/*.json` | new namespace file per feature; 07 owns locale machinery, not other namespaces |
| `resources/js/features/shell/navigation.ts`, `resources/js/features/settings/*` nav lists | append item. 03 owns inbox section of navigation; 07 owns settings nav shape |
| `resources/js/routes/d/**` | add route file for own page; replace placeholder only if listed as owned |
| `Makefile` | new target only |
| `tests/arch/*` | new test file only |
| `package.json`, `go.mod` | add own dep; do not bump others |
| `specs/*` | own spec only; fix spec in same merge when reality differs |

## Bug-fix agent on main

Another agent is landing 96 fixes (F01..F96) in `app/actions/email`, `app/integrations/email`, `framework/mailbox`, `app/actions/{conversations,messages,contacts,auth,tokens,members}`, `app/broadcasting`, `framework/broadcasting`, `resources/js/features/inbox`, `resources/js/features/settings`.

| Track | Relation |
|---|---|
| 02, 03, 04, 05, 06, 07, 08 | **merge `main` daily**, before each new milestone, and resolve toward main |
| 01, 13 (mostly new code), 09, 10, 11, 12 | merge `main` at each milestone start |

**Must wait** until named fix is on `main` (check code, not commit message). Rest of track proceeds:

| Track | Milestone | Waits for |
|---|---|---|
| 06 | 04 M1 to M4 (parse, auth, routing, ingest finishing) | F01 to F08, F19 to F24, F40 to F46, F50, F53 to F55, F58, F59, F66, F72 to F80 |
| 06 | 04 M5 | F22, F48 |
| 06 | 04 M9, M16 | F22, F23 |
| 05 | 04 M6 | F15, F16, F17, F28, F56 |
| 05 | 04 M7 | F35, F48, F50, F51, F52 |
| 05 | 04 M8 | F66 |
| 05 | 04 M10 | F62, F84 |
| 05 | 04 M15 | F18, F23, F51 |
| 03 | W5 snooze | F31 |
| 03 | W6, W8 composer, inline images | F14, F15, F38 |
| 03 | W12 search | F29, F30, F57 |
| 04 | R6 participants | F16 |
| 04 | R5 merges | F36 |
| 04 | R7 spam | F37, F61 |
| 04 | R8 attachment safety | F14, F47. Must land with F14 download route |
| 07 | M13 password | F11 |
| 07 | M16 audit | F71 |
| 01 | M12 live features, attachments | F14, F88 |

Tracks free to start at once, no waiting: 01 M1 to M11; 02 A1 to A4; 03 W1 to W4, W7, W9 to W11; 04 R1 to R4, R9 to R11; 05 M14 parts past M8, M20; 06 M17 to M19 driver code in new dirs, M16 UI; 07 M14, M15, M17, M18, M20; 08 all.

## Wave 1 (start now)

### Track 01 widget-platform

- **Goal:** real messenger for anonymous visitors end to end, then every setting.
- **Build:** [01 tasks](01-messenger-widget/tasks.md) M1 to M6 in that order, owner binding: backend API + default widget; sessions + messaging + SSE; connect static widget; settings page + live preview; install snippets + installed status; anti-abuse + offline email. Then M8 to M17. Not M7. Specs: 01 README, api, sdk-contract, edge-cases, settings, settings-page, client, hosted-and-domains.
- **Depends:** nothing to start. Local fake for `SendWidgetOfflineReply` until 05 M10. M13 inbox section needs 04 R2; ships behind panel mount when it lands.
- **Owns:** `app/actions/widget*`, `app/http/controllers/widget`, `app/integrations/widget`, `routes/widget*.go`, `database/queries/widget_*.sql`, `resources/widget/**`, `resources/js/features/settings/pages/channels.tsx`, `resources/js/features/widget-settings/**`, `sdk/packages/{browser,react,nextjs,server}`, `tests/feature/widget`, `tests/e2e/widget*.spec.ts`, `tests/contract/widget_v1*`.
- **Touches additively:** shared table; `app/broadcasting` widget payload file; `config/widget.go`.
- **Migrations:** `20261201010001`..`20261201019999`.
- **Tests:** loader 3 KB, core 35 KB gzip budgets in CI; contract snapshot + oasdiff from M3; Playwright widget + settings; SSE and polling fallback on real centrifuge + Valkey.

### Track 02 inbox-access

- **Goal:** one visibility rule everywhere, fair assignment, away status. Everything else builds on A1, so A1 lands first and fast.
- **Build:** [03 tasks](03-shared-inbox/tasks.md) A1, A2, A3, A4. Specs: 03 access.md, assignment.md, README gap map.
- **Depends:** nothing. Others wait on A1: 03 and 04 for new list queries, 07 member dialog, 11 fan-out, 13.
- **Owns:** `app/actions/conversations/visible_scope*.go`, `conversation_fanout.go`, `app/actions/assignment/**`, `app/actions/availability/**`, `app/broadcasting/authorizers.go` conversation rules, `resources/js/features/settings/pages/inbox.tsx`, `database/queries/{member_access_teams,away_reasons,team_assignment_cursors}.sql`, `tests/arch/visible_scope_test.go`, `tests/feature/access`, `tests/feature/assignment`, `tests/e2e/inbox-access.spec.ts`.
- **Touches additively:** `app/actions/conversations/filters.go` and existing queries only to route through `Scope` (A1 is the one wide edit: small, then done); `routes/inbox.go`.
- **Migrations:** `20261201020001`..`20261201029999`.
- **Tests:** arch test fails planted unscoped query; `Scope.Contains` equals SQL function across modes; real centrifuge + Valkey for channel denial; list budgets at 100k rows.

### Track 03 inbox-workbench

- **Goal:** inbox that feels fast and complete on screen.
- **Build:** 03 tasks W1 to W12; remaining parts of M1, M3 to M7, M10 to M12 (partly built: check code first). Specs: 03 views.md, workbench.md, collaboration.md "Mentions", "Reactions", search.md, api.md.
- **Depends:** 02 A1 for new list queries (until then use existing `VisibleScope{}` entry point, no direct SQL). 05 mounts `sender-select`, `sender-flag`, `throttle-banner` from `resources/js/features/email`: 03 leaves named slots.
- **Owns:** `resources/js/features/inbox/**` except `customer/`, `resources/js/features/command-menu/**`, `app/actions/messages/**`, `app/actions/conversations/**` not owned by 02 or 04, `app/actions/{conversation_views,drafts,bulk_operations,reactions}/**`, `routes/inbox_views.go`, `database/queries/{conversation_views,conversation_drafts,bulk_operations,snooze_lists,message_reactions,member_inbox_preferences}.sql`, `app/support/searchquery`, `tests/feature/conversations`, `tests/e2e/inbox*.spec.ts`.
- **Touches additively:** `resources/js/features/shell/navigation.ts`, 01 widget payload never.
- **Migrations:** `20261201030001`..`20261201039999`.
- **Tests:** `EXPLAIN` asserted indexes on sort columns; J/K without spinner e2e; undo send with fake clock; bulk job 1,000 ids and 10,000 filter matches; mention parse + `message_mentions`.

### Track 04 customer-records

- **Goal:** know the customer, keep data safe, answer privacy requests.
- **Build:** 03 tasks R1 to R11; M2, M8, M9, M9b remainder. Specs: 03 records.md, safety.md, api.md.
- **Depends:** 02 A1 for new list queries (panel, directory, exports). `contact_tag`, `company_tag` built here; 13 consumes.
- **Owns:** `app/actions/{contacts,companies,tags,merges,participants,erasures,data_exports,redaction,retention}/**`, `framework/malware` callers `app/jobs/scan_attachment*`, `resources/js/features/customers/**`, `resources/js/features/inbox/customer/**`, `resources/js/routes/d/customers*`, `resources/js/features/settings/pages/content.tsx` tags section, `database/queries/{tags,contact_tag,company_tag,companies,contact_erasures,data_exports,redaction_rules}.sql`, `tests/feature/{contacts,companies,tags,privacy}`, `tests/e2e/customers*.spec.ts`.
- **Touches additively:** providers (eraser registry), `.golangci.yml` for fpdf, `app/actions/conversations` merge entry file only.
- **Migrations:** `20261201040001`..`20261201049999`.
- **Tests:** clamd real where compose gives it, scanner fake only for malware boundary per testing.md; erase canary across all eraser hooks, attachments, raw mail, search; merge chain; redaction before storage with timeout per rule.

### Track 05 email-sending

- **Goal:** customers hear back from their own address, safely, with limits.
- **Build:** [04 tasks](04-email-channel/tasks.md) M6, M7, M8, M10, M11, M14, M15, M20. Specs: 04 outbound.md, sending.md, sender-rules.md, staging.md, README, 12 "Email sending limits".
- **Depends:** M20 round trip needs 06 M16 test table; everything else in M20 goes first. 03 slots for composer mounts.
- **Owns:** `app/actions/email/{send,compose,domains,suppressions,signatures,sender_rules,limits}*`, `app/integrations/email/{composer,outbound}*`, `framework/mail/**`, `framework/customdomain` use only, `resources/js/features/email/{sender-select,sender-flag,suppression-chip,throttle-banner,template-settings}*`, `resources/js/features/settings/pages/email.tsx` sending sections, `routes/email_senders.go`, `database/queries/{email_domains,email_suppressions,email_signatures,email_ignored_addresses,email_trusted_senders,outbound_emails,email_sending_*}.sql`, `deploy/cdk/**`, `deploy/domainconnect/**`, `tests/emailmatrix`, `docs/ops/ses-inbound.md`.
- **Touches additively:** `routes/webhooks.go` events routes, `config/channels.go` own block, `Makefile` targets `infra-check`, `email-matrix`.
- **Migrations:** `20261201050001`..`20261201059999`.
- **Tests:** `tc.Mail`, `a.DNS.Fake()`, `httpclienttest`; `make infra-check` with no credentials; matrix harness compiles, never runs in `make test`. No real AWS needed for any test.

### Track 06 email-inbound

- **Goal:** mail reaches dimmi from anywhere, and setup explains itself.
- **Build:** 04 tasks M1 to M5, M9, M12, M13, M16, M17, M18, M19. Specs: 04 pipeline.md, setup.md, sources.md, mailbox-connections.md, edge-cases, 11 operations.md section 15.
- **Depends:** domain instruction mail in M16 needs 05 M8. 03 `RecordExternalOutboundMessage`, `InboundMessage.SourceMerge`, unspam action: 06 adds them additively in own files, agreed shape in 04 mailbox-connections.md.
- **Owns:** `framework/mailbox/**`, `app/actions/email/{receive,ingest,setup,address_tests,mailbox_connections}*`, `app/integrations/email/{inbound,auth,inspect}*`, `resources/js/features/email/setup/**`, `resources/js/features/settings/pages/email.tsx` setup sections, `routes/email_mailboxes.go`, `database/queries/{inbound_emails,email_addresses,email_address_tests,email_mailbox_*}.sql`, `tests/feature/email`, `tests/e2e/email-settings.spec.ts`.
- **Touches additively:** `routes/webhooks.go`, `routes/web.go` callbacks, `.golangci.yml` for go-imap, `config/channels.go` own block.
- **Migrations:** `20261201060001`..`20261201069999`.
- **Tests:** fixture bodies through real receivers; `drivertest` contract per source; Google and Microsoft through `httpclienttest`, never real accounts. Google verification and CASA are human steps, tests never depend on them.

05 and 06 both edit `settings/pages/email.tsx` and `config/channels.go`: 06 splits page into section files in its first merge; 05 merges that before editing.

### Track 07 accounts-settings

- **Goal:** every personal and member setting works or is gone.
- **Build:** [00-accounts tasks](00-accounts/tasks.md) M10, M13 to M18, M20; whatever is still open in M1 to M9, M11, M12 (mostly built: check code first). M11b goes with 09-agent-native. Specs: settings-pages.md, audit.md, rate-limits.md, i18n.md, README, foundation api.md "Rate limits".
- **Depends:** M17 access dialog needs 02 A1. M17 `ReassignTeamConversations`: build here if 02 has not. M15 Available toggle calls 02 A2, hidden until merged. Signature section links 05 M14.
- **Owns:** `resources/js/features/settings/pages/{account,workspace,team,teams,audit-log,admin}.tsx`, `app/actions/{users,members,invitations,teams,audit}/**`, `app/http/middleware` throttle wiring, `framework/http/middleware/throttle.go`, `app/support/entitlements/**`, `lang/` machinery, `resources/js/lib/i18n*`, `routes/account.go`, `tests/feature/{accounts,members,ratelimit}`.
- **Touches additively:** `app/auth/access_policy.go` never (08's); `routes/api.go` throttle middleware line on API group.
- **Migrations:** `20261201070001`..`20261201079999`.
- **Tests:** real Valkey rate windows + `X-RateLimit-*`; CSV round trip; email-change confirmation; `lang:check`; no LLM in translation tests, machine first pass needs owner approval per run.

### Track 08 enterprise-identity

- **Goal:** companies can require how people sign in and manage them from their IdP.
- **Build:** [21 tasks](21-enterprise-identity/tasks.md) M1 to M8; [00-accounts tasks](00-accounts/tasks.md) M19. Specs: 21 README, access-policy.md, saml.md, scim.md, 00-accounts roles.md, foundation security.md "Sign-in rules".
- **Depends:** nothing to start. 21 M4 first proves `tests/arch` still passes with SAML library (it imports `html/template`); if it fails, stop and report. Finer permissions at call sites follow 03, 04 as routes land.
- **Owns:** `app/auth/{access_policy,login_policy}*.go`, `app/auth/rules/**`, `framework/saml/**`, `framework/scim/**`, `app/actions/{sso,scim,roles,organization_domains}/**`, `routes/{sso,scim,roles}.go`, `resources/js/features/settings/pages/security-policy*`, `resources/js/features/settings/pages/roles*`, `database/queries/{sso_connections,organization_domains,scim_*,roles}.sql`, `tests/feature/{sso,scim,roles,accesspolicy}`.
- **Touches additively:** `app/auth/roles.go` role resolution, `app/actions/auth` sign-in only through policy hooks, `.golangci.yml` for SAML + SCIM parser.
- **Migrations:** `20261201080001`..`20261201089999`.
- **Tests:** IdP fake at HTTP boundary, signed fixtures; attack table per saml.md; real Valkey + Postgres. Live Okta, Entra, Google check is human step.

## Wave 2

### Track 09 widget-identity

- **Goal:** signed-in customers, one contact across widgets, verified, with full history.
- **Build:** [01 tasks](01-messenger-widget/tasks.md) M7; [02 tasks](02-widget-identity/tasks.md) M1 to M6. Specs: 02 README, validations.md, snippets.md, edge-cases.
- **Depends:** 01 M6. Sign-out button needs 04 R2.
- **Owns:** `app/actions/identity/**`, `routes/identity.go`, `database/queries/{identity_secrets,identity_verification_stats}.sql`, `sdk/packages/{sveltekit,nuxt,astro,vue,svelte}`, `sdk/{laravel,rails,python,go}`, `resources/js/features/settings/pages/identity*`, `tests/feature/identity`.
- **Migrations:** `20261201090001`..`20261201099999`.
- **Tests:** JWT table incl. RS256, EdDSA, nonce replay on real Valkey; shared-device logout; merge on login.

### Track 10 native-sdks

- **Goal:** mobile app is another widget.
- **Build:** [20 tasks](20-native-sdks/tasks.md) M1 to M6. Specs: 20 README, api-mapping.md, edge-cases.
- **Depends:** 01 M3 merged and contract frozen (01 tasks "Freeze"). Audiences: native evaluates nothing, a tab with residual stays hidden; no wait on 13.
- **Owns:** `sdk/packages/{headless,react-native}`, `sdk/ios`, `sdk/android`, `examples/{expo,ios,android}`, `framework/push/**`, `app/actions/widget_devices/**`, `tests/feature/widget_devices`.
- **Touches additively:** `resources/widget/client/` no; 01 owns it, 10 consumes as source through build.
- **Migrations:** `20261201100001`..`20261201109999`.
- **Tests:** contract fixtures from `tests/contract/widget_v1_fixtures` per SDK; `framework/push` drivertest + `Fake()`; simulators in CI where runners allow; publishing accounts are human steps.

### Track 11 notifications

- **Goal:** right person told at right time.
- **Build:** [06 tasks](06-notifications/tasks.md) M1 to M10. Specs: 06 README, delivery.md, edge-cases.
- **Depends:** 02 A1 (fan-out honours access), 03 W9 mentions write side; 04 connection-status notice after 06 M19 in 04; 07 fires `team.member_added` payload.
- **Owns:** `app/actions/notifications/**`, `app/listeners/notify_*`, `resources/js/features/notifications/**`, `resources/js/routes/d/notifications.tsx`, `resources/js/features/settings/pages/notifications*`, `database/queries/notification_*.sql`, `tests/feature/notifications`, `tests/e2e/notifications.spec.ts`.
- **Migrations:** `20261201110001`..`20261201119999`.
- **Tests:** suppression pipeline on real Postgres + Valkey; Slack through `httpclienttest`; Web Push RFC vectors in framework.

### Track 13 audiences

- **Goal:** one way to say who. Core before 15 and 17 start, per owner.
- **Build:** [18](18-audiences/README.md) "Tasks": **M1 to M4 core** (language, attribute registry, contact columns, saved audiences, preview, test-as, builder, Data page); M5 widget surfaces: server after 01 M2, client evaluator after 01 M3; M6 events right after M5; M7 after 08 help center; M8 last.
- **Depends:** inbox tracks first: 02 A1, 03 W1, W2, 04 R1 (tags), R2 (panel). 01 M8 ships `resources/widget/lib/rules.ts`; M5 extends it. 09 for help center token in M7.
- **Owns:** `app/support/audience/**`, `app/actions/{audiences,attributes,events}/**`, `routes/audiences.go`, `app/mcp/tools/audiences.go`, `database/queries/{audiences,attributes,events}.sql`, `resources/js/features/audiences/**`, `tests/feature/audiences`, `tests/e2e/audiences.spec.ts`.
- **Touches additively:** `resources/js/features/automation/inspector.tsx` presets, 01 config `surface` block file, 03 contact resource file.
- **Migrations:** `20261201130001`..`20261201139999`. Printed `0018NN` are labels.
- **Tests:** Go evaluator equals SQL compiler on 10,000 random contacts per field and op, real Postgres; canary that widget response leaks no names; Go and vitest on same JSON corpus; LLM fake only.

## Wave 3

### Track 12 inbox-extras

- **Goal:** heavy inbox features that block nothing else. Still launch.
- **Build:** 03 tasks X1 to X6. Specs: 03 assignment.md "Away reasons", views.md "Folders", "Table layout", collaboration.md "Side conversations", "Similar", "Translation", records.md "Conversation attributes", safety.md "Scheduled export".
- **Depends:** 03 W2, W4, 04 R2, R11; 06 inbound thread lookup for side replies (`email_message_refs.side_conversation_id`); 07 LLM layer for translation; fake provider in tests.
- **Owns:** `app/actions/{side_conversations,translations,export_schedules,conversation_attribute_definitions}/**`, `resources/js/features/inbox/{table,folders,side}/**`.
- **Migrations:** `20261201120001`..`20261201129999`.
- **Tests:** per X milestones; PDF and S3 export against SeaweedFS; never a real model.

### Tracks 14 engage, 15 workflows (ranges reserved)

- 14: `20261201140001`..`20261201149999`: `15-announcements-surveys` tasks; starts after 13 M4; builds no evaluator.
- 15: `20261201150001`..`20261201159999`: `17-workflows` tasks; starts after 13 M4 and 02 A3; calls 02 picker, builds no conditions package.

### Cloud staging

[19-cloud-staging](19-cloud-staging/README.md) is draft awaiting owner, runs own tracks A to D, no range needed unless it adds migration (then `20261201190001` up). Its C1, C2 are 04 M5, M7 here (tracks 06, 05); its B5 uses `Mail` stack from 05 M20. Do not build twice.

## Merge conflict hot spots

| Spot | Who | Plan |
|---|---|---|
| `app/actions/conversations/filters.go`, list queries | 02, 03, 04 | 02 A1 first; others rebase then |
| `resources/js/features/inbox/details-panel.tsx` | 03, 04 | 04 moves panel into `customer/` in first merge, 03 keeps hands off |
| `resources/js/features/inbox/composer.tsx` | 03, 05 | 03 owns; exposes named slots; 05 mounts |
| `settings/pages/email.tsx` | 05, 06 | 06 splits first |
| `settings/pages/account.tsx` | 07, 05 signature | 07 splits into sections first |
| `app/auth/permissions.go` | all | append only, sort inside own block |
| `routes/webhooks.go` | 05, 06 | one group per provider file |
| `app/http/resources/member_resource.go` | 02, 07, 08 | fields append; resource test per field |

## Done for whole plan

`make lint test` clean on `main`; `make e2e` green; every gap map row in 00-accounts, 01, 02, 03, 04, 06, 21 ticked with its milestone merged; `docs/testing/manual-e2e.md` flow table run against release binary; owner's human steps listed in 04 staging.md section 8, 21 tasks, 20 tasks.

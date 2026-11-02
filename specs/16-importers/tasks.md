# 16 Importers: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Tests per `00-conventions/testing.md`: integration only for app logic (real Postgres `pgvector/pgvector:pg18`, real App via `testkit.New(t)`, real dispatcher, River drained with `tc.Queue.Drain(ctx)`, real RLS, real Valkey), no mocks of DB/actions/events. Fakes only at process boundaries: source APIs served by `httptest` upstream replaying recorded fixtures (`tests/fixtures/imports/<source>/`), `httpclienttest.New(tc.App.HTTP)` for attachment hosts, `tc.Mail`, `tc.Clock`, 08 fake embeddings. Never real source API or LLM in CI. Every edge case id in a test name (`dimmi spec:coverage`). Helper `tc.AssertNoSideEffects(t, fn)`: asserts zero `message.created`/`conversation.created`/`contact.*` dispatches, zero River jobs on `channels`, `notifications`, `ai`, `webhooks` (except `import.completed` fan-out), zero `tc.Mail` customer stream, zero ledger lines.

Prereqs: 00-foundation (KeyManager, Filesystem, River, ratelimit, entitlements), 00-accounts (permissions), 03 core + import write helpers, 08 articles/redirects/assets/crawl, 09 MCP registry + CLI client mode, 13/14 import helpers for those scopes.

## M1. Data core

- [ ] Migrations 001601-001609, sqlc queries, enums (status machines strict, `source` registry), prefixes `imp_ impr_ impm_ impf_` in `app/models/public_ids.go`, typed jsonb (`ImportOptions`, `ImportSourceAccount`).
  - AC: migrate up/down/up clean; RLS on all 8 tables; query lint (workspace_id) passes; unique live import per `connection_key`; one active run per import.
  - Tests: id round trip; partial unique indexes reject second active run (E-C cases); cross-tenant 404 scaffold.
- [ ] Permissions `imports:read|manage|rollback` (00-accounts catalog, roles), entitlement catalog keys (4), config `config/imports.go`, audit action names.
  - Tests: member 403; token ability intersect; `/me` permissions; entitlement defaults per edition.
- [ ] `contracts.ImportSource` + normalized types + `drivertest` suite skeleton + `fake` driver (synthetic tenant generator with fault injection: 429, 5xx, 401, truncated parts, expiring URL, duplicate pages).
  - AC: fake driver passes suite; deterministic seed.

## M2. Framework: imports, preview, mappings

- [ ] `POST/GET/PATCH/DELETE /imports` + `CreateImport`, `ConnectImportSource` (sync 10 s), credentials sealing (aad), resource redaction (`credentials.configured/hint`), archive (shred), delete preconditions, `GET /import-sources` catalog incl. `external` entries.
  - Tests: E1-E7 with fake driver; secrets never in response/logs (log capture grep); 409 `import_exists`; `password.confirm` on DELETE; audit rows.
- [ ] Runs API: `POST /imports/{imp}/runs` kind dispatch + preconditions, `PATCH /import-runs/{impr}` status machine, `GET` runs/errors (`?format=csv`)/records.
  - Tests: every precondition code; `invalid_transition`; idempotency key replay; cross-tenant.
- [ ] `PreviewImport` job + `DiscoverImport` (mappings upsert + suggestions: teammate by email, tags by name, attributes by key/type, statuses by category) + preview payload (counts, samples, estimate, warnings), `ready` transition, staleness on option change.
  - Tests: E9-E16, E18; preview writes zero business rows (table count snapshot).
- [ ] Mapping endpoints single + bulk (rest.md rule 8 response), allowed actions per kind, conflicts block full run (`mapping_incomplete`).
  - Tests: per-kind validation table; bulk partial failure; E17 future-records rule.

## M3. Engine

- [ ] `RunImportPage` engine: fetch outside tx, short tx with tenant + advisory lock, writer call, `import_records` upsert, stage counters + cursor, heartbeat, `InsertTx` next page / next stage; cancel/pause checks; stage ordering; independent stage failure.
  - Tests: E19, E20, E22, E23, E25, E26, E28-E30, E32-E34; property test: kill worker at 50 random points (fake driver), final counts equal clean run (bar: 0 duplicates).
- [ ] `importkit/ratebudget`: Valkey token bucket per (source, account), header parse (Intercom, Zendesk, Help Scout, Productlane, Canny names), `Retry-After`, long wait -> `paused rate_limited` + `resume_at`; `ResumePausedImportRuns`, `RequeueStaleImportRuns`.
  - Tests: E21 (restart), E24 (expired cursor), E27 (shared budget two workspaces), E31 (tiny plan), budget never exceeded across 4 workers (counter assertions).
- [ ] `FinalizeImportRun`: aggregates recompute (contact/company/conversation counters touched), totals, watermark, `import.completed`/`failed`/`cancelled` events, `SendImportFinishedMail` (system stream), coalesced `import.progressed` (2 s) on `workspace:` + `member:`.
  - Tests: realtime publications captured (count bounded by coalescing); mail to starter only; webhook fan-out only for `import.completed` (10 catalog fixture).
- [ ] Entitlements enforcement: run start 422 (count cap) / 429 + `Retry-After` (monthly cap), mid-run pause with `resume_at` period reset, override bust resumes.
  - Tests: E138 (records monthly), E89 (attachment bytes pause keeps text going).

## M4. Writers (03 helpers + suppression)

- [ ] With 03: `ImportTags`, `ImportCompanies`, `ImportContacts` (match rules, identities unverified, attestation path, attribute normalization via `importkit/attrs`), writers `tags/companies/contacts/teams`.
  - Tests: E35-E41, E55-E63, E68, `AssertNoSideEffects`; arch test writers import no `app/events`.
- [ ] With 03: `ImportConversations` + `ImportMessages` (`tx.CopyFrom` into temp staging table + `INSERT ... SELECT` with ON CONFLICT dedupe; COPY cannot load RLS tenant tables for the app role, number block allocation, preserved timestamps, counters, `waiting_since` null, `ai_state off`, author tuple via `ResolveImportAuthor`, `source.imported`, `metadata.imported`), event rows cap 200, body split 64 KB, participants/tags caps.
  - Tests: E42-E49, E69-E83; throughput bench ≥ 2,000 messages/s; unread rule (03) shows zero unread; search finds imported text; `AssertNoSideEffects`.
- [ ] Attachments lane: `DownloadImportAttachment` (stream to Filesystem, sniff, block types, scan pending, size cap placeholder, sha256 dedupe, URL refresh once), inline images.
  - Tests: E84-E90; SSRF hop blocked; Zendesk auth host rule.
- [ ] `import` channel driver + 03 `ChannelRehomer` hook: reply rehomes to inbox email connection; 422 `import_channel_cannot_send`.
  - Tests: E53 (delta inbound not billable), E53 threading after rehome via 04 fake mail, E54 no email.
- [ ] Trusted attribute keys (07 `contacts.trusted_attribute_keys`): `trusted` flag on attribute mappings (attribute kind only), writer adds key for `kind user` contacts where import wrote value, re-apply on later runs by checksum, audit `import.attribute_trusted`.
  - Tests: E64-E67; default import leaves `trusted_attribute_keys` empty; 07 Stripe action ineligible on imported untrusted key, eligible after mapping trusted + re-run.
- [ ] Saved replies writer (Zendesk macros, Help Scout saved replies) with placeholder conversion.
  - Tests: placeholder map table; unknown placeholders kept as text.
- [ ] Billing guard verification with 12: replay fixture `import_never_billed`; live-after-import fixture 25.
  - Tests: E50, E51 on cloud edition harness.

## M5. Help center

- [ ] With 08: `ImportCollection`, `ImportArticle` (publish preserve via publish path without events, timestamps preserved), `ImportAsset`, `ImportRedirect`; `importkit/htmlmd` (callouts to alerts, tables, iframes to links), `importkit/paths` (id-only variants), slug conflicts.
  - Tests: E91-E104; XSS corpus; redirect resolves 301 on custom domain host fixture; `article.*` events absent; knowledge chunks exist after publish.
- [ ] `article_links` stage (cross-link rewrite) and `redirects` stage ordering.
  - Tests: links rewritten; non-imported links untouched.

## M6. Source drivers (each: client, mapping, recorded fixtures, drivertest pass)

- [ ] `intercom`: regions, version header, admins/teams/tags/attributes discovery, companies, contacts search, conversations search + detail batches (500-part truncation event), tickets, articles/collections (locales).
  - Tests: drivertest; E2 region hint; E69 truncation; end-to-end fixture tenant (200 contacts, 150 conversations, 20 articles) imports with expected counts.
- [ ] `zendesk`: plan detection via headers, incremental tickets cursor, ticket_events comment_events, comments gap fill, users/orgs, macros, Guide categories/sections/articles, private attachments.
  - Tests: drivertest; incremental `end_of_stream`; 1-minute start_time rule; Essential plan ETA.
- [ ] `helpscout`: client credentials (2-day token, refresh on 401), mailboxes, customers, conversations + threads (types table), attachments base64 stream, saved replies, Docs API (sites, collections, categories, articles).
  - Tests: drivertest; E6 refresh; thread-type mapping table.
- [ ] `featurebase`: version header, cursor lists, boards/statuses/posts/voters/comments, changelogs, help center, contacts/companies, conversations.
  - Tests: drivertest; 14/13 helpers called; vote math E105.
- [ ] `canny`: POST + apiKey, v1 skip / v2 cursor, hourly budget in ETA, posts/votes/comments/statuses/entries.
  - Tests: drivertest; hourly cap pauses with `resume_at` next hour; merged posts skipped.
- [ ] `productlane`: v2 cursor/has_more, newest-first handling, threads, contacts, companies, changelogs, docs, portal issues/projects -> posts.
  - Tests: drivertest; delta checksum skip.
- [ ] `public_site`: platform detection (saved HTML fixtures per platform), sitemap enumerate via 08 crawl, extract per platform + readability fallback, breadcrumbs to collections, images, redirects per URL, page cap entitlement.
  - Tests: one fixture site per detector; JS-only skip; robots disallow respected; SSRF.
- [ ] `csv`: files endpoint (encoding, delimiter, BOM, sample), column mapping validation at preview, row keys.
  - Tests: encodings; 500k-row cap; idempotent re-run.

## M7. Feedback and changelog

- [ ] With 14: `UpsertImportedPost/Vote/Comment`; `feedback` stage (boards/status/category mappings applied, votes + `imported_vote_count`, comments flattened, merged skipped, caps).
  - Tests: E105-E112; no `feedback_*` events; no voter mail.
- [ ] With 13: `ImportEntry`; `changelog` stage (backdated, labels, cover asset, scheduled -> draft).
  - Tests: no mailing rows, no widget badge, no events.

## M8. Delta, rollback, GDPR

- [ ] Delta runs: windows from watermark with overlap, per-driver `updated_since`, append parts, untouched-check rules, `ScheduleImportDeltas`, `sync_until`, credentials purge interplay.
  - Tests: E113-E120; E8 purge extension.
- [ ] Rollback runs: reverse order deletes by `import_records` (created only), kept rules (modified after import), partial scope, RetentionGuard, resumable, files purge via 03.
  - Tests: E121-E128; 100k conversations rollback under 10 min (fake driver bench).
- [ ] Eraser `EraseImportContactData` registered in contact eraser registry; `import_erasures` tombstones (encryption MAC); delta skip.
  - Tests: E129, E130 (GDPR) incl. grep test no PII left in import tables after erase.
- [ ] Prune/purge jobs (`PurgeImportCredentials`, `PruneImportErrors`).
  - Tests: TTL behaviors with frozen clock.

## M9. Agent-native and docs

- [ ] MCP toolset `imports` (6 tools, risks, confirm gate for rollback and publish-preserve run), no credentials in tool I/O, `credentials_url`.
  - Tests: toolset token budget under 2,000; confirm required; secret never echoed (E135); E131-E134, E136, E137.
- [ ] CLI `dimmi migrate <source>` wizard (hidden secret prompt, env var, preview table, conflicts, `--map` YAML, `--yes`, progress via realtime, exit codes, next-steps checklist) + generated `dimmi imports ...`.
  - Tests: CLI integration against test server with fake source; secret absent from argv/history files; exit code 2 on errors.
- [ ] Docs: per-source guide (token creation screenshots list, scopes needed, rate share), switchover playbook (full, delta, widget/shim, email forwarding, DNS for help center redirects, archive), billing note (imports free, switchover window note E52).
- [ ] Observability: metrics, logs redaction tests (no tokens, emails, bodies at info), dashboard JSON for Grafana in `docs/ops`.

## Done when (demo script, under 15 minutes)

1. Fixture Intercom tenant (recorded fixtures served by local upstream, region eu): `npx dimmi migrate intercom --region eu --token-env INTERCOM_TOKEN`.
2. Wizard shows preview: 1,200 contacts (900 anonymous leads skipped), 300 conversations (1 over 500 parts), 40 articles in 2 locales, 2 teammates placeholder, 1 attribute conflict; fix conflict interactively (rename `plan` -> `intercom_plan`); choose help center `hc_...`, publish preserve; confirm.
3. Progress bars per stage stream; dashboard `workspace:` channel shows banner; run completes under 3 minutes; mail "Intercom import finished" in Mailpit (system stream) to starter only.
4. Inbox: conversation `imported:21534` shows original dates, author "Sarah Lee (former teammate)", note, attachments render, truncation event on long one; unread count 0; no notification, no AI run, no webhook except `import.completed` (webhook inspector), billing ledger empty (`dimmi billing:simulate` shows 0).
5. Help center: `curl -I https://help.acme.test/en/articles/8812-reset-your-password` returns 301 to `/reset-your-password`; images served from Dimmi assets; cross-links rewritten.
6. Enable delta (`--sync 10m --sync-until 7d`); add message to conversation in fixture upstream; within one interval it appears; teammate replies in Dimmi: conversation rehomes to email address, Mailpit shows outbound on customer stream.
7. Run `dimmi migrate intercom` again: 0 created, all skipped (idempotent).
8. Erase contact Bob (03): `import_records` gone, delta later skips Bob (`erased_contact_skipped`).
9. `dimmi imports rollback imp_... --scope articles` with typed confirmation + sudo: articles, collections, redirects gone; conversations stay. Full rollback: everything created gone except conversation with live reply (kept, reported).
10. `public_site` via MCP from Claude Code: `dimmi_start_import {source: "public_site", url: "https://docs.fixture.test"}` -> preview -> `dimmi_run_import` asks confirmation "40 articles will go live" -> done with redirects.
11. Canny fixture: posts with votes and comments land on 14 board with correct counts; Featurebase changelog entries backdated in 13, no emails sent.

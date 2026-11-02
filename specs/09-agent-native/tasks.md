# 09 Agent-native: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Depends on foundation skeleton, 00-accounts (users, sessions, members, PAT, `/me`), 03 actions for tools. Final sweep 2026-11-02: framework packages (`framework/oauth`, `framework/auth/token`, `framework/mcp` on go-sdk v1.8.0 with `oauthex`/`auth`, `framework/httpclient`, `framework/webhooks`), Valkey for caches/pubsub/rate limits, SDK-driven `init`, 13-15 toolsets + CLI, grant confirmation. Scope trim 2026-11-02: drafts milestone (B9), send modes (B8), `listen` client (C6), identity helper packages beyond Next.js/React/browser/`@dimmi/server` (C16, later: identity-helper-packages) replaced by raw snippet routes; milestones M6..M12 now M5..M11; edge case ids remapped. App logic tested by integration tests only (real Postgres + Valkey); framework primitives keep unit tests; edge case ids in test names.

## M1. OAuth core

- [ ] Migrations 000901..000907 (`oauth_clients` + seed `dimmi-cli` and `dimmi-mobile` (`first_party`), `oauth_grants` (nullable workspace for first-party, shape CHECK, `auth_time`, `auth_methods`, `device_id`, device fields, `revoked_reason` registry enum: format CHECK + `enums.GrantRevokedReason` registry), `oauth_authorization_requests` (+ `device_id`, `auth_methods`), `oauth_auth_codes`, `oauth_access_tokens`, `oauth_refresh_tokens`, `oauth_device_codes`), RLS policies for tenant ones, SECURITY DEFINER lookup functions, sqlc queries, factories.
  - Accept: `migrate` up/down clean; factory `OAuthGrant(t).For(member).WithScopes(...).Create(ctx)` returns plaintext tokens; `OAuthGrant(t).FirstParty(user).Create(ctx)`. Tests: RLS blocks cross-workspace select; first-party rows invisible to tenant queries; lookup and user-scoped functions work without tenant set; shape CHECK rejects first-party row with workspace and scoped row without; `revoked_reason` outside registry rejected by action, non-snake value by CHECK (edge case 229).
- [ ] `app/auth/oauth/scopes.go`: scope parse + hierarchy + clamp via `authz` (scopes = ability groups from 00-accounts `app/auth/permissions.go`, role permissions from `roles.go`, never copied). Token gen/hash = `framework/auth/token`; PKCE, redirect matcher, RFC 8707 canonicalization = `framework/oauth` (framework unit tests + fuzz canonicalizer live there).
  - Accept: integration tests through `/oauth/authorize` and `/oauth/token` for every rule in oauth.md redirect + resource sections.
- [ ] Config `config/oauth.go` + `config/mcp.go` + `config/cli.go`, env bindings, `config:show` redaction.
  - Accept: defaults match README tables; invalid durations fail boot with message.
- [ ] Metadata endpoints: AS metadata (`oauthex.AuthServerMeta`), PRM (`oauthex.ProtectedResourceMetadata` via go-sdk `auth.ProtectedResourceMetadataHandler`; root, `/mcp`, `/api/v1`), `/.well-known/dimmi.json`, CORS middleware. No own metadata JSON structs.
  - Accept: JSON matches README examples; DCR/CIMD toggles reflected; issuer equals APP_URL exactly. Tests: snapshot, CORS headers, `openid-configuration` 404.
- [ ] Authorization request flow: `GET /oauth/authorize` validation order, login redirect with `intended`, `max_age` / `prompt=login` forced re-login (00-accounts `reauth=1`), `auth_time` (`sessions.authenticated_at`) + `auth_methods` snapshot capture, `device_id` param (64 chars, charset), `StartAuthorizationRequest`, HTML error page (templ), RFC 9207 `iss` on every redirect.
  - Accept: each validation step test (oauth.md list items 1 to 13); unregistered redirect never redirects; stale session + `max_age` forces login; `prompt=none` + stale: `login_required`.
- [ ] Consent API: `GET` + `PATCH /api/v1/oauth/authorization-requests/{oar}` (`status` approved/denied, one controller `OAuthAuthorizationRequestController` Show/Update, route names `.show`/`.update`); workspace + scope clamp; auto-approve rules; transition guard; `oauth_grant.created` event; audit log.
  - Accept: feature tests for edge cases A8 to A22, N182, N183; other user 404; double approve 422 `invalid_transition`; OpenAPI operationIds match route names.
- [ ] Token endpoint: `authorization_code` grant with client auth (none/basic/post), PKCE verify, code reuse revoke, audiences, `Cache-Control: no-store`; foundation `client.version` middleware in OAuth error mode (426 `invalid_request` / `client_upgrade_required`, `min_version`, `upgrade_url`) for every grant type; grant copies `auth_time`, `auth_methods`, `device_id`.
  - Accept: edge cases 222 to 225 (nothing consumed on 426; header absent allowed; malformed ignored).
  - Accept: RFC clause tests (OAuth 2.1 4.1.1, 4.1.3, 7.5.2; RFC 8707); timing: p95 under 30 ms in bench.
- [ ] Refresh grant on framework `oauth` (rotation + reuse family revoke built, no grace window): app adds idle sliding expiry, security mail on `refresh_reuse`, scope downscope.
  - Accept: tests E54 to E56 with frozen clock; 100-day clock-forward test.
- [ ] Revocation endpoint (RFC 7009) + `RevokeGrant` action + `oauth_grant.revoked` event + cache invalidation listener.
  - Accept: refresh revoke kills grant; access revoke only token; unknown token 200; cache busted on second App instance in test harness via Valkey pubsub.

## M2. Client registration

- [ ] CIMD resolver: `oauth.ClientMetadataDocument.Fetch` on `a.HTTP` (no redirects, 16 KB, 5 s), validation, cache with Cache-Control clamp, ETag revalidation, stale-if-error 7 days, per-host limits, allow/block lists, reserved-name rule.
  - Accept: tests B23 to B34 using `httpclienttest.New(tc.App.HTTP)`; SSRF private IP and rebinding covered by `framework/httpclient` integration tests (loopback/private listeners).
- [ ] DCR endpoint: RFC 7591 fields, `application_type` rules, device grant refusal, idempotent re-register, per-IP limit, reserved names, bidi stripping.
  - Accept: tests C35 to C42; response shape validated against RFC 7591 example fields.
- [ ] `dimmi oauth:client list|disable|enable` command + `DisableClient` action (revokes grants reason `client_disabled`).
  - Accept: disabled client authorize fails `invalid_client`; tokens 401 within cache TTL.

## M3. Device flow and grants API

- [ ] Device authorization endpoint + polling in token endpoint (`authorization_pending`, `slow_down` +5 s stored, `expired_token`, `access_denied`, consumed), rate limits, `dimmi-cli` only.
  - Accept: RFC 8628 tests D43 to D53.
- [ ] Device consent API: `GET /api/v1/oauth/device-requests?filter[user_code]=` lookup (normalization, throttle, binds user), `PATCH /api/v1/oauth/device-requests/{odr}` (`status`); one controller `OAuthDeviceRequestController` Index/Update; access log + trace redaction of `filter[user_code]`; consent payload with device name, IP, GeoIP location (optional file), user code.
  - Accept: brute-force throttle test; phishing warning fields present; log capture test shows no full user code; N184 to N186.
- [ ] Connected apps API: `GET /api/v1/oauth/grants` (own, `oauth_grants:manage_any` all), `DELETE` revoke; policy via `authz.Can` (09 permissions `oauth_grants:manage_any`, `agent_access:manage` registered in 00-accounts catalog); resource.
  - Accept: member cannot see others; role with `oauth_grants:manage_any` can revoke any; revoked grant tokens 401; lint no role names in 09 policies (edge case 232).
- [ ] Lifecycle listener `RevokeMemberOAuthGrants` on `member.deactivated`, `member.role_changed` (downgrade), `user.deleted` (all workspaces), `workspace.deleted` + middleware member-status check.
  - Tests: each event revokes with right reason; deactivated member token 401 within 30 s even with job delayed; cross-workspace user deletion revokes every grant.
  - Accept: E58 to E60 tests; job unique per member.
- [ ] `SendOAuthGrantCreatedMail` mailable + templ view; suppression rule for official client on known device.
  - Accept: `tc.Mail` (`mail.AssertSent`) asserts; content includes client, device, IP, revoke link.
- [ ] `PruneOAuthRecords` periodic job incl. idle grant revoke.
  - Accept: seeded expired rows removed; active chains kept; stops after 60 s budget.

## M4. Resource server integration

- [ ] Extend `auth:token` middleware: `dmi_oat_` lookup via cache, audience check per resource, effective permissions (expanded scopes intersect role permissions, `authz`), principal carries grant `auth_methods`, `auth_time`, `device_id` for 00-accounts `AccessPolicy`, `WWW-Authenticate` on 401/403 with `resource_metadata` and `scope`, query-token rejection.
  - Accept: oauth.md "Protected resource behavior" table, one test per row, for `/api/v1` and `/mcp`.
- [ ] `app/auth/oauth/token_cache.go` (otter, 30 s) + Valkey pubsub invalidation (`a.PubSub`, topic `oauth.bust`; 2 s local TTL while disconnected) + buffered `last_used_at` flusher (flush in provider `Shutdown`).
  - Accept: bench p95 1 ms hit / 5 ms miss; flusher writes at most once per minute per grant; flush on shutdown.
- [ ] Agent-access settings: `agent_access` key declared in foundation settings registry (workspace scope), show/update endpoints, policy `agent_access:manage`, audit.
  - Accept: member 403; values validated; consent respects `allow_*` flags.
- [ ] Actor `Via` support per 00-foundation/contracts.md: actions accept `Actor{Type, ID, Via}` (OAuth = `Via.Kind=oauth_client`, PAT = `personal_token`); message `sent_via` metadata writer.
  - Accept: message created through token has metadata; audit rows show client.

## M4b. First-party apps (Dimmi mobile)

Depends on M1 to M4 and 00-accounts sessions (`authenticated_at`, `reauth=1` login mode, `GET/DELETE /sessions`).

- [ ] `config/apps.go` (`apps.enabled`, `apps.ios_app_ids`, `apps.android_package_fingerprints`) + `oauth.first_party.*` keys; `app/auth/oauth/first_party.go` (redirect kind, consent skip rule, scope/resource rules, User-Agent parser).
  - Accept: table tests: claimed redirect only with non-empty lists; scheme never skips consent; `first_party` scope refused for other clients; `/mcp` resource `invalid_target`; UA samples (iOS, Android, garbage).
- [ ] Association files: `GET /.well-known/apple-app-site-association`, `GET /.well-known/assetlinks.json` (200 direct, `application/json`, cache 1 h, 404 when empty or disabled); `dimmi.json` `apps` block; `dimmi doctor` check (200, content type, no redirect).
  - Accept: snapshot JSON per config; O195 to O197.
- [ ] `/oauth/callback/app` fallback page (templ, no scripts, no-store, no-referrer, query never logged or rendered).
  - Accept: O190 to O192; log capture shows no code.
- [ ] Authorize + consent for `dimmi-mobile`: `device_name`, claimed-link auto-approve (new grant each time), scheme consent payload (no workspace picker, no scopes, device warning), `PATCH {status}` with `workspace_id`/`scopes` prohibited, `apps.enabled=false` refusals.
  - Accept: O187, O193, O194, O202, O203; consent never skipped on scheme even with existing grant.
- [ ] Token endpoint first-party: grant with null workspace + `{first_party}` scope, 15-min access, `auth_time`, response without `workspace`, UA capture on exchange and refresh (no re-rotation grace: reuse revokes family).
  - Accept: O204, O205 with frozen clock; O220; `unauthorized_client` when disabled.
- [ ] Resolver `ResolveAccessToken`: `auth:app` vs `auth:token` by lookup only, principal `AuthTime` + `ConfirmedAt`, Via `app`, `/mcp` rejection, disabled check; plugged into 00-accounts guards and as go-sdk `auth.TokenVerifier`; `password.confirm` reads `max(AuthTime, ConfirmedAt)`.
- [ ] Grant confirmation: migration columns `oauth_grants.confirmed_at`, `confirmed_method` (registry format CHECK), `ConfirmGrant(grantID, method)` called by 00-accounts `ConfirmUser` for app principals, cache bust.
  - Accept: Q236 to Q238; refresh never changes `confirmed_at`.
  - Tests: first-party token on `auth:user` route passes, PAT/third-party token 403 `first_party_only`; workspace route needs `Dimmi-Workspace`; left workspace 403 `membership_deactivated`; sudo route passes within TTL after `max_age=0` login, 423 after.
  - Accept: O198 to O201, O207 to O209.
- [ ] User-scoped grant actions: `ListUserAppGrants` (rows incl. `client_name`, `auth_methods`, `auth_time`, `confirmed_at`), `RevokeAppGrant`, `RevokeUserAppGrants` (SECURITY DEFINER functions, caller tx, cache bust + centrifuge disconnect after commit); 00-accounts hooks wired (sessions list/revoke, logout others, password change/reset, 2FA disable, account delete, pre-hijack defense). Registry adds reason `pre_hijack_defense`.
  - Tests: each trigger revokes with right reason and except rule (Q239); other user's grant 404; revoked app token 401 on second App instance via Valkey pubsub; Q240 renamed client.
  - Accept: O210 to O218.
- [ ] Security mail for first-party login + security event (00-accounts), metrics `dimmi_oauth_first_party_*`.
  - Accept: `tc.Mail` asserts device name + revoke link; suppression on same device re-auth.

## M5. MCP server core

- [ ] `framework/mcp` integration (go-sdk v1.8.0, `StreamableHTTPOptions.Stateless = true`): server, `server/discover`, instructions, stateless HTTP handler at `/mcp`, bearer via go-sdk `auth.RequireBearerToken` (POST only, GET/DELETE 405, Origin check, header validation, 1 MB limit), legacy versions via compat (no session minting).
  - Accept: conformance suite (2026-07-28 server) green; inspector smoke for 2025-06-18 and 2025-11-25; tests F70 to F77.
- [ ] MCP auth: principal from bearer (OAuth/PAT), 401/403 headers at HTTP level, PAT ability errors as tool errors, rate limit bucket shared with REST, first-party token 403 `first_party_token_not_allowed`.
  - Accept: O201 test; unauthenticated `tools/list` returns 401 with PRM URL; Claude Code-style discovery walk test (401, PRM, AS metadata, CIMD, authorize, token, call) end to end with test client.
- [ ] Tool registry + schema generation from Go request types via huma registry, `$ref` inlining, deterministic order, precomputed `tools/list` bytes, cacheable result fields. Each tool declares `Operations` per README REST column (`conversations.update` for close/reopen/snooze/assign, `conversations.replies.store` for send).
  - Accept: drift test fails when tool operationId missing from `openapi.json`.
- [ ] Feature toolsets: `?toolsets=` / `--toolsets`, per-ability + edition filtering, per-toolset 2,000-token CI budget, registration API for other specs' tools, confirm-gate requirement check. Register names `ai_actions` (07), `docs_sync` (08), `changelog` (13), `feedback` (14), `engage` (15-announcements-surveys), `imports` (16) with gate rules from mcp.md (changelog notify always confirms; `dimmi_publish_ai_action` always confirms, write publish `human_review_required`; `dimmi_run_import` / `dimmi_rollback_import` always confirm; no docs_sync tool accepts or verifies).
  - Tests: Q248 to Q251; gate never bypasses owner policy.
  - Tests: default list = core 19; `toolsets=billing` on self-host omitted; member without `billing:read` never sees billing tools; unknown toolset 400.
  - Accept: CI token budget test (under 6,000); drift test vs `openapi.json` operationIds and param types.
- [ ] Renderer toolkit: token estimator, budgeted truncation with cursors, untrusted envelope + escaping, relative times in workspace tz, grapheme-safe cuts.
  - Accept: MCP integration tests G86, G87, G89, G107 through real tool calls; fuzz envelope escaping (pure function, allowed as unit fuzz).
- [ ] Error mapping (`app/mcp/errors.go`): domain errors to tool errors with codes + next step; protocol errors to JSON-RPC codes per version (-32602 vs legacy -32002 for resources).
  - Accept: table test per mapping row in mcp.md.
- [ ] Confirmation gate: requestState seal/open (HKDF key, previous key fallback), nonce store in `idempotency_keys` (Postgres), `confirm` param dedupe in Valkey (`SET NX EX 300`), elicitation via MRTR, `confirm` param fallback, no policy bypass (every token kind confirms), args digest, new-message recheck, decline/cancel stores nothing.
  - Accept: tests H108 to H117; replay returns stored result.

## M6. MCP tools, resources, prompts

- [ ] Workspace + conversation read tools: `dimmi_get_workspace`, `dimmi_list_conversations`, `dimmi_search_conversations`, `dimmi_get_conversation`.
  - Accept: per-tool feature tests; 1M-message fixture p95 under 80 ms; budget under 8,000 tokens.
- [ ] Conversation write tools: `dimmi_add_note`, `dimmi_tag_conversation`, `dimmi_assign_conversation`, `dimmi_close_conversation`, `dimmi_reopen_conversation`, `dimmi_snooze_conversation` (duration parser, DST).
  - Accept: per-tool tests incl. G94 to G98; idempotent no-op results `changed: false`.
- [ ] `dimmi_send_reply` with secret scan, closed guard (422 `invalid_transition` unless `reopen`/`close_after`, checked at preview and accept), arg mapping `reopen` -> `then: open`, `close_after` -> `then: close`, else `then: keep` (both: schema error), per-grant send limit, per-conversation 10 s limit, dedupe, channel errors passthrough.
  - Accept: edge cases 117, 230, 231; REST call recorded by fake 03 carries `then`, never `reopen`/`close_after`.
  - Accept: tests H118 to H123.
- [ ] Contact tools: `dimmi_search_contacts`, `dimmi_get_contact`, `dimmi_update_contact` (ambiguity, limits, verified email rule).
  - Accept: G85, G99, G100.
- [ ] Knowledge + article tools (with 08 actions): `dimmi_search_knowledge`, `dimmi_list_articles`, `dimmi_get_article`, `dimmi_propose_article`, `dimmi_propose_article_edit` (08 owns behavior); test: no registered tool reaches `PublishArticle`/accept/archive actions.
  - Accept: G101 to G105; publish/accept/archive unreachable from MCP.
- [ ] Resources (workspace + 3 templates) and prompts (4, Markdown in `resources/mcp/prompts`).
  - Accept: `resources/read` for each; prompts render with args; cacheScope values per mcp.md.
- [ ] Observability: metrics list, logs fields, OTel spans with `traceparent` from `_meta`.
  - Accept: metrics visible on `/metrics`; log redaction test (no bodies, no tokens).

## M7. CLI foundation

- [ ] Build flavors: `-tags cli` excludes server packages; depguard rules; size check in CI (under 15 MB).
  - Accept: `go build -tags cli` binary lacks `serve`; CI fails if import leak.
- [ ] Generated API client from committed `docs/openapi/v1.json` (oapi-codegen) + wrapper (retries, Idempotency-Key, error decoding, rate-limit retry), CI regen diff check.
  - Accept: wrapper tests against real test server (`testkit` App over HTTP), no mocks of app code.
- [ ] Output layer: `--json`, tables, exit codes, error rendering with hints, NO_COLOR, OSC 8, SIGPIPE handling, agent detection.
  - Accept: golden tests; every command registers `--json`; every write registers `--dry-run` (reflection test over cobra tree).
- [ ] Discovery + hosts/regions/profiles + project `dimmi.json` resolution + version gate (discovery `cli.min_version` and server 426); HTTP client sends `Dimmi-Client-Version: cli/<ver>` and `Dimmi-Version` on every request (API, OAuth, MCP bridge); 426 and 400 `invalid_api_version` map to exit 10.
  - Accept: fake server asserts both headers on every call; edge cases 130, 225, 235.
  - Accept: I128 to I130 tests.
- [ ] Credentials: keychain + file fallback (0600), file lock refresh, env token override.
  - Accept: I131, I132; two-process refresh test.
- [ ] `login` (device flow, browser open rules, `--with-token`, `--json` event lines), `logout`, `whoami`, `profiles`.
  - Accept: device flow tests against test server; logout offline path.
- [ ] npm packaging: launcher, platform packages, fallback downloader with sha256, goreleaser config (archives, cosign, SBOM, npm provenance, Homebrew tap, install script).
  - Accept: CI matrix (macOS arm64/x64, Linux glibc/Alpine, Windows) runs `npx dimmi --version`; cold-start under 4 s on clean runner; checksum mismatch rejected.

## M8. CLI commands

- [ ] `conversations list|show|reply|close` (+ `--watch` via centrifuge-go client on `/connection/websocket`, bearer connect token, `workspace:{ws_}` with recovery; Q246, Q247), editor flow, preview + confirm, idempotency; `close` = `PATCH /conversations/{conv} {status: "closed"}`, `reply` = `POST /replies {body, then}` (`--reopen` = `open`, `--close` = `close`, default `keep`; closed without flag exit 7 `invalid_transition`) (cli.md command map).
  - Accept: golden outputs; non-interactive reply without `--yes` exits 2; fake server asserts exact method + path per command map row.
- [ ] `articles pull|push` (frontmatter, lock file, conflicts, suggestions for published, `--publish`, `--prune`, image warnings).
  - Accept: K159 to K163; dry-run plan table snapshot.
- [ ] 13-15 command groups: `changelog` (list/new/draft/publish/subscribers), `feedback` (import helper with dry run then `--run`, status shortcut; rest generated), `announcements`/`surveys`/`segments` (15-announcements-surveys); front matter parser shared with `articles`; confirm on publish/notify.
  - Accept: golden output + `--dry-run` REST map per command; publish without `--yes` non-TTY exits 2.
- [ ] 07/08/10/16 command groups: `actions` (list/get/test/publish/calls/apply/import, YAML file format, no secrets), `docs` (suggestions/sync/stale/verify, push/pull aliases) + `articles publish` + `github repos add|list`, `webhooks endpoints|deliveries|redeliver` + `trigger`, `migrate <source>` client mode for 16 wizard + `imports ...` + hidden-input prompt helper `app/client/prompt`.
  - Accept: golden output + `--dry-run` REST map per command; `docs verify` / `docs suggestions --accept-all` / `imports rollback` non-TTY without `--yes` exit 2; secret prompt never echoes, never in argv; E253 to E264.
- [ ] `tokens create|list|revoke` (00-accounts API), `api` raw command with `--paginate`, `version`.
  - Accept: token printed alone with `--quiet`; `api` validates path against cached spec.
- [ ] `mcp` bridge (stdio to HTTP, token refresh on 401, legacy passthrough) and `mcp --in-process` (full flavor).
  - Accept: Claude Code-style stdio client test through bridge; EOF exit; in-process requires `DIMMI_TOKEN`.

## M9. `init` (SDK-driven for Next.js/React/`@dimmi/server`, 02 raw snippet route for other stacks; `01-messenger-widget/sdk-contract.md` "`dimmi init` result")

- [ ] Init framework: steps runner, preflight, hosting choice, inline login, plan + atomic writes + backup + rollback, re-run detection (`<Dimmi`, `withDimmi(`, `dimmi.config.*`, `api/dimmi/token` route file, `dimmi.json`), `--json` result, agent mode.
  - Accept: J153, J154; rollback test with injected write failure.
- [ ] Detectors for all SDK framework ids (`nextjs-app`, `nextjs-pages`, `react`, `react-router`, `sveltekit`, `nuxt`, `vue`, `astro`, `rails`, `laravel`, `django`, `fastapi`, `go`, `express`, `hono`, `html`, `proxy`) + auth library detection (Clerk, Auth.js, Supabase, better-auth, Devise/`Current.user`, Laravel auth, Django auth) + package manager detection + monorepo selection.
  - Accept: fixture per framework id (17) and variants (`src/`, JS vs TS, monorepo); detected id equals 01 `BuildInstallSnippets` id list (shared constant, drift test).
- [ ] Apply 01 install endpoint text (`GET /widget-settings/{widget}/install?framework=<id>`); Next.js codemods: `dimmi.config.ts` with `identify` line from auth table, `withDimmi` wrap (incl. nested plugin wrap), `<Dimmi />` insert; package stacks: no JWT endpoint files, no signing code.
  - Accept: snapshot diffs per fixture equal sdk-contract example; Q241; grep test: package-stack generated files never contain `sign`/`HS256`/`jose`; ambiguous anchor yields partial (J142).
- [ ] `@dimmi/server` stacks: React/React Router resource route, express/hono `identityHandler`; `html` with `data-identity-url` copy-paste line. No identity modes set or read (later: identity-validations).
  - Accept: Q244.
- [ ] Raw-snippet stacks (later: identity-helper-packages): SvelteKit, Nuxt, Vue, Astro, Rails, Laravel, Django, FastAPI, fastify backends, Supabase SPA (Edge Function from edge snippet + inline `{getToken, subscribe}` identity source, E156): script tag or `identityUrl` + route file from 02 raw snippet (`GET /identity-verification/snippets?language=`; edge `jose`, PyJWT, ruby-jwt, firebase/php-jwt), framework default user lookup; Go print-only.
  - Accept: J155, J156, J157, Q245; route file text equals 02 snippet for stack (secret from env, never literal); generated non-JS edits pass language linters in CI containers (rubocop, php -l, python -m py_compile).
- [ ] Env writing (`DIMMI_APP_ID` `wgt_`, `DIMMI_IDENTITY_SECRET`) with gitignore check, public-prefix refusal, `.env.example` update, identity secret creation (02 API) with role fallback; `dimmi.json` with `wgt_` (pre-release `app_` rewrite).
  - Accept: J146 to J148, Q242, Q243.
- [ ] Help center seeding: file collection, secret scan, caps, upload to 08 cold-start, progress polling.
  - Accept: J149 to J151.
- [ ] 3-line summary (sdk-contract text) + line-2 variants + `--json` `summary` array; self-host loopback deploy link (11).
  - Accept: golden summary per mode.
- [ ] E2E: `npx dimmi init` on Next.js fixture against test server with scripted device approval; time budget assertion.
  - Accept: median under 90 s over 5 CI runs (network mocked for package install).

## M10. Install-by-prompt deliverables

- [ ] `agents install`: skill, Cursor rule, AGENTS.md block, MCP configs (Claude Code `.mcp.json`, Cursor, VS Code, Codex instructions), idempotent merges.
  - Accept: JSON merge preserves other servers; markers idempotent; size budgets (skill under 250 lines, rule under 120 lines, AGENTS block under 40 lines).
- [ ] Write content: SKILL.md, dimmi.mdc, AGENTS snippet, copy-paste prompt, per outlines in cli.md.
  - Accept: review checklist; frontmatter parse tests.
- [ ] `trydimmi/agent-kit` repo with Claude Code plugin marketplace manifest (skill + `.mcp.json`), synced from `resources/cli/agents` on release.
  - Accept: `/plugin marketplace add trydimmi/agent-kit` then install works in manual QA.
- [ ] Docs `llms.txt` + `llms-full.txt` generator in docs build, `.md` per page.
  - Accept: llms.txt under 4 KB, valid per llmstxt.org structure; link checker passes.
- [ ] Prompt benchmark harness (manual, no CI LLM spend): scripted checklist for "Add Dimmi support chat with secure login" in Claude Code, Cursor, Lovable, Bolt; record time-to-first-message vs Fin CLI.
  - Accept: results table in release notes; each run approved individually (real model cost).

## M11. Hardening

- [ ] Security review: token storage, SSRF, open redirect, CSRF on consent API, device phishing, prompt-injection envelope, requestState crypto.
  - Tests: injection corpus (customer text instructing sends, webhook creation, cross-customer leak) against renderer envelope and confirmation gate; webhook URL change via MCP always confirms; feature toolset gate audit (every write feature tool with customer-visible/public/money/egress effect wired to `app/mcp/confirm`, CI lists violations); no tool output contains `dmi_`/signed URL; `Referrer-Policy: no-referrer` on consent + redirects.
  - Accept: findings fixed; pen-test checklist in repo.
- [ ] Load: 10k pending device codes polling, 1k concurrent MCP clients calling read tools.
  - Accept: budgets in README met; RSS impact under 3 MB idle.
- [ ] Docs pages: OAuth for apps, MCP, CLI reference (generated from cobra), troubleshooting.
  - Accept: every error code in README/mcp.md documented.

## Done when

- All boxes checked; conformance suite green; budgets in README met in CI.
- Demo script (cloud staging and `docker compose` self-host, same steps, only host differs):

```
# 1. Zero to widget
npx create-next-app@latest demo --ts --app && cd demo
npx dimmi@latest init                 # pick cloud, approve device code in browser
#   expect: SDK + dimmi.config.ts + withDimmi + .env.local (no JWT route), 3-line summary, 10+ help drafts, skill/rule files, under 90 s
npm run dev                           # open localhost:3000 as logged-in test user, send "I was charged twice"
dimmi conversations list --watch      # message appears with verified email

# 2. Claude Code inbox work
claude mcp add --transport http dimmi https://staging.trydimmi.com/mcp   # browser consent (CIMD client), approve
#   in Claude Code: "reply to #1 with the refund steps"
#   expect: dimmi_send_reply returns elicitation dialog with recipient + body; decline: nothing sent, nothing stored
#   then: "send it" ; accept; customer sees reply in widget

# 3. Terminal + docs as code
dimmi articles pull && edit docs/help/billing/refunds.md && dimmi articles push --dry-run && dimmi articles push
#   expect: published article change lands as suggestion in review queue with diff

# 4. Revoke
dimmi whoami                          # shows grant, scopes, no absolute expiry
#   dashboard connected apps (API: GET /api/v1/oauth/grants) shows CLI + Claude Code; revoke Claude Code
#   next Claude Code tool call gets 401 and re-auth prompt

# 4b. Mobile app (cloud staging)
#   app: sign in -> system browser sheet, existing web session, no consent (claimed link) -> inbox loads
#   dashboard session list (GET /api/v1/sessions) shows phone as kind app; revoke -> app shows signed out
#   self-host custom domain: app sign in shows consent with device warning (scheme fallback)

# 5. Self-host
dimmi login --host http://localhost:8080 --profile local   # same flow (loopback http allowed)
dimmi conversations list --profile local --json | jq '.data | length'
```

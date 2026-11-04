# Security and privacy baseline

Style: caveman. Scope trim 2026-11-02: hosted survey links, send modes, provider inbound mail endpoints deferred (later names inline). Applies to every feature. Feature specs list only extra risks.

## Tenancy isolation

- Every query scoped by `workspace_id`. sqlc queries take `workspace_id` param; review rule: query on tenant table without `workspace_id` predicate fails lint (custom check in CI over `database/queries`).
- Cloud: Postgres RLS on all tenant tables as second wall. `workspace` middleware puts tenant in context (`tenancy.With`); `framework/database` sets `app.workspace_id` with `SET LOCAL` at start of every transaction. Tenant data reachable only inside `db.Transaction`; outside one tenant tables look empty (fail closed). Background jobs put tenant in context before opening tenant transactions.
- RLS policies written once through one function: `USING (workspace_id = ANY (dimmi_visible_workspaces()))`. `dimmi_visible_workspaces()` (SQL, STABLE) returns `ARRAY[current_setting('app.workspace_id')::uuid]` at MVP. Later org-level reads (org audit, consolidated billing, reports) set `app.workspace_ids` and function returns that list: one function change, no policy rewrite. Btree index handles `= ANY(array)`. Perf test: RLS overhead under 5% on list endpoints.
- Every tenant-owned row carries `workspace_id`, join tables included (P8). Migration lint: table with FK to tenant table must have `workspace_id` unless annotated `-- lint:user-scoped` (`user_id` rows: sessions, `user_auth_factors`, push subscriptions), `-- lint:org-scoped` (keyed by `billing_account_id` / `organization_id`: 12 account tables, `entitlement_overrides`; RLS via `dimmi_visible_billing_accounts()` (12, SQL STABLE: billing accounts of organizations owning `dimmi_visible_workspaces()`)) or `-- lint:global` (`organizations`, `encryption_keys`).
- `organizations` global, not tenant: no RLS; reached via `workspace -> organization` join in actions or SECURITY DEFINER functions (same pattern as 00-accounts lookups).
- Foreign IDs from other workspace return 404, never 403 (no existence leak).
- Cross-tenant tests: every feature test suite includes "other workspace cannot read/write" cases.

## Secrets

- Envelope encryption via `KeyManager` (`framework/encryption`, Tink; contracts.md): one keyset per (`encryption.Scope`, purpose `aead`/`stream`/`mac`), stored wrapped in `encryption_keys` (data-model.md; one row per keyset, not per data key), wrapped by KEK (`local`: derived from app key `DIMMI_APP__KEY` / `<data_dir>/app.key`; or `awskms`: own thin driver on aws-sdk-go-v2 `kms`, Tink-compatible encryption context, every wrap/unwrap a KMS call in CloudTrail). Columns `*_encrypted bytea` (`encryption.Sealed`). Ciphertext carries Tink key-id prefix + scope: decrypt never guesses scope.
- Scopes: `workspace` for tenant secrets; `system` for user-level secrets (auth factor secrets in `user_auth_factors`, OAuth identity tokens), not workspace key (users span workspaces). `organization` reserved: CMK later = org-scoped customer KEK, keysets re-wrapped under it, no data re-encryption.
- `dimmi key:generate` creates app key (`base64:` + 32 bytes). Compose and container image: `app.key_auto_generate` on, key generated on first boot, persisted to `<data_dir>/app.key` (0600; `/var/lib/dimmi` in images), logged once with backup warning. Without key and auto-generate off: boot fails `ErrNoAppKey`.
- Key rotation: KEK change = set new `encryption.kek`, keep old in `encryption.previous`, run `dimmi key:rotate` (`RewrapAll`: re-wraps every keyset); no data re-encryption. Data key rotation per scope (`Rotate`): new primary key inside `aead` + `stream` keysets, old keys stay to decrypt. MAC keys never rotated automatically (deterministic lookups and tombstones must keep matching). Crypto-shred = delete scope keysets.
- Never log: tokens, API keys, passwords, JWT, email bodies at info level, LLM prompts with customer data (debug only, opt-in, redacted).
- Single exception: `log` mail driver (dev, and self-host before a mail provider is configured) prints full email including magic/verification links. Boot warns loudly; `dimmi doctor` flags it; never allowed when `app.edition = cloud`.
- Redaction for logs (`framework/log` masq + `log.redact` keys): known secret patterns (`sk-`, `dmi_` incl. `dmi_pts_`, `xoxb-`, `whsec_`, bearer headers), query param `cf_connect` (realtime connect token), path segments of `/changelog/*-confirmations/*`, `/changelog/unsubscriptions/*`, `/email-unsubscriptions/*`.
- Import source credentials (16): sealed with workspace DEK (aad bound to row), write-only in API (`configured`, `hint` only), never in logs, errors, job args, MCP or CLI history; shredded on archive, after `credentials_purge_after`, on workspace delete. Agent action connection secrets (07) same rules; MCP never reads or writes them.
- Valkey: password/ACL auth always (`redis.url` carries credentials, `_FILE` secret supported), never bound to public interface (compose: internal network only), TLS in cloud (ElastiCache for Valkey in-transit encryption). Holds only rebuildable state with TTL; no secrets, no message bodies; presence carries public ids + display names only.

## Passwords and sessions

- argon2id (`framework/hashing`), OWASP params. Breached password check optional (k-anonymity HIBP via app `password` validation rule on `a.HTTP`, off on self-host by default).
- Session cookie: `__Host-dimmi_session`, `Secure`, `HttpOnly`, `SameSite=Lax`. Plain-http self-host (no TLS, local network): falls back to `dimmi_session` without `Secure`, boot warning. Rotate on login and privilege change. Idle 14 days, absolute 30 days. Session list + revoke in account settings.
- CSRF: Go `net/http.CrossOriginProtection` (`framework/http/middleware` `CSRF`) on cookie-authenticated unsafe methods: rejects cross-site `Sec-Fetch-Site`, falls back to `Origin` vs `Host`; requests carrying neither treated as non-browser (allowed, still need cookie). No token cookie, no `X-XSRF-TOKEN`, no `GET /auth/csrf`. Trusted extra origins (`csrf` middleware params, e.g. `csrf:https://dash.example.com`; `app.url` origin always trusted) only for split-host deployments. Contact-side forms (portal 14, CSAT rating page 15, unsubscribe pages) use same middleware plus per-form token where page is cross-site embeddable.
- Auth factors: `user_auth_factors` rows + `factors.Factor` driver registry (00-accounts). MVP drivers `totp` + `recovery_code`; later `webauthn`, `email_otp`. `sms` never (SIM swap). Workspace setting `security.require_two_factor` (MVP).
- Sessions and first-party grants record how user proved identity (P3): `auth_methods` (every method passed), `authenticated_at` (last full login incl. factors), `confirmed_method` (last sudo), `device_id` (SHA-256 hex of `__Host-dimmi_device` cookie; app install id on grants). Missing data cannot be backfilled, so recorded from day one.
- Workspace (later org) policies enforced only through `AccessPolicy` middleware `access.policy` (contracts.md): one place, one 403 body `{code, message, requirement, action_url}`. MVP rule `security.require_two_factor`.
- Every primary login method calls `LoginPolicyResolver.ForEmail` before verifying secrets (MVP allow-all; later domain-level SSO enforcement, no account enumeration).

## Sign-in rules (added 2026-11-04, `21-enterprise-identity`)

- `LoginPolicyResolver` driver answers `sso_required` at domain level for verified, enforced organization domains. Never by user. Owner bypass gives same answer as wrong password to anyone else.
- `AccessPolicy` rules, order: IP allowlist, SSO enforced, sign-in method, Google domain, session length, 2FA. No query per request; settings read from loaded workspace.
- Session length only shortens install limits; evaluated per workspace, so one strict workspace never ends sessions in others.
- SSO connection may sign in only addresses under domains its organization verified, plus identities linked from a signed-in session. Stops one customer's IdP asserting another's people.
- SAML: unsolicited responses never start a session. Assertion ids stored until expiry in Valkey; Valkey down refuses sign-in. Responses and client secrets never logged.
- Client IP for allowlists from framework trust config; `dimmi doctor` warns when proxy trust looks wrong.
- Admin-started 2FA reset: mailbox + password of target, refused across organizations ([settings-pages.md](../00-accounts/settings-pages.md)).
- Log redaction adds `SAMLResponse`, `dmi_scim_`.

## First-party apps (Dimmi mobile, later desktop)

- Login runs in system browser sheet (ASWebAuthenticationSession on iOS, Custom Tabs on Android), never an embedded webview: password, magic link, GitHub, Google (Google blocks embedded webviews) and 2FA reuse web login unchanged. App never sees a password for login.
- OAuth authorization code + PKCE S256 via system client `dimmi-mobile` (`first_party = true`, public client, no secret). Details: `specs/09-agent-native/oauth.md` "First-party apps". Optional authorize param `device_id` (app install id, random UUID in keychain/keystore, max 64 chars) stored on grant with `auth_methods`.
- Client version gate: first-party apps and CLI send `Dimmi-Client-Version: <platform>/<semver>`. Middleware `client.version` on `/api/v1` and `/oauth/token` returns 426 `client_upgrade_required` when `apps.min_versions.<platform>` set and client older (api.md). Header absent = allowed.
- Anyone can send `client_id=dimmi-mobile`. Defenses:
  - Consent skipped only when redirect is a claimed HTTPS link (iOS Universal Link / Android App Link) on `{app.url}/oauth/callback/app`, verified by OS against `/.well-known/apple-app-site-association` and `/.well-known/assetlinks.json` served by the monolith. A malicious app cannot receive that redirect.
  - Private-use scheme fallback (`com.trydimmi.app:/oauth/callback`, RFC 8252 7.1) always shows consent screen naming the device and warning "Only continue if you are signing in to the Dimmi app on your phone".
  - PKCE binds code to the app instance that started the flow.
- First-party grant is user-wide (all workspaces), so blast radius is bigger than a PAT. Mitigations: access token 15 min (not 1 h), refresh rotation with reuse detection, grant listed in `GET /sessions` as `kind: app` with device name, platform, last IP, last used; revocable there and by "log out other sessions"; revoked on password reset and account deletion (all grants); on password change and 2FA disable all app grants except the one making the change (when made from the app). Browser sessions follow 00-accounts rules. Removal from one workspace does not revoke grant (user may belong to others); workspace membership re-checked every request.
- Self-host: official app cannot claim a custom self-host domain (associated domains compiled into app), so official app on self-host always uses private-use scheme + consent screen. `/.well-known` files still served from config (`apps.ios_app_ids`, `apps.android_package_fingerprints`) for white-label builds that list the instance domain. Instance admin can disable first-party apps (`apps.enabled=false`): `dimmi-mobile` grants refused 403 `first_party_app_disabled`.
- Sudo (`password.confirm`) from app: re-auth in browser sheet with `max_age=0` (token `auth_time` refreshed), or `POST /user/confirmation` with `method` `password`, `email_code` or any factor type whose driver has `Confirmation` capability (MVP `totp`). Session stores `confirmed_method`.
- DPoP sender-constrained tokens: post-MVP (needs app support).
- OAuth server = `framework/oauth` (storage + migration in framework: `oauth_clients`, `oauth_authorization_codes`, `oauth_grants`, `oauth_tokens`): code + PKCE S256, rotating refresh tokens; refresh token reuse always revokes the whole family (grant), no grace window. Deferred: RFC 8707 resource indicators / audiences (needed by MCP), client ID metadata documents (CIMD), device grant, consent-skip rules, last-use tracking.

## Outbound HTTP (SSRF)

- All outbound via `framework/httpclient` (`a.HTTP`; adapter-zone SDKs get `a.HTTP.StdClient()`). Tests: `httpclienttest.New(a.HTTP)` stubs + `PreventStrayRequests`; every framework Fake (httpclient, dns, mail, malware, socialite) panics unless app env is `testing`, so a fake can never reach production. Guard = `code.dny.dev/ssrf` dialer `Control` hook: checks resolved IP at connect (DNS rebinding safe, every redirect hop): blocks private, loopback, link-local, CGNAT, multicast, cloud metadata (169.254.169.254, fd00:ec2::254), IPv6 unique local, IANA special-purpose ranges. Wrapper adds: max 3 redirects, timeouts (connect 5 s, total 30 s default), response size cap, only http/https. Non-HTTP dials (websocket clients; later IMAP adapter) via `httpclient/safedial`.
- Self-host setting `http_client.allow_private_networks` for intranet webhooks/LLMs (off by default, warns in `dimmi doctor`). Narrower `http_client.allow_hosts` (host:port list, e.g. `localhost:11434` for Ollama; mapped to ssrf allowed prefixes/ports) preferred.
- Used by: webhooks (10), knowledge crawls (08), GitHub API (08 shared layer, used by 08 + 13), AI providers + tool calls (07), agent actions (07: customer APIs and remote MCP servers; no redirects followed; Dimmi's own hosts blocked), importers (16: fixed provider hosts per driver, `public_site` crawl with robots + politeness; attachment URLs checked every hop), avatar fetch, link previews, CIMD fetch (09).

## Operations

- Production schema changes need confirmation: `migrate`, `down` and `migrate:rollback` in production print database host/name and pending migrations, then ask; `--force` skips prompt; non-interactive without `--force` refuses. Rollback in production also needs exact version (`--confirm-version`). Dev-only commands (`make:*`, `route:lint`, `lang:check`) not compiled into production binary (`dev` build tag).

## Untrusted content

- Markdown rendering: goldmark with raw HTML disabled; links `rel="nofollow noopener ugc"`; allowed schemes http, https, mailto, tel.
- Email HTML: bluemonday strict policy, remote images proxied or blocked until click, rendered in sandboxed iframe (`sandbox` without `allow-scripts`, CSP `default-src 'none'; img-src data: https:; style-src 'unsafe-inline'`).
- Help center pages (incl. changelog 13, feedback portal 14): CSP `default-src 'self'`, no inline scripts except nonce'd; portal pages `form-action 'self'`, `frame-ancestors` from `feedback_settings.frame_ancestors`.
- Dashboard CSP strict, nonce-based.
- File uploads: sniff real type, block executables and HTML/SVG served inline (SVG as attachment only), images re-checked dimension limits (decompression bomb guard), EXIF orientation applied then metadata stripped on re-encode (`framework/image`), AVIF refused (`ErrUnsupportedFormat`), optional malware scan driver (GuardDuty, ClamAV).
- Prompt injection: see 07. Rule here: AI output treated as untrusted user content everywhere (sanitized render, no auto-executed actions without policy).

## Widget specifics

- Widget public endpoints keyed by app id; visitor session token random 32 bytes, stored hashed, bound to contact.
- Origin allowlist per workspace (optional, recommended): requests with other `Origin` rejected 403 `origin_not_allowed`. Allowlist empty = any origin (dev friendly), dashboard warns.
- Identity: unverified attributes never grant access to existing verified contact history (02). Verified JWT sets `email_verified` only when claim `email_verified: true` present.
- Trusted attributes (`contacts.trusted_attribute_keys`, data-model.md): only verified JWT claims (02), token API upserts (03), admin-marked import mappings (16) and integrations add keys; widget-set attributes never trusted (widget write removes key). 07 agent actions bind identity only to trusted keys.
- Identity tokens (widget JWT, portal SSO token) never inlined in server-rendered HTML or cached pages: minted per request by host app endpoint (`identity source` in `01-messenger-widget/sdk-contract.md`), short TTL (recommended 10 min), replay set in Valkey (`SET NX EX` on `nonce`/`jti`).
- Realtime: widget connects with 60 s single-purpose connect token (`POST /widget/v1/realtime-tokens`), never visitor session token in a URL.

## Webhooks in

- Verify provider signatures before parsing body beyond size limit. Reuse SDK verifiers where they exist (build-vs-reuse.md 3.4):
  - SES: SNS message signature (cert host allowlist, `framework/aws/sns`).
  - Postmark: webhook basic auth credentials + IP allowlist optional; Resend: Svix-style `svix-signature` HMAC with timestamp tolerance; Mailgun: HMAC-SHA256 over `timestamp + token` with signing key, token replay set in Valkey; SendGrid: signed Event Webhook ECDSA (`X-Twilio-Email-Event-Webhook-Signature`). Applies to `/webhooks/{provider}/events` (04). Inbound mail at MVP = SES (SNS); provider inbound endpoints later (`extra-mail-adapters`).
  - Slack signing secret (5-minute window, `slack.NewSecretsVerifier`), Discord Ed25519 (interactions only, disgo), Telegram secret token header, WhatsApp `X-Hub-Signature-256`, Stripe (`webhook.ConstructEvent`), GitHub `X-Hub-Signature-256` with per-repository secret (`/webhooks/github/repositories/{ghr}`, 08 shared layer; delivery id dedupe).
- Discord messages arrive over Gateway WebSocket (outbound connection held by one replica via lease, 05), not webhooks. Authenticated by bot token.
- Replay protection: durable provider event ID (`dedupe_key`, `stripe_events`) where spec stores the event; short replay windows (Mailgun token, Slack/Telegram retries) in Valkey `SET NX EX`.
- Respond fast (under 1 second): persist raw event, enqueue job, return 200.

## Privacy and GDPR

- Contact export: `POST /api/v1/contacts/{ctc}/exports` builds JSON + attachments zip, signed URL valid 24 hours.
- Contact erase: `DELETE /api/v1/contacts/{id}?erase=true`: deletes messages bodies, attachments, identities, custom attributes; keeps anonymized conversation shells for metrics (`contact_id` points to tombstone "Deleted contact"). Irreversible, audit logged. Billing ledger keeps conversation id only (no personal data).
- Workspace delete: soft delete, 30-day grace, then hard purge job (database rows + files).
- Data retention setting (optional): auto-erase conversations closed more than N days.
- Every purge, erase, prune job calls `RetentionGuard.MayDestroy` before destroying (contracts.md). MVP noop driver allows all; later legal hold = new driver. Held subject skipped and recorded `retention_held`.
- Data residency: cell model. Each region = separate deployment + database (`app.region`: `self` self-host, `us` cloud; later `eu`). No region column. No hardcoded hostnames in clients (CLI, MCP, mobile, snippets take base URL). `/auth/config` and `/me.instance` expose `region`. UUIDv7 ids globally unique so workspace can move between cells by export/import later.
- Bulk email to contacts (13 changelog, 14 feedback updates): double opt-in where subscription-based (13), RFC 8058 one-click unsubscribe + footer link, no tracking pixels, no open/click tracking, sent only on customer stream via 04 actions.
- Contact sessions (portal, 14): contact-side sessions in `portal_sessions` (never `users`); cookie names, fragment tokens, CSRF in `14-feedback-roadmap/portal.md`. Later support portal reuses.
- Customer source code (08 docs sync): processed in memory only; sent only to workspace BYOK key; never stored, logged, traced or put in job args (`ai_run_steps.request` keeps params, hashes and sizes only); secret-like lines redacted before LLM.
- Imports (16): imported email identities `verified = false` unless admin attests (`email_trust`, audited); imported content sanitized like live input; GDPR erase writes HMAC tombstones so delta sync never re-imports erased contact.
- Sub-processors list for cloud (AWS, Stripe, user-chosen LLM provider). BYOK: Dimmi cloud never sends customer data to an LLM provider the workspace did not configure.

## Audit log

- `audit_logs`: who, what, when, IP, user agent, subject, changes (redacted). `actor_type`, `via_kind` registry enums (accept every `ActorVia.Kind`, incl. `app`). Covers: login, auth factor changes, member/role changes, token create/revoke, channel connect/disconnect, AI key changes, webhook endpoints, exports, erasures, settings, billing.
- API `GET /api/v1/audit-logs`, retention 1 year (cloud), unlimited self-host. Default descending; `sort=created_at` (ascending, stable cursor) + `filter[created_after]` for pull-based SIEM export.
- `AuditSink` contract (noop at MVP) receives every audit entry via queued listener; later webhook, S3 JSONL, Datadog, Splunk drivers per org.

## Abuse (cloud)

- Signup: email verification required before channels go live. Disposable email domains flagged.
- Outbound email reputation: customer stream only (contracts.md "Mail"): per-workspace send limits (entitlement keys) until verified domain; bounce and complaint rates tracked per workspace and per substream (`changelog`, `feedback_update`), auto-pause above SES thresholds (bounce 5%, complaint 0.1%); cloud uses shared provider IP pools, so one workspace's pause never touches system stream or other workspaces (SES tenant per workspace, configuration sets `dimmi-system` / `dimmi-customer`).
- CSAT rating page (15, `/csat-ratings/{svr}`): signed link, per-IP form limits, answers scanned for card numbers/secrets and redacted before storage. Public/hosted survey links later (`hosted-survey-page`).
- Public feedback portal (14): anonymous voting off by default; when on, per-IP daily vote cap + per-contact rate limits; spam moderation queue.
- Spam: inbound classifier hook (heuristics first: links count, known spam patterns, honeypot field in widget, rate limits). AI classifier optional via BYOK.
- Kill switch: `workspaces.suspended_at`.
- Anonymous AI surfaces (help center Ask AI, public MCP, widget agent for anonymous visitors) spend workspace BYOK key: per-IP and per-visitor rate limits (`throttle:public`), per-workspace daily anonymous AI budget (default 20% of monthly AI cap), bot checks before first LLM call, spend cap always enforced.

## Agent safety rules (07, 08, 09)

- `agent_only` knowledge means "not linked or listed", never "secret". Agent may quote it to customers. Ingestion runs secret scanner (API key, token, private key, password patterns); matches block indexing of that chunk and flag source for review.
- MCP write tools: central confirm gate in `app/mcp` middleware, not per tool. Every tool declares `risk` (read, write, send, destructive). `send` and `destructive` tools always require human confirmation (elicitation or `confirm` param); no bypass at MVP. Workspace send modes (`drafts_only`, `direct`) later: `agent-send-modes`. Reason: customer-written text in conversations can steer external agents (prompt injection).
- AI output never triggers tool side effects without policy check; retrieved and customer content always wrapped as untrusted data in prompts.
- Agent actions (07 `actions.md`): identity values bound only from verified context (verified widget session, aligned-DMARC email, trusted attribute keys), never from model output or customer text; every write needs customer click on Dimmi-rendered confirmation card (plus teammate/admin approval when configured; DB CHECK blocks live write without `confirmed_at`); writes never on email at MVP (later: `email-action-confirmations`); MCP never executes, approves or publishes write actions; action results wrapped untrusted, secret-scanned and capped.
- Nothing AI-generated or imported auto-publishes, except a 16 import run where a human explicitly set `articles.publish: preserve` (audited).

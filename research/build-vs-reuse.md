# Build vs reuse audit

Date: 2026-11-02. Scope: every piece the specs (`specs/00-*` to `16-*`) plan to hand-write, checked against maintained open source. Rule from product owner: no code that a strong open source library already provides; write only domain code.

How I checked. Stars, license, last push: `gh api repos/<owner>/<repo>` on 2026-11-02. Versions and dates: `proxy.golang.org/<module>/@latest` on 2026-11-02. Interfaces: read from source (shallow clone of `centrifugal/centrifuge`, sparse clone of `centrifugal/centrifugo/internal`, GitHub contents API). Claims I could not confirm are marked [unverified].

License rule (PO decision 2026-11-02: Dimmi is MIT). Every dependency must be MIT-compatible: MIT, BSD, ISC, 0BSD, Apache-2.0, and MPL-2.0 (file-level copyleft: unmodified use fine; changes to MPL files stay MPL). No GPL, LGPL or AGPL dependencies. Reject cgo, to keep a static single binary.

## TL;DR: top 10 replacements

| # | Planned custom piece | Replace with | Saves |
|---|---|---|---|
| 1 | Pusher-protocol v7 server, hub, presence gossip, `realtime_events` replay, widget Pusher client (00-foundation) | `github.com/centrifugal/centrifuge` v0.40.1 (MIT) embedded. Multi-replica via centrifuge built-in Redis broker + presence on Valkey (Valkey always required, PO 2026-11-02). Widget uni-SSE handler is our own code on the public API. No Centrifugo code ported (PO rejected porting). | Protocol server, presence, history/recovery, fan-out, and the widget protocol client |
| 2 | Discord gateway, sharding, rate-limit buckets, Ed25519 interactions verify (05) | `github.com/disgoorg/disgo` v0.19.6 (Apache-2.0) | `gateway.go`, `shards.go`, `buckets.go`, `interactions.go` verify |
| 3 | LLM wire clients, SSE reader, tool-call delta assembly (07) | Official SDKs: `anthropic-sdk-go` v1.84.0, `openai-go/v3` v3.91.1 (also covers OpenRouter, Ollama, vLLM via base URL), `google.golang.org/genai` v1.77.0, `aws-sdk-go-v2` bedrockruntime | `drivers/llm/internal/sse`, per-driver stream parsers |
| 4 | Web Push VAPID JWT and RFC 8291 encryption (06) | `github.com/SherClockHolmes/webpush-go` v1.4.0 (MIT) | `app/services/webpush` crypto |
| 5 | Slack Web API client, Socket Mode, signature verify (05) | `github.com/slack-go/slack` v0.33.0 (BSD-2) | `slack/client.go`, `socket.go`, signature middleware |
| 6 | Telegram client and long-poller (05) | `github.com/go-telegram/bot` v1.27.0 (MIT, zero deps, Bot API 10.3) | `telegram/client.go`, `poller.go` |
| 7 | Standard Webhooks signing (10) | `github.com/standard-webhooks/standard-webhooks/libraries` (Apache-2.0, `Sign`/`Verify`) | `support/webhooksig/sign.go`, `verify.go` |
| 8 | Language detection with lingua-go (07) | Drop lingua-go: about 1,800 MB in high-accuracy mode, about 110 MB in low-accuracy mode. Use the classifier's `language` output. Optional: `abadojack/whatlanggo` (tiny) | Avoids blowing the 50 MB idle budget |
| 9 | Session CSRF double-submit `XSRF-TOKEN` + `GET /auth/csrf` (00-accounts) | Go 1.25+ `net/http.CrossOriginProtection` | Token cookie, endpoint, middleware |
| 10 | SSRF IP guard in `support/safehttp`, `support/memlimit`, backup chunked AES-GCM, trusted proxy parsing, TOTP, prefixed IDs | `code.dny.dev/ssrf` v0.3.0, `KimMachineGun/automemlimit` v1.0.1, `filippo.io/age` v1.3.2, `realclientip-go` v1.0.0, `pquerna/otp` v1.5.0, `go.jetify.com/typeid` v1.3.0 | Security-sensitive code that needs no domain knowledge |

Also notable: the OAuth server stays custom, because no maintained Go library covers the MCP profile. Even so, metadata and DCR types plus bearer middleware come from the MCP go-sdk `oauthex` and `auth` packages, and the client side uses `golang.org/x/oauth2`.

## 1. Realtime

### Decision: embed `centrifuge`, drop the Pusher protocol

Facts (centrifuge source at commit 2026-10-31, tag v0.40.1):

- **Module:** `github.com/centrifugal/centrifuge`, MIT, 1,495 stars, pushed 2026-11-01. `go 1.26.0`. Pre-1.0: "API may change".
- **Dependencies:** `centrifugal/protocol`, `otter/v2` (already in our stack), `prometheus/client_golang` (already ours), `redis/rueidis`, protobuf, `google/uuid`. Rueidis is linked but unused unless the Redis broker is built. No cgo.
- **Transports in the library:** `NewWebsocketHandler` (its own internal websocket package, JSON or Protobuf), `NewSSEHandler` and `NewHTTPStreamHandler` (bidirectional emulation, needs centrifuge-js), `NewEmulationHandler`, and experimental WebSocket over HTTP/2 (RFC 8441).
- **Unidirectional transports:** uni-SSE, uni-HTTP-stream, uni-WS, uni-gRPC exist as `_examples` in the library. Production handlers live in Centrifugo `internal/unisse` (179-line handler + 108-line transport, Apache-2.0, built only on the public `centrifuge.NewClient` and `ProtocolConnect`).
- **Brokers and presence in the library:** memory and Redis only (`NewMemoryBroker`, `NewRedisBroker`, `NewMemoryPresenceManager`, `NewRedisPresenceManager`, plus Map brokers). Custom implementations plug in via `Node.SetBroker`, `SetPresenceManager`, `SetController`.
- **Postgres broker (superseded: Valkey decision, no porting):** none in the library and no community one found. Centrifugo v6.8+ server has Apache-2.0 Postgres code in non-importable `internal/` packages. `internal/pgstreambroker` (1,953 lines, `var _ centrifuge.Broker`): transactional publish, history with TTL and size, polling outbox, optional LISTEN/NOTIFY, daily partitions. Its support packages are `internal/pgoutbox` (917, LISTEN loop + partitioner), `internal/pgshard` (179) and `internal/pgschema` (390). `internal/controllers/controller_postgres.go` (635) implements `centrifuge.Controller`. `internal/pgmapbroker` implements `MapBroker`. Centrifugo labels these "new and evolving". It claims "low single-digit milliseconds with `use_notify`". There is no Postgres PresenceManager (Centrifugo presence is memory or Redis).
- **NATS:** Centrifugo `internal/natsbroker` (Broker + Controller, no history, Apache-2.0) and library example `_examples/custom_broker_nats`. No maintained importable module.
- **Interfaces the adapter must implement** (verbatim method sets, v0.40.1):
  - `Broker`: `RegisterBrokerEventHandler(BrokerEventHandler) error`, `Subscribe(channels ...string) error`, `Unsubscribe(channels ...string) error`, `Publish(ch string, data []byte, opts PublishOptions) (PublishResult, error)`, `PublishJoin(ch string, info *ClientInfo) error`, `PublishLeave(ch string, info *ClientInfo) error`, `History(ch string, opts HistoryOptions) ([]*Publication, StreamPosition, error)`, `RemoveHistory(ch string) error`.
  - `BrokerEventHandler` (called by the adapter): `HandlePublication(ch, pub, sp, useDelta, prevPub)`, `HandleJoin(ch, info)`, `HandleLeave(ch, info)`.
  - `PresenceManager`: `Presence(ch) (map[string]*ClientInfo, error)`, `PresenceStats(ch) (PresenceStats, error)`, `AddPresence(ch, clientID string, info *ClientInfo) error` (must expire entries not touched within TTL), `RemovePresence(ch, clientID, userID string) error`.
  - `Controller` (needed for multi-node: node discovery, cross-node disconnect, unsubscribe, surveys): `RegisterControlEventHandler(ControlEventHandler) error`, `PublishControl(data []byte, nodeID, shardKey string) error`. When the broker also implements `Controller`, `SetBroker` installs it as controller.
- **Client SDKs:** centrifuge-js 5.8.0 is 98.6 KB min, **23.2 KB gzip** (bundlephobia). For comparison, pusher-js 8.6.0 is 17.6 KB gzip. Mobile: centrifuge-swift (MIT), centrifuge-java (MIT, Android), centrifuge-dart (MIT, Flutter), centrifuge-go (MIT, useful for the CLI `dimmi listen`).
- **Memory per connection:** Centrifugo reports about 27 KB RSS per idle WebSocket (200k connections, 5.4 GB, two goroutines per connection, 2026-08) plus about 4 KB counted only by the cgroup. The 2020 million-connection test measured about 30 KB. The FAQ estimates 30 to 50 KB. At 1,000 sockets that is about 30 to 35 MB on top of idle, inside the 150 MB budget (50 idle + about 35 + headroom). Keep the CI load test (`00-foundation/tasks.md`) as the gate.

Alternatives (brief):

| Option | Verdict | Why |
|---|---|---|
| Own Pusher v7 server on coder/websocket (current spec) | reject | Re-implements protocol, presence, auth, recovery. Laravel-echo compatibility is the only gain |
| Centrifugo sidecar | reject | Second process; breaks one-binary rule |
| Mercure (dunglas/mercure, AGPL-3.0, 5.4k) | reject | AGPL; SSE-only; hub designed as separate server |
| soketi (Node, AGPL-3.0, last push 2025-03) | reject | Node, AGPL, stale |
| NATS WebSocket (nats-server Apache-2.0) | reject as client transport | Needs server process; no channel auth model fit. Keep as optional broker later |
| Laravel Reverb | n/a | PHP |

### Spec changes for realtime

| Topic | New rule |
|---|---|
| Endpoints | `GET /connection/websocket` (centrifuge `NewWebsocketHandler`), `GET /connection/uni_sse` (widget, own small handler on centrifuge public API (`NewClient`, `ProtocolConnect`)). Remove `/app/{key}` and `POST /broadcasting/auth` |
| Channel names | Centrifuge namespaces with `:`: `workspace:{ws_}`, `conversation:{conv_}`, `member:{mem_}`, `widget_contact:{ctc_}`. Presence is a channel option (`EmitPresence`, `EmitJoinLeave`), not a `presence-` prefix. Collapse `presence-workspace.*` into `workspace:` and `presence-conversation.*` into `conversation:` with presence on |
| Connect auth | `node.OnConnecting` reads credentials from the HTTP request (native auth). Dashboard: session cookie on same origin (WebSocket upgrade carries cookie; check `Origin`). Mobile app, CLI and integrations: bearer token in connect `token` field, resolved by the same guard as REST. Widget: short-lived connect token (60 s, single purpose) from new `POST /widget/v1/realtime-tokens`, because EventSource cannot send headers and the token sits in the URL. Never put the visitor session token in a URL |
| Subscribe auth | `client.OnSubscribe` calls `routes/channels.go` authorizers (Laravel `Broadcast::channel` shape kept). Widget gets server-side subscriptions in `ConnectReply.Subscriptions` (uni transports cannot subscribe) |
| Typing | Dashboard and app: client publications on `conversation:` via `client.OnPublish` (members only, server throttle 1 per 2 s, `HistorySize` 0 = ephemeral). Widget: existing `POST /widget/v1/conversations/{id}/typing-indicators` (REST, uni transport is receive-only) |
| Presence | centrifuge PresenceManager. Single node: memory. Multi-replica: Postgres PresenceManager (new, about 4 methods on an UNLOGGED table with `expires_at`, sweeper) or Redis. Member `online` rule unchanged |
| Resume | Replace `realtime_events` table and `last_event_id` replay with centrifuge history + recovery. Per-channel `HistorySize` and `HistoryTTL` (example 300 publications, 10 min), client sends `offset` + `epoch`, server sets `recovered` or tells client to refetch. Ephemeral events published without history. Dedupe by event `id` stays |
| Fan-out | Centrifuge built-in Redis broker + Redis presence manager on Valkey, every deployment (single node included). No custom or ported broker. |
| Widget client | Plain `EventSource('/connection/uni_sse?cf_connect=' + encodeURIComponent(JSON.stringify({token, subs:{[ch]:{recover:true, offset, epoch}}})))`. Parse push frames (JSON), track `offset`/`epoch`. On `error`, close and reopen with a fresh token and the last offset (EventSource auto-retry would reuse the expired URL). About 1 KB gz. Remove the hand-written Pusher client and the long-poll fallback (SSE passes most proxies; keep `update_controller.go` polling only if a proxy blocks streaming) |
| Dashboard client | centrifuge-js (23.2 KB gz, fine for SPA). Drop laravel-echo, pusher-js, `@laravel/echo-react` |
| Kick on member removal | `node.Disconnect(userID)` (works across nodes via Controller) |
| Metrics | centrifuge Prometheus metrics registered on admin port |

## 2. OAuth 2.1 authorization server (09)

| RFC / need | fosite (Apache-2.0) | zitadel/oidc `pkg/op` (Apache-2.0) | MCP go-sdk `oauthex` + `auth` |
|---|---|---|---|
| Auth code + PKCE S256 | yes | yes | n/a (types only) |
| Refresh rotation + reuse detection | rotation yes; "automatic revocation of derived tokens if abuse is detected" | refresh yes; reuse detection not stated | n/a |
| Device flow RFC 8628 | yes (`handler/rfc8628`) | yes (`device.go`) | n/a |
| Revocation RFC 7009 | yes (`revoke_handler.go`) | yes (`token_revocation.go`) | n/a |
| Introspection RFC 7662 | yes | yes | n/a |
| DCR RFC 7591 | no | no | request/response types (`dcr.go`) |
| CIMD | no | no | no [unverified for 2026-11 main] |
| AS metadata RFC 8414 | no builder | OIDC Discovery only | `AuthServerMeta` type |
| PRM RFC 9728 | no | no | `ProtectedResourceMetadata` + `ProtectedResourceMetadataHandler` + `ParseWWWAuthenticate` |
| Resource indicators RFC 8707 | `audience` param, not `resource` | no | `MatchesResource` helper |
| Opaque tokens | yes | JWT-centric, needs signing keys | n/a |
| Maintenance | last release v0.49.0 2024-12-12, last commit 2025-07-03 | v3.51.16 2026-10-15, active | v1.8.0 2026-09-04, active |

Decision: **keep the AS core custom**. The domain is opaque `dmi_oat_` tokens, workspace grants, role clamp, first-party rules, CIMD, DCR and RFC 8707. fosite is stalled upstream and lacks DCR, CIMD and 8707. zitadel/oidc is OIDC-centric (its `op.Storage` requires key sets and ID-token plumbing) and lacks DCR, CIMD and 8707. Ory Hydra is a separate server.

Reuse inside the custom AS:
- `github.com/modelcontextprotocol/go-sdk/oauthex`: `AuthServerMeta`, `ProtectedResourceMetadata`, `ClientRegistrationMetadata`/`Response`/`Error`, `MatchesResource`. `go-sdk/auth`: `RequireBearerToken`, `ProtectedResourceMetadataHandler`, `TokenVerifier`. Replaces `support/oauth/metadata.go`, most of `resources.go`, and `WWW-Authenticate` builders in `app/mcp/auth.go`.
- `golang.org/x/oauth2` v0.37.0 (BSD-3) on the client side. `Config.DeviceAuth` + `DeviceAccessToken` (RFC 8628 client) replace `app/client/auth/device_flow.go`. `GenerateVerifier` + `S256ChallengeOption` + `VerifierOption` replace PKCE code in the GitHub and Google login clients. Server-side S256 verify is one line of `crypto/sha256` (keep).
- Token random + hash + constant-time compare: stdlib (keep, tiny).

## 3. Area tables

Columns: planned custom piece | spec location | decision | library + version + license | why | memory/size note | spec edits.

### 3.1 Platform and foundation

| Piece | Spec | Decision | Library | Why | Mem/size | Spec edits |
|---|---|---|---|---|---|---|
| SSRF guard (`support/safehttp`) | 00-foundation security.md "Outbound HTTP" | reuse core, keep wrapper | `code.dny.dev/ssrf` v0.3.0 (MIT, 2026-07) | IANA special-purpose deny lists, `net.Dialer.Control` checks the resolved IP at connect (covers DNS rebinding and every redirect hop). Keep thin wrapper for timeouts, size caps, redirect count, `allow_hosts` (map to `WithAllowedV4Prefixes`/`WithPorts` per host:port) | tiny | security.md Outbound HTTP: name library; conventions "Every outbound HTTP via `support/safehttp`" add "(dialer Control = code.dny.dev/ssrf)" |
| Prefixed public IDs (`support/ids`) | conventions Database rules; data-model.md | reuse | `go.jetify.com/typeid` v1.3.0 (Apache-2.0; spec repo jetify-com/typeid 3.7k stars) | TypeID is exactly prefix + `_` + 26-char lowercase Crockford base32 of UUIDv7 | tiny | conventions "Public IDs": "encoding = TypeID spec"; foundation tasks `support/ids` becomes prefix registry + typeid calls |
| Config | conventions | already reuse | `knadh/koanf/v2` v2.4.1 (MIT) | | | none |
| CLI | conventions | already reuse | `spf13/cobra` v1.10.2 (Apache-2.0) | | | none |
| Migrations + advisory lock (`support/migrator`) | 11 README, foundation tasks | reuse lock | `pressly/goose/v3` v3.29.0 (MIT) `Provider` + `lock.NewPostgresSessionLocker` | goose ships the Postgres advisory session locker | | 11 tasks `support/migrator`: drop own lock; keep skew guard, dry-run planner, lint |
| Jobs, cron, leader | conventions | already reuse | `riverqueue/river` v0.51.0 (MPL-2.0) | | | none |
| Cache | conventions | already reuse | Valkey via `redis/rueidis`: `rueidisaside` (Remember: cross-process stampede protection + server-assisted client-side caching), `rueidislock` (auto-extending locks), same rueidis repo/version; `maypok86/otter/v2` v2.4.0 (Apache-2.0) only for per-process L1 caches specs ask for (updated 2026-11-02) | no own lock, no own memory store, no cache fake: tests on real Valkey | | framework.md `cache` row |
| Rate limiter (`contracts.RateLimiter`) | foundation tasks, api.md | reuse | `go-chi/httprate` v0.16.1 (MIT) with in-memory counter. For multi-replica, a custom `LimitCounter` (4 methods) on the UNLOGGED `rate_limits` table (superseded 2026-11-02: framework `ratelimit` on Valkey via `rueidis/rueidislimiter`, local mode on `golang.org/x/time/rate`; no Postgres table) | sliding window, Laravel-like headers, key funcs | tiny | foundation tasks RateLimiter line |
| Request validation | conventions Request | reuse + own | `danielgtaylor/huma/v2` v2.40.0 built-in JSON Schema validation; own `validate:"..."` rule engine in `framework/validation` (updated 2026-11-02: go-playground/validator removed, its reflection disables linker dead method elimination) | same tag syntax, Laravel-style English messages via i18n | own engine small | conventions: state "validation tags = huma schema tags" |
| i18n (`translator`, `lang/en/*.json`) | foundation tasks | reuse x/text + own loader | `golang.org/x/text` (`language` matching, `feature/plural` CLDR rules) + own JSON loader, `{{.name}}` placeholders (updated 2026-11-02: nicksnyder/go-i18n removed, text/template disables dead method elimination) | CLDR plurals, JSON files, fallback | small | foundation tasks "translator" line |
| Envelope encryption drivers (KeyManager local, AWS KMS) | security.md Secrets, conventions | reuse Tink + own thin KEK drivers | `tink-crypto/tink-go/v2` keysets (one per scope + purpose); `local` KEK from app key; `awskms` own thin driver on aws-sdk-go-v2 `kms` (updated 2026-11-02: gocloud.dev/secrets removed for size; tink-go-awskms needs SDK v1) | per-scope keysets + `encryption.Sealed` stay ours | links only imported drivers | security.md Secrets; foundation tasks KeyManager |
| Filesystem drivers (local, s3) | conventions Storage, foundation tasks | reuse SDK | `aws-sdk-go-v2` `s3` (+ transfermanager, presign) and stdlib `os.Root` for `local` (updated 2026-11-02: gocloud.dev/blob removed, 63.9 to 36.0 MB with the other removals); own `filesystem/drivertest` | signed URLs, multipart | | foundation tasks Filesystem line |
| Signed URLs, cursor pagination, idempotency store | api.md, foundation tasks | keep custom | stdlib HMAC, Postgres table | no library fits; each about 50 to 100 lines | | none |
| Clock | conventions | keep custom | | 20 lines | | none |
| Metrics, traces | foundation README | already reuse | `prometheus/client_golang` v1.25.0, `go.opentelemetry.io/otel` v1.47.0 (Apache-2.0) | | | none |
| GOMEMLIMIT from cgroup (`support/memlimit`) | 11 README, tasks | reuse | `github.com/KimMachineGun/automemlimit` v1.0.1 (MIT) | cgroup v1/v2 detection done | tiny | 11 code map + tasks |
| Trusted proxy (`support/trustedproxy`) | 11 README | reuse | `github.com/realclientip/realclientip-go` v1.0.0 (0BSD) | rightmost-trusted-range strategy, CIDR allowlist | tiny | 11 code map |
| sd_notify | 11 packaging.md | reuse | `github.com/coreos/go-systemd/v22/daemon` v22.7.0 (Apache-2.0, pure Go) | | tiny | packaging.md "sd_notify implemented in-process (tiny, no lib)" |
| Backup encryption (`support/backupfmt` chunked AES-256-GCM) | 11 README, tasks | reuse | `filippo.io/age` v1.3.2 (BSD-3) STREAM (ChaCha20-Poly1305 chunks), passphrase or X25519 recipient + stdlib `archive/tar` | audited format, streaming, CLI decrypt by users with `age` | constant memory | 11 code map + tasks; backup docs |
| `dimmi doctor`, settings registry | 11 | keep custom | | domain | | none |
| Tests: embedded Postgres | conventions Testing | change | `fergusstrange/embedded-postgres` v1.34.0 binaries lack pgvector (README: extensions live in "caller-owned distributions"). Use `pgvector/pgvector:pg18` from root `compose.yaml` via `DATABASE_URL` (updated 2026-11-02: testcontainers-go not used; services started once by `make services`, tests read the app's normal env vars) | pgvector, citext, pg_trgm required by migrations | | conventions Testing; foundation tasks "Test harness"; tech-stack.md |
| Test helpers (`AssertJSONPath`) | conventions | reuse | `tidwall/gjson` v1.20.0 (MIT), `stretchr/testify` (MIT) | keep Laravel-style helper names as wrappers | | none |
| OpenAPI diff | api.md | already reuse | `oasdiff/oasdiff` v1.35.0 (Apache-2.0) | | | none |
| CLI API client | 09 | already reuse | `oapi-codegen/v2` v2.8.2 (Apache-2.0), `zalando/go-keyring` v0.2.8 (MIT) | | | none |
| Feature flags / entitlements | extensibility.md | keep custom | | OpenFeature needs a provider; catalog is domain | | none |
| Audit log | security.md | keep custom | | domain | | none |

### 3.2 Auth (00-accounts, 02, 09)

| Piece | Spec | Decision | Library | Why | Mem/size | Spec edits |
|---|---|---|---|---|---|---|
| Password hashing | 00-accounts `support/auth/password_hasher.go` | reuse + thin | `golang.org/x/crypto/argon2` + `alexedwards/argon2id` v1.0.0 (MIT) for PHC encode/decode and params (`NeedsRehash`). Semaphore: `golang.org/x/sync/semaphore` | | argon2 memory per hash = OWASP params; semaphore caps it | 00-accounts README support line |
| TOTP | 00-accounts `support/auth/totp.go` | reuse | `pquerna/otp` v1.5.0 (Apache-2.0, 3k stars) `totp.ValidateCustom`, `totp.Generate` (otpauth URL), `Key.Image()` for QR if API returns one | RFC 6238 vectors pass upstream; keep replay-step check (domain) | small | 00-accounts README + tasks; QR: return `otpauth_url` and render QR in dashboard, or `Key.Image()` |
| Recovery codes, API token format (base62 + CRC32 + prefix) | 00-accounts | keep custom | | 20 lines each, product format | | none |
| WebAuthn (post-MVP) | 00-accounts Out | reuse later | `go-webauthn/webauthn` v0.18.2 (BSD-3) | | | none |
| Sessions | 00-accounts `sessions` table | keep custom | (not `alexedwards/scs`) | session list per user, `kind: app` rows, revoke others, device labels. scs stores opaque blobs with no per-user listing | | none |
| CSRF double-submit | security.md; 00-accounts `verify_csrf_token.go`, `GET /api/v1/auth/csrf`, edge cases 26-27 | replace | `net/http.CrossOriginProtection` (Go 1.25+) | Sec-Fetch-Site, falls back to Origin vs Host; requests without either are treated as non-browser | none | security.md "CSRF"; 00-accounts api.md (remove `auth.csrf.show`), flows.md step 1, edge-cases 26-27, middleware list; conventions `auth:session` row; foundation tasks guard line; rest.md exceptions if it lists the route |
| Common passwords bloom | 00-accounts `common_passwords.go` | reuse | `bits-and-blooms/bloom/v3` v3.7.1 (BSD-2) | | about 120 KB vs about 2 MB map | 00-accounts README |
| HIBP range client | 00-accounts `services/hibp` | keep custom | (`wneessen/go-hibp` exists, 13 stars) | one GET with `Add-Padding`; low-adoption lib not worth it | | none |
| GitHub / Google login | 00-accounts `services/github/oauth_client.go`, `services/google/oidc_client.go` | reuse | `golang.org/x/oauth2` v0.37.0 (`github.Endpoint`, PKCE helpers), `coreos/go-oidc/v3` v3.21.0 (Apache-2.0) | authorize URL, exchange, PKCE, id_token verify | | 00-accounts README services line |
| Widget identity JWT verify (`support/jwtverify`) | 02 README code map | reuse core, keep wrapper | `github.com/go-jose/go-jose/v4` v4.1.7 (Apache-2.0), already transitive via go-oidc | `ParseSigned(token, []SignatureAlgorithm{...})` forces an alg allowlist; `jose.JSONWebKey` imports JWK/PEM; `jwt.Claims.ValidateWithLeeway`. Keep wrapper for duplicate-key rejection, `crit`, lifetime cap, `hints.go` (domain) | one JOSE lib in tree | 02 code map `jwtverify/` description; 02 tasks first line |
| SAML (post-MVP) | 00-accounts Out | reuse later | `crewjam/saml` v0.5.1 (BSD-2) | | | none |
| User-Agent parse | 00-accounts `device.go`, 03 `support/useragent`, 09 `first_party.go` | reuse | `medama-io/go-useragent` v1.2.5 (MIT; trie, 0 allocs, about 287 ns/op) | uap-go regex set is heavier at boot and per call; enough for "Firefox on Linux" labels | small | 03 README (`support/useragent`, "uap-core regexes embedded" line 127), 00-accounts device.go. Keep the 09 `Dimmi/<ver>` UA parse (own format) |
| GeoIP | 00-accounts config `auth.geoip.database_path` | reuse | `oschwald/maxminddb-golang/v2` v2.9.0 (ISC) | mmap, optional file | mmap, not heap | 00-accounts config row: name library |
| Turnstile verify | 00-accounts | keep custom | | one POST | | none |
| Widget PoW (`support/pow`) | 01 README + tasks | reuse server | `altcha-org/altcha-lib-go` v1.0.0 (MIT): HMAC-signed SHA-256 challenge, same design as spec | widget implements ALTCHA solver (about 30 lines, no ALTCHA web component). Single-use tracking stays ours | tiny | 01 code map + tasks; sdk-contract if it names the challenge shape |
| Origin normalize (`support/origins`) | 01 | keep custom | `golang.org/x/net/idna` + `net/url` | wildcard rule is product-specific | | none |

### 3.3 MCP and agent-native (09)

| Piece | Spec | Decision | Library | Why | Spec edits |
|---|---|---|---|---|---|
| MCP server | 09 mcp.md, README | already reuse | `modelcontextprotocol/go-sdk` v1.8.0 (2026-09-04; MIT moving to Apache-2.0 per LICENSE). Release notes: 2026-07-28 "remains the newest version the SDK negotiates"; v1.7.0 accepts 2026-07-28 only when `StreamableHTTPOptions.Stateless = true`; backward compatible with 2025-11-25 and earlier; `server/discover`; MRTR replaces server-to-client requests | stateless + legacy shim in SDK; keep `http_handler.go` thin | 09 README "v1.7+" becomes "v1.8.0"; mcp.md line 3 same; tech-stack.md license note |
| mark3labs/mcp-go | | reject | v1.5.0 (MIT, 9.2k) | official SDK covers 2026-07-28; one SDK only | none |
| OAuth AS | 09 | keep core (section 2) | go-sdk `oauthex`/`auth`, `x/oauth2` | | 09 README OAuth code map: `metadata.go` and `resources.go` use `oauthex`; `pkce.go` stays; `app/mcp/auth.go` uses `auth.RequireBearerToken`; `app/client/auth/device_flow.go` uses `x/oauth2`; contradiction 8 add fosite/zitadel rationale |
| JSON Schema from Go types | 09 `app/mcp/schema.go` | already reuse | huma registry | | none |
| Confirm gate request state (XChaCha20-Poly1305 + HKDF) | 09 | keep custom | `golang.org/x/crypto/chacha20poly1305`, `hkdf` | 30 lines on x/crypto | none |
| Keychain | 09 | already reuse | `zalando/go-keyring` | | none |

### 3.4 Webhooks out (10) and inbound signatures

| Piece | Spec | Decision | Library | Why | Spec edits |
|---|---|---|---|---|---|
| Standard Webhooks v1 sign/verify | 10 `support/webhooksig/{sign,verify}.go` | reuse | `github.com/standard-webhooks/standard-webhooks/libraries` v0.0.1 (Apache-2.0) `NewWebhook`, `Sign(msgID, ts, payload)`, `Verify` | same lib the CI compatibility job checks; multi-signature header = join `Sign` outputs with spaces (keep) | 10 code map + tasks line 7 (keep `secret.go`) |
| Standard Webhooks v1a (Ed25519) | none at MVP (old status page subscribers removed) | not built | | `framework/webhooks` stays only signer, v1 HMAC | none |
| Inbound provider signatures (Slack, Meta, Telegram, Stripe, Discord, GitHub) | security.md Webhooks in; 05 middleware; 08 GitHub dispatcher | reuse where SDK has it | Slack: `slack.NewSecretsVerifier`; Stripe: `stripe-go/webhook.ConstructEvent`; Discord: disgo `httpserver` Ed25519; GitHub: go-github `ValidatePayload` (per-repo secret); Meta and Telegram: keep (HMAC / header compare, 5 lines) | | 05 middleware list |
| SNS message signature (SES) | 04 `sns/verify.go` | keep custom | (`robbiet480/go.sns` 37 stars, 2023) | no maintained lib; x509 + cert host check is about 80 lines | none |

### 3.5 Email (04, 13)

| Piece | Spec | Decision | Library | Why | Mem/size | Spec edits |
|---|---|---|---|---|---|---|
| MIME parse | 04 `parser.go` | already reuse | `jhillyerd/enmime/v2` v2.5.0 (MIT) | spec must say v2 | streaming caps ours | 04 code map: `enmime/v2` |
| Build + send | 04 `composer.go`, Mailer smtp | already reuse | `wneessen/go-mail` v0.8.1 (MIT) | `go-mail-middleware` (DKIM) archived, so keep DKIM via go-msgauth | | none |
| DKIM sign/verify, DMARC, Authentication-Results | 04 `auth.go`, `dkim/signer.go` | already reuse | `emersion/go-msgauth` v0.7.0 (MIT, 2025-04) | thin bus factor (risk) | | none |
| SPF | 04 `auth.go` | already reuse | `blitiri.com.ar/go/spf` v1.6.0 (2026-08) [license not checked] | | | verify license before merge |
| IMAP | 04 (later: `imap-adapter`) | deferred | `emersion/go-imap/v2` v2.0.0-beta.8 (2025-12) when built | still beta (risk); not at MVP (inbound = SES receiving) | | none |
| SMTP receive | 04 (removed) | dropped | none | PO decision: no SMTP receiver; inbound via SES receipt at MVP (IMAP later: `imap-adapter`; provider inbound later: `extra-mail-adapters`). `emersion/go-smtp` not a dependency (depguard ban) | | none |
| Reply/quote stripping | 04 `quotes.go`, pipeline.md 4 | keep custom (port) | none maintained: `teamwork/mailstrip` archived 2016, `recapco/emailreplyparser` 2015, `web-ridge/email-reply-parser` 13 stars, no license; talon is Python | port GitHub `email_reply_parser` (MIT) with attribution + own `quote_headers.yaml` | | pipeline.md: add attribution note |
| HTML to Markdown | 04 `htmlbody.go`, 08 crawl | reuse | `JohannesKaufmann/html-to-markdown/v2` v2.5.2 (MIT) | | | name version in 04 + 08 |
| HTML sanitize | 04, 08 | already reuse | `microcosm-cc/bluemonday` v1.0.27 (BSD-3) | low churn (risk) | | none |
| DSN (RFC 3464), ARF (RFC 5965) parse, bounce classify | 04 `dsn.go`, `arf.go` | keep custom | none in Go | parse header blocks with `net/textproto` on enmime parts; RFC 3463 status table | | none |
| Subject prefixes, auto-reply classifier, form relays, loop header | 04 | keep custom | | domain heuristics | | none |
| Email HTML for changelog (`support/emailhtml`, inline styles) | 13 email.md | keep custom (templ) | | MJML needs Node; keep templ + small CSS inliner [no maintained pure-Go MJML found] | | none |

### 3.6 Chat channels (05)

| Piece | Spec | Decision | Library | Why | Mem/size | Spec edits |
|---|---|---|---|---|---|---|
| Slack Web API, Socket Mode, events types | 05 `slack/{client,socket,inbound}.go` | reuse | `slack-go/slack` v0.33.0 (BSD-2, 5k, 2026-10-30) incl. `socketmode`, `slackevents` | | | 05 code map: client/socket wrap slack-go; keep `format.go` (Markdown to mrkdwn), `manifest.go` |
| Discord REST, gateway, sharding, rate limits, interactions | 05 `discord/{client,gateway,shards,buckets,interactions}.go` | reuse | `disgoorg/disgo` v0.19.6 (Apache-2.0, 2026-06; repo pushed 2026-11-01): `gateway` (zstd-stream, zlib-stream), `sharding` ShardManager, REST rate limiter, `httpserver` Ed25519 | discordgo (BSD-3) last release 2025-05, weaker sharding | gateway compression via `klauspost/compress` | 05 code map; providers.md section 2 (gateway, close codes still ours). Keep lease ownership of shard IDs per replica (configure ShardManager with owned shard IDs). Do not import `disgo/voice` (pulls cgo `godave`) |
| Telegram | 05 `telegram/{client,poller}.go` | reuse | `go-telegram/bot` v1.27.0 (MIT, zero deps, Bot API 10.3, `WithHTTPClient` for safehttp, webhook handler, getUpdates) | matches providers.md Bot API 10.3 | tiny | 05 code map; keep `entities.go` (UTF-16 offsets for Markdown conversion) and `format.go` |
| WhatsApp Cloud API | 05 `whatsapp/*` | keep custom | (`piusalfred/whatsapp` v0.1.12, "alpha", 111 stars) | too immature; Graph API surface used is small (messages, media, templates, phone registration) | | none |
| Markdown to provider formats, splitter | 05 `integrations/channels/markdown_ast.go`, `splitter.go` | keep custom on goldmark AST | | domain | | none |

### 3.7 Notifications (06)

| Piece | Spec | Decision | Library | Why | Spec edits |
|---|---|---|---|---|---|
| Web Push (RFC 8030/8291/8292) | 06 delivery.md, `app/services/webpush` | reuse | `SherClockHolmes/webpush-go` v1.4.0 (MIT; deps `golang-jwt/jwt/v5`, `x/crypto`). `Options`: `HTTPClient` (inject safehttp), `TTL`, `Urgency`, `Topic`, `RecordSize`, `VapidExpiration` (default now + 12 h), `Subscriber` | exactly the spec's cross-check reference | 06 delivery.md line 33 ("Implemented in app/services/webpush (fork-free...)") becomes "via webpush-go"; tasks line 54; README code map `webpush/` = thin wrapper (error mapping, JWT reuse not needed) |
| APNs (post-MVP) | 06 | reuse later | `sideshow/apns2` v0.25.0 (MIT, 2024-10) token auth | | 06 README Out note |
| FCM (post-MVP) | 06 | reuse later | `firebase.google.com/go/v4/messaging` v4.22.0 (Apache-2.0); check binary delta in CI | | 06 README Out note |
| Quiet hours, burst, suppression | 06 | keep custom | | domain | none |

### 3.8 LLM (07, 08)

| Piece | Spec | Decision | Library | Why | Spec edits |
|---|---|---|---|---|---|
| OpenAI, OpenRouter, openaicompat drivers | 07 llm-contract.md Drivers | reuse | `openai/openai-go/v3` v3.91.1 (Apache-2.0): `option.WithBaseURL`, `option.WithHTTPClient`, `option.WithMaxRetries(0)`, streaming + `ChatCompletionAccumulator`, extra headers | removes the hand SSE reader and tool-delta assembly for these three | llm-contract.md: drivers wrap SDK; remove `internal/sse` (keep only if a compat server breaks the SDK parser) |
| Anthropic | 07 | reuse | `anthropics/anthropic-sdk-go` v1.84.0 (Apache-2.0): streaming, `Message.Accumulate`, `cache_control` | | same |
| Gemini | 07 | reuse | `google.golang.org/genai` v1.77.0 (Apache-2.0) | | same |
| Bedrock | 07 | already reuse | `aws-sdk-go-v2/service/bedrockruntime` | | none |
| Voyage embeddings | 08 knowledge.md | keep custom | no official Go SDK | one POST | none |
| Multi-provider libs | | reject | bifrost v1 (Apache-2.0, gateway shape), langchaingo (MIT, heavy tree), eino | BYOK per-request keys, safehttp, usage mapping, typed errors are ours anyway; extra abstraction layer | none |
| Tokenizer | 07 usage estimation | no library | (`pkoukk/tiktoken-go` OpenAI-only, embeds BPE files) | use provider-reported usage; rune/3.5 estimate stays | none |
| Circuit breaker | 07 `support/llmretry/breaker.go` | reuse | `sony/gobreaker/v2` v2.4.0 (MIT) | state machine done; retry policy (budgets, fallback) stays domain | 07 code map |
| Backoff with jitter | 07 retry.go | reuse | `cenkalti/backoff/v5` (MIT) | | 07 code map |
| Language detect | 07 pipeline.md, `support/langdetect` | replace | drop `pemistahl/lingua-go` (README: about 1,800 MB high accuracy all languages, about 110 MB low accuracy). Use classifier language; optional `abadojack/whatlanggo` v1.0.1 (MIT, 2019, tiny trigram tables) | memory budget | 07 pipeline.md line 102, README `support/langdetect`, tasks line 58, edge-cases 47 and 68 |
| PII redaction, prompts, link guard, office hours, price table | 07 | keep custom | | domain | none |
| Markdown chunker | 08 `knowledge/chunk.go` | keep custom on goldmark AST | (langchaingo `textsplitter` drags whole langchaingo) | heading-path aware chunks are product logic | none |

### 3.9 Help center, knowledge, docs sync, changelog (08, 13)

| Piece | Spec | Decision | Library | Why | Spec edits |
|---|---|---|---|---|---|
| Markdown dialect | 08 `support/markdown` | already reuse + extensions | `yuin/goldmark` v1.8.7 (MIT) GFM, `parser.WithAutoHeadingID`; `yuin/goldmark-highlighting/v2` (MIT) + `alecthomas/chroma/v2`; `go.abhg.dev/goldmark/toc` v0.12.0 (BSD-3) | | 08 code map `toc.go` uses goldmark-toc |
| GitHub alerts `> [!NOTE]` | 08 dialect.go | keep custom | extensions found have 0-1 stars | about 100 lines AST transformer | none |
| Front matter | 08 knowledge git/upload sources via framework `markdown.FrontMatter` | reuse | `go.abhg.dev/goldmark/frontmatter` v0.3.0 (BSD-3) | YAML + TOML, goldmark-native | 08 code map |
| Line/word diff | framework `support/diff` (08 `app/support/textdiff` hunk rules on top) | already reuse | `aymanbagabas/go-udiff` v0.4.1 (BSD-3), `sergi/go-diff` v1.4.0 (MIT) | | none |
| diff3 merge | 08 (later: `review-rebase`) | deferred | `epiclabs-io/diff3` (MIT, 17 stars) candidate when built | no 3-way rebase at MVP | none |
| Slug | 08 `support/slug`, 14 `feedback/slug.go` | reuse | `gosimple/slug` v1.15.0 (MPL-2.0, transliteration) | | 08 + 14 code maps |
| Safe unzip | 08 `support/ziputil` | keep custom on stdlib | `archive/zip` + `filepath.IsLocal` + size/ratio caps | 40 lines; no lib needed | none |
| Readability extraction | 08 `crawl/extract.go` | reuse successor | `codeberg.org/readeck/go-readability/v2` v2.1.3 (MIT, 2026-09). `go-shiori/go-readability` archived 2025-12-30 and points here | | 08 code map + any go-shiori mention |
| Crawler | 08 `support/crawl` | keep River job-per-page | reject `gocolly/colly` (Apache-2.0, 25k) | colly owns queue/storage in-process; Dimmi needs durable River jobs, safehttp, progress rows | none |
| robots.txt | framework `httpclient/robots` (08 crawl, 16 `public_site`) | reuse | `temoto/robotstxt` v1.1.2 (MIT; release 2021, repo pushed 2026-05; used by colly) | keep RFC 9309 test vectors as our test; 5xx = disallow rule is ours | 08 code map + tasks line 74 |
| Sitemap parse + generate | 08 crawl, sitemap controller | keep stdlib | `encoding/xml`, `compress/gzip` | two structs | none |
| Politeness limiter | 08 | reuse | `golang.org/x/time/rate` per host (framework `ratelimit` local mode) | | none |
| Custom domain TLS | framework `customdomain` (08 wires authorizer) | already reuse | `caddyserver/certmagic` v0.25.6 (Apache-2.0) embedded; keep `caddyask` driver for external Caddy | | none |
| certmagic Postgres storage | framework `customdomain` storage, table `certmagic_storage` (framework migration set) | keep custom | `yroc92/postgres-storage` has no license, 2023 | `certmagic.Storage` + `Locker` about 150 lines | none |
| Feeds RSS (Atom/JSON Feed deferred: later `atom-json-feeds`) | framework `view/feed` (13 public site) | reuse | `gorilla/feeds` v1.2.0 (BSD-3) | | 13 code map |
| OG card PNG | 13 `support/ogimage` layout on framework `image` | keep custom on x/image | `golang.org/x/image/font/opentype` + `x/image/draw` + embedded font | layout is product; no cgo | none |
| Avatar/asset re-encode, dimension guard | framework `image` (security.md uploads, 00-accounts, 08 assets, 15 engage assets) | stdlib + small libs | `image/*`, `golang.org/x/image/{webp,draw}`, `image.DecodeConfig`, `bep/imagemeta` (EXIF orientation applied before metadata strip), `sergeymakinen/go-ico` (ICO, largest icon) | reject `bimg`/`govips` (cgo libvips); `disintegration/imaging` last release 2019; AVIF refused (`ErrUnsupportedFormat`: pure-Go decoders +4 MB; browser-side conversion later if needed) | none |
| GitHub REST (repos, hooks, contents, compare, pulls, releases) | shared adapter `app/services/github` (08 owner: knowledge git sync, docs sync; 13 adds releases/compare/pulls files); clients only via `github.ClientFor` | reuse | `google/go-github/v93` v93.0.0 (BSD-3, 11.3k) with `a.HTTP.StdClient()`; depguard `adapter-github` allows `github.com/google/go-github/v93` only | one adapter, one token per workspace, one webhook per repo | 08 docs-sync.md section 4, 13 code map |
| PDF (later) | 08 edge 86 | reuse later | `ledongthuc/pdf` (BSD-3) text extraction | | none |

### 3.10 Search, segments, inbox (03, 14, 15)

| Piece | Spec | Decision | Why |
|---|---|---|---|
| Search engine | 03 search.md, 08 | confirmed: Postgres FTS + `pg_trgm` + pgvector (`pgvector/pgvector-go` v0.4.1) | no external engine needed |
| Search query language (`support/searchquery`) | 03 | keep custom, delegate free text | operators are product; pass phrase/OR/negation text to `websearch_to_tsquery` where prefix not needed |
| Saved reply templating | 03 `support/templating` | keep custom | deliberately no logic; Liquid (`osteele/liquid`) would add loops/conditionals surface |
| Segments rule language (Go evaluator + SQL compiler parity) | 15 | keep custom | JSON tree from UI, Go/SQL parity; CEL (`google/cel-go`) evaluates but has no maintained SQL compiler |
| URL glob matcher | 15 `support/urlmatch` | keep custom | must match widget JS byte for byte (shared vectors) |
| Trending/revenue, mentions, spam heuristics, excerpt | 03, 14 | keep custom | domain |

### 3.11 Agent actions (07) and importers (16)

| Piece | Spec | Decision | Library | Why |
|---|---|---|---|---|
| OpenAPI 3.0/3.1 parse for action import | 07 `app/support/openapiimport` | reuse | `github.com/pb33f/libopenapi` [verify version, license MIT] | 3.1 + `$ref` resolution; kin-openapi 3.1 support partial [verify]. Depguard exception `support-openapiimport` (one module, one dir) |
| JSON Schema validate (action inputs, portable subset) | 07 `app/support/actionschema` | reuse | `github.com/google/jsonschema-go` (already transitive via MCP go-sdk; 2020-12) | no second schema lib (`santhosh-tekuri/jsonschema/v6` rejected). Depguard exception `support-actionschema`; also allowed in `framework/mcp` |
| URL template (RFC 6570) | 07 `app/support/actiontemplate` | reuse | `github.com/yosida95/uritemplate/v3` (already transitive via MCP go-sdk) | correct RFC 6570 encoding. Depguard exception `support-actiontemplate`; also allowed in `framework/mcp` |
| Response field paths | 07 `app/support/actionresult` | reuse | `github.com/tidwall/gjson` (already in go.mod for testkit) | JMESPath rejected (second dialect). Depguard exception `support-actionresult` |
| Body placeholder substitution | 07 `app/support/actiontemplate` | keep custom (~120 lines) | | no lib does typed JSON placeholder substitution; `text/template` rejected (logic surface) |
| MCP client to customer servers | 07 | reuse | `framework/mcp` client over `modelcontextprotocol/go-sdk` v1.8.0 | one MCP SDK for server and client |
| Import bulk writes | 16 | reuse | pgx `CopyFrom` via `(*database.Tx).CopyFrom` | COPY into staging + `INSERT ... ON CONFLICT DO NOTHING` |
| Import source API clients (Intercom, Zendesk, Help Scout, Featurebase, Productlane, Canny) | 16 drivers | keep custom on `framework/httpclient` | | few endpoints each; official Go SDKs absent or stale [verify per provider at build] |

### 3.12 Billing (12)

| Piece | Decision | Library | Spec edits |
|---|---|---|---|
| Stripe client + webhook verify | already reuse | `stripe/stripe-go/v87` v87.1.0 (MIT, 2026-10-29). Pin the major in spec, use `webhook.ConstructEvent` | 12 stripe.md line 3 (name major) |
| Metering rules D1-D15 | keep custom | domain (open source package) | none |
| AWS (S3, SES v2, KMS, Bedrock) | already reuse | `aws/aws-sdk-go-v2` (Apache-2.0) | none |

## 4. Keep custom (reasons)

- **OAuth AS core (09):** MCP profile (CIMD, DCR, RFC 8707, opaque tokens, first-party rules) is not covered by any maintained Go library. fosite stalled (last release 2024-12); zitadel/oidc OIDC-centric. Reuse metadata types from MCP go-sdk.
- **Sessions table and guards (00-accounts):** per-user session listing, `kind: app` grants, revoke-others. Not a generic session store problem.
- **Email reply/quote stripping, DSN/ARF parse, classifier (04):** no maintained Go library. Port GitHub `email_reply_parser` (MIT) with attribution.
- **SNS signature verify (04):** no maintained Go library; small and well specified.
- **WhatsApp Graph client (05):** only Go client is alpha (v0.1.x).
- **Provider format converters (05), Markdown chunker (08), GitHub alerts extension (08):** product logic on goldmark AST.
- **Search query parser (03), segments compiler (15), urlmatch (15), saved-reply templating (03):** product DSLs with parity or safety constraints.
- **certmagic Postgres storage (08):** no licensed, maintained implementation.
- **Doctor, settings registry (11), metering (12), OG card layout (13):** domain.
- **Signed URLs, cursors, idempotency, token formats, clock:** each under 100 lines on stdlib; libraries would add more surface than they save.
- **Turnstile, HIBP, Voyage clients:** one HTTP call each.

## 5. Rule for future specs

Before a spec lists any file under `app/support`, `app/services`, `app/drivers/*/internal` or a protocol implementation, the writer adds a "Reuse check" line. It names the library searched (Go module path, license, latest version from `proxy.golang.org`, last push from `gh api`), and gives the reason it is or is not used. Default to the library when four things hold: it is maintained (release or commit within 12 months), its license is MIT/BSD/ISC/Apache-2.0 (MPL-2.0 unmodified OK), it is pure Go, and its memory fits the budget. Hand-write only domain logic, product formats, or glue under about 100 lines. Never hand-write crypto, protocol servers, provider SDKs, parsers for public standards, or security filters that a vetted library already provides. If the best option lives in another project's `internal/` package under a permissive license, port it with attribution and a pinned upstream commit instead of rewriting it.

## 6. Risks

| Risk | Where | Mitigation |
|---|---|---|
| Pre-1.0 APIs | centrifuge v0.40, River v0.51, disgo v0.19, certmagic v0.25, typeid-go v1.3 (stable spec); go-imap v2 beta only when `imap-adapter` lands | pin versions, wrap behind contracts, upgrade in dedicated PRs |
| Centrifuge pre-1.0 API churn | realtime | pin version, upgrade deliberately; MIT library, no PRO features needed |
| Small bus factor | go-msgauth (last push 2025-04), bluemonday (2025-04), code.dny.dev/ssrf (10 stars), epiclabs-io/diff3 (17 stars), temoto/robotstxt (release 2021), gorilla/feeds (2024) | vendor + fuzz/vector tests; each is small enough to fork |
| cgo creep | disgo `voice` imports cgo `godave`; bimg/govips; mattn/go-sqlite3 | CI `CGO_ENABLED=0` build; depguard deny `disgo/voice`, `h2non/bimg`, `davidbyttow/govips` |
| License | MCP go-sdk mid-transition MIT to Apache-2.0 (both permissive); gosimple/slug and River MPL-2.0; Mercure and soketi AGPL (rejected); `blitiri.com.ar/go/spf` license not checked | license scan (`go-licenses`) in CI; block AGPL/GPL deps |
| Memory | lingua-go (rejected); centrifuge about 27 to 35 KB per socket; uap-go regex set (rejected for medama) | CI RSS benchmark: 1,000 sockets, idle |
| Laravel-echo compatibility lost | dashboard | centrifuge-js is maintained by the same author as the server lib; React hooks are thin |
| Token in URL for widget SSE | uni_sse | 60 s single-purpose connect token; redact `cf_connect` in logs |
| Test harness | embedded-postgres lacks pgvector | `pgvector/pgvector:pg18` from root `compose.yaml` via `DATABASE_URL` (no testcontainers) |

## 7. Spec files and sections to edit (by decision)

Record. Applied 2026-11-02 by owners. Superseded inside this list: "Postgres broker port, PresenceManager" and "broker stream tables" (realtime uses centrifuge Redis broker + presence on Valkey; no realtime tables).

1. **Realtime to centrifuge:**
   - `specs/00-foundation/events-and-realtime.md` sections "Realtime: Pusher protocol compatible", "Channels", "Cross-replica fan-out", "Presence and typing", "Ephemeral events", "Edge cases" (kick, merge re-auth, dedupe).
   - `specs/00-foundation/README.md` Decisions item 4.
   - `specs/00-foundation/api.md` Surfaces row "Realtime".
   - `specs/00-foundation/tasks.md` "Realtime" block (replace Pusher tasks; add uni_sse port, Postgres broker port, PresenceManager, Echo test becomes centrifuge-js test).
   - `specs/00-foundation/data-model.md` `realtime_events` table (drop, replaced by broker stream tables).
   - `specs/00-conventions/README.md` Stack row "Broadcasting", directory `app/broadcasting/` comment, `routes/channels.go` comment.
   - `specs/01-messenger-widget/README.md` World-class bar (core bundle realtime client), Out table "SSE transport: not planned" (reverse), code map `routes/channels.go` authorizer + new realtime-token controller, `update_controller.go` note.
   - `specs/01-messenger-widget/sdk-contract.md` line 61 proxy paths (`/connection/*`), line 147 realtime.
   - `specs/03-shared-inbox/README.md` "Collision detection and typing", `app/broadcasting/` authorizers.
   - `specs/06-notifications/README.md` `private-member.{id}` channel name.
   - `specs/09-agent-native/cli.md` `dimmi listen` (if it uses the socket).
   - `specs/00-accounts` member deactivation kick (4009) wording.
   - `research/tech-stack.md` Realtime row and section D "Realtime across replicas".
2. **OAuth reuse:** `specs/09-agent-native/README.md` OAuth server code map (`metadata.go`, `resources.go`, `app/mcp/auth.go`, `app/client/auth/device_flow.go`), Contradictions item 8. `specs/09-agent-native/oauth.md` metadata section. `specs/09-agent-native/tasks.md` `support/oauth` line.
3. **MCP go-sdk version:** `specs/09-agent-native/README.md` `app/mcp/server.go` row; `specs/09-agent-native/mcp.md` line 3.
4. **Standard Webhooks lib:** `specs/10-webhooks/README.md` code map `support/webhooksig`; `specs/10-webhooks/tasks.md` line 7.
5. **SSRF lib:** `specs/00-foundation/security.md` "Outbound HTTP (SSRF)"; `specs/00-conventions/README.md` Code style last bullet; `specs/00-foundation/tasks.md` safehttp line.
6. **CSRF via CrossOriginProtection:** `specs/00-foundation/security.md` "Passwords and sessions" CSRF bullet; `specs/00-accounts/api.md` (`auth.csrf.show` row + note), `flows.md` signup step 1 and login, `edge-cases.md` 26-27, `README.md` middleware `verify_csrf_token.go`; `specs/00-conventions/README.md` middleware `auth:session`; `specs/00-foundation/api.md` guards table (`GET /auth/csrf`); `specs/00-foundation/rest.md` exceptions if listed.
7. **Auth libs (TOTP, argon2id, bloom, x/oauth2, go-oidc, go-jose, medama, maxminddb):** `specs/00-accounts/README.md` "Support, services, middleware" and config `auth.geoip.*`; `specs/00-accounts/tasks.md` lines 13-15; `specs/02-widget-identity/README.md` code map `jwtverify/`; `specs/02-widget-identity/tasks.md` first task; `specs/03-shared-inbox/README.md` line 127 + support list `useragent/`.
8. **IDs via TypeID:** `specs/00-conventions/README.md` Database rules "Public IDs"; `specs/00-foundation/data-model.md` "Public ID prefixes" intro; `specs/00-foundation/tasks.md` `support/ids`.
9. **Foundation drivers (gocloud.dev secrets/blob and go-i18n later removed 2026-11-02, httprate, huma validation):** `specs/00-foundation/tasks.md` "Contracts and drivers" lines; `specs/00-foundation/security.md` Secrets; `specs/00-conventions/README.md` Stack rows Storage, Encryption, Request validation.
10. **Tests harness:** `specs/00-conventions/README.md` "Testing" (embedded Postgres line), `specs/00-foundation/tasks.md` Test harness, `research/tech-stack.md` section B.
11. **Chat SDKs:** `specs/05-chat-app-channels/README.md` code map "Contracts / drivers / managers" and middleware list; `specs/05-chat-app-channels/providers.md` sections 1-3 (library notes); `specs/05-chat-app-channels/tasks.md` driver tasks.
12. **Web push lib:** `specs/06-notifications/delivery.md` lines 32-34 and Sources; `specs/06-notifications/README.md` code map `webpush/`; `specs/06-notifications/tasks.md` line 54.
13. **LLM SDKs, breaker, drop lingua:** `specs/07-dimmi-agent/llm-contract.md` "Drivers", "SSE parsing", "Tool call assembly", retry rule line 136; `specs/07-dimmi-agent/README.md` code map (`internal/sse`, `llmretry`, `langdetect`); `specs/07-dimmi-agent/pipeline.md` line 102; `specs/07-dimmi-agent/tasks.md` lines 32, 58; `specs/07-dimmi-agent/edge-cases.md` 47, 68, 75.
14. **Help center libs:** `specs/08-help-center/README.md` code map `support/` block (frontmatter, crawl, slug, textdiff, markdown toc); `specs/08-help-center/tasks.md` lines 17, 74, 87.
15. **Changelog and others:** `specs/13-changelog/README.md` code map (`services/github` with go-github, feeds with gorilla/feeds); `specs/14-feedback-roadmap/README.md` `feedback/slug.go`.
16. **Self-host:** `specs/11-self-host/README.md` code map (`memlimit`, `trustedproxy`, `backupfmt`, `migrator`); `specs/11-self-host/tasks.md` lines 13, 28, 60; `specs/11-self-host/packaging.md` sd_notify line 342.
17. **Widget PoW:** `specs/01-messenger-widget/README.md` code map `support/pow`; `specs/01-messenger-widget/tasks.md` line 97.
18. **Stripe major:** `specs/12-cloud-billing/stripe.md` line 3.
19. **Email:** `specs/04-email-channel/README.md` code map (`enmime/v2`, html-to-markdown v2), `specs/04-email-channel/pipeline.md` attribution for reply parser port.
20. **Process rule:** `specs/00-foundation/spec-writer-brief.md` add "Reuse check" line (section 5 above).

## Sources

- Repo metadata: `gh api repos/<owner>/<repo>` (stars, license, pushed_at, archived), 2026-11-02, about 170 repos.
- Module versions: `https://proxy.golang.org/<module>/@latest`, 2026-11-02.
- centrifuge source: https://github.com/centrifugal/centrifuge (`broker.go`, `presence.go`, `controller.go`, `node.go`, `_examples/`), https://pkg.go.dev/github.com/centrifugal/centrifuge
- Centrifugo internal Postgres code: https://github.com/centrifugal/centrifugo/tree/master/internal (`pgstreambroker`, `pgoutbox`, `controllers`, `unisse`, `natsbroker`)
- Centrifugo Postgres controller blog: https://centrifugal.dev/blog/2026/05/25/pg-controller-multi-node
- Memory per connection: https://centrifugal.dev/blog/2026/08/17/tuning-idle-connections, https://centrifugal.dev/blog/2020/02/10/million-connections-with-centrifugo, https://centrifugal.dev/docs/faq
- Bundle sizes: https://bundlephobia.com/api/size?package=centrifuge, https://bundlephobia.com/api/size?package=pusher-js
- MCP go-sdk releases: https://github.com/modelcontextprotocol/go-sdk/releases; packages `oauthex`, `auth`
- fosite: https://github.com/ory/fosite (handlers `rfc8628`, `par`, `pkce`, `revoke_handler.go`)
- zitadel/oidc: https://github.com/zitadel/oidc (feature table, `pkg/op`)
- SSRF: https://pkg.go.dev/code.dny.dev/ssrf
- CrossOriginProtection: https://pkg.go.dev/net/http#CrossOriginProtection
- webpush-go: https://github.com/SherClockHolmes/webpush-go (`webpush.go` Options, `go.mod`)
- disgo: https://github.com/disgoorg/disgo (`gateway/gateway_transports.go`, `sharding/`, `httpserver/ed25519.go`, `go.mod`)
- go-telegram/bot: https://github.com/go-telegram/bot
- piusalfred/whatsapp: https://github.com/piusalfred/whatsapp
- Standard Webhooks Go: https://github.com/standard-webhooks/standard-webhooks/tree/main/libraries/go
- go-readability deprecation: https://github.com/go-shiori/go-readability; successor https://codeberg.org/readeck/go-readability
- lingua-go memory: https://github.com/pemistahl/lingua-go (README section 9.5)
- Go email reply parser search: https://pkg.go.dev/github.com/teamwork/mailstrip, https://pkg.go.dev/github.com/recapco/emailreplyparser, https://www.github.com/web-ridge/email-reply-parser
- embedded-postgres extensions note: https://github.com/fergusstrange/embedded-postgres
- medama useragent benchmarks: https://github.com/medama-io/go-useragent
- TypeID spec: https://github.com/jetify-com/typeid
- gocloud.dev drivers (evaluated, removed 2026-11-02): https://github.com/google/go-cloud (`secrets/`, `blob/`)
- httprate LimitCounter: https://github.com/go-chi/httprate (`limiter.go`)
- gorilla/feeds JSON Feed 1.1: https://github.com/gorilla/feeds (`json.go`)

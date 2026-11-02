# 12 Cloud billing: tasks

Style: caveman. Scope trim 2026-11-02: survey link cap, echo waiver, message edit case removed; E refs renumbered. Ordered. Each task about 1 day, merged with tests. Tests per `00-conventions/testing.md`: app logic integration only (real App + Postgres `pgvector/pgvector:pg18`, `testkit.New(t)`), no mocks; pure rating/cap math as table tests without mocks or I/O; tests named in plain words, every edge case covered; fakes only at process boundary (`httpclienttest.New(tc.App.HTTP)` Stripe fixtures, `tc.Mail`, `tc.Clock`). Valkey real in tests (compose `valkey/valkey:8` via `REDIS_URL`, per-test key prefix), never faked. Prereqs: foundation (in-tx listener abort + `AfterRollback`, partitioned migrations), 03 reply action + spam/merge events, 11 instance admin.

## M1 Rules engine (pure, open source)

- [ ] `support/metering` skeleton: `RuleSet` interface + registry (`RulesAt`), version `2026-11-01`, 60-day notice registry test, money helpers (micros, round half up).
  - Tests: registry ordering, notice rule, rounding table.
- [ ] Classification: `IsActivity`, `IsQualifyingInbound`, `IsWorkspaceReply`, `FreeReason` (internal/test) + window math (`ClosesAt`, close reasons).
  - Tests: table per message type/author/flags; window extension and max age; boundary `t == closes_at`.
- [ ] Price book `2026-11-01` + `Rate()` (allowance override, contract tiers, percent discount, grants FIFO) + validation of contract tiers.
  - Tests: reference bills ($0/$12/$87/$162/$312/$562/$662), OSS, nonprofit, startup grant, rounding of odd band-2 counts.
- [ ] Cap math (`CapCount` binary search, `BufferCount`) + `Decide()`.
  - Tests: default $50 gives 2,533 / 507; $100 gives 5,866 / 1,174; cap 0; null cap; no card 100 + 20; restricted; test mode 1,000; locked.
- [ ] Forecast + threshold crossing + anomaly functions.
  - Tests: early-period suppression, combined alerts on cap lowering, anomaly min 20.
- [ ] Replay engine over fact history + scenario vectors (metering-rules.md section 14) as YAML testdata.
  - Tests: all vectors pass online (real App + Postgres) and in replay, identical ledger.
- [ ] `dimmi billing:simulate` (all editions): loads facts from DB for workspace + period, prints summary/JSON/CSV.
  - Tests: feature test on self-host edition with factory conversations.

## M2 Online meter (cloud)

- [ ] Migrations: billing_accounts (`organization_id` unique, `currency`), spend_caps, billing_periods (keyed by `billing_account_id`), billing_windows, usage_ledger (partitioned, default partition, append-only trigger, grants), invoices_cache, credit_grants(+applications), billing_adjustments, billing_disputes, billing_program_applications, stripe_events, billing_reconciliation_issues, entitlement_overrides; `dimmi_visible_billing_accounts()` + RLS on account-scoped tables (`-- lint:org-scoped`); sqlc queries; factories; `CreateLedgerPartitions` job.
  - Tests: UPDATE/DELETE on ledger raises; partition routing; RLS cross-tenant (workspace-scoped and account-scoped tables); account of other org invisible.
- [ ] `ResolveBillingAccount` (workspace -> organization -> account, L1 otter 60 s + Valkey pubsub bust) + `EnsureBillingAccount` by org + workspace initializer (after org insert in `CreateWorkspace`) + close/anonymize on org soft-delete.
  - Tests: initializer failure rolls back workspace; repair path; forced two-workspace org shares one account, one period row, one cap (later-path proof, no schema change); workspace delete with sibling workspace leaves account open.
- [ ] `billing_service_provider` (cloud only), config/billing.go + services.stripe, boot fails on cloud without Stripe keys.
  - Tests: edition matrix (self-host: no listeners, 404 `feature_unavailable`).
- [ ] `RecordBillableActivity` `InTx` listener calling `MeterMessageActivity` (same tx as 03 reply) + `ResolveBillingAccount` + account period lock + window open/extend/close + ledger insert (`workspace_id` + `billing_account_id`) + account counters + `billing.conversation_activated`.
  - Tests: vectors 1-10, 20, 24, 25 online; latency benchmark (p95 under 5 ms).
- [ ] Blocked activation path: `ErrActivationBlocked` -> 423 with `billing` block (03 reply controller integration), `AfterRollback` pause marker, `billing.activations_paused` once per period.
  - Tests: reply 423 leaves no message row; inbound still stored; note allowed; follow-up without inbound allowed; reply in open window allowed while paused; bulk reply per-id 423 without rolling back committed ids; MCP/API token reply 423 identical; `conversations/{id}/billing` preflight equals actual decision.
- [ ] Concurrency suite: parallel replies same conversation, parallel activations across conversations at cap boundary, parallel activations across two workspaces of one account (forced org), merge vs activation, spam vs activation.
  - Tests: exactly one charge; no overshoot of cap_count + buffer; no deadlocks (lock order assertion).
- [ ] `PreflightActivation` action (07, UI, MCP) + `SweepBillingWindows` job.

- [ ] `BillingStatusReader` (`app/contracts/billing_status.go`) + cloud `BillingStatus` action (`HasPaid`, `AccountStatus`, `Restricted`) + self-host noop + `billing_accounts.first_paid_at` set on first `invoice.paid` + `BustWidgetBrandingOnFirstPayment` listener. Tests: free account false; first paid invoice true and stays true after refund; active contract true, expired contract false; self-host noop false; cache busted on `billing.invoice_paid`; 01 `ResolveBranding` hides badge after first paid invoice (real reader, factory invoice).
  - Tests: preflight matches actual decision for same state.
- [ ] Property test: random event streams online (real DB) vs replay equality (10k streams in CI nightly, 500 per PR).
- [ ] Valkey independence: billing decisions read Postgres only; meter coalescing key + cache busts on Valkey.
  - Tests: E123 Valkey unreachable mid-suite (client through in-test TCP proxy, proxy cut): replies activate/block exactly as with Valkey up, ledger equals replay; flushing the test's keyspace between activations changes nothing; arch test `app/support/metering` + activation/credit actions never import `framework/cache`.
- [ ] Launch feature fixtures 30-35 (metering-rules.md "Launch feature replay fixtures"): `changelog_reply_billing` (13), `feedback_activity_not_billable` (14), `csat_request_not_billable` (15), `announcement_reply_billing` (15), `import_never_billed` (16), `agent_action_not_billable` (07). Arch test: `app/support/metering` + `app/actions/billing` never import 07 actions/08 docs sync/13/14/15/16 packages or read their tables.
  - Tests: online ledger equals expected and replay; E120-E122, E27, E124 to E126.

## M3 Credits

- [ ] `ApplyBillingCredits` `Queued` listener: spam credits (14 days), unmark reversal + deferred activation, merge clustering (needs 03 `conversation.merged`), delivery credit + delayed re-evaluation + reversal.
  - Tests: vectors 11-19, 27-28; idempotency on duplicate events; cycles; delivery credit only on `final = true` (transient `failed` ignored); River retry of credit job after commit produces no second credit line (idempotency key unique).
- [ ] Late credits on invoiced periods: `billing_adjustments` deltas (negative and positive), absorb when zero.
  - Tests: delta at band boundaries; free-tier absorb; grant-covered absorb.
- [ ] Disputes: open (auto-credit under $50/30 days, rate limit 10/h), pending review, staff resolve; mails.
  - Tests: budget exhaustion; waiver line 422; duplicate 409; age 90 days.
- [ ] Spam-credit ratio review job + anomaly job + review flags.

## M4 Caps, alerts, meter API

- [ ] Spend cap API (show/update) with permission limits (`billing:write` up to admin limit, `billing:cap_unrestricted` above/null via `authz.Can`; no role names in 12 code), default cap on first payment method, `RecomputeCapMath` on every input change, `billing.spend_cap_updated`, resume event.
  - Tests: admin over $1,000 403; null cap owner only; recompute triggers (grant add/expire, discount, contract, card add/remove).
- [ ] Meter endpoint + realtime `billing.meter_updated` on `workspace:{ws}` (centrifuge, coalesced, counts only) + member vs admin field masking.
  - Tests: masking; state values; p95 benchmark.
- [ ] Alerts: thresholds, forecast, anomaly via mail + 06 notifications + webhook events; dedupe in `alerts_sent`.
  - Tests: each fires once per period; lowering cap fires combined alert.
- [ ] Usage API: periods index/show, lines index/show with filters + cursor, CSV export streaming with formula guard (`GET /billing/usage-lines?format=csv`, same route `billing.usage-lines.index`; no `/export` path: route lint verb denylist), conversation billing endpoint.
  - Tests: filters allowlist, cross-tenant 404, member 403, 100k CSV under 10 s.
- [ ] MCP tools: `dimmi_get_billing_meter`, `dimmi_list_usage_lines`, `dimmi_get_conversation_billing`, `dimmi_get_spend_cap`, `dimmi_update_spend_cap` (09 confirm gate `app/mcp/confirm`: elicitation, then `confirm` param fallback).
- [ ] Outbound fair-use guard (`InTx` on `conversation.created` with `initiated_by` in (member, agent, api)); floor from entitlement `billing.outbound_fair_use_min`, account-level previous period.
  - Tests: limit math from previous account period; 429 code; override raises limit.

## M4b Entitlements (cloud driver, overrides)

- [ ] Cloud `Entitlements` driver (`app/support/entitlements/cloud`): catalog cloud default -> account program/contract `entitlements` -> overrides (org, then workspace; unexpired); L1 60 s cache + `pubsub.PubSub` (Valkey) bust; fail to default; register 12 keys `billing.test_mode_cap`, `billing.outbound_fair_use_min`; bind in `billing_service_provider`. Resolves 13/14/15 keys (README Entitlements table) with no 12 code per key.
  - Tests: each 13/14/15 key returns cloud default, override wins, self-host value; over limit `entitlement_limit_reached` with `key`, `limit`: 422 count caps, 429 + `Retry-After` per-period caps.
  - Tests: precedence table; expiry; bust on set; DB error returns default; self-host binds `self_host` driver (11) and ignores table.
- [ ] `dimmi entitlement:set` / `entitlement:list` + actions `SetEntitlementOverride`, `RemoveEntitlementOverride` (catalog key + value type validation, reason required, audit `entitlement.set`/`entitlement.removed`).
  - Tests: unknown key 1 exit, wrong type rejected, audit row, cloud-only guard.
- [ ] `used` counters hook for `GET /workspace/entitlements` (12 provides none at MVP; framework only if foundation asks).
  - Tests: endpoint returns cloud defaults + override for member.

## M5 Stripe: customers, payment methods, invoices

- [ ] `services/stripe` adapter on `stripe-go/v87` (HTTP via `a.HTTP.StdClient()`, SDK retries off, app-level job retries with idempotency keys, restricted key checks, `webhook.ConstructEvent`), recorded fixtures for `httpclienttest.New(tc.App.HTTP)`, stripe-mock contract tests; depguard adapter row `app/services/stripe` (stripe-go only).
- [ ] Billing account API (address/tax validation, tax ids), customer create/update with `tax[validate_location]=immediately`.
  - Tests: `unrecognized_location` 422; tax id sync.
- [ ] Payment method sessions (Checkout setup, `currency=usd`), portal sessions (`auth:session` `S`: any bearer incl. first-party app 403 `session_required`; Stripe return URL needs browser cookie), sync on `checkout.session.completed` + success-page sync.
  - Tests: webhook-first and redirect-first orders; card removal via portal events.
- [ ] Stripe webhook controller (signature, multi-secret, dedupe, livemode check) + `ProcessStripeWebhook` handlers table (stripe.md section 5).
  - Tests: bad signature 400; replay dedupe; out-of-order events re-fetch.
- [ ] Period lifecycle: `OpenBillingPeriods`, `CloseBillingPeriods` (snapshot), roll-over logic (max 3), zero periods, final invoice on workspace deletion.
  - Tests: roll-over chain cases (3.20 + 2.10 + 1.00), zero periods neutral, write-off under $0.50.
- [ ] Currency: `billing_accounts.currency`, price book registry keyed by `(version, currency)`, every Stripe create call passes account currency, `currency_locked` after first invoice, boot check `billing.currency` has book, reconciliation `currency_drift`.
  - Tests: non-usd account without book rejected; HTTP fake asserts currency on every Stripe request; lock after invoice.
- [ ] `IssueInvoice` (per billing account): draft, items (Dimmi-rounded amounts), coupons (percent forever per account, grant once per invoice), finalize, subtotal assertion + void on mismatch, pay or send, own invoice mail; crash-resume.
  - Tests: HTTP-faked Stripe failures at each step resume correctly; mismatch voids; grant applications recorded.
- [ ] Dunning: `RetryInvoicePayment` schedule, `AdvanceDunning` (past_due, restricted day 14, uncollectible day 60), retry-now endpoint, SCA path, restore on paid.
  - Tests: `tc.Clock`-driven schedule; restricted decisions (free allowance only); chargeback restricts.
- [ ] Credit notes: `IssueCreditNotes` batching (>= $1 or 25 days), positive corrections on next invoice, staff credit notes/refunds.
  - Tests: caps at creditable amount; balance shown in credits endpoint.

## M6 Staff tools, reconciliation, transparency

- [ ] Staff API: accounts index/show/update (filter by org/workspace, show org + workspaces + overrides; program, allowance, discount, contract incl. `entitlements`, currency before first invoice, lock, review flag, price book pin), credit grants, conversation credits, credit notes, disputes, program applications, reconciliation issues; audit with mandatory note; 2FA gate.
  - Tests: non-staff 403; money actions require note; contract tier validation warnings.
- [ ] Program applications (workspace submit, staff approve applies OSS allowance 2,000 / startup $300 grant 12 months / nonprofit 50%).
- [ ] `ReconcileStripe` (all kinds), `VerifyLedgerReplay`, `billing:recompute-counters`, staff alerts.
  - Tests: injected drift per kind produces issue; auto-resync only for status/payment-method/counters.
- [ ] Public pricing (+ `included` feature list: changelog, feedback board and roadmap, announcements and surveys) + estimate endpoints, comparison endpoint (Fin $0.99 per AI-only window estimate with source date), grant expiry reminders.
  - Tests: estimate equals Rate(); comparison counts AI-only windows.
- [ ] Docs: public "How we count" page generated from rule set constants (D1-D15 plain English + version + effective date), API reference for billing endpoints, staff runbook (invoice run, dunning, refunds, reconciliation).

## Done when

- M1-M6 checked; property tests green; Stripe contract tests green; staging run with Stripe test clocks completes full month (activations, cap, invoice, failed payment, recovery).
- Demo script (staging, Stripe test mode):
  1. New workspace, no card. Widget conversations: 100 answered by AI and humans. Meter shows 100, $0.00.
  2. Conversation 101: reply goes through (buffer waiver), banner "Add card". Conversation 121: reply 423 `payment_method_required`; inbound still lands; AI run skipped `billing_paused`.
  3. Add card via Checkout (4242...). Cap $50 auto-set; resumed; reply sends.
  4. Seed 2,600 activations (factory). Alerts 50/80/100% received (mail + webhook). 2,534th waived buffer; after 3,040 replies blocked `spend_cap_reached`.
  5. Owner raises cap to $100: resumed. Admin tries $5,000: 403.
  6. Mark one charged conversation spam: credit line appears, meter drops. Unmark: reversal.
  7. Merge two same-contact threads 3 days apart: one credit.
  8. Click a ledger line: opens thread, shows actor AI, rule version. Export CSV.
  9. Advance test clock to next month: invoice $50.00 + tax finalized, paid; lines show band math; usage link works.
  10. Switch card to 4000 0000 0000 0341, advance month: payment fails, `past_due`, retries, day 14 `restricted` (only 100 free activations), update card: paid, restored.
  11. `dimmi billing:simulate` on self-host copy of same data prints identical counts.
  12. `dimmi billing:reconcile`: zero issues.

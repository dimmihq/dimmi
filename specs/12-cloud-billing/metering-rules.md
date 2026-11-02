# 12 Cloud billing: metering rules as code

Style: caveman. Exact algorithm for `research/pricing.md` section 2 rules D1-D15, rating, caps. Code: `app/support/metering` (pure Go, no Stripe, no HTTP, open source, same package used online, in replay, and by `dimmi billing:simulate` on self-host). Rule set version `2026-11-01`. Price book version `2026-11-01` (currency `usd`).

Two units (README Data model): **metering** (windows, ledger lines, D1-D15) per workspace conversation; **money** (period counters, free allowance, tiers, cap, buffer, test cap, fair-use, invoices) per billing account (owned by organization). MVP one workspace per account, so both coincide.

## 1. Vocabulary

| Term | Definition (rule set 2026-11-01) |
|---|---|
| Thread | one `conversations` row |
| Activity message | `messages.type = 'message'`, `author_type IN ('contact','member','agent','integration')`, `metadata.imported` not true. Notes, events, system messages never activity |
| Qualifying inbound | activity message with `author_type = 'contact'`, `substantive = true`, `metadata.auto_reply` not true (bounce, OOO, newsletter, `Auto-Submitted` flagged by 04/05) |
| Workspace reply (substantive) | activity message with `author_type IN ('member','agent','integration')`, `substantive = true`. `integration` = API token or external AI agent writing as workspace (D11, D6) |
| External contact | conversation `contact_id` contact where: not internal (below), `blocked_at IS NULL` at evaluation time |
| Internal contact (D8, D10) | contact has verified email identity (`contact_identities.type='email' AND verified`) equal to email of active `workspace_members` user of same workspace, or contact `kind='user'` with `external_id` registered as member test id (01 widget test mode), or conversation `is_test = true` (03 column) (widget preview/test mode, AI playground) |
| Window | billing window row (`billing_windows`), one per activation of thread |
| Activation | first substantive workspace reply that opens new window (D1) |
| Period | UTC calendar month of activation (D14) |

### Not activity (launch features 07 actions, 08 docs sync, 13, 14, 15, 16; rule set 2026-11-01 unchanged)

None of these write `messages` rows that pass `IsActivity`, so D1 never runs. Metering code never reads their tables.

| Spec | Not activity | Billable path left |
|---|---|---|
| 13 changelog | entries, publish, feeds, widget tab, reactions, subscriptions, mailings (04 `SendChangelogEmail`, `outbound_emails.kind changelog`), AI generations (BYOK, 07 spend) | reply to changelog email or widget "Send feedback" = normal inbound conversation; D1 when workspace replies substantively. Auto-replies to changelog mail discarded by 04 |
| 14 feedback/roadmap | posts, votes, comments, requests (`feedback_requests` link only, conversation unchanged), status changes, voter mail (04 `SendContactNotification`: not conversation outbound, D7 fair-use unaffected: no `conversation.created`), portal sign-in mail, AI capture (BYOK) | customer reply to feedback email = new email conversation; D1 |
| 07 agent actions | action calls (HTTP, MCP), tests, playground, tool syncs; confirmation card, pending-approval, cancel/decline/expiry notices (`substantive = false`, D2); no "procedure handoff" fee | follow-up answer after write (`substantive = true` when outcome `answered`) and read-action answers = normal D6 agent reply |
| 08 docs sync | GitHub webhooks, docs sync runs, suggestions, freshness scans, AI drafting (BYOK, 07 spend) | none (no conversations) |
| 16 importers | imported conversations, messages, contacts, articles, feedback, changelog (`metadata.imported`, `source.imported`; no `message.created` dispatched), delta runs, rollback, redirects | live contact message after import + workspace reply = D1 (fixture `import_never_billed`) |
| 15 announcements/surveys | announcements, surveys, impressions, clicks, dismissals, responses, receipts; CSAT request = system message (`author_type system`, `substantive false`; D2, never activity by `author_type` alone); D7: publishing creates no conversation | chat-format announcement reply = contact-initiated widget conversation; D1 when workspace replies |


## 2. Window state machine (D3)

```
            activation (charge or waiver)
   (none) ---------------------------------> open
   open   -- activity message at t < closes_at --> open (last_activity_at = t, closes_at = min(activated_at + 30d, t + 7d))
   open   -- now >= closes_at (lazy on next message, or sweeper) --> closed (closed_at = closes_at, close_reason = quiet | max_age)
   open   -- conversation merged as source --> closed (close_reason = merged, closed_at = merge time)
   open   -- conversation hard-deleted --> closed (close_reason = conversation_deleted)
```
- `closes_at` computed at activation: `min(activated_at + 30 days, activated_at + 7 days)` = `activated_at + 7d` initially; extended by later activity.
- Durations are exact (`7 * 24h`, `30 * 24h`), not calendar days; DST irrelevant (UTC).
- Message with `created_at >= closes_at` never extends window: window closes first, then message evaluated against no-open-window state.
- Window status `closed` is final. Never reopened.

## 3. Online algorithm (action `MeterMessageActivity`, called by `InTx` listener `RecordBillableActivity` on `message.created`)

`InTx` phase per `00-foundation/contracts.md`: runs inside 03 reply transaction that created message (cloud edition only). Activation (window + ledger + counters) recorded in same tx. Returns error to abort transaction only for blocked activation (423 `spend_cap_reached` or other block code). Never queued: queued activation would let replies past cap.

```go
func (m *Meter) OnMessageCreated(ctx context.Context, tx *database.Tx, msg models.Message) error {
    if !metering.IsActivity(msg) { return nil }
    conv := tx.FindConversationForMeter(ctx, msg.WorkspaceID, msg.ConversationID) // contact, spam, source, inbox channel
    t := msg.CreatedAt
    rules := metering.RulesAt(t) // version registry

    win := tx.FindLatestBillingWindow(ctx, conv.ID) // may be nil
    if win != nil && win.Status == Open {
        if !t.Before(win.ClosesAt) {
            tx.CloseBillingWindow(ctx, win.ID, win.ClosesAt, rules.CloseReason(win))
            win.Status = Closed; win.ClosedAt = win.ClosesAt
        } else {
            tx.TouchBillingWindow(ctx, win.ID, t, rules.ClosesAt(win.ActivatedAt, t))
            return nil // inside open window: covered, never gated (D3)
        }
    }
    if !rules.IsWorkspaceReply(msg) { return nil }
    since := time.Time{}
    if win != nil { since = win.ClosedAt }
    if !tx.HasQualifyingInbound(ctx, conv.ID, since, t) { return nil } // D1a, D4, D7 follow-ups
    if conv.Spam { return nil }                    // D9a: answered while spam = no activation (see unmark)
    if conv.Contact.BlockedAt != nil { return nil } // D9d
    if conv.Source.Imported && msg.Metadata.Imported { return nil } // D11 (IsActivity already excludes imported msgs)
    return m.activate(ctx, tx, conv, msg, rules.FreeReason(conv)) // FreeReason: test_mode | internal | ""
}
```

`activate` (lock order always: billing_periods row, then billing_windows insert):
```go
func (m *Meter) activate(ctx, tx, conv, msg, freeReason) error {
    acct := m.ResolveBillingAccount(ctx, tx, conv.WorkspaceID)         // workspace -> organization -> account (cached 60 s); EnsureBillingAccount repair on miss
    p := tx.LockBillingPeriod(ctx, acct.ID, MonthUTC(msg.CreatedAt))   // upsert + SELECT ... FOR UPDATE; one row per account per month
    // re-check after lock: concurrent reply may have activated
    if w := tx.FindLatestBillingWindow(ctx, conv.ID); w != nil && w.Status == Open && msg.CreatedAt.Before(w.ClosesAt) { return nil }
    d := metering.Decide(p, acct, conv.Workspace, freeReason, m.limits(ctx, conv.WorkspaceID)) // pure function, section 6; limits = entitlement values read before lock
    switch d.Result {
    case Charge:  return m.openWindow(ctx, tx, conv, msg, p, "charge", "activation")
    case Waive:   return m.openWindow(ctx, tx, conv, msg, p, "waiver", d.Reason)
    case Block:
        tx.AfterRollback(func() { m.recordPause(conv.WorkspaceID, p.Period, d.Reason) }) // separate autocommit tx
        return exceptions.ErrActivationBlocked{Reason: d.Reason} // 423, aborts reply
    }
}
```
`openWindow`: insert `billing_windows` (seq = last seq + 1, unique `(conversation_id, seq)`, `billing_account_id = acct.ID`), insert `usage_ledger` line (`workspace_id` of conversation, `billing_account_id = acct.ID`, idempotency key `act:<conversation_id>:<seq>`), update account period counters, dispatch `billing.conversation_activated` (charge or waiver, `billable` flag) in same tx.

Lock contention: period row is per account. Shared account later (2 workspaces) serializes their activations on one row; budget (README) holds at 50 activations/s per account.

Read-only twin (no lock, no write): `PreflightActivation` (07, UI, MCP) runs same checks + `Decide` on unlocked period row. Advisory: concurrent activation, new inbound or cap crossing between preflight and send can change result; send path above is only truth.

Window opened by waiver behaves exactly like charged window (covers later replies, closes same way). Later credit or charge never converts waiver into charge.

## 4. Rule mapping D1-D15

| Rule | Code | Behavior |
|---|---|---|
| D1 billable conversation | `IsWorkspaceReply` + `HasQualifyingInbound` + `activate` | Charge at first substantive workspace reply after qualifying inbound; `activated_at = reply.created_at` |
| D2 substantive | `messages.substantive` (set by writers: 01 welcome, 04 auto-ack/OOO, 06/07 notices, 15 CSAT request = false + `author_type system`) + `metadata.auto_reply` on inbound | Non-substantive replies never activate; non-substantive inbound never qualifies |
| D3 one charge per window | window state machine | 7 days quiet or 30 days max; all channels, teammates, AI turns inside window = one line |
| D4 reopen | `since = win.ClosedAt` in `HasQualifyingInbound` | New charge only when contact wrote after previous window closed AND workspace replies. Workspace-only follow-up after close: no charge |
| D5 channels | no channel branching in rules | Same rule all channels; carrier fees (WhatsApp) not in ledger (pass-through billing out of MVP, see README Out) |
| D6 AI-only | `author_type='agent'` counts as workspace reply | Same charge as human; ledger `actor_type='agent'` |
| D7 outbound | inbound requirement | Teammate-started thread billable only after contact replies and workspace answers. Fair-use cap separate (section 9) |
| D8 internal | `FreeReason = internal` | Waiver line (`reason internal`), never billable |
| D9 spam/bots/abuse | (a) `conv.Spam` at activation: no activation. (b) mark spam within 14 days of `activated_at`: credit. (c) bot-check failures / rate-limited sessions: messages never created or conversation auto-marked spam by 01/03, so (a) applies. (d) blocked contact: no activation | Un-mark spam: reverse spam credits (charge `credit_reversed`) and evaluate deferred activation (section 5.2) |
| D10 test mode | `workspaces.test_mode` or `FreeReason=test_mode` | Waiver `test_mode`; sandbox cap entitlement `billing.test_mode_cap` (cloud 1,000) waivers per account period then block `test_mode_limit_reached` |
| D11 API-created, imports | `author_type='integration'` normal; `metadata.imported` excluded from activity | Imported history never activity; live messages after import normal. Importer 16 (launch: importers); 16 writers dispatch no `message.created`, so `RecordBillableActivity` never runs for import writes. Delta-imported inbound is imported, not qualifying inbound: live teammate reply after only delta-imported inbound not billed (16 E52 trade-off). Fixture `import_never_billed` |
| D12 merges | `ApplyMergeCredits` on `conversation.merged` | Section 5.3 |
| D13 failed delivery | `ApplyDeliveryCredit` on `message.delivery_updated` | Section 5.4 |
| D14 month boundary | `period = date_trunc('month', activated_at at UTC)` | Window spanning months counts once, in activation month |
| D15 audit | `usage_ledger` append-only + rule_version + links | Section 7; rule changes: new version file, 60-day notice |

## 5. Credits and reversals (`Queued`, `billing` queue, listener `ApplyBillingCredits`)

All credit actions: lock account period row of charge's period (`billing_account_id` + `period` from charge line, `FOR UPDATE`), check charge has no active credit (latest line referencing it is not a credit), insert credit line `quantity -1` with `reverses_entry_id = charge.id`, idempotency key `cr:<charge_id>:<cycle>` where cycle = count of prior credit lines for charge. Update counters. Dispatch `billing.conversation_credited`. If charge's period already invoiced: also insert `billing_adjustments` row (money computed at credit-note time, stripe.md section 7).

Reversal: insert charge line `reason credit_reversed`, `reverses_entry_id = credit.id`, key `rv:<credit_id>`. Dispatch `billing.conversation_credit_reversed`. Reversal never blocked by cap (prevents mark-spam/unmark cap evasion).

### 5.1 Spam marked (D9b)
On `conversation.marked_spam`: for each charge line of conversation without active credit and `activated_at >= marked_at - 14d`: credit `reason spam`. Charges older than 14 days: no credit (still spam-foldered). Waiver lines: nothing (never billed).

### 5.2 Spam unmarked
On `conversation.unmarked_spam`: reverse every active `spam` credit of conversation. Then deferred activation: if no open window and `HasQualifyingInbound(since last close, now)` and exists substantive workspace reply after that inbound: `activate` with `activated_at = unmark time`, `activating_message_id = earliest such reply`, ledger `reason spam_unmarked`. Decision uses section 6 except Block becomes Waive `over_cap_deferred` (never abort teammate's unmark).

### 5.3 Merges (D12)
On `conversation.merged {source_id, target_id}`: resolve contacts through `merged_into_id` chain. Same contact: collect active charges (charge lines without active credit) of both conversations; sort by `activated_at`; greedy cluster: keep earliest, credit (`reason merge`) every other charge whose `activated_at` within 7 days of kept charge; next charge outside 7 days becomes new kept anchor. Different contacts: no credit. Source open window closed `merged`; target window continues. Ledger lines keep source `conversation_id` (immutable); API link resolves to target via 03 merge redirect.

### 5.4 Failed delivery (D13)
On `message.delivery_updated` with `to = failed` and `final = true` (channel-contract section 2 payload; 04 hard bounce, 05 permanent codes, cancelled-before-send, retries exhausted): window = window covering message. If window charge active and window has no other workspace reply with `delivery_status IN ('sent','delivered','read','not_applicable')`: if any other reply in window still `pending`, schedule `EvaluateDeliveryCredit` in 1 hour (unique per window); else credit `reason failed_delivery`. Later reply in same window reaching `sent|delivered|read` while `failed_delivery` credit active: reverse (`reason credit_reversed`). Transient failures (retrying) ignored until final.

### 5.5 Disputes and support
Dispute credit (`reason dispute`) and staff credit (`reason support`) via actions in README; same mechanics, `created_by_type` member/staff, `note` stored. Waiver lines never creditable (no money): API 422 `line_not_creditable`.

## 6. Decision function (pure)

Inputs: account period counters (`billable_count` = charges + reversals - credits, `waived_buffer_count`, `waived_test_count`; all workspaces of account), account (status, has_payment_method, free_allowance, price inputs), activating workspace (`test_mode`), entitlement limits (`billing.test_mode_cap`, read via `Entitlements.Limit` before lock; pure function receives value), cap math results (`cap_count`, `buffer_count`).

```
Decide(p, acct, ws, freeReason, limits):
  if freeReason == test_mode or ws.test_mode:
      if limits.test_mode_cap == -1 or p.waived_test_count < limits.test_mode_cap (1000 cloud default): Waive(test_mode) else Block(test_mode_limit_reached)
  if freeReason == internal: Waive(internal)
  if acct.status == restricted:
      if p.billable_count < acct.free_allowance: Charge else Block(billing_restricted)
  if acct.status == locked: Block(billing_locked)                 // staff fraud lock only
  if !acct.has_payment_method and acct.collection_method != send_invoice:
      if p.billable_count < acct.free_allowance: Charge           // free band, $0
      elif p.waived_buffer_count < buffer_count(free_allowance): Waive(cap_buffer)
      else Block(payment_method_required)
  if p.cap_count == unlimited or p.billable_count < p.cap_count: Charge
  if p.waived_buffer_count < p.buffer_count: Waive(cap_buffer)
  Block(spend_cap_reached)
```
`buffer_count(n) = max(cfg.buffer_min (20), ceil(n * cfg.buffer_percent (20%)))`.

Buffer conversations free forever (waiver lines never billed, also not after cap raise). Credits lower `billable_count` and so reopen paid headroom; buffer waivers already used stay used for period.

## 7. Ledger line contract

Every line: `workspace_id, billing_account_id, conversation_id, billing_window_id, window_seq, contact_id, channel_type, actor_type, actor_id, activating_message_id, activated_at, period, entry_type, quantity, reason, rule_version, reverses_entry_id, created_by_type, created_by_id, idempotency_key, created_at`. Credits copy workspace/account/conversation/window/contact/channel/actor/activated_at/period from charge (credit lands on account that was charged, even if workspace later moves org). No message bodies, names, emails. Append-only (trigger + no UPDATE/DELETE grant). Public id `ul_`.

## 8. Rating

### Price book `2026-11-01` (`pricebook_2026_11_01.go`)
Unit prices in micro-units of account currency (1 USD = 1,000,000 micros). Price books keyed by `(version, currency)`; MVP only `usd` (account with currency lacking book: boot check fails for `billing.currency`, staff currency change rejected 422). Graduated, per account period, on account `billable_count` at period close.

| Tier | Conversations (ordinal in period) | Unit |
|---|---|---|
| free | 1 to `free_allowance` (100 default; OSS 2,000) | 0 |
| band 1 | 101 to 1,000 | 30,000 ($0.03) |
| band 2 | 1,001 to 10,000 | 15,000 ($0.015) |
| band 3 | 10,001 to 50,000 | 10,000 ($0.01) |
| band 4 | above 50,000 | 10,000 ($0.01) unless contract |

- `free_allowance` above 100 replaces free tier upper bound only; ordinals above allowance priced by band their ordinal falls in (OSS: 2,001st conversation at $0.015).
- Above 50,000 without contract: still $0.01 (never blocked, never unbilled). At 40,000 billable in period: staff alert + email offering contract.
- Contract (`billing_accounts.contract.tiers`): full tier list replaces book (same shape). Validation: ascending bounds, last unbounded, units >= 0; warn when unit under `billing.contract_min_unit_micros`.
- Rounding: per tier line `cents = round_half_up(count * unit / 10,000)` (cents = minor units; currencies with other exponents need own book + rounding rule, not MVP). Subtotal = sum of tier line cents. Example: 901 at band 2 = 13,515,000 micros = $13.515 -> $13.52.
- Percent discount (nonprofit 50%): `discount_cents = round_half_up(subtotal * pct / 100)`.
- Credit grants: applied after percent discount, FIFO by `expires_at`, up to remaining amount. Grants valid at invoice date only.
- `payable_pre_tax = subtotal - discount - grants_applied` (>= 0). Tax computed by Stripe on payable.

### Reference bills (test vectors, free_allowance 100, no discounts)
| Billable | Lines | Total |
|---|---|---|
| 50 | free 50 | $0.00 |
| 100 | free 100 | $0.00 |
| 101 | free 100 + 1 x $0.03 | $0.03 |
| 500 | 400 x $0.03 | $12.00 |
| 1,000 | 900 x $0.03 | $27.00 |
| 5,000 | 900 x $0.03 + 4,000 x $0.015 | $87.00 |
| 10,000 | 900 x $0.03 + 9,000 x $0.015 | $162.00 |
| 25,000 | ... + 15,000 x $0.01 | $312.00 |
| 50,000 | ... + 40,000 x $0.01 | $562.00 |
| 60,000 | ... + 10,000 x $0.01 band 4 | $662.00 |
| OSS 2,500 (allowance 2,000) | 500 x $0.015 | $7.50 |
| Nonprofit 5,000 | $87.00 - 50% | $43.50 |
| Startup 500 with $300 grant | $12.00 - $12.00 grant | $0.00 (grant left $288) |

Match `research/pricing.md` TL;DR bills ($0 / $12 / $87 / $162 / $312 / $562). Rating table test asserts (pure function, no mocks).

## 9. Cap math (`capmath.go`)

- Per billing account. `payable(n)` = rating pipeline above for account `billable_count = n` with current account inputs (allowance, contract, percent discount, grants remaining valid at period end).
- `cap_count = max n such that payable(n) <= cap_cents` (binary search over 0..10,000,000; monotonic). Cap null: unlimited. Cap 0: `cap_count = free_allowance` (free only).
- Default cap $50: 100 free + 900 x $0.03 ($27.00) leaves $23.00 for band 2; 1,533 x $0.015 = $22.995, rounds to $23.00 (fits); 1,534 gives $23.01 (over). `cap_count` = 100 + 900 + 1,533 = **2,533**. Buffer = max(20, ceil(2,533 x 0.2)) = **507**. Hard stop after 3,040 activations, last 507 free.
- Recomputed and stored on period row (`cap_count`, `buffer_count`) whenever: cap updated, grant added/expired, discount/contract/allowance changed, payment method added/removed, new period opened. Stored values used inside activation lock (no binary search on hot path).
- Outbound fair-use (D7): `outbound_limit = max(Entitlements.Limit(ws, "billing.outbound_fair_use_min") (cloud 1,000), billing.outbound_fair_use_multiplier (10) x previous account period billable_count)` teammate-initiated conversations per account period (conversation whose first activity message is workspace reply); limit -1 = no limit. Over limit: conversation create 429 `outbound_fair_use_exceeded` (guard listener on `conversation.created` with `initiated_by` in (member, agent, api)). Self-host: no limit.

## 10. Forecast and alerts (`forecast.go`, per billing account)

- `avg_daily` = activations (charges) in last 7 days of current period / days elapsed in that window (min 1 day). Before day 3 of period and fewer than 20 charges: no forecast.
- `projected_count = billable_count + avg_daily x remaining_days` (remaining fractional days to period end).
- Meter shows `payable(billable_count)` "so far" and `payable(projected_count)` "on pace for".
- Threshold alerts at `thresholds` (default 50, 80, 100) percent of cap on `payable(billable_count)`; each once per period (`alerts_sent`). Lowering cap mid-period can fire multiple thresholds at once: one combined alert (highest).
- Forecast alert: `payable(projected_count) > cap` once per period.
- Anomaly: today's charges (UTC day so far) >= 3 x trailing 7-day daily average and >= 20: alert once per UTC day.

## 11. Period close and roll-over

- Period (per account) `open` during month. At 00:05 UTC on 1st, `CloseBillingPeriod` sets `closed`, snapshots counters + `pricing_snapshot` (price book version, allowance, contract, discount, grants view).
- Activations always land in month of `activated_at`; transaction committing just after midnight with `activated_at` before midnight updates previous period row (still allowed: period row lock works regardless of status; close job snapshot waits behind lock). Close job re-reads counters at invoice time (2nd 06:00 UTC), not at 00:05.
- Roll-over: at invoice time, `payable_pre_tax < $5.00` and fewer than 2 prior consecutive rolled periods pending: invoice row `rolled_over`; amount carried to next invoice as own lines ("October usage (rolled over)"). Third period issues invoice regardless of amount (max 3 months). $0 periods: invoice row `zero`, nothing sent; zero periods neither count toward roll-over chain nor reset it.
- Account closed (workspace deleted) or switched to contract: issue immediately regardless of amount (Stripe minimum $0.50 charge: amounts under $0.50 written off, row `zero`, `written_off_cents` recorded).

## 12. Replay engine (`replay.go`) and simulation

`Replay(history) []LedgerLine`: history = ordered list of facts for workspace (messages minimal fields, spam marks/unmarks with times, merges, delivery updates, contact block times, test flags, membership email matches over time). Applies same rule set functions in time order. Cap/card states replayed from `billing_accounts` history (audit log) when verifying; simulation ignores caps (reports "uncapped"). Replay unit = workspace (ledger lines per workspace); account counters verified as `SUM` of member workspaces' replayed lines.

Uses:
- `dimmi billing:simulate --workspace ws_... --period 2026-10 [--rules 2026-11-01] [--price-book 2026-11-01] [--json|--csv]`: works on self-host and cloud; prints count, lines per reason, amount. Self-host operators can verify "what cloud would charge".
- `VerifyLedgerReplay` nightly: compare online ledger vs replay for workspaces active yesterday (all when under 10k, else 10% sample + all with credits/disputes). Mismatch: `billing_reconciliation_issues` kind `replay_mismatch`; never auto-fix.
- Property test: random event streams (messages, spam toggles, merges, delivery failures, concurrent replies) applied online (with real DB) and via replay produce identical ledger (ignoring cap blocks, which replay records as "would block").

## 13. Versioning

- `RuleSet` interface: `Version()`, `EffectiveFrom()`, `AnnouncedAt()`, `IsActivity`, `IsQualifyingInbound`, `IsWorkspaceReply`, `FreeReason`, `ClosesAt`, `SpamCreditWindow`, `MergeWindow`, ...
- Registry test: every version's `EffectiveFrom >= AnnouncedAt + 60 days` (except initial). Old versions never deleted (replay of old windows).
- Window stores `rule_version` at activation; credits evaluated with charge window's version.
- Price book versions same pattern; account `price_book_version` pinned; price increase: accounts keep old version until `price_book_locked_until` (activation date of new book + 12 months) per guardrail 8. Price decrease applies to all accounts at effective date.
- Public endpoint `GET /api/v1/billing/pricing` exposes current + scheduled versions and effective dates.

## 14. Scenario test vectors (`tests/feature/billing/testdata/*.yaml`)

Run through real App + Postgres (testing.md): each vector builds facts with factories and 03/01/04 actions, `MeterMessageActivity` runs in real reply tx, ledger asserted, then `Replay` over same facts asserted equal. Test named in plain words (`t.Run("bills a changelog reply once", ...)`).

| # | Timeline (UTC) | Expected ledger |
|---|---|---|
| 1 | contact msg d0 10:00; member reply d0 10:05 | charge d0 10:05 |
| 2 | contact msg; AI reply; member reply same day | one charge, actor agent |
| 3 | contact msg; no reply ever | nothing |
| 4 | auto-ack (substantive false) only | nothing |
| 5 | member starts thread d0; contact replies d1; member answers d1 | charge d1 |
| 6 | member starts thread; no contact reply; member follow-ups | nothing |
| 7 | charge d0; messages d3, d9; contact msg d15; member reply d15 | one charge only (window extended to d16; d15 inside) |
| 8 | charge d0; last msg d2; contact msg d10; reply d10 | second charge d10 (window closed d9) |
| 9 | charge d0; daily messages to d31; contact msg d29; reply d31 | window closed d30 (max age); inbound d29 before close -> no new charge at d31; contact d32 + reply d32 -> charge |
| 10 | charge d0 (Jan 31 23:00); messages continue Feb | one charge, period January |
| 11 | charge; spam marked d10 | credit spam |
| 12 | charge; spam marked d15 | no credit |
| 13 | charge; spam d3; unmark d4 | credit, then reversal (net 1) |
| 14 | conversation spam before reply; reply; unmark | deferred activation at unmark, reason spam_unmarked |
| 15 | two threads same contact, charges d0 and d5; merged | one credit (later), reason merge |
| 16 | charges d0 and d9 same contact; merged | no credit |
| 17 | charges d0, d5, d11 same contact; merged | credit d5 only (d11 anchors new cluster) |
| 18 | email reply hard-bounces, only reply in window | credit failed_delivery |
| 19 | bounce credit, then second reply delivered | reversal |
| 20 | contact is member (verified email match) | waiver internal |
| 21 | test_mode workspace, 1,001st activation in period | block test_mode_limit_reached |
| 22 | cap $50, 2,533 charges, 2,534th | waiver cap_buffer; 3,041st blocked spend_cap_reached |
| 23 | no payment method, activations 1-100, then 101st, then 121st | 1-100 charge lines at $0 (free tier); 101st-120th waiver `cap_buffer`; 121st blocked `payment_method_required` |
| 24 | blocked contact writes, member replies | nothing |
| 25 | imported messages only, then live contact msg + reply | one charge at live reply |
| 26 | two members reply concurrently to fresh inbound | exactly one charge (lock + recheck) |
| 27 | cap raised from $50 to $100 after 2,600 activations (67 buffer waivers used) | next activation charges (billable 2,533 < new cap_count); 67 waivers stay free |
| 28 | spam credit frees headroom while capped | next activation charges (billable_count dropped below cap_count) |


## Echo replies

Later (`teammate-echo`, 05 X2). MVP: teammate native Slack/Discord messages ignored, never ledger. Test vector 29 retired (numbers 30-33 kept).

## Launch feature replay fixtures (07, 13, 14, 15, 16)

Same harness as section 14 (real App + Postgres, then replay parity).

| # | Fixture | Timeline | Expected ledger |
|---|---|---|---|
| 30 | `changelog_reply_billing` | entry published with mailing to 500 subscribers; 20 reactions; 5 new subscribers (mail replies into inbox later: `changelog-replies-to-inbox`) | nothing (mailing, reactions, subscriptions) |
| 31 | `feedback_activity_not_billable` | post, 50 votes, comments (public + internal), request linked to existing conversation, status change with notify (voter mail), AI capture on `llm.Fake()` | nothing; linked conversation ledger unchanged |
| 32 | `csat_request_not_billable` | conversation closed; 15 sends CSAT request (system, `substantive false`) via widget + in-thread email (`outbound_emails.kind system`); contact answers survey | nothing (no new window, no activation) |
| 33 | `announcement_reply_billing` | chat-format announcement shown to 1,000 visitors; 3 reply (contact-initiated conversations); member answers 1 | one charge; impressions/receipts nothing |
| 34 | `import_never_billed` | full 16 import of 1,000 conversations (contact + teammate + bot messages, closed and open), then delta run, then live inbound widget message on imported conversation + member reply | exactly one charge (live reply); import + delta nothing |
| 35 | `agent_action_not_billable` | agent read action answer (answered), write action: confirmation card, customer confirms, call succeeds, follow-up answer; second write declined | one charge (first substantive AI answer in window, D6); cards, notices, action calls, decline nothing |

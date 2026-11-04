# 12 Cloud billing: honest metering

Style: caveman. Scope trim 2026-11-02: survey public-link cap seam (later: `hosted-survey-page`), echo-reply waiver (later: `teammate-echo`), changelog reply fixture clause (later: `changelog-replies-to-inbox`) removed. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`, `research/pricing.md`. Extra files: [metering-rules.md](metering-rules.md) (D1-D15 as code, rating, cap math, replay, test vectors), [stripe.md](stripe.md) (Stripe objects, invoices, webhooks, dunning, credit notes, reconciliation).

Infrastructure (EC2 + RDS + ElastiCache, CloudFormation): [infrastructure.md](infrastructure.md).

## Goal

Dimmi Cloud charges per billable conversation exactly as `research/pricing.md` defines: 100 free per month, graduated $0.03 / $0.015 / $0.01, caps on by default, every charge traceable to thread. Module runs only when `app.edition = cloud`; counting code lives open source in main repo and runs on self-host as simulator.

Why: roadmap launch item 12 ("Hosted cloud with honest metering"), `research/roadmap.md`. Revenue; pricing is top complaint against every competitor; surprise bills kill trust.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Every ledger line links to thread, rule version, actor, reason | 100%; CSV + API export |
| Online ledger equals offline replay of same history | 0 mismatches in property tests (10k random streams per CI run); nightly prod verifier, alert on any |
| Invoice math equals published examples | $0 / $12 / $87 / $162 / $312 / $562 rating table tests (pure, no mocks) |
| Stripe subtotal equals Dimmi subtotal | exact cents, every invoice (void + alert otherwise) |
| Invoice never exceeds cap | guaranteed by decision function (buffer free, block after); only exception documented: credit reversals |
| Live meter freshness | under 2 s after activation (realtime nudge), endpoint p95 under 30 ms |
| Activation overhead on reply path | p95 under 5 ms added inside reply transaction |
| Messages dropped due to billing | zero. Inbound always stored; blocked only: new activations |
| Alerts | 50/80/100% + forecast + anomaly, delivered under 60 s after crossing |
| Self-serve dispute | auto-credit under $50 per 30 days, under 1 s |
| Card data | never touches Dimmi servers (Stripe Checkout/Portal; PCI SAQ A) |
| Stripe outage | zero impact on messaging; invoices retry; meter keeps working (Dimmi ledger is truth) |

## Scope

### In (MVP)

- Billable conversation rules D1-D15 as pure Go package + online in-transaction meter + replay engine.
- Append-only `usage_ledger`, `billing_windows`, `billing_periods` counters.
- Billing account owned by organization (`00-foundation/extensibility.md` sections 3, 6, 13): periods, caps, credits, invoices, programs keyed by `billing_account_id`; resolve path workspace -> organization -> billing account (cached 60 s). Metering stays per workspace conversation (`usage_ledger`, `billing_windows` keep `workspace_id`). MVP: one org per workspace, so one account per workspace in practice.
- `billing_accounts.currency` (MVP `usd` only); every `*_cents` / `*_micros` = minor units (or micro-units) of account currency.
- Graduated price book `2026-11-01` (keyed by version + currency), contract tiers per billing account, percent discount, credit grants (startup $300 / 12 months), OSS allowance (2,000 free), nonprofit 50%.
- Cloud `Entitlements` driver (contracts.md) + `entitlement_overrides` table + `dimmi entitlement:set` (staff raises one customer's caps without deploy).
- Card required past 100 (20 free buffer, then block with `payment_method_required`).
- Spend caps: default $50 on first payment method, alerts 50/80/100% + forecast + anomaly, 20% buffer (min 20) free, then activations paused; precise over-cap behavior below.
- Monthly invoicing via Stripe one-off invoices + Stripe Tax, roll-over under $5 (max 3 months), Checkout setup mode for cards, Customer Portal for card changes and invoice history.
- Dunning: retries, `past_due`, `restricted` after 14 days, never data loss.
- Usage API: meter, periods, lines (filters, cursor), CSV export, per-conversation billing view.
- Disputes (self-serve, auto-credit under $50), staff credits, credit notes, refunds, program applications (OSS, startup, nonprofit).
- Reconciliation job (ledger vs Stripe, counters vs ledger, online vs replay).
- `metered` middleware (cloud).
- Transparency data: public pricing endpoint (incl. `included` list: "Changelog, feedback board and roadmap, announcements and surveys included"), calculator estimate, "Fin would have charged" comparator.
- Not-metered contract for launch features 13, 14, 15 (metering-rules.md section 1 "Not activity"): changelog, feedback/roadmap, announcements/surveys never create ledger lines; replay fixtures prove it.
- `BillingStatus.HasPaid` for 01 badge.
- `dimmi billing:simulate` (all editions).

### Out (later)

- Prepaid annual credits with bonus (10% at $1,000, 20% at $10,000): `credit_grants.kind = prepaid` reserved; purchase flow later.
- Carrier pass-through billing (WhatsApp Meta fees, SMS): separate meter later; MVP: WhatsApp fees billed by Meta directly to customer's own WABA (05 uses customer's Meta account).
- Dimmi AI credits at cost (pricing.md section 5 "Later").
- Merchant of record (Stripe Managed Payments, Polar): revisit when tax registrations multiply.
- Second workspace sharing billing account: later, when org UI ships (`POST /workspaces {organization_id}`); second workspace in same org shares account, periods, cap, credits (no schema change). MVP: every workspace gets new org, so new account (extensibility.md open question 1).
- Annual contract invoicing automation (commit drawdown): staff handles in Stripe manually; Dimmi rates usage at contract tiers.
- EU region billing entity.
- Pricing page UI and calculator UI (frontend later; data endpoints here).
- Migration promo (3 months free for Fin switchers): implementable as credit grant by staff; no automation.
- `hosted-survey-page` (audit C12): `HasPaid`-gated daily cap for 15 public survey links (`engage.link.public_daily_limit_unpaid`): later with 15 hosted page.
- `teammate-echo` (audit B4): over-cap waiver for Slack/Discord echo replies (vector 29): later with 05 echo.

## Over-cap behavior (precise)

Applies when activation decision is Block (`spend_cap_reached`, `payment_method_required`, `billing_restricted`, `test_mode_limit_reached`, `billing_locked`). "Activation" = workspace reply that would open new billing window (metering-rules.md section 3).

| Thing | Behavior when blocked |
|---|---|
| Inbound customer message (any channel, new or existing conversation) | always stored, conversation created/reopened, routed, realtime, teammate notifications, webhooks. Never blocked, never dropped |
| Teammate reply in conversation with open billing window | allowed (already paid for) |
| Teammate reply that would activate | rejected before persist: 423 `spend_cap_reached` (or other code); draft kept client-side; response includes `billing` block with state + `manage_url` |
| Teammate follow-up with no qualifying inbound since last window | allowed (not activation) |
| Internal notes, tags, assign, close, snooze | allowed |
| Non-substantive messages (auto-ack, business-hours notice, 15 CSAT request = `author_type system`, `substantive false`; in-thread mail `outbound_emails.kind system` via Channel.Send) | allowed |
| 13/14 non-conversation mail via 04 `SendContactMail` (`kind` `changelog`, `feedback_update`, `portal_sign_in`) | allowed; not activity, never gated |
| Dimmi Agent (07) | `PreflightActivation` before any LLM call: would-block means no LLM call (no token spend), run `skipped` reason `billing_paused`; if `ai_state=active`, 07 sends its handoff notice (non-substantive); event message in timeline "AI paused: workspace usage limit reached" (type event, teammates only) |
| External AI agent / API token / MCP / CLI reply (`author_type=member` with contracts.md `Via` `personal_token` or `oauth_client`; or `integration`) | same 423 as teammates |
| MCP `dimmi_send_reply` | tool error `spend_cap_reached` with manage URL |
| Channel outbound for allowed messages | normal |
| Widget | unaffected (customer side); expected-reply text unchanged |
| After cap raised / card added / new month / paid | `billing.activations_resumed`; nothing auto-sent by billing; 07 re-triggers AI for conversations with `billing_paused` skips in last 24 h whose last message is from contact |
| Reversal of earlier credit | applies even over cap (never blocked) |
| Deferred activation on un-spam | over limit: waiver `over_cap_deferred` (free), never blocks teammate action |

## User stories

- Workspace owner: "Card asked only after 100 conversations; first 20 extra free while I add it."
- Owner: "Header shows `312 conversations, $10.60, on pace for $31`. Alert at 80% of $50 cap. I raise cap in one click."
- Owner: "I click any invoice line and see exact thread, who answered (AI or Ana), and why it counted."
- Owner: "Spam thread got charged; I mark spam within 14 days, credit appears instantly."
- Owner: "Dispute a charge; under $50 auto-credited."
- Member: "Reply blocked with clear message who can raise cap. Inbound still arrives."
- Developer: "`GET /api/v1/billing/usage-lines?filter[period]=2026-10` gives every line; CSV for finance."
- Self-hoster: "`dimmi billing:simulate --period 2026-10` shows what cloud would charge."
- AI agent (MCP): "`dimmi_get_billing_meter` before bulk replies; respects pause."
- Dimmi staff: "Grant OSS allowance, startup credits, refund via API; reconciliation shows zero drift."

## Data model

Two scopes:
- Account-scoped (owned by organization through `billing_accounts`): `billing_accounts`, `spend_caps`, `billing_periods`, `invoices_cache`, `credit_grants`, `credit_grant_applications`, `billing_adjustments`. Keyed by `billing_account_id`, no `workspace_id` (annotated `-- lint:org-scoped`, Contradictions 7). Free allowance, tiers, cap, buffer, alerts computed per account.
- Workspace-scoped (attribution): `usage_ledger`, `billing_windows`, `billing_disputes`, `billing_program_applications`. Keep `workspace_id`, add `billing_account_id`.

**No FK cascade** on `usage_ledger`, `invoices_cache`, `billing_accounts`, `billing_adjustments`, `credit_grants`, `credit_grant_applications` (tax/audit retention 7 years after workspace and org purge; explicit exception to conventions, see Contradictions 3). `billing_accounts.organization_id` has no FK (org row soft-deleted, kept while billing retention needs it). `usage_ledger` also has no `updated_at` (append-only; second explicit exception). Personal data on these rows limited to billing name/email/address on `billing_accounts` (anonymized when org soft-deleted after last workspace purge: name "Deleted organization", email null, address country only). Other billing tables cascade normally (`spend_caps`, `billing_periods` reference `billing_accounts`, never deleted). RLS on all (cloud): workspace-scoped tables use `dimmi_visible_workspaces()` (security.md); account-scoped tables use `billing_account_id = ANY (dimmi_visible_billing_accounts())` (SQL STABLE SECURITY DEFINER, `search_path` pinned: accounts of orgs owning `dimmi_visible_workspaces()`). Same one-function pattern: org-level reads later change nothing.

### billing_accounts
| Column | Type | Notes |
|---|---|---|
| id | uuid | public `ba_` |
| organization_id | uuid not null | no FK (retention). Unique index at MVP (one account per org); droppable later for cost centers (several accounts per org) |
| currency | text not null default 'usd' CHECK (currency ~ '^[a-z]{3}$') | ISO 4217 lower-case. Set at creation from `billing.currency`; immutable after first Stripe invoice (Stripe fixes customer currency); staff may change before (`system.billing.accounts.update`). MVP price books: `usd` only |
| status | text CHECK in ('free','active','past_due','restricted','locked','closed') default 'free' | `free` = no payment method ever |
| stripe_customer_id | text unique | |
| has_payment_method | boolean not null default false | |
| payment_method | jsonb | `{brand,last4,exp_month,exp_year,type}` display only |
| billing_email | citext | default owner email at creation |
| billing_name | text | |
| address | jsonb | `{line1,line2,city,state,postal_code,country}` |
| tax_status | text CHECK in ('supported','unrecognized_location','not_collecting','failed') | from Stripe |
| tax_ids | jsonb not null default '[]' | `[{id,type,value,verification}]` |
| collection_method | text CHECK in ('charge_automatically','send_invoice') default 'charge_automatically' | |
| free_allowance | int not null default 100 | OSS 2,000 |
| discount_percent | smallint not null default 0 CHECK 0-100 | nonprofit 50 |
| program | text CHECK in ('none','oss','startup','nonprofit','enterprise') default 'none' | |
| contract | jsonb | `{tiers:[{up_to,unit_micros}], days_until_due, commit_cents, starts_at, ends_at, notes}` |
| price_book_version | text not null | pinned |
| price_book_locked_until | timestamptz | grandfathering |
| past_due_since, restricted_at, locked_at, closed_at | timestamptz | |
| first_paid_at | timestamptz | set once when first invoice `paid` (never cleared, even after refund); drives `BillingStatus.HasPaid` |
| review_flag | text | `spam_credit_ratio`, `dispute_volume`, `chargeback` |
| review_flagged_at | timestamptz | |
| created_at, updated_at | timestamptz | |

### spend_caps
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| billing_account_id | uuid unique not null references billing_accounts on delete cascade | one cap per account |
| amount_cents | bigint null CHECK (amount_cents >= 0) | null = no cap (`billing:cap_unrestricted`). Minor units of account currency |
| thresholds | smallint[] not null default '{50,80,100}' | 1-100, max 5 values, sorted unique |
| set_by_type | text CHECK in ('default','member','staff') | |
| set_by_id | uuid | |
| created_at, updated_at | timestamptz | |

### billing_periods
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| billing_account_id | uuid not null references billing_accounts on delete cascade | |
| period | date not null | first day of month UTC; unique `(billing_account_id, period)` |
| status | text CHECK in ('open','closed','invoiced','rolled_over','zero') | |
| billable_count | int not null default 0 | charges + reversals - credits, all workspaces of account |
| charge_count, credit_count, reversal_count | int not null default 0 | |
| waived_buffer_count, waived_test_count, waived_internal_count, waived_deferred_count | int not null default 0 | |
| cap_amount_cents | bigint | snapshot of cap used for cap_count |
| cap_count | int | null = unlimited |
| buffer_count | int not null default 20 | |
| activations_paused_at | timestamptz | first block in period |
| paused_reason | text | |
| alerts_sent | jsonb not null default '{}' | `{"50":"ts","80":"ts","forecast":"ts","anomaly":["2026-11-03"]}` |
| pricing_snapshot | jsonb | set at close |
| invoice_id | uuid | |
| created_at, updated_at | timestamptz | |
Row lock target for activations (one row per account per month: every workspace of account serializes on it). Index `(status, period)`.

### billing_windows
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid not null references workspaces on delete cascade | |
| billing_account_id | uuid not null | account at activation (no FK; attribution) |
| conversation_id | uuid not null references conversations on delete cascade | |
| seq | int not null | unique `(conversation_id, seq)` |
| status | text CHECK in ('open','closed') | |
| result | text CHECK in ('charged','waived') | |
| activated_at | timestamptz not null | |
| activating_message_id | uuid not null | |
| ledger_entry_id | uuid not null | |
| last_activity_at | timestamptz not null | |
| closes_at | timestamptz not null | min(activated + 30d, last_activity + 7d) |
| closed_at | timestamptz | |
| close_reason | text CHECK in ('quiet','max_age','merged','conversation_deleted') | |
| rule_version | text not null | |
| created_at, updated_at | timestamptz | |
Indexes: `(workspace_id, conversation_id, seq desc)`, partial `(closes_at) WHERE status = 'open'`.

### usage_ledger (append-only, partitioned by `period`)
| Column | Type | Notes |
|---|---|---|
| id | uuid | public `ul_` |
| workspace_id | uuid not null | no FK; metering stays per workspace conversation |
| billing_account_id | uuid not null | no FK; account at activation (credits copy it from charge) |
| period | date not null | partition key; activation month |
| entry_type | text CHECK in ('charge','credit','waiver') | |
| quantity | smallint CHECK (charge 1, credit -1, waiver 0) | composite CHECK with entry_type |
| reason | text CHECK in ('activation','spam_unmarked','credit_reversed','spam','merge','failed_delivery','dispute','support','cap_buffer','over_cap_deferred','test_mode','internal') | |
| conversation_id | uuid not null | no FK |
| billing_window_id | uuid not null | no FK |
| window_seq | int not null | |
| contact_id | uuid | pseudonymous; tombstone id after erase |
| channel_type | text | widget, email, slack, discord, telegram, whatsapp, api |
| actor_type | text CHECK in ('member','agent','integration') | who activated |
| actor_id | uuid | |
| activating_message_id | uuid | |
| activated_at | timestamptz not null | |
| rule_version | text not null | |
| reverses_entry_id | uuid | credit -> charge, reversal -> credit |
| dispute_id | uuid | |
| created_by_type | text CHECK in ('system','member','staff') | |
| created_by_id | uuid | |
| note | text | max 500, staff/dispute |
| idempotency_key | text not null | unique `(workspace_id, idempotency_key, period)` |
| created_at | timestamptz not null default now() | no `updated_at` (append-only) |
Primary key `(id, period)`. Indexes: `(workspace_id, period, created_at, id)`, `(workspace_id, conversation_id, created_at)`, `(billing_account_id, period)` (counter recompute, invoice drill-down), `(reverses_entry_id)`. Trigger `usage_ledger_append_only` raises on UPDATE/DELETE; app role granted INSERT/SELECT only. Partitions monthly, created 3 months ahead by `CreateLedgerPartitions`, plus `usage_ledger_default` partition as safety net (alert when non-empty); retention 7 years then detach + archive to S3 (job later).

### invoices_cache
Dimmi invoice record (amounts truth) mirrored with Stripe (payment status truth).
| Column | Type | Notes |
|---|---|---|
| id | uuid | public `ivc_` (not `inv_`: 00-accounts invitations own it) |
| billing_account_id | uuid not null | no FK |
| status | text CHECK in ('rolled_over','zero','draft','open','paid','past_due','uncollectible','void') | |
| periods | date[] not null | covered periods |
| primary_period | date not null | unique `(billing_account_id, primary_period)` |
| lines | jsonb not null | `[{period,tier,from,to,count,unit_micros,amount_cents,description}]` |
| subtotal_cents, discount_cents, grants_applied_cents, tax_cents, total_cents, amount_paid_cents, amount_remaining_cents, written_off_cents | bigint | |
| currency | text not null | copied from account at creation; every `*_cents` above in this currency |
| collection_method | text | |
| stripe_invoice_id | text unique | |
| stripe_number | text | |
| hosted_invoice_url, invoice_pdf_url | text | |
| rolled_into_invoice_id | uuid | |
| finalized_at, paid_at, next_payment_attempt_at | timestamptz | |
| attempt_count | int not null default 0 | |
| last_payment_error | jsonb | `{code,decline_code,message}` |
| created_at, updated_at | timestamptz | |

### credit_grants, credit_grant_applications
`credit_grants`: `id (cg_), billing_account_id (no FK), kind CHECK in ('startup','promo','support','goodwill','prepaid'), amount_cents (account currency), remaining_cents CHECK >= 0, starts_at, expires_at, granted_by_user_id, note, created_at, updated_at`. Index `(billing_account_id, expires_at)`.
`credit_grant_applications`: `id, credit_grant_id, invoice_id, amount_cents, created_at`. Unique `(credit_grant_id, invoice_id)`.

### billing_adjustments
Late credits against invoiced periods. `id, billing_account_id (no FK), invoice_id, period, ledger_entry_ids uuid[], amount_cents, status CHECK in ('pending','issued','absorbed'), stripe_credit_note_id, created_at, updated_at`.

### billing_disputes
`id (bd_), workspace_id, billing_account_id, usage_ledger_entry_id, period, reason CHECK in ('spam','not_a_conversation','duplicate','test','other'), message text (max 1,000), status CHECK in ('auto_credited','pending_review','accepted','rejected'), created_by_member_id, resolved_by_user_id, resolution_note, created_at, updated_at`. Unique `(usage_ledger_entry_id)` (one dispute per line).

### billing_program_applications
`id (bpa_), workspace_id (applicant workspace), billing_account_id (program applied to account), program CHECK in ('oss','startup','nonprofit'), details jsonb (repo URL, company age, funding, org registration), status CHECK in ('pending','approved','rejected'), reviewed_by_user_id, review_note, created_at, updated_at`. Partial unique `(billing_account_id, program) WHERE status = 'pending'`.

### stripe_events
`id text primary key (evt_...), type text, livemode boolean, payload jsonb, received_at, processed_at, attempts int, last_error text`. Instance-level (no workspace). Pruned after 90 days.

### billing_reconciliation_issues
`id (bri_), workspace_id null, billing_account_id null, kind text, details jsonb, status CHECK in ('open','resolved','ignored'), resolved_by_user_id, resolution_note, created_at, updated_at`. Index `(status, kind)`.

### entitlement_overrides (cloud; `00-foundation/extensibility.md` section 6)
Staff raises or lowers one customer's limits, enables rollout flags. Read by cloud `Entitlements` driver only.
| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `ent_` |
| organization_id | uuid not null references organizations on delete cascade | |
| workspace_id | uuid null references workspaces on delete cascade | narrower than org when set |
| key | text not null | catalog key (`app/support/entitlements/catalog.go`), validated in action; unknown key 422 `unknown_entitlement` |
| value | jsonb not null | bool for kind `feature`, integer for kind `limit` (-1 = unlimited); type checked against catalog kind |
| reason | text not null | staff note, 1..500 |
| granted_by_user_id | uuid null references users on delete set null | staff (instance admin) |
| expires_at | timestamptz | null = permanent; expired rows ignored at read, kept for audit |
| created_at, updated_at | timestamptz | |
Unique `(organization_id, workspace_id, key) NULLS NOT DISTINCT`. Index `(workspace_id) WHERE workspace_id IS NOT NULL`. Global-ish (org-scoped, `-- lint:org-scoped`); no tenant API; staff only via `dimmi entitlement:set`. Every change audit logged (`entitlement.set`, `entitlement.removed`, reason in metadata). Table exists on every edition (schema identical), empty on self-host.

### Core table changes

None. Relies on existing columns: `workspaces.organization_id` (foundation, extensibility.md section 3), `messages.substantive`, `messages.metadata.auto_reply` / `metadata.imported`, `messages.delivery_status`, `conversations.spam`, `conversations.is_test`, `conversations.source.imported`, `contacts.blocked_at`, `workspaces.test_mode`. Uses 11's `users.is_instance_admin` for staff.

Migrations (owned):
- `20261101001201_create_billing_accounts_table.sql`
- `20261101001202_create_spend_caps_table.sql`
- `20261101001203_create_billing_periods_table.sql`
- `20261101001204_create_billing_windows_table.sql`
- `20261101001205_create_usage_ledger_table.sql` (partitioned, trigger, grants)
- `20261101001206_create_invoices_cache_table.sql`
- `20261101001207_create_credit_grants_tables.sql`
- `20261101001208_create_billing_adjustments_table.sql`
- `20261101001209_create_billing_disputes_table.sql`
- `20261101001210_create_billing_program_applications_table.sql`
- `20261101001211_create_stripe_events_table.sql`
- `20261101001212_create_billing_reconciliation_issues_table.sql`
- `20261101001213_create_entitlement_overrides_table.sql`
`...1201` also creates SQL function `dimmi_visible_billing_accounts()` used by RLS on account-scoped tables.
All created on every edition (schema identical everywhere; empty on self-host).

## Code map

```
app/support/metering/                       # pure, open source, no I/O except interfaces
  rules.go                                  # RuleSet interface, registry, RulesAt(t)
  rules_2026_11_01.go
  classify.go                               # IsActivity, IsQualifyingInbound, IsWorkspaceReply, FreeReason
  window.go                                 # ClosesAt, state transitions
  decide.go                                 # Decide(period, account, freeReason)
  pricebook.go, pricebook_2026_11_01.go     # tiers in micros, keyed by (version, currency); MVP usd only
  rating.go                                 # Rate(count, inputs) -> lines, subtotal, discount, grants, payable (inputs carry currency)
  capmath.go                                # CapCount, BufferCount
  forecast.go                               # projection, threshold crossing, anomaly
  replay.go                                 # Replay(history) -> ledger lines
  comparator.go                             # competitor estimates with dated sources
  money.go                                  # micros, cents (minor units of currency), round half up
app/contracts/billing_status.go             # BillingStatusReader interface + BillingStatus struct (internal API, below); domain contract, ids.UUID
app/support/entitlements/cloud/             # cloud Entitlements driver (contracts.md interface; catalog in app/support/entitlements/catalog.go, foundation)
  driver.go                                 # Has/Limit: catalog cloud default -> account program/contract -> entitlement_overrides (workspace row beats org row); L1 in-process (otter) 60 s per workspace, busted via Valkey pubsub
  used.go                                   # cheap `used` counters for GET /workspace/entitlements (registered by owning specs)
app/actions/billing/
  resolve_billing_account.go                # ResolveBillingAccount(ctx, workspaceID): workspace -> organization -> billing account; L1 otter 60 s + Valkey pubsub bust
  ensure_billing_account.go                 # by organization_id (INSERT ... ON CONFLICT (organization_id) DO NOTHING); repair path for pre-hook workspaces
  billing_status.go                         # BillingStatus(ctx, workspaceID): HasPaid etc.; L1 otter 60 s + Valkey pubsub bust; self-host noop driver returns zero value
  meter_message_activity.go                 # online algorithm (called by `RecordBillableActivity` InTx listener)
  preflight_activation.go                   # read-only "would this reply activate / be blocked" (07, UI, MCP)
  record_activation_block.go                # after-rollback pause marker
  credit_ledger_charge.go, reverse_ledger_credit.go
  apply_spam_credits.go, reverse_spam_credits.go, activate_deferred_after_unspam.go
  apply_merge_credits.go
  apply_delivery_credit.go, evaluate_delivery_credit.go
  update_spend_cap.go, recompute_cap_math.go
  update_billing_account.go, add_tax_id.go, remove_tax_id.go
  create_payment_method_session.go, sync_payment_method.go, create_portal_session.go
  build_usage_meter.go, export_usage_lines_csv.go, build_conversation_billing.go
  close_billing_period.go, issue_invoice.go, pay_invoice.go, advance_dunning.go, set_account_status.go
  handle_stripe_event.go                    # dispatch to sync_* actions
  open_dispute.go, resolve_dispute.go
  grant_credit.go, expire_credit_grants.go, issue_credit_notes.go, issue_staff_credit_note.go
  submit_program_application.go, review_program_application.go, apply_program.go, update_contract.go
  evaluate_spend_alerts.go, detect_usage_anomaly.go, review_spam_credit_ratio.go
  guard_outbound_fair_use.go
  reconcile_stripe.go, verify_ledger_replay.go, recompute_period_counters.go
  close_account_for_deleted_workspace.go    # closes only when org has no other live workspace
  estimate_price.go, build_comparison.go
app/actions/entitlements/                   # 12 owns overrides (cloud)
  set_entitlement_override.go, remove_entitlement_override.go   # catalog validation, audit, cache bust
app/services/stripe/                        # adapter zone (framework.md 7.3): only SDK `github.com/stripe/stripe-go/v87` v87.1.0; HTTP via `a.HTTP.StdClient()`, SDK retries off (`MaxNetworkRetries 0`); no stripe-go type in exported API
                                            # customers.go, checkout.go, portal.go, invoices.go, invoice_items.go, coupons.go, credit_notes.go, tax_ids.go, webhooks.go (`webhook.ConstructEvent`, multi-secret loop)
app/http/middleware/metered.go              # replaces foundation stub
app/http/controllers/api/v1/billing/
  billing_account_controller.go             # Show, Update
  billing_tax_id_controller.go              # Store, Destroy
  billing_payment_method_session_controller.go  # Store
  billing_portal_session_controller.go      # Store
  billing_meter_controller.go               # Show
  billing_spend_cap_controller.go           # Show, Update
  billing_usage_controller.go               # Index, Show (per period)
  billing_usage_line_controller.go          # Index (JSON, or CSV stream via `?format=csv` / `Accept: text/csv`), Show
  billing_usage_line_dispute_controller.go  # Store
  billing_dispute_controller.go             # Index
  billing_invoice_controller.go             # Index, Show
  billing_invoice_payment_controller.go     # Store (retry now)
  billing_credit_controller.go              # Index
  billing_program_application_controller.go # Index, Store
  billing_comparison_controller.go          # Show
  billing_pricing_controller.go             # Show (public)
  billing_estimate_controller.go            # Show (public calculator)
app/http/controllers/api/v1/conversation_billing_controller.go     # Show
app/http/controllers/api/v1/system/billing/
  system_billing_account_controller.go      # Index, Show, Update
  system_billing_credit_grant_controller.go # Store
  system_billing_conversation_credit_controller.go # Store
  system_billing_credit_note_controller.go  # Store
  system_billing_dispute_controller.go      # Index, Update
  system_billing_program_application_controller.go # Index, Update
  system_billing_reconciliation_issue_controller.go # Index, Update
app/http/controllers/webhooks/stripe_webhook_controller.go
app/http/requests/billing/ (update_billing_account, store_tax_id, update_spend_cap, store_dispute, store_program_application, index_usage_lines, system_update_billing_account, store_credit_grant, store_conversation_credit, store_credit_note, update_dispute, update_program_application)
app/http/resources/billing/ (billing_account, spend_cap, usage_meter, usage_period, usage_line, invoice, credit_grant, dispute, program_application, conversation_billing, pricing, estimate, comparison, reconciliation_issue)
app/policies/billing_policy.go              # authz.Can(principal, permission) only (Permissions matrix); cap amount limits; no role names
app/exceptions/billing.go                   # ErrActivationBlocked{Reason}, ErrInvalidTaxLocation ...
app/events/billing/ (conversation_activated, conversation_credited, conversation_credit_reversed, threshold_reached, forecast_exceeds_cap, usage_anomaly_detected, activations_paused, activations_resumed, spend_cap_updated, invoice_finalized, invoice_paid, payment_failed, account_status_changed, meter_updated)
app/listeners/
  record_billable_activity.go               # InTx (contracts.md), message.created: calls MeterMessageActivity in 03 reply tx; error aborts tx (423)
  guard_outbound_fair_use.go                # InTx, conversation.created
  apply_billing_credits.go                  # Queued billing: marked_spam, unmarked_spam, conversation.merged, delivery_updated
  evaluate_spend_alerts_on_activation.go    # queued billing, debounced
  broadcast_billing_meter.go                # AfterCommit, coalesced 2 s per account via Valkey key `SET billing:meter:{ba} NX PX 2000` (lost key = one extra nudge, harmless)
  notify_billing_contacts.go                # queued mail
  close_billing_account_on_workspace_deleted.go, reopen_billing_account_on_workspace_restored.go
  bust_widget_branding_on_first_payment.go  # AfterCommit, billing.invoice_paid when first_paid_at just set: 01 widget config cache bust (badge hides) for every workspace of account
  bust_entitlements_cache.go                # AfterCommit of Set/RemoveEntitlementOverride and program/contract update: `pubsub.PubSub` (Valkey) bust per workspace of org; every replica drops L1
app/jobs/ (see Jobs)
app/mail/ (system stream Mailables, owners/admins + billing email only, never contacts (04 outbound.md 15); spend_threshold_mail, spend_forecast_mail, usage_anomaly_mail, activations_paused_mail, card_required_mail, invoice_issued_mail, payment_failed_mail, payment_action_required_mail, account_restricted_mail, credit_grant_expiring_mail, dispute_resolved_mail)
app/notifications/ (BillingThresholdReachedNotification, BillingActivationsPausedNotification, BillingPaymentFailedNotification) # in-app via 06
app/mcp/tools/ (get_billing_meter.go, list_usage_lines.go, get_spend_cap.go, update_spend_cap.go, get_conversation_billing.go)
app/console/commands/ (billing_simulate_command.go, billing_close_period_command.go, billing_issue_invoices_command.go, billing_reconcile_command.go, billing_verify_replay_command.go, billing_recompute_counters_command.go, entitlement_set_command.go, entitlement_list_command.go)
app/providers/billing_service_provider.go   # registered only when app.edition = cloud (simulate command always); binds *App.Entitlements to cloud driver (self-host binds 11 `self_host` driver)
config/billing.go, config/services.go (stripe section)
routes/api.go (billing group, system billing group), routes/webhooks.go (stripe), routes/console.go (schedules)
database/migrations/202611010012{01..13}_*.sql
database/queries/ (billing_accounts.sql, spend_caps.sql, billing_periods.sql, billing_windows.sql, usage_ledger.sql, invoices_cache.sql, credit_grants.sql, billing_adjustments.sql, billing_disputes.sql, billing_program_applications.sql, stripe_events.sql, billing_reconciliation_issues.sql, entitlement_overrides.sql)
database/factories/ (billing_account, usage_ledger_line, billing_window, invoice, credit_grant, entitlement_override)
tests/feature/billing/metering_vectors_test.go (testdata/*.yaml vectors through real App + Postgres: messages via 03 actions, ledger asserted, then replay asserted equal; incl. 13/14/15 fixtures)
tests/feature/billing/rating_test.go (rating, capmath, decide, forecast tables: pure funcs, no mocks, no I/O)
tests/feature/billing/replay_property_test.go (random streams online (real DB) vs replay)
tests/feature/billing/ (every endpoint, reply blocked 423, concurrency, webhooks, dunning, invoices with `httpclienttest.New(tc.App.HTTP)` Stripe fixtures, cross-tenant, two workspaces on one account (forced org, later-path proof), entitlements driver); tests named in plain words, every edge case covered
tests/contracts/stripe_test.go (stripe-mock; adapter only)
```

## Email sending limits

Owner 04 (`outbound.md` 13; tasks 04 M11, track 05 email-sending). 12 supplies cloud `Entitlements` driver and overrides only; 04 enforces. Not billing: nothing here reaches ledger.

| Key | Kind | Cloud default | Over limit |
|---|---|---|---|
| `email.verified_daily` | daily | 2,000 | message stays `pending`, retried next UTC day; 429 `entitlement_limit_reached` on direct send API |
| `email.unverified_daily` | daily | 200 | same; banner says verify domain to raise |
| `email.new_workspace_hourly` | hourly, first 7 days | 50 | retry next hour |
| `email.unverified_relay_workspace_daily` | daily | 200 | widget offline `skipped limited` |
| `email.connected_mailboxes` | count | 25 | 422 `entitlement_limit_reached` at connect |

Auto-pause (not an entitlement): trailing 500 sends, min 100: hard bounce over 5% or complaint over 0.1%: `paused`. Owner requests review (`paused -> review`); ops clears `review -> active` per `outbound.md` 13. Cap too low: staff raises with `dimmi entitlement:set`. Connected mailbox sends skip caps and pause (customer's own account). Staff raise applies within 60 s through entitlement bust.

Fair use D7 stays separate (`GuardOutboundFairUse`).

## Internal API (other specs)

```go
// app/contracts/billing_status.go; *App.BillingStatus. Cloud: app/actions/billing; self-host: noop (zero value, HasPaid false).
type BillingStatusReader interface {
    Status(ctx context.Context, workspaceID ids.UUID) (BillingStatus, error) // ids = github.com/dimmihq/dimmi/framework/ids
}
type BillingStatus struct {
    HasPaid       bool   // first_paid_at set (any invoice paid) OR active contract (contract non-null, now within starts_at..ends_at)
    AccountStatus string // billing_accounts.status
    Restricted    bool   // status restricted or locked
}
```
Consumers: 01 `ResolveBranding` (cloud badge shown only while `!HasPaid`). 15 public survey link cap: later (`hosted-survey-page`). Resolves workspace -> organization -> billing account (`ResolveBillingAccount`). Cached 60 s per workspace; busted on `billing.invoice_paid`, `billing.account_status_changed`, contract update (for every workspace of account). Error: callers fail open to badge shown (never hide badge on error).

### Valkey vs Postgres (Valkey always deployed: 11 decision 18)

- Valkey holds only rebuildable state for 12: L1 cache busts (`pubsub.PubSub` on Valkey) for entitlements, `ResolveBillingAccount`, `BillingStatus`; meter broadcast coalescing key; rate limits (`ratelimit.Limiter` on Valkey: `throttle:api`, `throttle:public`, disputes 10/hour/workspace).
- Postgres only (durable): ledger, windows, period counters, activations (period row lock `FOR UPDATE`), credits, reversals, disputes, invoices, `stripe_events` dedupe (unique event id), reconciliation.
- Valkey is never consulted for a billing decision. `MeterMessageActivity`, `Decide`, credits, `PreflightActivation` read Postgres (+ L1 entitlement values, which fall back to Postgres on miss). Valkey flush or outage changes no charge, waiver or block (edge 123).

### Entitlements (cloud driver)

Interface + catalog owned by foundation (`app/contracts/entitlements.go`, `app/support/entitlements/catalog.go`, extensibility.md section 6). 12 owns cloud driver and overrides.
- `Limit(ctx, workspaceID, key)` / `Has(ctx, workspaceID, feature)`: catalog cloud default, then account `program` / `contract` (`contract.entitlements` jsonb map `{key: value}`, optional), then `entitlement_overrides` (org row, then workspace row; unexpired only). Last match wins. `-1` = unlimited.
- Cache: L1 in-process (otter) 60 s per workspace; Valkey pubsub bust on override set/remove, program or contract change. Valkey down: L1 serves up to 60 s stale, then driver reads Postgres (cache is rebuildable, never truth).
- Unknown key at call site: panic at boot (catalog registry), never at request time.
- Failure (DB down): returns catalog cloud default + error logged; callers enforce default (never unlimited on error).
- Backs `GET /api/v1/workspace/entitlements` (route owned by foundation/00-accounts; `used` counters registered by owning specs).
- Cloud keys owned by specs (01 widgets, 04 email caps, 07 AI concurrency, 08 help centers/knowledge, 12 below). 12 keys:

| Key | Kind | Self-host | Cloud default | Use |
|---|---|---|---|---|
| `billing.test_mode_cap` | limit | -1 (no billing) | 1000 | D10 test waivers per account period (was config) |
| `billing.outbound_fair_use_min` | limit | -1 | 1000 | D7 floor of outbound limit (was config); staff raises per customer via override |

Launch feature keys (07 agent actions, 08 docs sync, 13, 14, 15, 16 importers) resolved by cloud driver like any catalog key (rows owned by those specs; foundation catalog `app/support/entitlements/catalog.go`; staff override via `dimmi entitlement:set`):

| Owner | Key | Cloud default | Self-host |
|---|---|---|---|
| 07 | `ai.actions.max_connections` | 20 | config (0 = -1) |
| 07 | `ai.actions.max_published` | 50 | config (0 = -1) |
| 07 | `ai.actions.calls_hourly` | 2,000/hour (Dimmi egress abuse guard, not money) | config (0 = -1) |
| 08 | `docs_sync.max_sources` | 10 | config (0 = -1) |
| 08 | `docs_sync.runs_daily` | 200/day (GitHub API + queue guard; BYOK tokens) | -1 |
| 13 | `changelog.max_labels` | 20 per changelog | 20 |
| 13 | `changelog.max_sources` | 5 per changelog | 5 |
| 13 | `changelog.email.default_sender_max_recipients` | 50 | -1 |
| 13 | `changelog.email.monthly_recipient_cap` | 10,000 recipients/month (cost guard, not ledgered) | -1 |
| 14 | `feedback.boards` | 25 | -1 |
| 14 | `feedback.posts` | 100,000 | -1 |
| 14 | `feedback.statuses` | 12 | 12 |
| 14 | `feedback.import_rows_per_month` | 100,000 | -1 |
| 15 | `engage.max_live_announcements` | 50 | config (50) |
| 15 | `engage.max_live_surveys` | 20 | config (20) |
| 15 | `engage.max_segments` | 200 | config (200) |
| 16 | `imports.concurrent_runs` | 2 active runs | config (2) |
| 16 | `imports.records_monthly` | 1,000,000/month (staff override for bigger switches, free white-glove) | -1 |
| 16 | `imports.attachment_bytes_monthly` | 100 GB/month | -1 |
| 16 | `imports.public_site_pages` | 5,000 per run | config (5000) |

All are entitlement catalog keys (extensibility.md section 6), never config. None touches metering.

Over limit, every spec: code `entitlement_limit_reached` with `key`, `limit`; status 422 for count caps (resources held), 429 + `Retry-After` for per-period caps (daily/hourly/monthly) (one shared code; replaces feature codes like `help_center_limit_reached`, `source_limit_reached`, board or live-item caps). Not a billing state: never 423, never ledgered.

## HTTP API

Workspace routes: `auth` (`A`: session, first-party app or token; abilities apply to tokens), `workspace`, `throttle:api`. Cloud only (other edition: 404 `feature_unavailable`). Abilities: `billing:read`, `billing:write` (api.md); policies check permissions (Permissions matrix). Paths unchanged and workspace-scoped: each request resolves account via `ResolveBillingAccount` (workspace -> organization -> billing account, cached 60 s). Responses never expose `organization_id` at MVP (extensibility.md open question 2).

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | /api/v1/billing/account | billing.account.show | `billing:read` | account, status, payment method display, tax, program |
| PATCH | /api/v1/billing/account | billing.account.update | `billing:write` | billing name/email/address (Stripe sync, tax validation) |
| POST | /api/v1/billing/account/tax-ids | billing.account.tax-ids.store | `billing:write` | add tax id |
| DELETE | /api/v1/billing/account/tax-ids/{tax_id} | billing.account.tax-ids.destroy | `billing:write` | remove |
| POST | /api/v1/billing/payment-method-sessions | billing.payment-method-sessions.store | `billing:write`; `S` (Stripe Checkout URL opens in browser, return page needs session cookie) | Checkout setup URL |
| POST | /api/v1/billing/portal-sessions | billing.portal-sessions.store | `billing:write`; `S` (Portal URL opens in browser, return page needs session cookie) | Customer Portal URL |
| GET | /api/v1/billing/meter | billing.meter.show | any active member (money fields only with `billing:read`) | live meter |
| GET | /api/v1/billing/spend-cap | billing.spend-cap.show | `billing:read` | cap + thresholds + cap_count |
| PATCH | /api/v1/billing/spend-cap | billing.spend-cap.update | `billing:write` (+ `billing:cap_unrestricted` above limit or null) | set/remove cap, thresholds |
| GET | /api/v1/billing/usage | billing.usage.index | `billing:read` | periods list with totals |
| GET | /api/v1/billing/usage/{period} | billing.usage.show | `billing:read` | period breakdown (tiers, reasons, actors, channels) |
| GET | /api/v1/billing/usage-lines | billing.usage-lines.index | `billing:read` | ledger lines, cursor, filters; `?format=csv` (or `Accept: text/csv`) streams CSV with same filters, no cursor |
| GET | /api/v1/billing/usage-lines/{line} | billing.usage-lines.show | `billing:read` | one line + related lines |
| POST | /api/v1/billing/usage-lines/{line}/disputes | billing.usage-lines.disputes.store | `billing:write`, `idempotent` | dispute |
| GET | /api/v1/billing/disputes | billing.disputes.index | `billing:read` | |
| GET | /api/v1/billing/invoices | billing.invoices.index | `billing:read` | |
| GET | /api/v1/billing/invoices/{invoice} | billing.invoices.show | `billing:read` | lines, Stripe links |
| POST | /api/v1/billing/invoices/{invoice}/payments | billing.invoices.payments.store | `billing:write`, `idempotent` | retry now |
| GET | /api/v1/billing/credits | billing.credits.index | `billing:read` | grants, Stripe balance |
| GET | /api/v1/billing/program-applications | billing.program-applications.index | `billing:read` | |
| POST | /api/v1/billing/program-applications | billing.program-applications.store | `billing:write` | apply OSS/startup/nonprofit |
| GET | /api/v1/billing/comparison | billing.comparison.show | `billing:read` | "Fin would have charged" for period |
| GET | /api/v1/billing/pricing | billing.pricing.show | public, `throttle:public` | price book + rule set versions + `included` (feature list shown on pricing page: "Changelog, feedback board and roadmap, announcements and surveys included"; no per-feature fee, never ledgered) |
| GET | /api/v1/billing/estimate | billing.estimate.show | public, `throttle:public` | calculator `?conversations=5000&program=none` |
| GET | /api/v1/conversations/{conversation}/billing | conversations.billing.show | `conversations:read` (money with `billing:read`) | windows + lines for thread + `preflight {would_activate, would_block, reason}` (`PreflightActivation`; composer warns before send, no money fields) |
| POST | /webhooks/stripe | webhooks.stripe | Stripe signature | events |

Staff routes (`/api/v1/system/billing`, middleware `instance_admin` + 2FA, abilities `system:read|write`):

| Method | Path | Route name | Purpose |
|---|---|---|---|
| GET | /api/v1/system/billing/accounts | system.billing.accounts.index | filter status, review_flag, program, `organization_id`, `workspace_id` (resolves org) |
| GET | /api/v1/system/billing/accounts/{account} | system.billing.accounts.show | includes `organization {id, name}`, `workspaces [{id, name}]`, active entitlement overrides |
| PATCH | /api/v1/system/billing/accounts/{account} | system.billing.accounts.update | program, free_allowance, discount_percent, contract (incl. optional `entitlements` map), currency (only before first Stripe invoice, else 422 `currency_locked`), lock/unlock, clear review flag, price book pin |
| POST | /api/v1/system/billing/accounts/{account}/credit-grants | system.billing.accounts.credit-grants.store | grant credits |
| POST | /api/v1/system/billing/accounts/{account}/conversation-credits | system.billing.accounts.conversation-credits.store | credit ledger line `support` |
| POST | /api/v1/system/billing/accounts/{account}/credit-notes | system.billing.accounts.credit-notes.store | credit note / refund |
| GET | /api/v1/system/billing/disputes | system.billing.disputes.index | pending review |
| PATCH | /api/v1/system/billing/disputes/{dispute} | system.billing.disputes.update | accept / reject |
| GET | /api/v1/system/billing/program-applications | system.billing.program-applications.index | |
| PATCH | /api/v1/system/billing/program-applications/{application} | system.billing.program-applications.update | approve / reject (applies program) |
| GET | /api/v1/system/billing/reconciliation-issues | system.billing.reconciliation-issues.index | |
| PATCH | /api/v1/system/billing/reconciliation-issues/{issue} | system.billing.reconciliation-issues.update | resolve / ignore |

Spend cap limits: `billing:write` may set cap 0 to `billing.admin_cap_limit_cents` (100,000 = $1,000) and lower any cap; `billing:cap_unrestricted` (owner via roles.go) may set any amount or `null` (no cap). Without it, raising above limit or removing: 403 `owner_required` (code kept; message names permission).

### Examples

`GET /api/v1/billing/meter`
```json
{
  "data": {
    "object": "usage_meter",
    "period": "2026-11",
    "period_ends_at": "2026-12-01T00:00:00Z",
    "currency": "usd",
    "state": "ok",
    "conversations": { "billable": 312, "free_allowance": 100, "free_remaining": 0, "credited": 4, "waived_buffer": 0, "waived_test": 12, "waived_internal": 3 },
    "amount_cents": 636,
    "forecast": { "conversations": 1040, "amount_cents": 2760, "available": true },
    "spend_cap": { "amount_cents": 5000, "cap_count": 2533, "buffer_count": 507, "percent_used": 12.7 },
    "account_status": "active",
    "server_time": "2026-11-14T09:12:00Z"
  }
}
```
`state`: `ok`, `warning` (>= 80%), `buffer` (cap reached, buffer in use), `paused` (blocked), `card_required` (no card, past free), `restricted`, `test_mode`. Members without billing read: `amount_cents`, `forecast.amount_cents`, `spend_cap` null.

`PATCH /api/v1/billing/spend-cap` `{ "amount_cents": 10000, "thresholds": [50, 80, 100] }` 200:
```json
{ "data": { "object": "spend_cap", "amount_cents": 10000, "currency": "usd", "thresholds": [50,80,100], "cap_count": 5866, "buffer_count": 1174, "state": "ok", "updated_at": "2026-11-14T09:13:00Z" } }
```

`GET /api/v1/billing/usage-lines?filter[period]=2026-11&filter[entry_type]=charge&limit=2`
```json
{
  "data": [
    {
      "id": "ul_01jcx...",
      "object": "usage_line",
      "entry_type": "charge",
      "reason": "activation",
      "quantity": 1,
      "period": "2026-11",
      "activated_at": "2026-11-14T09:01:12Z",
      "conversation": { "id": "conv_01jcw...", "object": "conversation", "number": 1042, "deleted": false, "links": { "self": "https://app.trydimmi.com/api/v1/conversations/conv_01jcw..." } },
      "contact": { "id": "ctc_01jc...", "object": "contact" },
      "channel_type": "widget",
      "actor": { "type": "agent", "id": "aia_01jc..." },
      "window": { "seq": 1, "status": "open", "closes_at": "2026-11-21T09:05:40Z" },
      "rule_version": "2026-11-01",
      "credit": null,
      "unit_price_micros": 30000,
      "disputable": true
    }
  ],
  "meta": { "next_cursor": "eyJ...", "prev_cursor": null, "per_page": 2 }
}
```
Filters: `period`, `entry_type`, `reason`, `actor_type`, `channel_type`, `conversation`, `created_after`, `created_before`. Sort `created_at` / `-created_at`. `unit_price_micros` = marginal band at line's ordinal (informational; invoice rates net count).

CSV columns: `line_id,period,entry_type,reason,quantity,activated_at,conversation_id,conversation_number,conversation_url,contact_id,channel_type,actor_type,actor_id,window_seq,rule_version,reverses_line_id,created_at`. RFC 4180, UTF-8, header row, formula-injection guard (cells starting `=,+,-,@` prefixed with `'`).

`POST /api/v1/billing/usage-lines/{line}/disputes` `{ "reason": "spam", "message": "Bot traffic" }` 201:
```json
{ "data": { "id": "bd_01jc...", "object": "billing_dispute", "status": "auto_credited", "credit_line_id": "ul_01jcy...", "created_at": "..." } }
```

Reply blocked (from 03 endpoint `POST /api/v1/conversations/{id}/replies`) 423:
```json
{
  "message": "Workspace reached its monthly spend cap. New conversations are captured; replies that start a new billable conversation are paused until an owner or admin raises the cap.",
  "code": "spend_cap_reached",
  "billing": { "state": "paused", "spend_cap_cents": 5000, "manage_url": "https://app.trydimmi.com/settings/billing", "can_manage": false }
}
```

Error codes: `spend_cap_reached`, `payment_method_required`, `billing_restricted`, `billing_locked`, `test_mode_limit_reached` (all 423), `outbound_fair_use_exceeded` (429), `owner_required` (403), `currency_locked` (422, staff), `unknown_entitlement` (422, staff command), `invalid_tax_location` (422), `billing_address_required` (422), `stripe_unavailable` (503, payment session/portal only), `line_not_creditable` (422), `line_already_disputed` (409), `dispute_window_closed` (422, lines older than 90 days), `invoice_not_payable` (409), `feature_unavailable` (404, non-cloud).

## MCP tools and CLI

MCP (`app/mcp/tools`, 09 `billing` toolset: opt-in via `/mcp?toolsets=billing`, cloud only, hidden without `billing:read`; `dimmi_update_spend_cap` uses 09 `app/mcp/confirm` gate): `dimmi_get_billing_meter` (read; money fields only when token effective permissions include `billing:read`), `dimmi_list_usage_lines` (read, same filters), `dimmi_get_conversation_billing` (read), `dimmi_get_spend_cap` (read), `dimmi_update_spend_cap` (write; requires `billing:write` and argument `confirm: true`, else returns preview of new cap_count without applying).

CLI:
| Command | Edition | Purpose |
|---|---|---|
| `dimmi billing:simulate --workspace ws_ --period 2026-10 [--rules v] [--price-book v] [--json\|--csv]` | all | replay rules over stored messages, print count, lines, amount |
| `dimmi billing:close-period --period 2026-10 [--account ba_\|--workspace ws_]` | cloud | manual run of close (idempotent); `--workspace` resolves its account |
| `dimmi billing:issue-invoices --period 2026-10 [--account ba_\|--workspace ws_] [--dry-run]` | cloud | manual invoice run |
| `dimmi billing:reconcile [--since 2026-10-01]` | cloud | run reconciliation now |
| `dimmi billing:verify-replay --workspace ws_ [--period]` | cloud | compare ledger vs replay (per workspace: metering unit) |
| `dimmi billing:recompute-counters (--account ba_\|--workspace ws_) --period` | cloud | rebuild account period counters from ledger (`SUM` over `billing_account_id`) |
| `dimmi entitlement:set (--org org_\|--workspace ws_) --key k (--value v\|--unset) --reason "..." [--expires 2027-01-01]` | cloud | staff override; validates key + value type against catalog; audit logged (actor system via cli); busts cache |
| `dimmi entitlement:list (--org org_\|--workspace ws_) [--json]` | all | effective values with source (`default`, `config`, `program`, `contract`, `override`) |

## Events

Emitted:
| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| billing.conversation_activated | no | yes | existing catalog row; payload line resource (charge or waiver, `billable` flag) |
| billing.threshold_reached | yes (`workspace:{ws}`) | yes | existing; `{percent, amount_cents, cap_cents}` |
| billing.conversation_credited | no | yes | new; credit line + reason |
| billing.conversation_credit_reversed | no | yes | new |
| billing.forecast_exceeds_cap | yes | yes | new; once per period |
| billing.usage_anomaly_detected | yes | yes | new; once per UTC day |
| billing.activations_paused | yes | yes | new; `{reason}` |
| billing.activations_resumed | yes | yes | new; `{reason: cap_raised, payment_method_added, period_started, account_restored}` |
| billing.spend_cap_updated | yes | yes | new; from/to |
| billing.invoice_finalized | no | yes | new |
| billing.invoice_paid | no | yes | new |
| billing.payment_failed | yes | yes | new |
| billing.account_status_changed | yes | yes | new; from/to, reason |
| billing.meter_updated | yes (coalesced 2 s via Valkey key, counts only, no money) | no | new; nudges dashboard to refetch meter |
Realtime (`framework/broadcasting`, centrifuge) on `workspace:{ws}` channel; money fields never broadcast (channel has members without billing access); clients refetch REST.
Account-level events (threshold, forecast, anomaly, paused, resumed, cap, invoice, payment, account status) are dispatched once per live workspace of account (`workspace_id` set per copy; MVP exactly one). Activation and credit events carry the line's own `workspace_id`.

Consumed:
| Event | Listener | Mode |
|---|---|---|
| message.created | RecordBillableActivity (calls action MeterMessageActivity) | `InTx` (contracts.md): same tx as 03 reply; ledger + window + counters written in that tx; Block returns `ErrActivationBlocked`, aborts whole reply, 423 `spend_cap_reached` (or other block code) |
| conversation.created | GuardOutboundFairUse | `InTx` (when `initiated_by` in (member, agent, api)) |
| conversation.marked_spam / unmarked_spam | ApplyBillingCredits | `Queued` `billing` |
| conversation.merged (03) | ApplyBillingCredits | `Queued` `billing` |
| message.delivery_updated | ApplyBillingCredits | `Queued` `billing` |
| billing.conversation_activated | EvaluateSpendAlertsOnActivation | queued `billing`, unique per billing account per 30 s |
| workspace.deleted (00-accounts) | CloseBillingAccountOnWorkspaceDeleted | queued `billing`: closes account + final invoice only when org has no other live workspace (MVP: always) |
| workspace.restored (00-accounts) | ReopenBillingAccountOnWorkspaceRestored | queued `billing`: account back to prior status if final invoice not yet issued; else `active` with new period |

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| ProcessStripeWebhook | billing | webhook persisted | 10, backoff to 6 h | event id |
| ApplyBillingCredit (ApplyBillingCredits listener worker) | billing | listener | 10 | event id |
| EvaluateDeliveryCredit | billing | delayed 1 h | 5 | window id |
| EvaluateSpendAlerts | billing | listener + hourly sweep | 5 | billing account, 30 s |
| DetectUsageAnomalies | billing | hourly | 3 | hour |
| SweepBillingWindows | billing | every 10 min: close windows with `closes_at < now` | 3 | window of time |
| OpenBillingPeriods | billing | 00:00 UTC 1st: create period rows for accounts active last 3 months, recompute cap math, emit resumed for paused | 5 | period |
| CloseBillingPeriods / CloseBillingPeriod | billing | 00:05 UTC 1st, fan-out per billing account | 5 | account + period |
| IssueInvoices / IssueInvoice | billing | 06:00 UTC 2nd, fan-out | 10, resumable | account + period |
| RetryInvoicePayment | billing | dunning schedule | 3 per attempt | invoice + attempt |
| AdvanceDunning | billing | daily 07:00 UTC | 3 | day |
| IssueCreditNotes | billing | daily 08:00 UTC | 5 | day |
| ExpireCreditGrants | billing | daily; mail 30 days before expiry | 3 | day |
| ReconcileStripe | billing | daily 04:00 UTC | 3 | day |
| VerifyLedgerReplay | billing | daily 05:00 UTC | 1 | day |
| ReviewSpamCreditRatio | billing | daily, per account | 3 | day |
| CreateLedgerPartitions | maintenance | monthly + boot | 5 | month |
| SyncStripeCustomer | billing | account update | 10 | account |
| PruneStripeEvents | maintenance | daily (90 days) | 3 | day |

Retention: billing never hard-deletes tenant data (ledger, invoices, accounts kept 7 years; archive job later calls `RetentionGuard.MayDestroy(ws, {workspace, id})` per workspace before detaching partitions). `PruneStripeEvents` not guarded (instance-level provider payload log, no tenant subject).

## Config keys (`config/billing.go`, `config/services.go`)

| Key | Env | Default |
|---|---|---|
| billing.enabled | (derived) | `app.edition == cloud` |
| billing.currency | DIMMI_BILLING__CURRENCY | usd (currency of new billing accounts; price book must exist for it, boot check) |
| billing.price_book | DIMMI_BILLING__PRICE_BOOK | 2026-11-01 |
| billing.free_allowance | | 100 |
| billing.default_spend_cap_cents | | 5000 |
| billing.admin_cap_limit_cents | | 100000 |
| billing.buffer_percent / buffer_min | | 20 / 20 |
| billing.spam_credit_days | | 14 |
| billing.merge_window_days | | 7 |
| billing.rollover_min_cents / rollover_max_periods | | 500 / 3 |
| billing.invoice_day / invoice_hour_utc | | 2 / 6 |
| billing.dunning.retry_days | | [3,5,7,10] |
| billing.dunning.restrict_after_days | | 14 |
| billing.dunning.uncollectible_after_days | | 60 |
| billing.dispute_auto_credit_cents_30d | | 5000 |
| billing.dispute_max_age_days | | 90 |
| billing.review_spam_ratio / review_min_charges | | 0.25 / 40 |
| billing.anomaly_multiplier / anomaly_min | | 3 / 20 |
| billing.enterprise_alert_count | | 40000 |
| billing.outbound_fair_use_multiplier | | 10 (floor is entitlement `billing.outbound_fair_use_min`) |
| billing.contract_min_unit_micros | | 6000 (warning only) |
| services.stripe.secret_key | DIMMI_SERVICES__STRIPE__SECRET_KEY (+_FILE) | required when cloud |
| services.stripe.webhook_secrets | DIMMI_SERVICES__STRIPE__WEBHOOK_SECRETS | required when cloud |
| services.stripe.api_version | | pinned in code |
| services.stripe.tax_code | | SaaS business-use code (configured) |
| services.stripe.portal_configuration_id | | required when cloud |

Boot with `app.edition=cloud` and missing Stripe keys: fails (prevents silent unbilled cloud).
Moved to entitlement catalog (Internal API, Entitlements): `billing.test_mode_cap`, `billing.outbound_fair_use_min`. Entitlement cache TTL fixed 60 s (extensibility.md).

## Permissions matrix

Implemented as permissions (`00-foundation/extensibility.md` section 4, P2). `billing_policy.go` calls `authz.Can(principal, permission)`; role columns derived from 00-accounts `roles.go`, never checked in 12 code. Token effective permissions = role permissions intersect expanded abilities (replaces "token never exceeds creator's role"; re-evaluated per request, so demotion shrinks token power at once).

12 permissions (registered in 00-accounts catalog `app/auth/permissions.go`; each in exactly one ability group):

| Permission | Ability group | Roles (roles.go) | Gates |
|---|---|---|---|
| `billing:read` | billing:read | owner, admin | money fields, usage, lines, invoices, credits, disputes, comparison |
| `billing:write` | billing:write | owner, admin | account, tax ids, payment/portal sessions, cap within admin limit, thresholds, disputes, program applications, invoice retry |
| `billing:cap_unrestricted` | billing:write | owner | cap above `billing.admin_cap_limit_cents` or `null` |

| Action | Owner | Admin | Member | Staff (instance admin) | Permission (token ability group) |
|---|---|---|---|---|---|
| View meter counts + state | yes | yes | yes | yes | active member, any workspace token (no permission) |
| View money (meter amounts, usage, invoices, lines) | yes | yes | no | yes | `billing:read` |
| Set cap up to $1,000, lower cap, thresholds | yes | yes | no | yes | `billing:write` |
| Raise cap over $1,000 or remove cap | yes | no | no | yes | `billing:cap_unrestricted` (billing:write) |
| Payment method / portal / address / tax ids | yes | yes | no | no | `billing:write`; `S` (browser session) only for payment/portal |
| Dispute line | yes | yes | no | n/a | `billing:write` |
| Program application | yes | yes | no | review | `billing:write` |
| Conversation billing status (no money) | yes | yes | yes | yes | `conversations:read` |
| Grants, contracts, credit notes, refunds, discounts, lock, currency | no | no | no | yes (2FA) | `system:write` |
| Entitlement overrides | no | no | no | yes (`dimmi entitlement:set`, CLI on server) | n/a (no HTTP route at MVP) |
Payment method and portal sessions are `auth:session` (`S`): any bearer (PAT, third-party OAuth, first-party Dimmi app) gets 403 `session_required`; browser flows only (Stripe return URLs land on dashboard with session cookie). Mobile app opens dashboard billing page in browser.

## Performance budgets

| Metric | Budget |
|---|---|
| MeterMessageActivity, message inside open window | p95 under 1 ms (1 indexed read + 1 update) |
| Activation (period lock + window + ledger insert) | p95 under 5 ms; lock wait p99 under 20 ms at 50 activations/s per billing account |
| PreflightActivation | p95 under 3 ms |
| `GET /billing/meter` | p95 under 30 ms (counters, no ledger scan) |
| `GET /billing/usage-lines` | p95 under 50 ms at 1M lines/workspace |
| CSV export | 100k lines under 10 s, streaming, memory flat |
| Webhook ack | p95 under 200 ms |
| Period close + invoice issue | 100k accounts under 1 h (fan-out, Stripe rate limit aware: 25 req/s default budget) |
| Replay of one workspace month | 100k messages under 5 s |

## Observability

Metrics: `billing_activations_total{result,reason}`, `billing_activation_lock_wait_seconds`, `billing_ledger_lines_total{entry_type,reason}`, `billing_blocked_replies_total{reason,author_type}`, `billing_paused_workspaces`, `billing_invoices_total{status}`, `billing_invoice_amount_cents` (histogram), `billing_payment_attempts_total{result}`, `billing_accounts{status}`, `billing_stripe_requests_total{op,status}`, `billing_stripe_webhook_lag_seconds`, `billing_reconciliation_issues_open{kind}`, `billing_replay_mismatches_total`, `billing_credit_notes_total`, `billing_disputes_total{status}`.
Logs: every activation decision at DEBUG with workspace, conversation, window seq, result; blocks at INFO; Stripe calls with idempotency key and request id (never card data, never full payloads at INFO).
Alerts (staff): any `amount_mismatch`, replay mismatch, webhook failures over 5 min, invoice run incomplete by 12:00 UTC on 2nd, payment failure rate over 10% day.

## Security and privacy

- PCI: Checkout + Portal hosted by Stripe; Dimmi stores brand/last4/expiry only.
- Stripe restricted API key, least privilege (stripe.md section 9). Webhook signature + timestamp tolerance; replay dedupe by event id.
- Ledger: no message content, no names/emails; contact id pseudonymous; survives purge for 7 years (tax) with no personal data.
- Staff endpoints: instance admin + 2FA + audit log with reason note required for every money-affecting action.
- Disputes rate-limited (10/hour/workspace) and auto-credit budget per 30 days stops abuse; beyond budget goes to review.
- RLS on all tenant billing tables (workspace-scoped via `dimmi_visible_workspaces()`, account-scoped via `dimmi_visible_billing_accounts()`); `stripe_events` instance-level, staff only. `entitlement_overrides` never readable through tenant API (driver reads with service role).
- Abuse signals: spam-credit ratio over 25% with at least 40 charges (30 days) flags review; dispute volume flag; chargeback restricts. Flags never auto-charge or auto-ban.
- Price/rule changes: 60-day notice enforced by registry test; grandfathering 12 months on increases.
- Self-host: module not registered; no Stripe code paths reachable; simulator reads local DB only.

## Metering impact

This spec implements metering. Launch features 07 agent actions, 08 docs sync, 13, 14, 15, 16 importers not metered (metering-rules.md section 1 "Not activity", launch feature fixtures). Other specs' obligations (also in Dependencies): writers set `messages.substantive=false` for non-substantive outbound and `metadata.auto_reply=true` for automated inbound; importers set `metadata.imported=true`; widget test/preview sets `conversations.is_test=true` (via `InboundMessage.IsTest`); 03 emits `conversation.merged` and `initiated_by` on `conversation.created`; 07 calls `PreflightActivation` before LLM.

## Dependencies on other specs

| Spec | Needs |
|---|---|
| 00-foundation | `InTx` listeners able to abort tx (error propagation, contracts.md) + `tx.AfterRollback` hook; `metered` middleware stub replaced; abilities `billing:write` added; `billing` queue; 423 mapping; RLS + `dimmi_visible_workspaces()`; partitioned table support in migrations; `organizations` table + `workspaces.organization_id`; `Entitlements` contract + catalog (12 registers `billing.*` keys, cloud driver); `GET /workspace/entitlements` route; `RetentionGuard`; lint annotation `-- lint:org-scoped`; `ent_` prefix |
| 00-accounts | roles + `authz.Can` + permission catalog (12 permissions registered there), `workspace.deleted` / `workspace.restored` events, `CreateWorkspace` creates org before initializers (12 initializer ensures account per org), `PurgeWorkspace` soft-deletes org (12 anonymizes account then), audit log, 2FA status, owner email |
| 01-messenger-widget | `conversations.is_test=true` for preview/test mode; offline behavior unaffected; consumes `BillingStatus.HasPaid` (Internal API) for badge rule, config cache bust hook; test cap reads entitlement `billing.test_mode_cap` |
| 03-shared-inbox | reply action surfaces `ErrActivationBlocked` as 423 with `billing` block; `conversation.merged` event `{source_id,target_id}`; `conversation.created` carries `initiated_by`; spam mark/unmark events with `occurred_at`; merge redirect for ledger links |
| 04-email-channel / 05-chat-app-channels | `substantive=false` on auto-ack/OOO; `metadata.auto_reply=true` on automated inbound; permanent vs transient delivery failure distinction in `message.delivery_updated` |
| 06-notifications | in-app + email delivery of billing notifications to owners/admins |
| 07-dimmi-agent | gate rule calling `PreflightActivation` (new skip reason `billing_paused`, distinct from 07's own AI spend cap rule 9); re-trigger on `billing.activations_resumed` |
| 09-agent-native | MCP `billing` toolset registration + confirm gate, `billing:*` OAuth scopes |
| 10-webhooks | new billing events in catalog |
| 13-changelog | none billable; mailings/reactions/subscriptions write no messages; changelog mail replies into inbox later (`changelog-replies-to-inbox`); `changelog.email.monthly_recipient_cap` catalog key |
| 14-feedback-roadmap | none billable; no `messages`/`conversations` rows from feedback; voter mail via 04 `SendContactNotification` (not conversation outbound, D7 fair-use unaffected); catalog keys |
| 15-announcements-surveys | CSAT request system messages `substantive=false` (D2), in-thread mail `outbound_emails.kind system`; engage content never ledger; chat-format reply = contact-initiated conversation (D1) |
| 11-self-host | `users.is_instance_admin` + `instance_admin` middleware for staff; edition flag; exports exclude billing tables; `self_host` Entitlements driver (binds when billing provider not registered) |

## Contradictions found in foundation docs

1. Resolved: `RecordBillableActivity` is `InTx` in catalog (contracts.md phases); credits in `Queued` `ApplyBillingCredits`. Catalog listener row for `ApplyBillingCredits` still omits `conversation.unmarked_spam` (12 consumes it): foundation to add.
2. Resolved: `conversation.merged` (03) and `workspace.deleted` (00-accounts) now in catalog. Open: prefixes `cg_` (credit_grants), `bd_` (billing_disputes), `bpa_` (billing_program_applications), `bri_` (billing_reconciliation_issues) and table `credit_grant_applications` missing from data-model.md registry/ownership map.
3. Conventions require FK `on delete cascade` on tenant tables and `updated_at` on every table. Explicit exception (accepted): `usage_ledger` has no workspace FK, no cascade on workspace purge (tax retention 7 years) and no `updated_at` (append-only, trigger rejects UPDATE/DELETE). Same no-cascade rule for `invoices_cache`, `billing_accounts`, `billing_adjustments`, `credit_grants`, `credit_grant_applications` (these keep `updated_at`). Conventions should list this exception.
4. `research/pricing.md` section 7 picks Stripe Billing meter; spec picks one-off invoices (one-off invoices excluded from 0.7% Billing fee; Invoicing 0.4%; rating stays in Dimmi).
5. `workspaces.suspended_at` described "abuse or billing"; billing never suspends (restricted state instead).
6. "Spend cap" also used by 07 for AI token budget; billing uses `billing.*` names and skip reason `billing_paused` to avoid collision.
7. Account-scoped tables (keyed by `billing_account_id`, no `workspace_id`) break P8 by design (org-owned, not workspace-owned). Foundation lint (security.md tenant FK rule) needs annotation `-- lint:org-scoped` next to existing `user-scoped` / `global`; RLS via `dimmi_visible_billing_accounts()`. Foundation to add.
8. Billing role: 00-accounts matrix and ability ceiling table say billing owner-only (`billing:read` admin `no`); extensibility.md section 4 lists `billing:read`/`billing:write` among "owner-only extras". 12 (owner of billing) keeps owner + admin for `billing:read`/`billing:write` with owner-only `billing:cap_unrestricted` (cap over $1,000, remove). roles.go must follow this table; 00-accounts to align.
9. Unit split made explicit (extensibility.md sections 3, 6 leave it implicit): period counters, cap, tiers, free allowance, D10 test cap (`billing.test_mode_cap`) and D7 fair-use limit are per account (shared when org holds 2 workspaces); ledger, windows, D1-D15 evaluation, disputes stay per workspace conversation.
10. Resolved: 13 (`changelog.email.monthly_recipient_cap`) and 15-announcements-surveys (`engage.*` caps) moved cloud caps from config to entitlement catalog; over-limit code `entitlement_limit_reached`.
11. Billing mail recipients include `billing_accounts.billing_email` (may be non-member). 04 arch test (`email_streams_test.go`) says Mailables route to `models.User` only. Billing email is Dimmi's own customer (not a workspace contact), so system stream is right; arch rule needs `BillingAccount` notifiable allowed. 04/foundation to confirm.

## Open questions

1. Bands 2 and 3 publication pending measured cost (`pricing.md` section 3). Default: ship price book `2026-11-01` as published; new version if raised before launch.
2. Resolved: badge rule owned by 01; 12 exposes `BillingStatus.HasPaid` (Internal API): any invoice paid or active contract.
3. Anomaly alert also auto-lowers cap? Default no (alert only).
4. Dispute window 90 days; default kept.
5. Shared account later: should workspace A admins see workspace B usage lines? **No**: lines and per-conversation views stay filtered by request workspace; account totals (meter, invoices) shown to both. Revisit with org roles.
6. Org-level billing role later (`organization_members.role = billing`)? **Yes, additive** when org UI ships; until then account access derives from workspace roles above.

# 12 Cloud billing: Stripe integration

Style: caveman. Client: `github.com/stripe/stripe-go/v87` v87.1.0 (MIT) wrapped in `app/services/stripe` (adapter zone, framework.md 7.3: only this SDK in dir, no stripe-go type in exported API). HTTP via `a.HTTP.StdClient()` (timeouts 10 s connect / 30 s total); SDK retries off (`MaxNetworkRetries 0`); retry policy in app (River job retries on network, 5xx, 429 with same Stripe idempotency key). Webhook verify `webhook.ConstructEvent` (tolerance 300 s) looped over `services.stripe.webhook_secrets`. API version pinned in `services.stripe.api_version` (must match v87 default). Facts verified 2026-11-01 against docs.stripe.com and stripe.com/billing/pricing; params marked [verify] must be checked against pinned API version during implementation.

## 1. Decision: one-off invoices with Dimmi-rated lines, no Stripe meters, no subscriptions

`research/pricing.md` section 7 suggested Stripe Billing meter + graduated price. Spec overrides:

| Factor | Stripe meter + graduated price | Dimmi-rated invoice items (picked) |
|---|---|---|
| Rating rules (allowance, contract tiers, buffer waivers, credits, late adjustments, roll-over, rule versions) | must duplicate in Stripe price objects; drift risk; roll-over and buffer waivers not expressible | one implementation: `app/support/metering`, open source, tested, replayable |
| Late credits after month close | meter events cannot be retracted after invoice | credit notes from exact delta |
| Reconciliation | two raters to compare | Stripe subtotal must equal Dimmi subtotal exactly (integer cents) |
| Fees | Billing 0.7% of billing volume (stripe.com/billing/pricing, "excludes one-off invoices") | Invoicing Starter 0.4% per paid invoice (support.stripe.com "Stripe Invoicing pricing"); cheaper |
| Transparency | invoice lines generic | invoice lines show band math + link to usage page |

Stripe objects used: Customer, Checkout Session (`mode=setup`), SetupIntent (via Checkout), PaymentMethod, Customer Portal session, Invoice (one-off), InvoiceItem, Coupon (discounts), Credit Note, Tax ID, Event (webhooks). Not used: Subscription, Price/Meter, Billing Meter Events, Stripe Managed Payments (MoR, revisit later per pricing.md).

## 2. Customer lifecycle

| Moment | Action |
|---|---|
| Workspace created (cloud) | workspace initializer hook (contracts.md) runs after `CreateWorkspace` inserted organization; inserts `billing_accounts` row for that org if absent (`ON CONFLICT (organization_id) DO NOTHING`, currency from `billing.currency`), inside `CreateWorkspace` tx. MVP: new org per workspace, so always new account; later workspace added to existing org reuses its account. `EnsureBillingAccount` on first billing touch stays as repair safety net. No Stripe call |
| Owner/admin opens billing setup | `PATCH /billing/account` with country (+ postal code for US, province or postal code for CA, per Stripe Tax requirements), billing name, billing email |
| First need of Stripe customer (payment method session, tax id, invoice) | `POST /v1/customers` with `email`, `name`, `address`, `tax[validate_location]=immediately`, `expand[]=tax`, `metadata[organization_id]`, `metadata[billing_account_id]`, `preferred_locales` from workspace locale, Idempotency-Key `cust:<billing_account_id>`. One Stripe customer per billing account (never per workspace). Store `stripe_customer_id`, `tax_status` from `tax.automatic_tax` (`supported`, `unrecognized_location`, `not_collecting`, `failed`) |
| Address invalid for tax (`unrecognized_location` or validation error) | 422 `invalid_tax_location` with field errors; nothing stored in Stripe on create failure |
| Address change | `POST /v1/customers/{id}` with same validation; future invoices use new location |
| Tax ID add | `POST /v1/customers/{id}/tax_ids` `{type, value}` (e.g. `eu_vat`); verification async (`customer.tax_id.updated`); Stripe Tax applies reverse charge for valid B2B EU VAT automatically |
| Last live workspace of org deleted | final invoice (section 4); customer kept (invoices history), metadata `deleted_at`; never `DELETE /v1/customers`. Workspace deleted while org keeps others (later): nothing |

`tax_status = not_collecting` (no registration in that jurisdiction): allowed; Stripe returns zero tax. `unrecognized_location`: blocks payment method session until fixed (prevents untaxable invoices).

## 3. Payment method collection

- `POST /api/v1/billing/payment-method-sessions` creates Checkout Session: `mode=setup`, `currency=<account currency>` (required in setup mode with dynamic payment methods), `customer=<id>`, `success_url=<app>/settings/billing?setup=success&session_id={CHECKOUT_SESSION_ID}`, `cancel_url`, `metadata[billing_account_id]`, `setup_intent_data[metadata][billing_account_id]` [verify]. Payment methods: card (+ Apple Pay / Google Pay via Checkout automatically); Link allowed; no bank debits at MVP.
- Returns `{url, expires_at}`; frontend redirects.
- Completion: webhook `checkout.session.completed` (authoritative; success_url redirect may never happen). Job retrieves session with `expand[]=setup_intent`, gets `payment_method`, sets `POST /v1/customers/{id}` `invoice_settings[default_payment_method]=<pm>`, stores brand, last4, exp month/year, `has_payment_method=true`. Success redirect page also calls `GET /api/v1/billing/account?sync=1` which performs same sync idempotently (fast UX when webhook lags).
- Side effects of first payment method: create default `spend_caps` row ($50) if none; recompute cap math; account `free` -> `active`; resume paused activations (event `billing.activations_resumed`); retry open past-due invoices immediately.
- Replace/remove card: Customer Portal (`POST /v1/billing_portal/sessions` with `customer`, `return_url`, `configuration` = portal config with payment method update + invoice history enabled, subscriptions features disabled). Portal changes arrive as `customer.updated` (default payment method), `payment_method.attached`, `payment_method.detached`.
- Default payment method removed and none left: `has_payment_method=false`; next activations follow no-card decision (free allowance + buffer); open invoices fall back to hosted invoice page link.

## 4. Invoice issuance (`IssueInvoice`, 2nd of month 06:00 UTC, unique per account+period)

1. Load closed period(s): current period + rolled-over periods pending. Rate each period with its `pricing_snapshot` (metering-rules.md section 8). Apply percent discount and credit grants (grants valid on invoice date).
2. Pre-tax payable under $5.00 and roll-over chain allows: row `rolled_over`, stop. Payable $0: row `zero`, stop.
3. Ensure Stripe customer. Choose collection: contract `send_invoice` (days_until_due from contract, default 30); has default payment method `charge_automatically`; else `send_invoice` with `days_until_due=14`.
4. `POST /v1/invoices` (Idempotency-Key `inv:<invoice_id>:create`): `customer`, `currency=<account currency>`, `collection_method`, `auto_advance=false` (Dimmi controls finalize and collection attempts), `automatic_tax[enabled]=true`, `pending_invoice_items_behavior=exclude`, `description="Dimmi usage October 2026"`, `footer="Every conversation listed at https://app.trydimmi.com/billing/usage?period=2026-10"`, `metadata[dimmi_invoice_id]`, `metadata[billing_account_id]`, `metadata[periods]=2026-08,2026-09,2026-10`. Footer link per workspace when account has several (later).
5. One `POST /v1/invoiceitems` per tier line (Idempotency-Key `inv:<invoice_id>:item:<n>`): `customer`, `invoice=<draft id>`, `currency=<account currency>`, `amount=<cents>` (quantity 1; Dimmi-rounded total, avoids Stripe decimal rounding drift), `description="October 2026: conversations 1,001-10,000, 901 x $0.015"`, `tax_behavior=exclusive`, `tax_code=<services.stripe.tax_code>` (SaaS business use; configured, not hard-coded) [verify param names on pinned version], `metadata[period]`, `metadata[tier]`. Free tier line included at $0 for transparency only when invoice has other lines.
6. Discounts on invoice (`POST /v1/invoices/{id}` `discounts[]`): percent discount via account-level coupon (`percent_off`, `duration=forever`, created once per account, id `dimmi_pct_<account>`); credit grants via per-invoice coupon (`amount_off=<cents>`, `currency=<account currency>`, `duration=once`, `max_redemptions=1`, id `dimmi_grant_<invoice_id>`). Invoice discounts apply pre-tax, proportionally across lines (Stripe docs).
7. `POST /v1/invoices/{id}/finalize` (`auto_advance=false`). Stripe calculates tax at finalization. Store tax, total, number, `hosted_invoice_url`, `invoice_pdf`.
8. Assert Stripe `subtotal` == Dimmi subtotal and `total_discount_amounts` sum == Dimmi discount + grants. Mismatch: `POST /v1/invoices/{id}/void`, reconciliation issue `amount_mismatch`, staff alert, no charge.
9. `charge_automatically`: `POST /v1/invoices/{id}/pay` (Idempotency-Key `inv:<id>:pay:<attempt>`). Result via response + webhooks. `send_invoice`: `POST /v1/invoices/{id}/send`.
10. Dimmi sends own mail "Invoice for October: $87.00 + tax" with usage summary + hosted invoice link. Stripe customer emails (receipts, invoice emails) disabled in Stripe settings to avoid duplicates, except `send_invoice` flow which relies on Stripe hosted page link we send ourselves.

Crash recovery: each step idempotent by stored Stripe ids on `invoices_cache` + idempotency keys (Stripe keeps keys at least 24 h). Job resumes from first step whose result id is missing. Older than 24 h with missing ids: search `GET /v1/invoices?customer=` filtered by `metadata[dimmi_invoice_id]` client-side before creating new.

## 5. Webhooks (`POST /webhooks/stripe`)

- Verify `Stripe-Signature` (HMAC-SHA256 `v1` over `<t>.<raw body>`, tolerance 300 s, multiple `v1` values accepted during secret roll). Body limit 1 MB. Failure 400.
- Persist `stripe_events (id, type, livemode, payload, received_at)`; duplicate id: 200 no-op. Enqueue `ProcessStripeWebhook{event_id}` (unique by event id). Respond 200 under 200 ms.
- Livemode mismatch with config (test event on live endpoint): store, ignore, metric.
- Handler always re-fetches object from Stripe API (events may arrive out of order; Stripe-recommended), then derives state.

| Event | Handling |
|---|---|
| `checkout.session.completed` (mode setup) | section 3 sync |
| `setup_intent.succeeded` | same sync (redundant safety) |
| `customer.updated` | sync default payment method, address, email, tax status |
| `payment_method.attached` / `payment_method.detached` | sync `has_payment_method`, card display fields |
| `customer.tax_id.created` / `customer.tax_id.updated` / `customer.tax_id.deleted` | sync `tax_ids` with verification status |
| `invoice.finalized` | sync totals, URLs |
| `invoice.paid` | status `paid`, `paid_at`; if account `past_due`/`restricted` and no other open past-due invoice: restore `active`, event `billing.account_status_changed`, resume activations |
| `invoice.payment_failed` | status `past_due` (first failure sets `past_due_since`), store `last_payment_error` (decline code), schedule next retry per dunning, mail owners/admins + billing email, event `billing.payment_failed` |
| `invoice.payment_action_required` | SCA needed: mail hosted invoice link (customer completes 3DS there); no automatic retry until resolved or new card |
| `invoice.marked_uncollectible` | status `uncollectible` (accounting only; account stays restricted until paid) |
| `invoice.voided` | status `void` (staff action or mismatch path) |
| `credit_note.created` | link to `billing_adjustments` / staff credit note, sync amounts |
| `charge.refunded` | sync refund amounts on invoice row |
| `charge.dispute.created` | account `restricted` immediately (chargeback), staff alert, event `billing.account_status_changed` reason `chargeback` |
| `charge.dispute.closed` | won: restore if nothing else past due; lost: stays restricted until staff review |
| `customer.deleted` | staff mistake guard: alert, clear `stripe_customer_id`, account needs new payment method |
| other | stored, ignored |

## 6. Dunning (Dimmi-driven)

| Day (from finalize) | Action |
|---|---|
| 0 | first `pay` attempt; failure: `past_due`, mail "payment failed" with update-card link (portal) |
| 3, 5, 7, 10 | `RetryInvoicePayment` (`pay`); skipped while `payment_action_required` unresolved |
| any | new default payment method or customer clicks "retry now" (`POST /billing/invoices/{invoice}/payments`): immediate attempt |
| 14 | still unpaid: account `restricted` (activations blocked beyond free allowance; inbound captured; open windows still answerable; data never touched), mail + in-app banner, event |
| 14 to 60 | weekly reminder mail; no more automatic charge attempts except on new card |
| 60 | `POST /v1/invoices/{id}/mark_uncollectible` (accounting); account stays `restricted`; payable any time via hosted invoice page (uncollectible invoices remain payable) |
| paid any time | `invoice.paid` restores `active` when no other past-due invoices |

Never: automatic suspension, data deletion, or inbound blocking for non-payment. `workspaces.suspended_at` not used by billing (abuse only). Staff may lock (`billing_locked`) for fraud.

## 7. Late credits, credit notes, refunds

- Credit (spam/merge/delivery/dispute/support) on charge whose period already invoiced: `billing_adjustments` row pending. Money delta = `payable(snapshot, billable_before) - payable(snapshot, billable_after)` for that period (graduated: removing ordinal at top band). Delta 0 (inside free tier or covered by grant): row `absorbed`.
- `IssueCreditNotes` daily: per invoice, sum pending deltas. Sum >= $1.00 or invoice older than 25 days: create credit note `POST /v1/credit_notes` on that invoice, distributed over invoice line items (`lines[][type]=invoice_line_item`, `lines[][invoice_line_item]`, `lines[][amount]`, so tax reverses proportionally) [verify], `credit_amount` when invoice paid (customer balance, auto-applied to next invoice), reduces amount due when invoice open. Under $1.00 and young: wait. Rolled-over period (no invoice): delta reduces rolled amount directly (no Stripe call).
- Credit note amount cannot exceed invoice remaining creditable amount: cap and record remainder as `absorbed` with note.
- Reversal (positive delta) against invoiced period: `billing_adjustments` row with positive `amount_cents`; netted against pending negative deltas of same invoice first; remainder added to next issued invoice as line "Prior-period correction (September 2026): +N conversations". Never re-opens finalized invoice.
- Staff refunds (`POST /api/v1/system/billing/accounts/{account}/credit-notes` with `refund=true`): credit note with `refund_amount` (card refund) on paid invoice. Reason enum: `duplicate`, `fraudulent`, `order_change`, `product_unsatisfactory` (Stripe reasons) plus Dimmi `reason_note`.
- Customer balance (credits from notes) shown in `GET /billing/credits` as `stripe_balance_cents` (synced from `customer.balance`, account currency).

## 7a. Currency

- `billing_accounts.currency` sent on every Checkout Session, invoice, invoice item, coupon. Stripe locks customer currency at first invoice/charge; Dimmi locks account currency then too (`currency_locked`).
- Mismatch guard: `IssueInvoice` asserts invoice, items, coupons, `pricing_snapshot` all carry account currency; else void path (`amount_mismatch`).
- MVP: `usd` only (price book `2026-11-01/usd`). New currency = new price book file + Stripe Tax check; no schema change.

## 8. Reconciliation (`ReconcileStripe`, daily 04:00 UTC, `billing` queue)

Checks (last 40 days of invoices + all accounts with Stripe customer):
| Kind | Rule |
|---|---|
| `amount_mismatch` | Stripe invoice subtotal/discounts != Dimmi row |
| `status_drift` | Stripe status != Dimmi status after 1 h (missed webhook): auto-resync status only (status is Stripe-truth), issue logged |
| `orphan_stripe_invoice` | Stripe invoice for Dimmi customer without `dimmi_invoice_id` or unknown id |
| `missing_stripe_invoice` | Dimmi row `open` without Stripe id older than 2 h |
| `payment_method_drift` | `has_payment_method` != Stripe default PM presence: auto-resync |
| `missing_billing_account` | workspace whose org has no billing account (repaired by `EnsureBillingAccount`, issue logged) |
| `currency_drift` | Stripe customer `currency` != account `currency` (money-affecting, staff resolves) |
| `ledger_counter_drift` | account `billing_periods` counters != `SUM(quantity)` over ledger `WHERE billing_account_id = ? AND period = ?`: recompute counters from ledger (ledger is truth), issue logged |
| `replay_mismatch` | from `VerifyLedgerReplay` |
| `credit_grant_drift` | grant `remaining` != amount - sum applications |
Money-affecting kinds never auto-fixed; staff resolves via API. Metric `billing_reconciliation_issues_open`. Alert when any `amount_mismatch` or `orphan_stripe_invoice`.

## 9. Environments and testing

- Config: `services.stripe.secret_key` (restricted key: write Customers, Invoices, Invoice Items, Credit Notes, Coupons, Checkout Sessions, Billing Portal, Tax IDs; read Events, Payment Methods, Charges), `webhook_secret` (list, for rolling), `api_version`, `tax_code`, `portal_configuration_id`, `mode` (test/live, checked against key prefix `sk_test_`/`rk_test_` vs live).
- Tests: feature tests (real App + Postgres) fake only the process boundary: `httpclienttest.New(tc.App.HTTP)` with recorded Stripe fixtures (deterministic ids; stray request fails test); adapter contract suite `tests/contracts/stripe_test.go` against stripe-mock in CI (no network to Stripe); live tag (testing.md section 6) for Stripe test mode with test clocks (staging only, never `go test ./...`).
- Staging: Stripe test mode, Stripe Tax test registrations, test cards (4242..., 4000 0025 0000 3155 SCA, 4000 0000 0000 0341 attach-then-fail) in demo script.

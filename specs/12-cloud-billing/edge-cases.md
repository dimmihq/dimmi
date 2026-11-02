# 12 Cloud billing: edge cases

Style: caveman. Scope trim 2026-11-02: message edit (old 19), echo reply (old 28d), public survey link cap (old 124) removed, renumbered. Format: Case. Behavior. Rule refs: metering-rules.md (MR), stripe.md (ST).

## Activation and windows

1. Two teammates reply same second to fresh inbound. Both txs reach period lock; second re-checks window after lock, sees open window from first, no activation. Unique `(conversation_id, seq)` final guard. One charge.
2. AI reply and human reply race. Same as 1; ledger actor = whichever committed first.
3. Reply transaction rolls back for unrelated reason (validation after meter ran). Window + ledger + counters roll back with it; no phantom charge.
4. Reply blocked (423). Whole reply tx aborted; pause marker written by after-rollback hook in own tx; teammate draft kept client-side.
5. After-rollback pause marker write fails (DB blip). Logged; next block retries; alert dedupe tolerates; meter state derived from counters anyway (`state` computed live, `activations_paused_at` informational).
6. Customer writes many messages, nobody replies for months. No charge ever.
7. Reply exactly at `closes_at`. `t >= closes_at` means closed: new evaluation; needs qualifying inbound after `closed_at` (MR section 2).
8. Customer message arrives 1 ms before window close, reply 1 h later. Inbound extends window (activity before close), reply inside window: no new charge.
9. Clock skew between app nodes. `messages.created_at` default DB `now()` (03 rule) and meter uses message `created_at`; ordering per DB clock, not node clock.
10. Message `created_at` backdated (channel provider timestamp used for inbound). Meter uses `created_at` assigned at insert (DB time) for windows; provider time kept in metadata only. Requirement on 03/channels: `created_at` = insert time.
11. Window spans DST change. UTC durations; unaffected.
12. Activation at 23:59:59.999 Oct 31 committing 00:00:00.010 Nov 1. Period from `activated_at` (October); October period row lock still works; invoice reads counters on Nov 2.
13. Conversation reopened by customer after close (03 state machine). Billing independent of `status`; only windows matter.
14. Reopen beyond `reopen_window_days` creates new conversation (data model). New thread, new windows; previous thread windows unaffected.
15. Email thread: customer replies to month-old notification creating message in old conversation. Window closed; inbound qualifies; reply activates new window (D4).
16. Teammate sends outbound first, contact answers with OOO auto-reply (auto_reply flag). Not qualifying inbound; teammate reply not billable.
17. Contact sends only attachments (no text). Qualifying (substantive true by default for contact messages); reply activates.
18. Non-substantive outbound mis-flagged as substantive by bug in channel writer. Activation happens; customer disputes; auto-credit. Replay cannot catch it (reads same flag). Mitigation: contract test per writer asserting `substantive=false` for auto-ack, OOO, CSAT, welcome; staff bulk credit (`support`) after fix.
19. Activating reply deleted by author. Charge stays (customer likely saw it); dispute path available.
20. Conversation hard-deleted (GDPR or spam purge). Windows cascade; ledger lines keep ids; line resource shows `conversation.deleted=true`, no link.
21. Contact erased (GDPR). Ledger `contact_id` now tombstone; lines remain; CSV shows contact id only.
22. Contact blocked after activation. Existing charge stays (D9d applies at activation only); mark spam to credit within 14 days.
23. Conversation moved between inboxes/channels. Windows per conversation; channel_type on line = conversation origin at activation.
24. Internal contact (member writes via widget with own verified email). Waiver `internal`. Member later removed from workspace: future activations normal billing (evaluated at activation time).
25. Unverified lead types member's email in widget. Not internal (verification required); billable normally. Prevents free threads by anyone claiming member email.
26. Workspace toggles `test_mode` mid-period. Activations evaluated at time; test waivers counted separately; switching off does not convert waivers.
27. Imported conversation receives live message then reply. Live messages count; charge normal (D11).
27a. Bulk reply / bulk action creating replies across 100 conversations near cap. Each conversation own tx; per-id result lists 423 code for blocked ids; earlier ids stay committed. Never one tx for whole batch (one block would roll back paid replies).
27b. Dimmi Agent passes `PreflightActivation`, then cap reached by teammate before AI reply insert. AI reply insert gets `ErrActivationBlocked`; 07 marks run `skipped billing_paused`, discards text, never retries job (no loop, tokens already spent recorded by 07). 
27c. AI reply job (River) retried after its tx committed (worker crash before ack). 07 dedupe by run id; even if second message inserted, it lands inside open window: no second charge.
27d. Reply via API with `Idempotency-Key` replayed after 423. Stored 423 response replayed only within same key; new key after cap raise re-evaluates.

## Spam, merges, delivery

28. Spam marked 13 days 23 h after activation. Credit (within 14 days, exact duration).
29. Spam marked, unmarked, marked again (within 14 days). Credit, reversal, credit again (cycle in idempotency key). Net per cycle correct.
30. Spam toggled after period invoiced. Credit creates adjustment; credit note when sum >= $1 or invoice 25 days old (ST 7). Reversal after credit note: reversal charge lands in original period counters; money for reversal billed as adjustment line on next invoice ("Prior-period correction"). No negative invoice lines.
31. Workspace mass-marks spam to dodge bill. Ratio check flags review over 25% (min 40 charges); credits still applied (customer trust), staff reviews.
32. Merge of two conversations of different contacts. No credit.
33. Merge where source charged in October and target in November (within 7 days across month). Later charge credited (November); October unchanged.
34. Merge chain A into B into C. Each merge event processed in order (queued, unique per event); clustering over remaining active charges each time; result identical to merging all at once (test).
35. Merge event processed twice (retry). Credits idempotent per charge cycle; second run finds charges already credited.
36. Contact merge (not conversation merge). No credit; conversations keep own windows.
37. Hard bounce on activating email reply but widget reply in same window delivered. No credit.
38. Bounce arrives 3 days after send. Credit if still no delivered reply in window; works on invoiced periods via adjustment.
39. Delivery status flips failed then delivered (provider correction). Credit then reversal.
40. WhatsApp reply fails because 24 h window closed (05 error). Permanent for that message: credit if no other delivered reply. Template resend later delivered: reversal.

## Caps, buffer, card

41. Cap lowered below current spend mid-month. Saved; `cap_count` below `billable_count`; next activation uses buffer (if buffer not exhausted) else blocked; one combined threshold alert (100%).
42. Cap set to $0 on paid workspace. Only free allowance billable; then 20 buffer waivers; then blocked.
43. Owner removes cap (null). Unlimited; alerts for forecast/anomaly still fire; thresholds ignored (no cap); meter shows "no cap" warning badge.
44. Admin tries cap $5,000. 403 `owner_required` (lacks `billing:cap_unrestricted`).
45. Cap raised while replies blocked. Event `billing.activations_resumed`; next reply passes; nothing auto-sent; 07 re-triggers AI for skipped conversations within 24 h.
46. Card added on free workspace past 120 (blocked). Default cap $50 created; cap math recomputed; resumed; 20 buffer waivers already used stay free.
47. Card added then removed same month (portal). Back to no-card decision: billable already above allowance, buffer exhausted: blocked `payment_method_required`; invoices for charges already incurred still issued (hosted invoice link).
48. Default cap row created by race (two webhooks). Unique `billing_account_id` on `spend_caps`, `INSERT ... ON CONFLICT DO NOTHING`.
49. Credit grant added mid-month. Cap math recomputed: grant raises `cap_count` (cap means payable after credits).
50. Grant expires mid-month. Cap math recomputed at expiry; may move workspace into buffer/blocked; alert sent.
51. Percent discount 100% (staff mistake or special deal). Payable always $0; cap irrelevant (`cap_count` unlimited when payable(n)=0 for all n under search bound); invoices `zero`.
52. Contract with `send_invoice` and no card. Not treated as "no payment method" (collection by invoice); cap optional.
53. Spend cap hit exactly at 100% boundary with rounding (e.g. $49.995 rounds to $50.00). `cap_count` computed with same rounding as invoice; invoice equals cap at most.
54. Thresholds custom `[90]`. Only 90% alert; forecast and anomaly unaffected.
55. Many activations per second near cap. Period row lock serializes; exact cut-off; no overshoot.
56. Buffer exhausted, credit frees one slot. Next activation charges (billable below cap_count); buffer counter unchanged.
57. Restricted account (dunning) with open windows. Replies in open windows allowed; new activations only while under free allowance.
58. Restricted account pays invoice. `invoice.paid` webhook restores `active` if no other past-due invoice; resumed event; cap math recomputed.
59. Account `locked` (fraud). All activations blocked `billing_locked`; inbound captured; staff only unlock.
60. Test-mode workspace reaches 1,000 waivers (`billing.test_mode_cap`). Blocked `test_mode_limit_reached`; message suggests live workspace. Staff override raises cap for one org without deploy.

## Rating, invoices, Stripe

61. Period total $3.20 (under $5). Rolled over; next month $2.10 rolled; third month $1.00: invoice $6.30 issued (max 3 months).
62. Rolled amounts then workspace deleted. Final invoice issued immediately for all rolled + current; under $0.50 written off (`zero`, `written_off_cents`).
63. Stripe down at invoice time. Job retries with backoff for 24 h; invoice stays `draft`; meter and messaging unaffected; staff alert if not done by 12:00 UTC.
64. Crash between Stripe invoice create and storing id. Retry uses same idempotency key, gets same invoice; beyond 24 h, lookup by metadata before create (ST 4).
65. Stripe subtotal differs from Dimmi (bug in coupon proportional split or tax-inclusive misconfig). Invoice voided before payment; reconciliation issue; staff fixes; reissue.
66. Customer tax location becomes `unrecognized_location` after address edit. PATCH rejected 422 `invalid_tax_location`; old address kept.
67. Tax registration missing for customer jurisdiction (`not_collecting`). Invoice issued with zero tax (Stripe behavior); tax report shows exposure; not customer-facing problem.
68. EU customer adds VAT ID; verification pending at invoice time. Stripe Tax decides with current status; later verification does not retro-change finalized invoice (tax fixed at finalization).
69. SCA required (`payment_action_required`). No auto retries; email with hosted link; dunning clock still runs; restricted at day 14 if unresolved.
70. Card declined `insufficient_funds` then customer updates card day 2. Immediate retry on new default payment method; success restores.
71. Card expired before invoice. Stripe card updater may refresh; else failure path.
72. Webhook events out of order (`invoice.paid` before `invoice.finalized`). Handler re-fetches invoice from API; state from latest object; order irrelevant.
73. Duplicate webhook delivery. `stripe_events` primary key dedupe; job unique by event id.
74. Webhook signed with old secret during rotation. Multiple secrets configured; any match accepted.
75. Test-mode event hits live endpoint. Stored, ignored, metric.
76. Chargeback. Account restricted, staff alert; ledger untouched; dispute evidence pack = usage CSV + invoice + thread links (staff manual).
77. Refund larger than invoice remaining. 422 from action before Stripe call.
78. Staff credits line in period already invoiced and paid. Credit note `credit_amount` to customer balance; shown in `GET /billing/credits`.
79. Customer balance (credit) larger than next invoice. Stripe applies balance; invoice total $0 paid automatically; Dimmi syncs `paid`.
80. Invoice amount between $0.01 and $0.49 forced (final invoice). Written off (Stripe minimum charge $0.50 USD); recorded.
81. Contract tiers change mid-month. `pricing_snapshot` taken at close uses contract effective at period end; staff sets `starts_at` on first of month to avoid mixed months (validation warns otherwise).
82. Price book increase scheduled. Existing accounts pinned 12 months (`price_book_locked_until`); new accounts get new book on effective date.
83. Rule set new version effective mid-month. Windows activated before keep old version; credits use window's version; replay handles both.

## API, export, disputes

84. Usage lines for workspace with 2M lines. Cursor pagination on `(period, created_at, id)`; partition pruning by `filter[period]` (default current period when filter absent).
85. Finance wants contact names in CSV. Never included (ids only, privacy); formula injection guarded.
86. Dispute on waiver line. 422 `line_not_creditable`.
87. Dispute on already-credited line. 409 `line_already_disputed` or 422 `line_not_creditable` when credited by spam.
88. Dispute beyond $50 auto budget. Status `pending_review`; staff resolves; mail on resolution.
89. Dispute older than 90 days. 422 `dispute_window_closed`.
90. Member (no billing access) calls usage lines. 403 `forbidden` (members are workspace insiders, not 404).
91. Other workspace's line id. 404 (tenancy rule).
92. Token with `billing:read` created by admin later demoted to member. Effective permissions = current role permissions intersect abilities, computed per request; member role lacks `billing:read`: 403.
93. Payment method or portal session via bearer (PAT, third-party OAuth, first-party Dimmi app). 403 `session_required` (`auth:session`: Stripe return URL needs browser session).
94. Public estimate endpoint abused for scraping. `throttle:public` 60/min/IP; response cacheable 1 h.
95. Comparison endpoint with zero AI conversations. Fin estimate $0 shown with explanation; never negative savings marketing.

## Edition, deploy, migration

96. Self-host calls billing endpoints. 404 `feature_unavailable`; meter listener not registered; `dimmi billing:simulate` works.
97. Cloud boot without Stripe keys. Boot fails (no silent unbilled cloud).
98. Billing tables exist on self-host (same schema). Empty; no jobs scheduled.
99. Rolling deploy where new version changes rule set. Rule versions selected by activation time, not binary version; both binaries contain both versions (new rule versions merged at least one release before effective date, enforced by 60-day notice).
100. Ledger partition missing for new month (job failed). Activation insert fails: default partition `usage_ledger_default` catches rows; alert; `CreateLedgerPartitions` moves rows (detach/attach) during low traffic.
101. Counter drift (bug). Reconciliation recomputes counters from ledger (ledger truth), logs issue.
102. Workspace restored from backup on cloud (support case). Billing tables never restored from workspace export; ledger stays authoritative; windows rebuilt by replay if needed (`billing:verify-replay`).
103. Outbound fair-use exceeded (teammate creates 1,001st outbound conversation, prior month 50 billable). 429 `outbound_fair_use_exceeded`; inbound unaffected; staff raises `billing.outbound_fair_use_min` via `dimmi entitlement:set` (or contract `entitlements`).

## Billing account, currency, entitlements

104. Workspace created: org inserted first by `CreateWorkspace`, then 12 initializer creates account for org in same tx. Initializer fails: whole `CreateWorkspace` rolls back (no workspace without account). Pre-hook workspace: `EnsureBillingAccount` repairs on first billing touch.
105. `ResolveBillingAccount` cache stale (workspace moved org later). L1 cache 60 s; move action busts it via Valkey pubsub. Activation inside 60 s lands on old account; ledger keeps that `billing_account_id`; credits follow charge line, never re-resolve.
106. Later: second workspace joins org with account. Shares account: one period row, one cap, one free allowance (100 total, not 100 each), combined tiers. No migration. Usage lines endpoint still filtered by request workspace; meter totals are account totals (labelled so).
107. Later: two workspaces of one account reply at same instant to fresh inbounds. Both lock same account period row; serialized; exact cap cut-off across workspaces; no overshoot.
108. Last workspace of org purged. Org soft-deleted (00-accounts); account closed, final invoice issued, billing fields anonymized ("Deleted organization"); ledger, invoices, account row kept 7 years (no FK). Workspace deleted while org has other live workspace (later): account untouched.
109. Workspace restored within grace. Org un-deleted by 00-accounts; `ReopenBillingAccountOnWorkspaceRestored` reopens account (prior status if final invoice not issued, else `active` new period).
110. Staff changes currency before first invoice. Allowed when Stripe customer has no invoices/charges and price book exists for currency; cap reset to default in new currency (no FX conversion), cap math recomputed, owner notified. After first Stripe invoice: 422 `currency_locked`.
111. Account currency without price book (bad config `billing.currency=eur`). Boot fails on cloud (price book registry check); never rate in wrong currency.
112. Credit grant in USD applied to account in other currency. Impossible: grants carry account currency (created against account); staff action rejects mismatched currency 422.
113. Entitlement override with unknown key or wrong type (`help_center.max_per_workspace = true`). `dimmi entitlement:set` rejects (catalog validation), nothing written.
114. Workspace-level override and org-level override for same key. Workspace row wins for that workspace; others in org use org row.
115. Override expires. Driver ignores expired rows at read (cache up to 60 s stale); limit drops to contract/default; existing resources over new limit kept (e.g. 5 help centers when cap back to 3): creation of more refused 422 `entitlement_limit_reached` (`key`, `limit`), nothing deleted.
116. Override lowers limit below current usage. Same as 116: no deletion; only new creation blocked.
117. Entitlements read fails (DB blip). Driver returns catalog cloud default, logs error; never treats as unlimited.
118. Self-host with billing tables present. `self_host` driver bound (11): features true, limits from config; `entitlement_overrides` ignored; `dimmi entitlement:set` exits 1 "cloud only".
119. Org with no billing account (repair gap) hits billing route. `ResolveBillingAccount` runs `EnsureBillingAccount` (insert for org) and continues; reconciliation logs `missing_billing_account` issue.

## Launch features 13, 14, 15 and Valkey

120. Changelog mailing to 10,000 subscribers (13). No ledger lines (fixture 30). Mail replies into inbox later (`changelog-replies-to-inbox`).
121. Feedback voter emails on status change (14) while workspace capped. Sent (not conversation outbound, never gated by 423); no `conversation.created`, D7 fair-use counter unchanged.
122. CSAT request sent on closed conversation whose window already closed (15). System message `substantive false`: no activation, window not reopened; contact's survey answer is a response row, not a message (fixture 32).
123. Valkey down or flushed during reply. Activation path reads Postgres only (period row lock, windows, ledger); entitlement values from L1, else Postgres. Charges, waivers, blocks identical; no reply refused because of Valkey. Lost effects only: missed cache bust (L1 stale up to 60 s, edge 116 rule), extra or missed `billing.meter_updated` nudge (dashboard refetch on focus), rate limits fail open per framework limiter policy. `stripe_events` dedupe unaffected (Postgres unique).

## Launch features 07 actions, 08 docs sync, 16 importers

124. Full 16 import of 1,000 conversations, then delta, then live inbound + member reply. Exactly one charge (fixture `import_never_billed`); import + delta write no ledger lines (no `message.created`, `metadata.imported`).
125. Agent write action: confirmation card, confirm, approval notice, follow-up answer. Cards and notices `substantive = false` (no activation); follow-up `answered` = normal D6 reply (one charge if window not yet activated). Action HTTP calls never ledgered (fixture `agent_action_not_billable`).
126. Docs sync run drafts 5 suggestions on BYOK key. No ledger line (no conversation); tokens count only toward 07 AI spend cap; cloud cap `docs_sync.runs_daily` 200 is fair use, never billed.


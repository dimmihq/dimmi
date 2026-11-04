# 04 Email channel: sender rules

Style: caveman. Owner: 04-email-channel. Ignored addresses (E-04), suppression screen (E-05), link stripping pointer (E-06), trusted senders + rate-limit label (I-142), unverified sender flag. Tasks: [tasks.md](tasks.md) M7, M15. Track 05 email-sending.

## 1. Ignored addresses (E-04)

Own teammate boxes (`sales@acme.com`, `billing@acme.com`) never become customer or copied party. F23 fix covers workspace's own addresses; this is custom list on top. Assume after F23: own-address sender files outgoing under first non-own recipient.

Table `email_ignored_addresses` (migration `20261201050003 create_email_ignored_addresses_table`, RLS, prefix `eig_`):

| Column | Type | Notes |
|---|---|---|
| pattern | citext not null | exact address or `@domain` (includes subdomains only when `@*.domain`); lowercase A-label |
| note | text | max 200 |
| created_by_member_id | uuid null references workspace_members on delete set null | |

Unique `(workspace_id, pattern)`. Cap 500 per workspace: 422 `ignored_address_limit`.

Refused pattern: public mailbox domain as `@gmail.com`: 422 `invalid_pattern` (would hide all customers).

Behavior:

| Where | Effect |
|---|---|
| Inbound To, Cc | not added as participant, no contact created |
| Inbound From, forwarded block present | treated as member forward (`pipeline.md` 7.3): original sender is contact |
| Inbound From, no forwarded block | `inbound_emails.status = ignored`, `ignore_reason = ignored_address`; row shown in activity log with "Stop ignoring" |
| Thread match on ignored From | ignored; no reopen |
| Assignment, auto-assign | no contact so no assignment |
| Reply defaults | removed from default Cc |
| Manual Cc to ignored address | allowed (explicit act), composer warns; recipient not stored as participant |
| Existing contact matching pattern | kept; no retroactive change; mail from it ignored from now on |

Check at routing step f (`pipeline.md` 7.0). Loaded with 60 s cache per workspace, busted on write.

API (`routes/email_senders.go`): `GET /email-ignored-addresses` (`email:read`), `POST`, `DELETE /email-ignored-addresses/{ignored_address}` (`email:manage`), route names `email-ignored-addresses.*`. Bulk add `POST` accepts `patterns[]` up to 100, partial result per bulk shape. Audit: `email_ignored_address.created/deleted`. Realtime none. Webhook none.

UI: `resources/js/features/email/senders/ignored-addresses.tsx`: section on Email settings; BoardUI `table`, `input`, delete with confirm. Activity log row action.

e2e `tests/e2e/email-senders.spec.ts`: add `sales@`; mail copied to it creates no participant; mail from it shows ignored in log.

## 2. See and lift suppression (E-05)

Builds on F51 fix (assume: policy bounces get expiry, bad-address codes only are permanent, lift action exists), F48 (only actual recipients suppressed), F52 (`OriginalRcptTo` respected). API in README table stays; M7 task builds it. Teammate lift is deliberate advantage over Intercom.

Additions:
- `email_suppressions` resource gains `plain_reason` (lang), `liftable` for caller (from permissions), `lift_requires_ops` (cloud SES account-level complaint).
- `GET /email-suppressions?filter[contact]=ctc_...` matches contact's identities.
- Realtime: `email_suppression.created`, `email_suppression.deleted` become realtime on `workspace:{ws}` (payload no extra PII beyond email, members with `contacts:read`), so chip updates live.

UI:
- `resources/js/features/email/suppressions/suppression-list.tsx`: section in Email settings, BoardUI `table`, search by email, reason chip, source, expiry, Lift action.
- `resources/js/features/email/suppressions/suppression-chip.tsx`: 03 customer panel mounts it (delta) and failed-delivery badge in thread (F35 fix). Text "Emails to this address are blocked: mailbox does not exist. Lift block".
- Lift modal: complaint path shows consent warning; `email_suppressions:lift_complaint` plus `confirm_complaint`; hard bounce warns "Will fail again if still invalid".
- After lift, failed message offers F35 retry; no auto resend.

e2e extends `email-settings.spec.ts`: seed bounce via test webhook, chip on contact, lift, list empties.

Edge: lifted then bounces again: new row, no loop guard needed (send is human-triggered). Lift of soft bounce before expiry: allowed by `manage`.

## 3. Strip links (E-06)

Folded into inbox S-12 (`03-shared-inbox`, track 04 customer-records): hover shows real URL, warnings, allow and block lists. 04 adds nothing; keeps sanitized `href` values intact so 03 can act, never rewrites stored mail. Raw mail reach: F18 fix. No setting in 04.

## 4. Trusted senders and rate-limit label (I-142)

Form relay or partner system hits `sender_rate` and vanishes from view. Fix both halves: list to trust, label to see.

Table `email_trusted_senders` (migration `20261201050004 create_email_trusted_senders_table`, RLS, prefix `ets_`): `pattern citext not null` (address or `@domain`), `note text`, `created_by_member_id`. Unique `(workspace_id, pattern)`. Cap 500. Same public-domain refusal.

Trust applies only when mail authenticates: `auth.result = pass` with aligned domain matching pattern. Why: else anyone spoofs partner to skip spam filter. Form relays that send as visitor: pattern is relay's domain, matched against envelope or DKIM `d=`, must still pass DKIM.

| Check | Trusted, auth pass | Trusted, auth not pass |
|---|---|---|
| Sender rate limit, quarantine rate | skipped | applied |
| Spam verdict (SES, heuristic), `X-Spam-Flag` | skipped | applied |
| Noreply and bulk classification (rules 10 to 12) | skipped | applied |
| Auto-reply **subject** heuristic | skipped | applied |
| Auto-reply **header** (RFC 3834 family) | still auto-reply | still auto-reply |
| Virus verdict | never skipped | never skipped |
| Loop, DMARC reject policy fail, blocked contact | never skipped | never skipped |
| AI spoof guard (`ai_skip`) | unchanged | unchanged |

`metadata.email.trusted = true` when applied. Activity log shows it.

### Label

Over `sender_rate` today: `metadata.email.throttled = true`, no notify, no AI (kept). New:

- Field **`conversations.source.email_throttled`** = `{"at": "<rfc3339>", "sender": "<address>"}`. Written through `InboundMessage.Source` merge on throttled inbound. 03 row reads presence and shows label "Too many emails"; resource exposes `email_throttled: {at, sender}` or null. Coordinator: name fixed for 03.
- Quarantined (over `sender_quarantine`) conversation also carries it; shows in spam view with same label.
- Recovery on row and thread, `resources/js/features/email/senders/throttle-banner.tsx` (03 mounts):
  - "Trust this sender": `POST /email-trusted-senders` `{pattern, release: true}` adds row, removes key from every conversation by that sender in last 30 days, moves quarantined ones out of spam (03 action), emits `conversation.updated`; no retro notify; no AI run.
  - "Dismiss": `DELETE /conversations/{conversation}/email-throttle` (`conversations.email-throttle.destroy`, `conversations:write`, `VisibleScope`). 204.

Pattern from auth: UI offers `sender` address and aligned domain.

API: `GET/POST /email-trusted-senders`, `DELETE /email-trusted-senders/{trusted_sender}`. Read `email:read`, write `email:manage`. Audit rows. Member with `conversations:write` only: "Dismiss" yes, "Trust" hidden.

Events: uses `conversation.updated`. No new event.

UI: `resources/js/features/email/senders/trusted-senders.tsx` section beside ignored list. Same pattern in both refused with 422 `pattern_conflict`.

e2e in `email-senders.spec.ts`: 21 mails via test webhook, label appears on row, Trust clears, next 21 unthrottled; Dismiss path.

Tests: table above row by row; spoofed trusted address with DKIM fail stays throttled and spam-checked; config value change.

## 5. Unverified sender flag

Was later: `unverified-sender-flag`. Now in.

`messages.metadata.email`: `unverified_sender` bool, `spoof_risk` (`low`, `high`). Computed at step c/f:

| Condition | Flag |
|---|---|
| `auth.result` = `pass` | none |
| From matches contact with verified identity, not pass | `spoof_risk = high` (also existing `ai_skip`) |
| `auth.result` = `fail` | `low` |
| `none`, From domain has no DMARC record | none: too common, flag would be noise |
| `none` or `unknown`, From domain publishes DMARC | `low` |
| Forwarded through customer's forward and DKIM broken, forwarder trusted (`pipeline.md` 5) | none |

UI: `resources/js/features/email/sender-flag.tsx`, 03 thread mounts at message header. `high`: warning chip "Could not confirm this came from jane@x.com" + tooltip with verdict; link to original (F18). `low`: quiet text in message details only. Row: `high` adds small icon, field read from `conversations.source.email_spoof_risk`, written like throttle key.

No API write. Reply goes to contact's identity address as today. `GET /messages/{id}/email` adds both fields.

Tests: matrix above. e2e: spoof fixture shows chip, passing mail none.

## 6. Permissions summary

Only one new: `email:send_as_any` ([sending.md](sending.md) 2). Rest reuse `email:read`, `email:manage`, `conversations:write`, `contacts:read`, `email_suppressions:*`.

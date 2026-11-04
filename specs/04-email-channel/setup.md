# 04 Email channel: forwarding setup

Style: caveman. Owner: 04-email-channel. Guides per mail host, working test button, mistake detection, "send to my IT admin". Tasks: [tasks.md](tasks.md) M9, M16. Track 06 email-inbound. Builds on F22 fix (assume: `X-Original-Sender`, `X-Original-From` read for own forwarding source; customer becomes contact; mail stays open).

## 1. Choosing guide

`EmailAddressResource.setup_guides` exists (array `{name, steps}`). New shape keeps those keys, adds `key`, `recommended`, `deep_link`, `warnings`:

`RecommendGuide(address)`: public domain list first (`gmail.com`, `googlemail.com` to `gmail`; `outlook.com`, `hotmail.com`, `live.com` to `outlook_com`; `icloud.com`; `yahoo.com`). Else MX of address domain via `a.DNS`:

| MX contains | Guide |
|---|---|
| `google.com`, `googlemail.com` | `google_workspace` |
| `protection.outlook.com` | `microsoft_365` |
| `fastmail` | `fastmail` |
| `zoho` | `zoho` |
| `icloud.com` | `icloud` |
| `cloudflare.net` | `cloudflare_routing` |
| other, none | `generic` |

Lookup cached 1 h on `email_addresses.settings.detected_mail_host`, timeout 3 s, failure: `generic`. User can pick any guide.

Guide content: `app/integrations/email/setupguides/guides.yaml` embedded + lang keys; `{forwarding_address}`, `{address}`, `{domain}` filled. Connect-mailbox option shown first when OAuth enabled for that host ([mailbox-connections.md](mailbox-connections.md)).

## 2. Guides

Every guide ends "Run the test".

### gmail (personal)

1. Gmail Settings, Forwarding and POP/IMAP, Add a forwarding address, paste `{forwarding_address}`.
2. dimmi shows confirmation code live (`pipeline.md` 11). Paste in Gmail.
3. Choose "Forward a copy", keep Gmail's copy.
4. Note: Gmail shows replies from dimmi outside Sent unless mailbox connected.

Deep link: `https://mail.google.com/mail/u/0/#settings/fwdandpop`.

### google_workspace (routing rule, recommended for Workspace)

Admin path, no per-user confirmation code, survives staff change, works for user, alias, or Group address.

1. Admin console, Apps, Google Workspace, Gmail, Routing. Deep link `https://admin.google.com/ac/apps/gmail/routing`.
2. Under Routing, Configure. Name `dimmi {address}`.
3. Messages to affect: Inbound and Internal-receive.
4. Envelope filter: Only affect specific envelope recipients, Single recipient, `{address}`.
5. Also deliver to: Add more recipients, Add, Basic, recipient `{forwarding_address}`.
6. Options: Account types to affect: tick Users, Groups, Unrecognized/Catch-all.
7. Save. Google says up to 24 h, usually minutes. Test can stay red until then.

Alternative listed second: recipient address map (Google's documented per-recipient redirect); same result, no account-type choice.

Why "Add more recipients", not Group: Group rewrites From to `'Jane' via Support`, adds list headers, can hold mail in moderation. Rule keeps original sender, passes DKIM unchanged.

Non-admin in Workspace: can use `gmail` steps unless admin blocked forwarding; "send to IT admin" button prominent.

### google_group ("my support@ is a Google Group")

Entered from finding `google_group_in_front` or guide list.

- Keep Group for humans if wanted. Add routing rule above on Group address (`Groups` account type on).
- Remove `{forwarding_address}` from Group members.
- Alternative if no admin: Group must allow external posting and external member; expect `via` From, F22 fix reads original sender, moderation can hold mail. Works but flagged warning stays.

### microsoft_365

Two blockers named up front:

- Outbound anti-spam policy default blocks automatic external forward: NDR `5.7.520`. Fix: Defender, Anti-spam outbound policy, Automatic forwarding: On, scoped to this mailbox.
- Shared mailbox needs mailbox forward set by admin; rule in Outlook needs user license.

Steps: Exchange admin center, Recipients, Mailboxes, `{address}`, Mail flow settings, Email forwarding, Forward to external SMTP address `{forwarding_address}`, keep copy on. Deep link `https://admin.exchange.microsoft.com/#/mailboxes`. Alternative: transport rule "Bcc to" for tenants that forbid forward policy change.

### outlook_com, fastmail, icloud, zoho, cpanel, cloudflare_routing, generic

Short step lists. `generic`: "Find mail forwarding in your host's panel, forward `{address}` to `{forwarding_address}`, keep a copy". Not an alias that rewrites From. iCloud: forwards whole mailbox only (Mail settings, Forward my email), no per-address rule; copy says so.

## 3. Test button

Today: `POST /email-addresses/{address}/tests` sends mail, sync outcome, mark on arrival. New: stored test with live result. rest.md rule 5 lets owner spec store.

Table `email_address_tests` (migration `20261201060004 create_email_address_tests_table`, RLS, prefix `eat_`):

| Column | Type | Notes |
|---|---|---|
| email_address_id | uuid not null references email_addresses on delete cascade | |
| status | text CHECK in (`sending`,`waiting`,`received`,`failed`,`timed_out`) | |
| token | text not null unique | HMAC-able token in `X-Dimmi-Test`, also last line of body (some forwards drop X-headers) |
| outbound_email_id | uuid null references outbound_emails on delete set null | |
| inbound_email_id | uuid null references inbound_emails on delete set null | |
| findings | jsonb not null default '[]' | section 4 |
| latency_ms | int | |
| expires_at | timestamptz not null | now + 10 min |
| created_by_member_id | uuid null | |
| completed_at | timestamptz | |

Retention 30 days (`PruneEmailAddressTests`). `kind` of mail: `test`, counted in stats, not in daily cap (test must work at cap). Rate 1 per 30 s per address, 20 per day per workspace: 429 `email_test_throttled`.

Flow:

```
POST /email-addresses/{address}/tests        201 email_address_test{status: waiting}
  sends real mail from sender of record to {address} (public address), subject "dimmi forwarding test <short id>"
  customer mail host forwards to {forwarding_address}
  -> normal inbound path (any source) -> pipeline rule 3 matches token -> no conversation
  -> test received, latency, inbound_verified_at, findings from headers
  -> event email_address_test.updated (realtime workspace:{ws})
GET /email-addresses/{address}/tests/{test}  poll fallback
CheckEmailAddressTest job at 10 min -> timed_out + findings
```

Outbound bounce on test mail: `failed` plus finding from diagnostic (section 4). Address with `connected_mailbox`: test proves mailbox sync: mail sent from default sender, arrives through Gmail or Graph sync within sync interval; button copy says so. Address equals forwarding address (default cloud): direct mail, still end to end.

Self-host with no customer-stream mailer: 422 `no_sender_available`; UI shows "Send this from your own mail" with `{forwarding_address}` plus token subject; mail still marks test received. Lets box with inbound only test.

Spam: forwarded test mail landing in customer's spam stops forward at some hosts. `timed_out` copy says check spam at host.

Permissions: `email:manage`. Backward compat: `email_address_test` keeps existing outbound fields. Code exists at `app/actions/email/email_address_test_mail.go`; bug-fix agent may edit it.

UI (`resources/js/features/email/setup/forwarding-test.tsx`, mounted by `AddressCard`): Button "Test forwarding". States: spinner `Sending test`, "Waiting for it to come back (usually under a minute)", green `Forwarding works, took 12 s`, red with findings and fix. Live over realtime, 5 s poll fallback, `aria-live`.

## 4. Mistake detection

`app/integrations/email/setupfindings.go`: pure function `Findings(parsed headers, address, envelope, outbound diagnostics)`; runs on test mail and on every inbound mail of an address during first 30 days or until 100 clean mails, then 1 in 20. Stores in `email_addresses.setup_findings` jsonb (`[{code, severity, first_seen_at, last_seen_at, inbound_email_id}]`, migration `20261201060005 add_setup_findings_to_email_addresses`). Cleared when 20 clean mails in a row. Event `email_address.setup_findings_changed` (realtime, no webhook).

| Code | Detection | Severity | Plain message and fix |
|---|---|---|---|
| `google_group_in_front` | `X-Google-Group-Id`, `List-ID` ending `googlegroups.com` or own domain with `X-Original-Sender`; From `via` pattern | warn | "support@ is a Google Group. Replace it with a routing rule so customers stay the sender." opens `google_group` guide |
| `mailing_list_in_front` | `List-Id`, Mailman, Listserv on first-hop with own domain | warn | list adds footers and can hold mail |
| `m365_group_in_front` | `X-MS-Exchange-Organization-` distribution expansion, `Sender` != `From` on own domain | warn | Microsoft 365 group or list |
| `m365_external_forward_blocked` | test outbound or bounce diagnostic has `5.7.520` | error | outbound policy fix |
| `forward_rewrites_from` | From domain = address domain, From display contains customer-looking name, Reply-To other domain | warn | SRS-style rewrite loses sender |
| `dkim_broken_by_forward` | DKIM signature exists and fails while first-hop host is customer's | info | identity not auto-verified; connect mailbox or fix footer rule |
| `subject_rewritten` | test subject changed (`[External]`, `[Support]`) | info | threading fallback affected |
| `wrong_address_forwarding` | arrives at forwarding address but `original_recipient` not `{address}` | warn | other address forwards here |
| `gmail_confirmation_pending` | Gmail confirmation stored, no mail since | error | shows code |
| `forward_loop` | more than 20 `Received`, or own stamp seen twice | error | host forwards back |
| `helpdesk_in_front` | `X-Zendesk-*`, `X-Freshdesk-*`, `X-HelpScout-*` | warn | another helpdesk still on the path |
| `slow_forward` | test over 5 min | info | host queues |
| `no_mx_for_address` | no MX at address domain | error | domain cannot receive |
| `not_receiving` | nothing for 14 days after working; was active | warn | forward may be off (daily job) |

Test `timed_out` adds guide-specific hints: Workspace rule delay, Gmail not confirmed, M365 policy, spam at host.

UI: `resources/js/features/email/setup/setup-findings.tsx` in `AddressCard`. Error: BoardUI alert style with `text-text-error-primary`, warn chip, info quiet. "Show the email" links `inbound-emails/{id}` for `email:read_raw`.

Test matrix: fixtures per code in `tests/fixtures/email/setup/`, captured from staging mailboxes (`staging.md`).

## 5. Send to my IT admin

`POST /email-addresses/{address}/instruction-emails` (`email-addresses.instruction-emails.store`, `email:manage`, 202): `{to: ["it@acme.com"], guide: "google_workspace", note?: "max 500 chars, plain text"}`. Max 5 recipients. Rate 5 per hour per address, 20 per day per workspace: 429 `instructions_throttled`. Same for `POST /email-domains/{domain}/instruction-emails` with records.

Mail: system stream Mailable `app/mail/email_setup_instructions_mail.go` (templ + text). Arch rule: `tests/arch/email_streams_test.go` allowlists this one Mailable for non-member recipient. Why system: recipient is not customer contact; no workspace customer-stream reputation risk. Body: who asked (member name, workspace name), fixed guide text, values, signed page link. User note plain text, no links rendered (URL text escaped so phish via our domain is pointless). `Reply-To` asking member. Audit `email_address.instructions_sent` with recipients.

Share page: `GET /email-setup/{address}` web route, `signed` (HMAC query), 14-day TTL, no login, noindex; regenerated per mail. Shows guide steps, values, record table for domain variant, test button: `POST /email-setup/{address}/tests` same signed route; result polls `GET` same page JSON fragment. Shows only address, forwarding address, guide: nothing else from workspace. Deleted address: 410 page. Served by Go view in `resources/views/email/setup_share.templ`, no React bundle, tokens from compiled CSS.

Rest note: paths nouns; signature query allowed under rest.md web-page rule.

UI: `resources/js/features/email/setup/send-to-admin.tsx` (BoardUI `modal`, `input`). Button beside guide: "Send these instructions to my IT admin". Toast with recipients.

e2e `tests/e2e/email-setup.spec.ts`: choose guide by seeded fake DNS; run test via `/webhooks/test/inbound` shows green; Group mail shows finding; send to admin reaches Mailpit, opened link renders no-login page, runs test.

## 6. Rejected: Gmail forwarding automation through domain-wide delegation

Workspace admin grants service account `gmail.settings.sharing`; dimmi sets forward itself. **Rejected.** Saves one admin step at best: admin still has to create delegation with broad scopes (bigger ask than routing rule). Fails for Google Groups (no user mailbox to set), the case owners struggle with. Rule from section 2 needs no credential on our side.

## 7. Edge cases

Numbers 216 to 224 in [edge-cases.md](edge-cases.md).

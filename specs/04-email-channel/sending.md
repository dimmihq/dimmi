# 04 Email channel: sending identity

Style: caveman. Owner: 04-email-channel. Read with [outbound.md](outbound.md) (send flow, sections 2, 11, 13, 14). This file: send from customer's own address (E-01), choose address (E-02), signatures (E-03), files hosted or attached (E-07), template style (E-08), guided DNS (E-09), provider domain APIs. Tasks: [tasks.md](tasks.md) M6, M8, M14. Track 05 email-sending.

Bug-fix agent still edits `app/actions/email` and `app/integrations/email`. Builders re-read those files first. Assumptions after fixes named per section.

## 1. Own address and domain authentication (E-01)

Rule: **unproven address never sends.** Three proofs, nothing else:

| `sending_state` (computed, on address resource) | Proof | From |
|---|---|---|
| `own_domain` | `email_domains.status = verified`, exact domain match (`outbound.md` 11.3) | `support@acme.com` through instance provider |
| `connected_mailbox` | active OAuth connection for that address or accepted alias ([mailbox-connections.md](mailbox-connections.md)) | `support@acme.com` through mailbox API |
| `default_sender` | none needed | forwarding address on our domain; Reply-To token |

Mailed-link address check (Intercom): not used. Why: DNS ownership proves more and covers every address on domain; link proves one inbox and adds step. Mistyped address on verified domain still shows `inbound_verified_at` null and setup test fails, so typo visible.

API additions on `EmailAddressResource`: `sending_state`, `sends_as` (exact From a customer sees now), `sending_blocked_reason` (null or `domain_degraded`, `mailbox_reauth_required`, `sending_paused`). Dashboard "Sends as" line reads `sends_as` only; `compose_reply.go` line-80 behavior (always forwarding address) replaced by `outbound.md` section 2 resolution.

Records, checker, status machine: `outbound.md` section 11, unchanged. Per-record live status lands on `email_domain.status_changed` plus new `email_domain.records_updated` (realtime only, `workspace:{ws}`, admins): page re-reads domain without reload.

### Plain-language errors

Every `warnings[]` code and record status has lang key `email.dns.<code>` with: what is wrong, which record, what to do. `failure_reason` is also lang key, never provider string. Raw provider text stays in `records[].observed` only. Lint: `lang:check` fails when checker code has no key. Sample:

| Code | Copy |
|---|---|
| `doubled_domain` | "We found this record at `_dmarc.acme.com.acme.com`. Your DNS host adds your domain for you, so enter only `_dmarc`." |
| `cloudflare_proxied` | "This record is proxied through Cloudflare. Open it, set Proxy status to DNS only, save." |
| `duplicate_spf` | "`acme.com` has two SPF records and mail servers reject both. Replace them with this one record." + merged value |
| `propagating` | "One DNS server sees this record, another does not yet. Usually clears within an hour. Nothing to do." |

### UI

- `resources/js/features/email/domains/domain-list.tsx`: section in `features/settings/pages/email.tsx` (Section block, `Chip` status, Add domain `Input` + `Button`).
- `resources/js/features/email/domains/domain-page.tsx`: route `/d/settings/email/domains/$domain` registered from `features/settings/views.tsx`. BoardUI `table` block: purpose, type, host, value (copy buttons, `font-mono` LTR region), status chip, observed value in `tooltip`. Check now, delete, DKIM rotate. Warnings as list above table. Host guide panel: section 6.
- Realtime through existing `features/realtime`; no polling when socket live, 30 s polling fallback.
- `resources/js/lib/api/email-domains.ts` query + mutations. Semantic tokens only.
- e2e `tests/e2e/email-domains.spec.ts`: add domain, see records, doubled-domain warning from fake resolver seed, check now, row turns green (light, dark).

Permissions: existing `email:read`, `email:manage`, `email:rotate_dkim`. Delete verified domain in use: existing 409 `domain_in_use`.

## 2. Choose sending address (E-02)

Builds on F16 fix: assume `OutboundMessage.Options`, `CC`, `ReplyTo` are filled from `channel_options.email` and stored participants.

Resolution stays `outbound.md` section 2: request `from_address_id`, else address conversation came in on, else inbox default, else workspace default.

Additions:
- `PATCH /email-addresses/{address}` `{is_default: true}`: one tx clears previous, sets new (partial unique index holds). Needs `email:manage`. Refuse `is_default: false` on current default: 422 `default_address_required`, choose other instead.
- `GET /conversations/{conversation}/email-sender-options` (`conversations.email-sender-options.show`, `conversations:read` + `VisibleScope`): `{default_address_id, options: [{id, address, display_name, sends_as, sending_state, selectable, reason}]}`. Singleton computed resource. Lists active addresses of workspace.
- New permission `email:send_as_any` (ability group `conversations:write`; owner, admin, member by default; custom roles may remove). Without it: picker shows conversation address only; request with other `from_address_id`: 403.
- Foreign or deleted address id: 404. `default_sender` state stays selectable, shown as "Sends from your dimmi address".
- `new_conversation`: picker required when workspace has two or more addresses; default preselected.

UI: `resources/js/features/email/sender-select.tsx` (BoardUI `select`). 03 composer mounts it on email conversations (delta to track 03: mount point, prop `conversationId`, value goes to `email.from_address_id`). Settings: "Default" chip plus "Make default" action on address card.

Event: `channel.updated` existing. No new realtime.

## 3. Signatures (E-03, cross-domain X4)

04 owns. Table exists; migration `20261201050001 alter_email_signatures_for_address_level`:

```sql
ALTER TABLE email_signatures
  ALTER COLUMN workspace_member_id DROP NOT NULL,
  ADD COLUMN logo_attachment_id uuid REFERENCES attachments ON DELETE SET NULL,
  DROP CONSTRAINT email_signatures_workspace_member_id_email_address_id_key;
-- one row per pair
CREATE UNIQUE INDEX email_signatures_scope_key ON email_signatures
  (workspace_id, workspace_member_id, email_address_id) NULLS NOT DISTINCT;
-- must name a member or an address; workspace-wide blank row is meaningless
ALTER TABLE email_signatures ADD CONSTRAINT email_signatures_scope_check
  CHECK (workspace_member_id IS NOT NULL OR email_address_id IS NOT NULL);
```

Kinds:

| member | address | Kind | Who edits |
|---|---|---|---|
| set | null | personal | that member (`manage_own`), others (`manage_any`) |
| set | set | personal for one address | same |
| null | set | address signature (team, brand) | `email_signatures:manage_any` |

Resolve at compose, first enabled wins: member + address, member, address. AI reply and system message: address signature only, never member's (`outbound.md` 14 updated). Reply option `email.signature: false` skips it for that send.

Body: Markdown subset (bold, italic, links, line breaks), 2,000 chars. Variables replaced at compose, unknown left literal and refused on save with 422 field error: `{{member_name}}`, `{{member_title}}` (00-accounts profile, empty line removed when blank), `{{address_name}}` (`display_name`), `{{company_name}}` (X5 chain). Logo: one PNG or JPEG, max 200 KB, max rendered height 48 px, uploaded through 03 attachment upload (`POST /attachments`), then `logo_attachment_id` on signature. Sent inline `cid`, never remote URL: no tracking possible and image still shows offline. Scan: 03 S-01; unscanned or infected logo left out of mail, chip on editor.

API: existing CRUD plus `POST /email-signatures/renderings` (`email-signatures.renderings.store`, 201, `email_signature_rendering`): body `{body, email_address_id?}` gives `{html, text}`. `GET /email-signatures` filters: `filter[member]`, `filter[address]`, `filter[kind]`.

Inbound own-signature stripping unaffected. Text twin: `-- ` delimiter plus Markdown as plain text.

UI:
- `resources/js/features/email/signature-editor.tsx` (BoardUI `textarea`, `file-upload`, live preview in sandboxed iframe from renderings endpoint).
- Personal: section "Email signature" in `features/settings/pages/account.tsx`, replacing static form. 00-accounts preferences page links here (X4).
- Address signature: section on address card in `pages/email.tsx`; list of other members' for `manage_any`.
- Composer: small "Signature" toggle beside sender select (03 mounts).
- e2e `tests/e2e/email-signatures.spec.ts`: member saves, reply preview shows it, address signature wins when no personal, member without `manage_any` sees no team editor.

Events: none. Edge cases 203 to 206.

## 4. Files hosted or attached (E-07)

Builds on F15 fix: assume real uploads link attachments to message and `OutboundMessage.Attachments` filled. If F15 only hides control, 03 track 03 composer work supplies upload (`03` S-01 and X7 own scan + download route). 04 only reads `Attachments`.

Address setting `settings.outbound_attachments`: `attach` (default) or `link`.

| Setting | Files under 20 MB total | Over 20 MB total | Executables |
|---|---|---|---|
| `attach` | MIME attachments | largest become links until under 20 MB (existing) | always link |
| `link` | all links | all links | always link |

Inline images (`cid`, signature logo) always inline. Links: X7 signed download route `/files/{att}/{filename}`, TTL `outbound.attachment_link_ttl` (7 days). Link block in HTML and text twin: filename, size, expiry date. Expired link page offers nothing to public: customer asks by replying. Mailbox send (`connected_mailbox`) lowers threshold per driver, same rule (largest become links, never rejected): gmail 25 MB (upload endpoint), microsoft 3 MB of attachments (`sendMail` with base64 MIME; request cap about 4 MB `[staging]`; large upload session needs `Mail.ReadWrite`, which we do not request).

UI: `Choice` on address card ("Send files as attachments / as links"). Test: golden `.eml` per row.

## 5. Template style (E-08)

Decision: two styles, no designer. Plain stays default: reads as person wrote it, best inbox placement. HTML designer and arbitrary HTML stay out.

`email_addresses.settings`: `template` (`plain` default, `branded`), `brand_logo_attachment_id`, `brand_color` (`^#[0-9a-f]{6}$`), `footer_markdown` (500 chars; same subset as signature).

`branded`: logo header (inline `cid`, 48 px), 2 px color rule, footer text, table layout that survives Outlook. Same quote, same signature, same text twin (logo dropped, footer kept). No remote assets. Applies to `reply`, `ai_reply`, `system`, `new_conversation`, `widget_offline`; contact mail (13, 14) keeps caller template.

API: `PATCH /email-addresses/{address}` `settings.*` (`email:manage`), `POST /email-addresses/{address}/template-renderings` (201, `email_template_rendering` `{html, text}`, sample content).

Templ: `app/mail/templates` untouched; `app/integrations/email/templates.go` picks `conversation_reply.templ` or `conversation_reply_branded.templ`.

UI: `resources/js/features/email/template-settings.tsx` on address card: `segmented-control` style, logo `file-upload`, BoardUI `color-picker`, preview iframe sandboxed (raw color is user's brand data, only inside mail preview; page itself uses tokens). e2e covers switch and preview.

Tests: golden HTML per style, no `http` image `src`, no tracking pixel, link hrefs not rewritten.

## 6. Guided DNS (E-09)

**Decided:** plain records + live checker + per-host instructions + deep links, for everyone. Domain Connect on top where provider onboarded. Entri rejected: paid third party, vendor widget in our page handling customer's DNS sign-in. No third party in default path.

### 6.1 Host guides

`app/integrations/email/dns/hosts.yaml` (embedded): one entry per `provider_hint`: `cloudflare`, `route53`, `godaddy`, `namecheap`, `squarespace` (ex Google Domains), `vercel`, `netlify`, `porkbun`, `hover`, `other`.

Each entry: NS match patterns; `deep_link` template with `{domain}`; steps lang keys; `host_form` (`relative` or `fqdn`); quirks (`cloudflare`: proxy off for CNAME; `route53`: quote TXT, full name; `godaddy`: `@` for root; `namecheap`: drop trailing domain, TTL Automatic); TXT splitting rule.

Deep links: best known page for that domain's DNS records. Wrong link is harmless (lands in host dashboard): staging checklist re-verifies quarterly (`staging.md` 7). `other`: BIND export and copy-all.

API on `GET /email-domains/{domain}`: `provider_hint`, `dns_host {name, deep_link, steps[], quirks[]}`, each record `host` already in host's form, plus `host_fqdn`. `?format=bind` existing. User can override guess: `PATCH /email-domains/{domain}` `{dns_host: "porkbun"}`; stored in new column `dns_host_override`.

Share: "Send to whoever manages DNS": `POST /email-domains/{domain}/instruction-emails` same rules as [setup.md](setup.md) 5.

### 6.2 Domain Connect

Verified: open standard, templates via public PR to `Domain-Connect/Templates`. Cloudflare takes onboarding by email after merge, supports only signed synchronous flow, no contract named in its doc. Other hosts vary and some may ask contract terms. So: **real path, no human contract needed for Cloudflare; each host is human onboarding; code must work with zero hosts onboarded.**

Flow (`app/integrations/email/dns/domainconnect.go`, no library: Go has none maintained; stdlib RSA-SHA256 plus `a.DNS`, `a.HTTP`, about 150 lines):

1. On domain load: TXT `_domainconnect.<domain>` gives API host. `GET https://<host>/v2/<domain>/settings` gives `urlSyncUX`. `GET .../v2/domainTemplates/providers/<provider_id>/services/<service_id>`: 200 means template onboarded. Cache 1 h in Valkey. Timeouts 3 s; any failure hides button.
2. Button "Set up automatically at Cloudflare". `POST /email-domains/{domain}/dns-setups` (`email:manage`, 201, `email_domain_dns_setup`) returns signed `apply_url` with vars `dkim1..3`, `mailfrom`, `token`, `region`, `redirect_uri`; signature last query param; `key` names TXT host.
3. Customer approves at host; redirect to `/d/settings/email/domains/{id}`; page starts check.

Template: `deploy/domainconnect/trydimmi.com.email.json`: providerId `trydimmi.com`, serviceId `email`, `syncBlock: false`, `syncPubKeyDomain`, SPF as `SPFM` (host merges into existing SPF), TXT ownership, three DKIM CNAME, MAIL FROM MX and SPF, DMARC TXT `essential: OnApply`. No `APEXCNAME`. No proxy on CNAME (declare in onboarding mail).

Config: `channels.email.domain_connect.{enabled, provider_id, service_id, private_key, key_host}`. Default off. Cloud on after human step. Self-host off: template is signed with our key and only fits `ses` records. Only `ses` driver offers button.

Reuse check: none needed; `domainconnect_python` exists but no Go module; signature is 10 lines of stdlib.

Tests: fake at `httpclienttest` and `a.DNS.Fake()`: template onboarded shows button; 404 hides; signature verifies with test public key; variable values match `records`.

### 6.3 Agents

`dimmi_get_email_domain` MCP keeps exact records so Claude Code can set them through a DNS tool. Nothing new.

## 7. Provider domain APIs

Manual provider setup stays valid. New: when `channels.email.<provider>.domain_api_token` set, per-record status comes from provider with no operator step and no `provider_verified_domains` entry.

Framework: optional interface in `framework/mail`:

```go
type DomainManager interface {
    CreateDomain(ctx context.Context, domain string) (ProviderDomain, error)
    GetDomain(ctx context.Context, id string) (ProviderDomain, error)
    VerifyDomain(ctx context.Context, id string) (ProviderDomain, error)
    DeleteDomain(ctx context.Context, id string) error
}
type ProviderDomain struct { ID string; Verified bool; Records []DomainRecord }
type DomainRecord struct { Purpose, Type, Host, Value string; Priority int; Valid bool }
```

| Driver | Create | Read and verify | Records come from | Credential |
|---|---|---|---|---|
| postmark | `POST /domains` | `GET /domains/{id}`, `PUT /domains/{id}/verifyDkim`, `PUT /domains/{id}/verifyReturnPath` | `DKIMHost`, `DKIMTextValue`, `DKIMPendingHost`, `DKIMPendingTextValue`, `ReturnPathDomain`, `ReturnPathDomainCNAMEValue`, `ReturnPathDomainVerified` | `X-Postmark-Account-Token` (account token, not server token) |
| resend | `POST /domains` | `GET /domains/{id}`, `POST /domains/{id}/verify` | `records[]` `{record, name, type, value, status, ttl, priority}` | API key |
| mailgun | `POST /v4/domains` | `GET /v4/domains/{name}`, `PUT /v4/domains/{name}/verify` | `sending_dns_records[]` `{record_type, name, value, valid}` | API key, basic auth |
| sendgrid | `POST /v3/whitelabel/domains` | `GET /v3/whitelabel/domains/{id}`, `POST /v3/whitelabel/domains/{id}/validate` | `dns.mail_cname`, `dns.dkim1`, `dns.dkim2` `{valid, type, host, data}` | API key, domain auth scope; EU base `api.eu.sendgrid.com` by config |

Exact fields captured in fixtures from provider docs; staging run confirms (`staging.md` 5). Resend and SendGrid verify paths from memory of public API references: fixture test pins them; first staging run corrects.

Domain row: migration `20261201050002 add_provider_fields_to_email_domains`: `provider_domain_id text`, `dns_host_override text`, `domain_connect_applied_at timestamptz`. `records` jsonb carries provider records as real `verified`/`missing` rows instead of `info`. `verified` when ownership TXT passes and provider says verified: sets `provider_trusted_at`. Tracking settings: mailgun, sendgrid, resend domain tracking turned off on create (Never list). `DELETE` calls `DeleteDomain`; failure logged, soft delete continues, provider row orphan listed by `dimmi email:doctor`.

Without token: `provider_verified_domains` flow unchanged; UI says "Your operator sets this up in Postmark". `smtp` relay: no API. Never per workspace.

Tests: `drivertest` contract on each driver with `httpclienttest` fixtures; checker test records status per driver; driver change edge 107.

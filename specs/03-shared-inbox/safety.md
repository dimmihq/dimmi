# 03 Shared inbox: safety and data lifecycle

Style: caveman. Gaps S-01, S-02, S-03, S-11, S-12 (with E-06), S-15, S-16, I-120, I-121, I-140, I-141, P-03. X7, X9. Track 04 customer-records, except I-121 in track 12. 01 and 04 reference this, never re-specify.

## Malware scan (S-01)

Must land with F14 download route. Assume after F14: timeline returns attachments, `GET /files/{att}/{filename}` exists with safe headers.

`framework/malware` drivers exist (clamav, guardduty, fake), manager `a.Malware`. `app/` finally calls it: action `attachments.ScanAttachment` in job `scan_attachment` (exists in README table).

| Scanner result | `scan_status` | Download |
|---|---|---|
| clean verdict | `clean` | yes |
| infected | `infected` (+ `scan_signature`) | never; bytes deleted, row kept, 410 `attachment_infected` |
| unscanned (too large, encrypted archive) | `inconclusive` | with warning, explicit |
| `ErrPending` | stays `pending`, job snoozes 30 s up to 15 min | no until then |
| repeated `ErrScanFailed`, retries out | `inconclusive`, metric, log | with warning |
| driver `none` | `skipped` | yes; UI "Not scanned" |
| sniffed type differs from name's type family | `mismatch`, set after clean verdict (infected wins) | with warning |

- `20261201040008_alter_attachments_scan_status.sql`: CHECK adds `inconclusive`, `mismatch`; columns `scan_signature text`, `scanned_at timestamptz`.
- Config `attachments.scan_driver`: self-host default `none`; cloud `clamav`. Human step: cloud runs clamd or enables GuardDuty; doctor warns on `none`.
- Sources: every upload path ends with job: 03 multipart, 01 complete, 04/05 inbound, 16 `AttachImportedFile`, draft inline.
- Warning download: `GET /files/...&acknowledge=1` signed variant only issued by `POST /attachments/{att}/download-grants` (`conversations:read`, audit `attachment.warned_download`). Plain link 409 `attachment_scan_warning`. Rest: grant has identity, child collection. `pending`: 409 `attachment_scan_pending`.
- Widget and customers: only `clean`, `skipped`. Warning states never served to customers.
- Outbound: waits per `scan_wait_max`; then `inconclusive`, `mismatch`, `pending` fail send `attachment_not_cleared`, teammate sees reason; `infected` fails immediately. `skipped` sends.
- Events: `attachment.scanned` `{id, status}` on `conversation:{conv}`, webhook no. UI `attachments.tsx`: badges "Scanning", "Not scanned", warning dialog; infected row "Removed: malware found".
- Executables refused on ingest (F47 fix hardens trailing dots); stays.

## Attachment policy (S-02)

Workspace settings (registry), page `resources/js/features/settings/pages/conversation-safety.tsx` (new, nav under Inbox; permission `settings:write`):

| Key | Default | Effect |
|---|---|---|
| `attachments.customer_uploads` | true | false: widget hides upload, `01` API 403 `uploads_disabled`; inbound mail keeps message, drops files into `skipped_attachments` metadata, thread shows "Files removed by policy" |
| `attachments.allowed_extra_extensions` | `[]` | add to default allowlist, max 50, never override hard block |

- Default allowlist constant `app/support/attachments/policy.go`: images, pdf, office, text, csv, json, log, zip, common audio and video. Customer-side only; teammates limited by hard block only.
- Hard block list (executables, scripts) fixed. Not configurable.
- Not allowed: 422 `attachment_type_blocked`; inbound: dropped with metadata. Size limits config only: `filesystems.max_upload_mb`, 50 file email override.
- 01 upload path calls `policy.Allowed(ws, name, type)`.

## Expiring links (S-03)

Better than Intercom: short links and member check on read.

| Who | How | Life |
|---|---|---|
| dashboard, API, MCP | `GET /attachments/{att}` (auth + `VisibleScope` of conversation) returns signed `url` | 5 min |
| widget | `WidgetMessageResource` url, signature bound to attachment + contact session contact | 30 min (01 aligns, was 1 h) |
| webhook payload | no url; `GET /attachments/{att}` fetch | n/a |
| email | bytes attached, never links | n/a |
| export zip | link to zip | 24 h |

Signed URL carries attachment id, expiry, HMAC (`framework/routing` signing). Revocation: message deleted, redacted, erased, infected: row or bytes gone, 404 or 410 even with live signature. Expired in open tab: client refetches resource (edge 90).

Signed link is bearer for its 5 minutes; accepted, same class as S3 presign; `Referrer-Policy: no-referrer`, `Cache-Control: private, no-store`.

## Redaction (S-11)

Before save, irreversible. Runs inside every write path: `ReceiveInboundMessage` (all channels, 01 widget, 18 SDK via 01), `ReplyToConversation`, `AddNote`, `post_contact_message`, agent replies, subject excerpt. Package `app/support/redaction`, stdlib `regexp` (RE2, linear time; no timeout needed, 64 KB body bound). No library.

Built-in rules, workspace settings:

| Key | Default | Rule |
|---|---|---|
| `redaction.card_numbers` | true | 13 to 19 digits with spaces or dashes, Luhn valid, known IIN length; keeps last 4: `•••• •••• •••• 4242` |
| `redaction.us_ssn` | false | `\b\d{3}-\d{2}-\d{4}\b` excluding invalid ranges; `•••-••-••••`. Off because false positives on order numbers |

Card on by default: storing card numbers is never wanted; Luhn keeps false positives rare.

Custom rules: `redaction_rules` (`rxr_`; `rdr_` taken): `workspace_id`, `name`, `pattern` max 256 chars, `replacement` default `[redacted]`, `enabled`, `position`, timestamps. Limit 10, 422 `redaction_rule_limit_reached`. Compile at save, bad regex 422. Migration `20261201040009_create_redaction_rules_table.sql`.

- Order: built-ins, then custom by position. Applied to `body`, `body_html`, subject. Compiled cache per workspace, busted by event.
- `messages.metadata.redactions: [{rule, count}]`, no original anywhere. Thread badge "2 items redacted". Metric count, never content. Logs never hold pre-redaction text.
- Email raw: ingest stores raw before parse. Any hit: 04 drops raw `.eml` (marks `raw_deleted_at`, reason `redacted`), full HTML reduced to redacted form. Attachments not scanned for text: stated in settings copy.
- Files, images: not touched.
- Old messages not rewritten when rule added. Teammate deletes specific message (I-36) for history.
- Dedupe, threading unaffected; applied after dedupe key computed from provider id.
- Test without storing: `POST /redaction-tests {text}` computed (rule 11; `settings:write`, `redaction:manage`), returns output; text not logged.
- API `GET/POST /redaction-rules`, `PATCH`, `DELETE`, `PATCH /redaction-rules {order}`. Permission `redaction:manage` (`settings:write`; owner, admin). Audit all rule changes. Events `redaction_rule.*` internal, webhook no.
- UI on `conversation-safety.tsx`: toggles, rule list, test box. e2e `tests/e2e/conversation-safety.spec.ts`.

## Link safety (S-12, E-06)

Workspace settings: `link_safety.enabled` true, `link_safety.allow_domains` `[]`, `link_safety.block_domains` `[]` (max 500 each, public suffix aware match), `link_safety.strip_inbound_email_links` false.

Client, in `message-body.tsx`, `features/inbox/thread/safe-link.tsx`; unsafe schemes stripped today stays:

- Every link in customer-authored message: tooltip with real URL, host emphasized.
- Warning `Modal` on click when: display text looks like different host, host is IDN or punycode mixed-script, URL shortener list, raw IP. Allow list skips warning; there is no allowlist-only mode. Copy: "Opens bad-host.example. Continue?"
- Blocked domain: not clickable, marked. `rel="noopener noreferrer"` always.
- Strip option: email messages render links as plain text with "Show links" per-view reveal gated `email:read_raw`; raw stays reachable (F18 view original, 04 retention).
- Rendered, not rewritten: stored body untouched, switching takes effect at once.
- No external reputation lookup: third party would receive customer links; self-host parity. Intercom's "malicious" feed narrowed to heuristics plus block list; reported.
- Settings delivered in `GET /workspace/settings`-style existing resource, `workspace.updated` realtime. Controls on `conversation-safety.tsx`.

## Spam folder (I-140)

Builds on F37 (blocked or spam sender no reopen) and F61 (spam sticks to sender thread).

- Sidebar `spam` item with count. Row actions: Not spam, Delete. Bulk both.
- `PATCH spam: false`: returns where it stood (status untouched, column comment rule). Marking spam does not change status: differs from Intercom close, on purpose: unmark restores exactly. Default views exclude spam anyway.
- Not spam dialog offers "Always trust this sender": calls 04 trusted sender API when sender email known (04 owns, I-142).
- Marking spam dialog offers "Also block this person" (`block_contact`).
- `spam_reason` already metadata from inbound: panel shows "Filtered: sender throttle" etc. Visible rate-limit label belongs to 04.
- Retention per S-16: `inbox.spam_retention_days` 30 then trash. Header copy "Deleted after 30 days". Spam never notifies, never bills (exists).
- e2e `tests/e2e/inbox-spam.spec.ts`.

## Block (I-141)

- Panel and profile: Block. `Modal` states effect. `PATCH /contacts/{id} {blocked}` exists.
- Effect: new inbound kept as spam, no reopen, no notification (F37). Refusing mail at SMTP level: no. Keeping is safer than losing, bounce invites attack; retention clears. Widget: 01 shows normal UI, conversation lands spam.
- `/d/customers/blocked` tab lists, Unblock. Blocked contact drops from default directory (`filter[blocked]` default false).
- Unblock: past spam stays spam until teammate says otherwise.
- API tokens: same route. MCP `dimmi_update_contact`.

## Trash and restore (P-03)

`DELETE /conversations/{id}` (trash, `conversations:delete`), API and dashboard. Hard delete only through purge. Reactivated: `conversation-restore`.

- `PATCH /conversations/{id} {trashed: false}`: action `RestoreConversation`, event `conversation.restored` (realtime + webhook), timeline row. Sent alone, like `trashed: true`. Status as it stood. Holder of `conversations:delete`.
- Restore conflict: non-closed conversation holds same thread key: 409 `thread_conflict` `data.conversation`; restored only after that closes or merge.
- `deleted` view with Restore. No immediate purge from UI: privacy path is erase. Copy "Removed after 30 days".
- Bulk: `changes.trashed: false` allowed. Filter bulk `filter.view = deleted` allowed.
- Trash has VisibleScope.

## Erase and export one customer (S-15, X9)

Actions, hook registries, API in README and api.md. Decisions added:

### Grace period

Recoverable 7 days, like Intercom; contains mistake cost. `contacts.erase_grace_days` config default 7, 0 allowed. Per request `immediate: true` when legal hold not in force and caller re-confirmed.

`contact_erasures` (`ces_`): `workspace_id`, `contact_id` (no fk, survives), `status CHECK in ('pending','pending_hold','completed','cancelled')`, `requested_by_member_id`, `scheduled_for`, `completed_at`, `counts jsonb`, `identities_stash bytea` (encrypted with `encryption`, wiped at completion), timestamps. Partial unique `(workspace_id, contact_id) WHERE status IN ('pending','pending_hold')`. Migration `20261201040010_create_contact_erasures_table.sql`.

Flow:

1. `DELETE /contacts/{id}?erase=true[&immediate=true]`: `contacts:erase` + `conversation_access = all` + recent confirmation for immediate (`password.confirm`). Tx: `deleted_at`, stash identities and delete their rows (unique index would hold address hostage), insert row, schedule `erase_contact` at `scheduled_for`. 202 `contact_erasure`.
2. During grace: contact hidden, conversations hidden from lists (contact deleted filter) and no outbound (422 `contact_unreachable_on_channel`). Inbound from same address creates new lead: person writing again is a new request, not reversal.
3. Cancel: `PATCH /contact-erasures/{ces} {status: "cancelled"}`, same permission. Restores `deleted_at`, identities unless address now taken (then listed in response `identities_conflicted`, surfaced for manual merge).
4. Job: `RetentionGuard`, core wipe, every `ContactEraser`. Retried until all nil. `completed`, stash wiped, audit.

`GET /contact-erasures`, `GET /contact-erasures/{ces}`: `contacts:erase`. Replaces audit-only status. New hooks required: 04 raw + outbound, 01 widget sessions, 18 no server state beyond sessions, 19 none. Search index: generated columns clear with body. Backups age out: documented in privacy page, 11.

Core wipe adds: drafts, reactions, translations, side conversation copies on contact's conversations, `contact_tag`, notes, attachments bytes (all stores), export zips.

### Export

`POST /contacts/{id}/exports` (`contacts:erase` + `conversation_access = all`). Zip: `contact.json`, `identities.json`, `conversations/<number>.json`, `conversations/<number>.txt` transcript, `attachments/`, hook sections. Internal notes excluded, as personal data request answers what customer gave and was told; `include_notes: true` optional for legal. Attachments with `infected` omitted, listed. `contact.export_ready`. UI on profile, ready toast with signed link. Includes data held under merged tombstones (`contact_merges`).

Rows, jobs exist. e2e `tests/e2e/customer-privacy.spec.ts` with Mailpit raw check absent.

## Retention (S-16)

Workspace settings, page `conversation-safety.tsx`, permission `settings:write`, changes audited, all pass `RetentionGuard`:

| Key | Default | Range | Effect |
|---|---|---|---|
| `retention.closed_conversation_days` | null (keep) | 30 to 3650 | closed conversations with no activity moved to trash; trash rule then purges |
| `retention.spam_days` | config `inbox.spam_retention_days` (30) | 7 to 365 | exists, now per workspace |
| `retention.trash_days` | config `inbox.deleted_retention_days` (30) | 7 to 365 | |
| `retention.raw_email_days` | config `inbound.raw_retention_days` (30, 04) | 0 to 365 | read by 04 `PruneInboundEmailRaw`, 0 here means delete at once after parse. 04's config 0 "keep forever" stays install-level only |
| `retention.export_days` | 7 | 1 to 30 | `data_exports` zips |

Job `apply_conversation_retention` (`maintenance`, daily, per workspace batch 500/tx, guard per row). Index `(workspace_id, closed_at) WHERE status = 'closed' AND deleted_at IS NULL`. First enabling: warning dialog with count. Lowering never purges same day: first run next day. Self-host: same, plus config defaults. Cloud shares defaults; no entitlement.

## Conversation export (I-120)

`data_exports` (`dex_`): `workspace_id`, `requested_by_member_id`, `kind CHECK in ('conversations','contacts')`, `format CHECK in ('txt','pdf','csv','json')`, `selection jsonb` (`{ids}` or `{filter}` or `{view_id}`), `include_notes bool default false`, `status CHECK in ('pending','running','ready','failed','expired')`, `row_count`, `warnings jsonb`, `disk`, `path`, `size_bytes`, `expires_at`, `error`, timestamps. Migration `20261201040011_create_data_exports_table.sql`.

| Format | Scope | Limit | Content |
|---|---|---|---|
| `txt` | conversation or selection | 1,000 | one file per conversation, zipped when several: headers, messages with author, time |
| `pdf` | same | 300 | one PDF per conversation, zipped |
| `csv` | view or filter | 50,000 rows | one row per conversation: number, subject, status, contact, company, assignee, team, inbox, channel, priority, tags, dates, counts, attributes |
| `json` | same | 10,000 | full resource + messages |

- Over limit 422 `export_too_large` `meta.matched`.
- Notes excluded unless `include_notes` (`conversations:moderate`). Redaction rules rerun over output (old messages). Requester's `VisibleScope` at job time.
- Permission `conversations:export` (group `conversations:read`; owner, admin), `contacts:export` for contacts. 11's `exports:*` remain install data export.
- API: `POST /data-exports` 202; `GET /data-exports`, `GET /data-exports/{id}` with signed `url` when ready; requester or holder. `DELETE` removes file. Event `data_export.ready` on `member:{mem}`; 06 `data_export_ready` in-app. Audit each.
- Job `build_data_export` (`default`), `expire_data_exports` hourly.
- Bulk bar, conversation overflow, view header buttons. `settings/pages/admin.tsx` no exports sample today; list page `features/settings/pages/exports.tsx`. e2e `tests/e2e/exports.spec.ts`.

PDF: reuse check `codeberg.org/go-pdf/fpdf` (MIT, no dependencies beyond stdlib, UTF-8 TrueType from bytes, `RTL()`). Thin wrapper `framework/pdf` (`New`, `Heading`, `Paragraph`, `Output`), generic. Font: Noto Sans subset embedded, about 600 KB: Latin, Cyrillic, Greek. Library has no complex-script shaping and CJK font would add 16 MB, so characters outside font render as replacement: export `warnings: ["missing_glyphs"]`, dialog recommends txt. Operator may set `inbox.exports.pdf_font_path` to own TTF. Inline images skipped, listed by name. Why not headless browser: 200 MB dependency. Depguard allowlist gains module (delta).

## Scheduled export (I-121, track 12)

`export_destinations` (`dxd_`): `workspace_id`, `name`, `kind CHECK in ('s3')`, `config jsonb` (endpoint, bucket, region, prefix), `credentials bytea` (encrypted), `status`, `last_error`, timestamps. `export_schedules` (`dxs_`): `workspace_id`, `destination_id`, `dataset text CHECK in ('conversations','contacts','members','audit_logs')`, `frequency CHECK in ('daily','weekly')`, `enabled`, `last_run_at`, `last_cursor`, timestamps. Migration `20261201120006_create_export_destination_tables.sql`.

- S3-compatible only: AWS, R2, GCS interop (HMAC), MinIO, SeaweedFS, reusing `framework/filesystem` s3 driver built per destination. Azure Blob: left out, no S3 API, would need second SDK; report to owner.
- Format: JSONL gz per run, `prefix/dataset/YYYY/MM/DD/part-0001.jsonl.gz`, incremental by `updated_at` cursor, manifest file last. Notes included for conversations only with `include_notes` on schedule.
- Job `run_export_schedule` (`maintenance`, periodic hourly checks due); retry 5; fail twice: destination `error`, `system_alert` via 06 `Notify`.
- `POST /export-destinations/{id}/tests` writes and deletes probe (rest rule 5). Credentials write-only. SSRF: endpoint through `framework/httpclient` deny private ranges on cloud; self-host allowed.
- Scope: system actor sees all; permission `exports:write` (owner only) is the guard; `conversation_access` not applied. Audited.
- Page section in `exports.tsx`. e2e against SeaweedFS.

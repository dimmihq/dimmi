# 16 Importers: API shapes

Style: caveman. Binding route table in README "HTTP API". Resource shape, errors, cursors per `00-foundation/api.md`.

## Sources catalog

`GET /api/v1/import-sources`

```json
{ "data": [
  { "object": "import_source", "key": "intercom", "name": "Intercom", "kind": "api",
    "objects": ["contacts","companies","tags","conversations","articles"],
    "credential_fields": [
      { "key": "access_token", "type": "secret", "required": true, "help_url": "https://trydimmi.com/docs/import/intercom#token" },
      { "key": "region", "type": "enum", "values": ["us","eu","au"], "required": false, "default": "us" } ],
    "enabled": true, "docs_url": "https://trydimmi.com/docs/import/intercom" },
  { "object": "import_source", "key": "public_site", "name": "Any public help center", "kind": "public_url",
    "objects": ["articles"], "credential_fields": [ { "key": "url", "type": "url", "required": true } ],
    "detects": ["intercom","zendesk_guide","helpscout_docs","crisp","featurebase","gitbook","mintlify","document360","docusaurus","generic"],
    "enabled": true },
  { "object": "import_source", "key": "feedback_csv", "name": "Feedback CSV (Canny, Featurebase)", "kind": "external",
    "objects": ["feedback"], "endpoint": "/api/v1/feedback-imports", "enabled": true },
  { "object": "import_source", "key": "markdown_zip", "name": "Markdown zip, Notion export, Git docs", "kind": "external",
    "objects": ["articles"], "endpoint": "/api/v1/knowledge/sources", "enabled": true }
] }
```
`objects` are arrays of strings only because values are a closed registry per source; entries with fields are objects (P6).

## Create import

`POST /api/v1/imports` (`Idempotency-Key` honored)

```json
{ "source": "intercom", "name": "Intercom (acme)",
  "credentials": { "access_token": "dG9rOm...", "region": "eu" },
  "scopes": ["contacts","companies","tags","conversations","articles"],
  "options": {
    "since": null,
    "conversations": { "status_mode": "close_all", "default_inbox_id": "inbox_01jbc...", "include_notes": true },
    "contacts": { "skip_anonymous_leads": true, "email_trust": "unverified", "tags_attribute": "intercom_tags" },
    "articles": { "help_center_id": "hc_01jbd...", "publish": "preserve", "locale": "en", "redirects": true, "old_hosts": ["help.acme.com"] },
    "attachments": { "enabled": true }
  } }
```

201:
```json
{ "data": {
  "id": "imp_01jbx...", "object": "import", "source": "intercom", "name": "Intercom (acme)", "status": "draft",
  "source_account": { "id": "ab12cd34", "name": "Acme", "region": "eu", "url": "https://app.eu.intercom.com/a/apps/ab12cd34" },
  "credentials": { "configured": true, "hint": "…x9Qa", "fields": ["access_token","region"], "purge_after": null },
  "scopes": ["contacts","companies","tags","conversations","articles"],
  "options": { "...": "echo with defaults filled" },
  "sync": { "enabled": false, "interval_minutes": 10, "until": null, "next_at": null },
  "watermark": null, "last_run": null, "last_preview_run": null,
  "created_by": { "id": "mem_...", "object": "member", "name": "Ada" },
  "archived_at": null, "rolled_back_at": null,
  "created_at": "2026-11-02T09:00:00Z", "updated_at": "2026-11-02T09:00:00Z",
  "links": { "self": "https://app.trydimmi.com/api/v1/imports/imp_01jbx..." } } }
```

Validation:
- `credentials` fields per catalog; secrets 1 to 4,096 chars; unknown field 422.
- `Connect` runs synchronously (10 s): 422 `source_credentials_invalid` `{field}` on 401/403 from source, 422 `source_scope_insufficient` `{missing: ["conversations"]}` when token lacks read on chosen scope (Intercom app permissions, Help Scout user role), 502 `source_unavailable` on timeout/5xx, 422 `source_plan_unsupported` (Zendesk Essential legacy 10 req/min: allowed with warning, not error; only when API disabled).
- 409 `import_exists` `{import_id}` when live import for same `connection_key`.
- `scopes` not supported by source: 422 `scope_not_supported` `{scope, source}`.
- `options.articles.publish` required when `articles` in scopes (no default; human choice). `help_center_id` required; must be in workspace (404 otherwise).
- `csv` source: created without credentials; files added after.
- `public_site`: `credentials.url` https only, public host (SSRF check), 422 `site_not_detected` only when no sitemap and no crawlable links found.

## Preview

`POST /api/v1/imports/imp_.../runs` `{"kind": "preview"}` 202 run (`status: queued`). Done when `import.completed` with `kind: preview`. `GET /api/v1/import-runs/impr_...`:

```json
{ "data": {
  "id": "impr_01jby...", "object": "import_run", "import_id": "imp_01jbx...", "kind": "preview", "status": "completed",
  "preview": {
    "source_range": { "oldest": "2021-03-04T10:11:00Z", "newest": "2026-11-02T08:58:00Z" },
    "counts": {
      "contacts": { "total": 18240, "users": 6120, "leads": 12120, "skipped_anonymous_leads": 9340, "will_match_existing": 41 },
      "companies": { "total": 920 }, "tags": { "total": 57, "will_create": 49, "will_map": 8 },
      "conversations": { "total": 9811, "open": 64, "with_attachments_estimate": 1430, "over_part_limit": 3 },
      "articles": { "total": 312, "published": 290, "draft": 22, "locales": ["en","de"], "selected_locale": "en", "skipped_other_locale": 141 },
      "collections": { "total": 18, "flattened": 2 }
    },
    "attachments": { "estimate_bytes": 2254857830, "basis": "sample_of_200_parts" },
    "estimate": { "requests": 21400, "rate_per_minute": 3000, "duration_seconds": 1320, "embedding_tokens_estimate": 610000 },
    "mappings": { "teammate": { "total": 7, "mapped": 5, "placeholder": 2 }, "attribute": { "total": 23, "conflicts": 1, "trusted": 0 } },
    "conflicts": [ { "kind": "attribute", "mapping_id": "impm_...", "code": "attribute_key_taken_other_type", "message": "plan is text in Dimmi, number in source" } ],
    "warnings": [
      { "code": "parts_truncated", "count": 3, "message": "3 conversations exceed Intercom's 500-part API limit; oldest parts not available" },
      { "code": "locales_skipped", "count": 141, "message": "German translations not imported (one locale per help center)" },
      { "code": "old_host_not_on_dimmi", "message": "Redirects work after help.acme.com points to this help center" } ],
    "samples": {
      "conversations": [ { "external_id": "21534", "subject": "Refund for March", "messages": 6 } ],
      "articles": [ { "external_id": "8812", "title": "Reset your password", "old_url": "https://help.acme.com/en/articles/8812-reset-your-password", "new_path": "/reset-your-password", "markdown_excerpt": "1. Open **Settings** ..." } ] }
  },
  "stages": [ { "stage": "discover", "status": "done", "created": 0, "updated": 0, "matched": 0, "skipped": 0, "failed": 0 } ],
  "totals": {}, "error_count": 0, "pause_reason": null, "resume_at": null,
  "started_by": { "type": "member", "id": "mem_..." },
  "queued_at": "...", "started_at": "...", "finished_at": "...", "eta_seconds": null } }
```
Preview samples stored in `preview` jsonb (max 256 KB; conversation sample bodies truncated to 2,000 chars each) and dropped when import archived. Sample rendering route is the run resource itself (`?include=samples` returns full sample JSON); no separate path.

## Mappings

`GET /api/v1/imports/imp_.../mappings?filter[kind]=teammate`

```json
{ "data": [
  { "id": "impm_01...", "object": "import_mapping", "kind": "teammate", "external_id": "4471", "external_name": "Sarah Lee",
    "external_meta": { "email": "sarah@acme.com", "messages": 3120 },
    "action": "placeholder", "target": null, "target_key": "Sarah Lee", "suggested": true, "conflict": null },
  { "id": "impm_02...", "object": "import_mapping", "kind": "teammate", "external_id": "4472", "external_name": "Ada",
    "external_meta": { "email": "ada@acme.com", "messages": 8120 },
    "action": "map", "target": { "id": "mem_...", "object": "member", "name": "Ada" }, "suggested": true, "conflict": null } ],
  "meta": { "next_cursor": null, "per_page": 25 } }
```

`PATCH /api/v1/import-mappings/impm_01...` `{"action": "map", "target_id": "mem_..."}`. Attribute trust: `PATCH /api/v1/import-mappings/impm_07...` `{"trusted": true}` (attribute kind only, else 422 `trusted_not_allowed`; audited; response `meta.applies_to: "future_records"` when a run already wrote values, next run re-applies to unchanged values). Allowed actions per kind:

| Kind | Actions | Target |
|---|---|---|
| teammate | map, placeholder | active member (`mem_`) |
| team | map, create, skip | team |
| inbox | map | inbox (`inbox_`); create via 03 first |
| tag | map, create, skip | tag (`tag_`); create uses `target_key` name |
| attribute | create, map, skip | `target_key` attribute key (`[a-z0-9_]{1,64}`), map = existing key same type; optional `trusted` (bool, default false) |
| status | map, create | feedback status (`fbs_`) |
| board | map, create, skip | feedback board (`fbd_`) |
| category | map, create, skip | feedback category |
| help_center | map | `hc_` |
| locale | map, skip | target help center (one source locale per help center) |
| label | map, create, skip | changelog label |

Bulk: `PATCH /api/v1/imports/imp_.../mappings` `{"ids": ["impm_..", "impm_.."], "changes": {"action": "skip"}}` -> rest.md rule 8 response. Mapping changes after a full run started affect only records written later (documented in response `meta.applies_to: "future_records"`).

## Runs

`POST /api/v1/imports/imp_.../runs`

```json
{ "kind": "full" }
{ "kind": "delta" }
{ "kind": "rollback", "scope": ["articles"] }
```
202 run resource. Preconditions:
- `preview`: import not archived; credentials present (`csv`: at least one file).
- `full`: last preview completed and newer than last import change (409 `preview_required`); no blocking mapping `conflict` (422 `mapping_incomplete` `{kinds, count}`). Second `full` allowed (re-run, idempotent; used after mapping fix or added scope); `scope` subset allowed.
- `delta`: at least one `full` completed (409 `full_run_required`); watermark set.
- `rollback`: `imports:rollback` + sudo; allowed in any import status incl. archived; no other active run; `scope` subset of imported scopes.
- Any: 409 `import_run_active` `{run_id}`; 422 `entitlement_limit_reached` (`imports.concurrent_runs`); 409 `import_archived` for preview/full/delta.

`PATCH /api/v1/import-runs/impr_...` `{"status": "paused"}`, `{"status": "running"}`, `{"status": "cancelled"}`. Illegal: 422 `invalid_transition` `{from, to}`. Resume of `credentials_invalid` pause before credentials updated: 409 `credentials_required`.

Running run resource (progress):

```json
{ "data": { "id": "impr_...", "object": "import_run", "kind": "full", "status": "running",
  "progress": { "percent": 43, "eta_seconds": 740, "rate_per_minute": 2980, "waiting_for_rate_limit": false },
  "stages": [
    { "stage": "contacts", "status": "done", "total_estimate": 8900, "created": 8851, "matched": 41, "updated": 0, "skipped": 8, "failed": 0 },
    { "stage": "conversations", "status": "running", "total_estimate": 9811, "created": 4210, "skipped": 0, "failed": 2 },
    { "stage": "attachments", "status": "running", "created": 512, "failed": 1, "bytes": 640221110 },
    { "stage": "articles", "status": "pending" } ],
  "error_count": 3 } }
```

`GET /api/v1/import-runs/impr_.../errors`:
```json
{ "data": [ { "object": "import_error", "stage": "conversations", "object_type": "conversation", "external_id": "21999",
  "code": "source_not_found", "message": "Conversation deleted at source during import", "attempt": 1, "created_at": "..." } ],
  "meta": { "next_cursor": "...", "per_page": 25, "truncated": false } }
```

## Records lookup

`GET /api/v1/imports/imp_.../records?filter[object_type]=conversation&filter[external_id]=21534`
```json
{ "data": [ { "object": "import_record", "object_type": "conversation", "external_id": "21534",
  "record": { "id": "conv_...", "object": "conversation", "number": 10422 }, "disposition": "created",
  "run_id": "impr_...", "source_updated_at": "2025-05-01T12:00:00Z", "created_at": "..." } ] }
```

## Files (csv source)

`POST /api/v1/imports/imp_.../files` multipart `kind=contacts_csv`, `file=@contacts.csv` -> 201
```json
{ "data": { "id": "impf_...", "object": "import_file", "kind": "contacts_csv", "filename": "contacts.csv", "size_bytes": 812331,
  "row_count": 12001, "header": ["Email","Name","User ID","Company","Plan","Signed up"], "sample": [["bob@x.dev","Bob","u_1","Globex","pro","2024-01-02"]] } }
```
Column mapping lives in `options.csv.columns` (`{"Email": "email", "User ID": "external_id", "Company": "company.name", "Plan": "custom.plan", "Signed up": "first_seen_at"}`), validated at preview.

## Options (`models.ImportOptions`)

| Path | Type | Default | Notes |
|---|---|---|---|
| since | timestamp null | null | only records created or updated at/after (conversations, posts); contacts/companies always full |
| rate_share | number 0.1-0.9 | `imports.rate_share` (0.5) | |
| conversations.status_mode | `close_all` \| `preserve` | close_all | preserve keeps open/snoozed (snooze date past: open) |
| conversations.default_inbox_id | inbox id | workspace default inbox | unmapped source inboxes |
| conversations.include_notes | bool | true | |
| conversations.include_spam | bool | false | true: imported with `spam = true` |
| contacts.skip_anonymous_leads | bool | true | lead without email, phone, external id and conversations |
| contacts.email_trust | `unverified` \| `verified_for_users` | unverified | attestation audited |
| contacts.tags_attribute | attribute key null | null | contact tags joined `, ` into one string attribute |
| articles.help_center_id | hc id | required with articles | |
| articles.publish | `preserve` \| `draft` | required | |
| articles.locale | BCP 47 | source default locale | other locales via `locale` mappings to other help centers |
| articles.redirects | bool | true | |
| articles.old_hosts | string[] | from source (custom domain) | informational: dashboard shows DNS step per host |
| articles.collection_root_id | collection id null | null | nest imported top level under existing collection |
| feedback.subscribe_voters | bool | true | |
| changelog.help_center_id | hc id | default | |
| attachments.enabled | bool | true | false: placeholder lines only |
| csv.columns | map | required for csv | |

## Errors (16 codes)

| Status | Code | When | Extra |
|---|---|---|---|
| 422 | `source_credentials_invalid` | source 401/403 at connect or on PATCH credentials | `field` |
| 422 | `source_scope_insufficient` | token lacks permission for chosen scope | `missing` |
| 502 | `source_unavailable` | connect timeout/5xx | `retry_after` |
| 409 | `import_exists` | live import for same source account | `import_id` |
| 422 | `scope_not_supported` | scope not offered by source | `scope`, `source` |
| 409 | `preview_required` | full run without fresh preview | |
| 409 | `full_run_required` | delta before full | |
| 422 | `mapping_incomplete` | blocking mapping conflicts | `kinds`, `count` |
| 409 | `import_run_active` | second active run | `run_id` |
| 409 | `import_archived` | run/edit on archived import (rollback excepted) | |
| 409 | `import_has_records` | DELETE with created records not rolled back and not archived | `hint` |
| 409 | `credentials_required` | resume `credentials_invalid` pause without new credentials | |
| 422 | `trusted_not_allowed` | `trusted` on non-attribute mapping | |
| 422 | `site_not_detected` | public_site URL has nothing to import | |
| 422 | `import_channel_cannot_send` | reply to imported conversation with no email route (03 rehome) | `reason` (`no_contact_email` \| `no_email_address`) |
| 422/429 | `entitlement_limit_reached` | catalog keys (422 count caps, e.g. `imports.concurrent_runs`; 429 + `Retry-After` per-period caps) | `key`, `limit`, `used` |
| 423 | `password_confirmation_required` | rollback, delete without sudo | |
| 422 | `invalid_transition` | run status PATCH | `from`, `to` |

### Record error codes (`import_errors.code`)

`source_not_found` (deleted at source mid-run), `source_forbidden` (record-level 403), `source_invalid_payload`, `parts_truncated` (warning row), `attachment_too_large`, `attachment_blocked_type`, `attachment_fetch_failed`, `attachment_infected`, `body_too_large` (split, see edge cases), `contact_unresolvable`, `attribute_dropped`, `attribute_limit`, `redirect_path_reserved`, `redirect_conflict`, `article_too_large`, `slug_conflict_renamed` (warning), `entitlement_limit_reached`, `erased_contact_skipped`, `duplicate_in_source`, `unsupported_part_type`.

## Webhook payload (`import.completed`)

Standard 10 envelope, object `import_run`:
```json
{ "type": "import.completed", "timestamp": "2026-11-02T09:31:02Z",
  "data": { "object": "import_run", "id": "impr_...", "import_id": "imp_...", "source": "intercom", "kind": "full",
    "status": "completed_with_errors", "started_at": "2026-11-02T09:05:00Z", "finished_at": "2026-11-02T09:31:00Z",
    "totals": { "contacts": { "created": 8851, "matched": 41 }, "conversations": { "created": 9809, "failed": 2 },
      "articles": { "created": 290 }, "redirects": { "created": 614 }, "attachments": { "created": 1428, "failed": 2 } },
    "error_count": 4 } }
```

## Realtime payloads

`workspace:ws_...` and `member:mem_...` (starter): `{id: "evt_...", event: "import.progressed", server_time, data: {object: "import_run", id, import_id, kind, status, progress: {percent, eta_seconds}, stage}}`. Coalesced 2 s per run; final `import.completed`/`failed`/`cancelled`/`rolled_back` always published.

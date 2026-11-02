# 15 Engage: HTTP API

Style: caveman. REST rules `00-foundation/rest.md` binding. Shapes, pagination, idempotency, errors per `00-foundation/api.md`. Admin base `/api/v1`, auth `A` (`auth` + `workspace` + `throttle:api`, POST also `idempotent`); abilities apply to tokens only, session and first-party app use member role (README permissions matrix). Widget base `/widget/v1` (01 middleware: `cors:widget`, `widget.origin`, `throttle:widget`). CSAT email rating page in `routes/web.go`.

Route lint: no verbs. Publish = `PATCH {status: "live"}`. Pause = `{status: "paused"}`. End = `{status: "ended"}`. Restore = `{trashed: false}`. Copy = `POST .../duplicates`. Preview = `POST .../previews`. Reorder = `PATCH .../questions {order}`.

## 1. Engage settings

`GET /api/v1/engage-settings` (`engage-settings.show`, engage:read):

```json
{
  "data": {
    "object": "engage_settings",
    "max_interruptive_per_day": 2,
    "min_minutes_between": 30,
    "survey_cooldown_days": 14,
    "quiet_while_messenger_open": true,
    "live_summary": { "active": true, "anonymous": false, "next_transition_at": "2026-11-05T09:00:00Z" },
    "limits": { "live_announcements": { "used": 3, "max": 50 }, "live_surveys": { "used": 2, "max": 20 }, "segments": { "used": 7, "max": 200 } },
    "version": 42,
    "updated_at": "2026-11-02T08:00:00Z"
  }
}
```

`PATCH /api/v1/engage-settings` (`engage-settings.update`, engage:write, admin): any of the four writable fields. Bumps version. Audit.

## 2. Segments

| Method | Path | Name | Ability |
|---|---|---|---|
| GET | /segments | segments.index | engage:read |
| POST | /segments | segments.store | engage:write |
| GET | /segments/{seg} | segments.show | engage:read |
| PATCH | /segments/{seg} | segments.update | engage:write |
| DELETE | /segments/{seg} | segments.destroy | engage:write |
| GET | /segments/{seg}/contacts | segments.contacts.index | engage:read + contacts:read |
| POST | /segment-previews | segment-previews.store | engage:read |

Filters `GET /segments`: `q` (name), `filter[used]=true` (referenced by live/paused item). Sort `name`, `-updated_at` (default).

`POST /api/v1/segments`:

```json
{
  "name": "Pro users, active 30d",
  "description": "NPS audience",
  "rules": { "all": [
    { "field": "contact.kind", "op": "eq", "value": "user" },
    { "field": "contact.custom.plan", "op": "in", "value": ["pro", "team"] },
    { "field": "contact.last_seen_at", "op": "within_last_days", "value": 30 }
  ] }
}
```

201:

```json
{
  "data": {
    "id": "seg_01jbx4...",
    "object": "segment",
    "name": "Pro users, active 30d",
    "description": "NPS audience",
    "rules": { "all": [ "..." ] },
    "rules_schema": 1,
    "used_by": [ { "id": "svy_01jb...", "object": "survey", "name": "Quarterly NPS", "status": "live" } ],
    "last_estimate": null,
    "created_at": "2026-11-02T09:00:00Z",
    "updated_at": "2026-11-02T09:00:00Z"
  }
}
```

Validation errors (422) point at JSON path: `errors["rules.all.1.op"] = ["Operator in requires an array value."]`. Codes: `unknown_field`, `invalid_operator`, `too_many_conditions` (over 30), `too_deep` (over 3), `invalid_value`.

`PATCH` with `rules` on segment used by live items: allowed, applies immediately, bumps engage version; response `meta.affected_items` lists them.

`DELETE` used by live/paused item: 409 `segment_in_use` with `items`. Used only by draft/ended/archived: allowed; those items keep `segment_id` reference shown as `deleted_segment: true`; publishing them 422 `segment_deleted`.

`POST /api/v1/segment-previews` (rest.md rule 11, computed, 201, nothing stored except `last_estimate` when `segment_id` given):

```json
{ "segment_id": "seg_01jbx4...", "rules": null, "visitors": "identified", "sample": 10 }
```

```json
{
  "data": {
    "object": "segment_preview",
    "count": 18234,
    "approximate": false,
    "total_contacts": 1002311,
    "sample": [ { "id": "ctc_01jb...", "object": "contact", "name": "Ada", "email": "ada@acme.com", "last_seen_at": "2026-11-01T19:00:00Z" } ],
    "computed_in_ms": 412
  }
}
```

Exact count over `engage.segment_preview_timeout` (3 s): falls back to sampled estimate (`approximate: true`, count rounded to 2 significant digits). `sample` omitted when caller lacks `contacts:read` (tokens) or for MCP. Anonymous visitors without session never counted (no rows); response `meta.note = "anonymous visitors not counted"` when `visitors` includes them.

`GET /segments/{seg}/contacts`: cursor on `(last_seen_at desc, id)`, `limit` max 100. Statement timeout 5 s: 422 `segment_too_slow` with hint "Add a more selective condition (e.g. kind or plan)."

## 3. Engage assets

| Method | Path | Name | Ability |
|---|---|---|---|
| GET | /engage-assets | engage-assets.index | engage:read |
| POST | /engage-assets | engage-assets.store | engage:write |
| PATCH | /engage-assets/{eas} | engage-assets.update | engage:write |
| DELETE | /engage-assets/{eas} | engage-assets.destroy | engage:write |

`POST` multipart field `file` (+ optional `alt_text`). 201 with `scan_status: "pending"`, `url` = `https://app.trydimmi.com/widget/v1/engage-assets/eas_01jb.../hero.png`. Duplicate sha256 returns existing asset 200 `meta.deduplicated: true`. Errors: 413 `asset_too_large`, 422 `asset_type_not_allowed`, 422 `svg_not_supported`, 422 `image_too_large_pixels`. `DELETE` while referenced by non-archived item: 409 `asset_in_use`.

## 4. Announcements

| Method | Path | Name | Ability |
|---|---|---|---|
| GET | /announcements | announcements.index | engage:read |
| POST | /announcements | announcements.store | engage:write |
| GET | /announcements/{ann} | announcements.show | engage:read |
| PATCH | /announcements/{ann} | announcements.update | engage:write (publish/resume admin) |
| DELETE | /announcements/{ann} | announcements.destroy | engage:write admin |
| POST | /announcements/{ann}/duplicates | announcements.duplicates.store | engage:write |
| POST | /announcements/{ann}/previews | announcements.previews.store | engage:read |
| GET | /announcements/{ann}/stats | announcements.stats.show | engage:read |
| GET | /announcements/{ann}/receipts | announcements.receipts.index | engage:read + contacts:read |

`GET /announcements` filters: `filter[status]=live,paused`, `filter[format]=banner`, `filter[segment]=seg_`, `filter[trashed]=with|only`, `q` (name, title). Sort `-updated_at` (default), `-published_at`, `priority`. Collection rows include `stats_summary {impressions, unique_viewers, clicks, dismissals}` (from daily table, totals cached 60 s).

`POST /api/v1/announcements`:

```json
{
  "name": "Dark mode launch",
  "format": "modal",
  "default_locale": "en",
  "content": {
    "title": "Dark mode is here, {{contact.first_name | fallback: \"friend\"}}",
    "body_markdown": "Toggle it in **Settings > Appearance**. Works on every page.",
    "cover_asset_id": "eas_01jb...",
    "ctas": [
      { "id": "try", "label": "Try it", "style": "primary", "action": "url", "url": "https://app.acme.com/settings/appearance" },
      { "id": "notes", "label": "Release notes", "style": "secondary", "action": "open_changelog_entry", "changelog_entry_id": "cle_01jb..." }
    ],
    "modal": { "size": "small" }
  },
  "translations": { "de": { "title": "Dark Mode ist da", "body_markdown": "Unter **Einstellungen > Darstellung**.", "ctas": [ { "id": "try", "label": "Ausprobieren" } ] } },
  "audience": { "visitors": "identified", "segment_id": "seg_01jbx4...", "rules": { "all": [ { "field": "contact.custom.signed_up_at", "op": "before", "value": "2026-10-01T00:00:00Z" } ] } },
  "trigger": { "on": "page_view", "url_rules": [ { "op": "starts_with", "value": "/dashboard" } ], "url_match": "any", "delay_seconds": 3, "device": "any" },
  "frequency": { "mode": "once" },
  "starts_at": null,
  "ends_at": "2026-11-30T23:59:59+01:00",
  "timezone": "Europe/Berlin",
  "priority": 60
}
```

201:

```json
{
  "data": {
    "id": "ann_01jbx9...",
    "object": "announcement",
    "name": "Dark mode launch",
    "format": "modal",
    "status": "draft",
    "is_running": false,
    "default_locale": "en",
    "content": { "...": "as sent, ids normalized" },
    "translations": { "de": { "...": "..." } },
    "audience": { "...": "..." },
    "trigger": { "...": "..." },
    "frequency": { "mode": "once", "max_impressions": 1, "min_hours_between": 0, "recur_days": null },
    "starts_at": null,
    "ends_at": "2026-11-30T22:59:59Z",
    "timezone": "Europe/Berlin",
    "priority": 60,
    "bypass_throttle": false,
    "revision": 1,
    "published_at": null,
    "ended_at": null,
    "end_reason": null,
    "metadata": {},
    "created_by": { "type": "member", "id": "mem_01jb...", "name": "Frantz" },
    "created_at": "2026-11-02T09:10:00Z",
    "updated_at": "2026-11-02T09:10:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/announcements/ann_01jbx9...", "preview": null }
  },
  "meta": { "warnings": [ { "code": "low_contrast", "path": "content.theme.accent_color", "message": "Accent #F5F5F5 has 1.2:1 contrast with white text." } ] }
}
```

### Status machine (`PATCH {status}`)

```
draft -> live            (publish; admin; runs publish validation)
live -> paused           (any member)
paused -> live           (resume; admin; re-validates)
live|paused -> ended     (any member; ends_at passed -> job sets ended, reason schedule)
ended -> live            (admin; requires ends_at null or future; new revision; receipts kept: contacts who saw it do not see it again under once)
draft|ended -> archived  (any member)
archived -> draft        (admin)
```

Illegal: 422 `invalid_transition` `{from, to}`. `is_running` = `status = live` and now in `[starts_at, ends_at)` and workspace not suspended.

Publish validation (422 `publish_blocked` with `errors` keyed by path): title/body present per format, cover alt (`missing_alt`), CTA targets valid (`article_not_published`, `survey_not_live`, `changelog_entry_not_published`, `invalid_url`), `sender_member_id` active member (chat), segment exists, rules valid, live cap (`entitlement_limit_reached` with `key`, `limit`; returned as its own 422, not inside `publish_blocked`), `ends_at` future, translation locales valid. Warnings (non-blocking): `low_contrast`, `long_title`, `no_url_rules_on_modal` (modal on every page), `audience_everyone_interruptive` (modal to everyone incl. anonymous).

Editing live: allowed for content, translations, audience, trigger, frequency, schedule, priority. `format` never after first publish (422 `format_locked`). Changes to content or audience bump `revision`; `meta.reshow` optional boolean on PATCH: `true` resets receipts for `once` items (clears `next_eligible_at`, increments cycle) so everyone sees updated version (admin only, audit). Default false.

`PATCH` with `If-Unmodified-Since` stale: 409 `stale_resource`.

`DELETE`: live or paused: 409 `item_running` ("End it first."). Else soft delete; `PATCH {trashed: false}` restores within 30 days.

`POST /announcements/{ann}/previews`:

```json
{ "widget_id": "wgt_01jbd2...", "locale": "de" }
```

201:

```json
{
  "data": {
    "object": "engage_preview",
    "item": { "id": "ann_01jbx9...", "object": "announcement" },
    "preview_url": "https://app.trydimmi.com/widget-preview/wgt_01jbd2...#test_token=wtt_...&preview=ann_01jbx9...&locale=de",
    "host_url_hint": "Add #dimmi-preview=ann_01jbx9... with test token to your own site via boot option `testToken` (SDK `Dimmi('boot', {appId, testToken})`)",
    "test_token_expires_at": "2026-11-02T09:20:00Z"
  }
}
```

Uses 01 `MintWidgetTestSession`. Draft items previewable. Works for any member.

`GET /announcements/{ann}/stats?filter[from]=2026-11-01&filter[to]=2026-11-30&filter[include_test]=false`:

```json
{
  "data": {
    "object": "announcement_stats",
    "totals": { "impressions": 48211, "unique_viewers": 15302, "anonymous_impressions": 0, "clicks": 2210, "dismissals": 6011, "replies": 0, "click_rate": 0.144, "dismiss_rate": 0.393 },
    "cta_clicks": { "try": 1802, "notes": 408 },
    "daily": [ { "day": "2026-11-02", "impressions": 9120, "unique_viewers": 6011, "clicks": 640, "dismissals": 1702 } ],
    "data_delay_seconds": 60
  }
}
```

Rates use `unique_viewers` (session contacts) denominator; anonymous counted separately. `daily` max 400 days.

`GET /announcements/{ann}/receipts?filter[state]=clicked|dismissed|seen&cursor=`: `[{id: "erc_...", object: "engage_receipt", contact: {id, name, email}, first_seen_at, last_seen_at, impressions, clicked_at, last_action_id, dismissed_at, cycle}]`.

## 5. Surveys

| Method | Path | Name | Ability |
|---|---|---|---|
| GET | /surveys | surveys.index | engage:read |
| POST | /surveys | surveys.store | engage:write |
| GET/PATCH/DELETE | /surveys/{svy} | surveys.show/update/destroy | engage:read / engage:write |
| GET | /surveys/{svy}/questions | surveys.questions.index | engage:read |
| POST | /surveys/{svy}/questions | surveys.questions.store | engage:write |
| PATCH | /surveys/{svy}/questions | surveys.questions.update-many | engage:write |
| PATCH | /surveys/{svy}/questions/{svq} | surveys.questions.update | engage:write |
| DELETE | /surveys/{svy}/questions/{svq} | surveys.questions.destroy | engage:write |
| POST | /surveys/{svy}/duplicates | surveys.duplicates.store | engage:write |
| POST | /surveys/{svy}/previews | surveys.previews.store | engage:read |
| GET | /surveys/{svy}/responses | surveys.responses.index | engage:read |
| POST | /surveys/{svy}/responses | surveys.responses.store | engage:write |
| GET | /surveys/{svy}/responses/{svr} | surveys.responses.show | engage:read |
| DELETE | /surveys/{svy}/responses/{svr} | surveys.responses.destroy | engage:write admin |
| GET | /surveys/{svy}/results | surveys.results.show | engage:read |
| GET | /surveys/{svy}/receipts | surveys.receipts.index | engage:read + contacts:read |
| GET | /survey-templates | survey-templates.index | engage:read |

`GET /survey-templates`: static list `nps`, `csat`, `ces`, `feature_feedback`, `csat_conversation` (rating 1..5 + optional text comment). Each `{key, kind, display, name, questions: [...]}` in workspace locale (`en` fallback).

`POST /api/v1/surveys` from template with override:

```json
{
  "name": "Quarterly NPS",
  "template": "nps",
  "display": "popover",
  "audience": { "visitors": "identified", "segment_id": "seg_01jbx4..." },
  "trigger": { "on": "page_view", "url_rules": [ { "op": "starts_with", "value": "/dashboard" } ], "delay_seconds": 20 },
  "frequency": { "mode": "recurring", "recur_days": 90 },
  "settings": { "dismissible": true }
}
```

Explicit questions instead of template:

```json
{
  "name": "Export feedback",
  "kind": "custom",
  "display": "popover",
  "trigger": { "on": "track_event", "event_name": "export_completed", "delay_seconds": 2 },
  "frequency": { "mode": "once" },
  "questions": [
    { "ref": "q1", "type": "rating", "prompt": "How easy was exporting?", "config": { "scale": 5, "style": "emoji" },
      "branching": [ { "when": { "op": "lte", "value": 2 }, "goto": "q2" }, { "when": { "op": "gte", "value": 3 }, "goto": "end" } ] },
    { "ref": "q2", "type": "text", "prompt": "What went wrong?", "required": false, "config": { "multiline": true, "max_length": 1000 },
      "attribute_key": null }
  ],
  "thank_you": { "title": "Thanks!", "body_markdown": "We read every answer." }
}
```

Inline `ref` (client temp ids, `[a-z0-9_]{1,24}`) let branching reference questions before ids exist; response maps `ref` to `svq_` ids (`meta.refs`).

201 survey resource (abridged):

```json
{
  "data": {
    "id": "svy_01jby2...",
    "object": "survey",
    "name": "Export feedback",
    "kind": "custom",
    "display": "popover",
    "status": "draft",
    "is_running": false,
    "questions": [
      { "id": "svq_01jby3...", "object": "survey_question", "position": 1, "type": "rating", "prompt": "How easy was exporting?", "required": true,
        "config": { "scale": 5, "style": "emoji" }, "branching": [ { "when": { "op": "lte", "value": 2 }, "goto": "svq_01jby4..." }, { "when": { "op": "gte", "value": 3 }, "goto": "end" } ],
        "attribute_key": null, "archived_at": null }
    ],
    "intro": {},
    "thank_you": { "title": "Thanks!", "body_markdown": "We read every answer." },
    "settings": { "dismissible": true, "show_progress": true, "accent_color": null },
    "audience": { "visitors": "everyone" },
    "trigger": { "on": "track_event", "event_name": "export_completed", "delay_seconds": 2, "url_rules": [], "device": "any" },
    "frequency": { "mode": "once" },
    "csat": null,
    "counts": { "requested": 0, "started": 0, "completed": 0 },
    "revision": 1,
    "created_at": "2026-11-02T09:30:00Z",
    "updated_at": "2026-11-02T09:30:00Z"
  },
  "meta": { "refs": { "q1": "svq_01jby3...", "q2": "svq_01jby4..." }, "warnings": [] }
}
```

CSAT survey create:

```json
{
  "name": "Support CSAT",
  "template": "csat_conversation",
  "display": "conversation",
  "csat": { "delay_minutes": 10, "channels": ["widget", "email"], "include_ai_only": true, "require_member_reply": false, "cooldown_days": 7, "inbox_ids": [], "expire_days": 14 }
}
```

`display: conversation` rejects `audience`, `trigger`, `frequency` (422 `not_applicable_for_display`). Second live CSAT survey: 409 `csat_survey_exists` with existing id.

Status machine same as announcements. Publish validation adds: at least one answerable question (`no_questions`), branching graph valid (`branch_backwards`, `branch_target_archived`), every required question reachable or skippable path consistent, `csat.channels` non-empty subset of `widget`, `email` (other values 422 `invalid_value`), `attribute_key` not reserved.

Question edits after responses exist:
- `type` change: 422 `question_type_locked`.
- `prompt`, `description`, labels: allowed (results keep id; CSV header uses current prompt plus id).
- option removed: archived (`archived: true`), never deleted.
- `DELETE /questions/{svq}`: archived when answers exist (200 with `archived_at`), else hard delete (204).
- adding questions: allowed; older responses count as `not_shown` for new question (separate from `skipped`).

`PATCH /surveys/{svy}/questions` reorder: `{"order": ["svq_a", "svq_b", ...]}` exact non-archived set else 422 `order_mismatch`; branching re-validated (reorder that makes jump backwards: 422 `branch_backwards`).

### Responses

`GET /surveys/{svy}/responses` filters: `filter[status]=completed` (default `completed`; `partial`, `requested`, `expired`, `any`), `filter[from]`, `filter[to]`, `filter[source]`, `filter[locale]`, `filter[contact]=ctc_`, `filter[conversation]=conv_`, `filter[score_min]`, `filter[score_max]` (first scored question), `filter[answer.<svq>]=<option id>`, `filter[ai_involved]=true|false`, `filter[include_test]=true`, `q` (text answers FTS). Sort `-completed_at` (default), `completed_at`. `include=contact,conversation`.

```json
{
  "data": [
    {
      "id": "svr_01jbz1...",
      "object": "survey_response",
      "survey_id": "svy_01jby2...",
      "status": "completed",
      "source": "widget",
      "locale": "en",
      "contact": { "id": "ctc_01jb...", "object": "contact", "name": "Ada", "email": "ada@acme.com" },
      "conversation_id": null,
      "page_url": "https://app.acme.com/exports",
      "ai_involved": null,
      "is_test": false,
      "answers": [
        { "question_id": "svq_01jby3...", "type": "rating", "value": 2, "skipped": false },
        { "question_id": "svq_01jby4...", "type": "text", "value": "CSV had wrong encoding for umlauts", "skipped": false }
      ],
      "started_at": "2026-11-02T10:00:01Z",
      "completed_at": "2026-11-02T10:00:40Z",
      "redacted": false
    }
  ],
  "meta": { "next_cursor": "c_...", "prev_cursor": null, "per_page": 25 }
}
```

`contact` null for anonymous API response. Token without `contacts:read`: `contact` = `{id, object}` only. Choice answer value = option ids array plus `labels` (current default-locale labels) and `other_text`.

CSV: `GET /surveys/{svy}/responses?format=csv` (or `Accept: text/csv`): streamed, same filters, `Content-Disposition: attachment; filename="svy_..._2026-11-02.csv"`. Columns: `response_id, status, source, completed_at, locale, contact_id, contact_email, contact_name, external_id, conversation_id, page_url, ai_involved, <one column per question: prompt [svq_id]>`. Multi-choice joined by `; ` using labels. Cells starting with `=`, `+`, `-`, `@`, tab, CR prefixed with `'` (formula injection). UTF-8 with BOM. Audit logged. Max 1,000,000 rows per export (beyond: 422 `export_too_large`, narrow dates).

`POST /surveys/{svy}/responses` (developer submits on own UI, `source: api`):

```json
{
  "contact": { "id": "ctc_01jb..." },
  "client_id": "acme-nps-2026q4-ada",
  "locale": "en",
  "answers": [ { "question_id": "svq_01jc...", "value": 9 }, { "question_id": "svq_01jd...", "value": "Love the API" } ],
  "completed": true
}
```

`contact` accepts `{id}`, `{external_id}`, or `{email}` (03 lookup; unknown external_id with `create: true` upserts per 03 rules) or null (anonymous). Survey must be `live` or `paused` (drafts: 409 `survey_not_live`). Branching validated (answers for questions not on path: 422 `answer_not_on_path`). Same `client_id` replay returns existing response 200.

`DELETE /surveys/{svy}/responses/{svr}`: hard delete, stats recomputed lazily (results cache bust), event `survey_response.deleted`, audit.

### Results

`GET /surveys/{svy}/results?filter[from]=&filter[to]=&filter[locale]=&filter[source]=&filter[ai_involved]=&filter[segment]=seg_&filter[include_test]=false`

```json
{
  "data": {
    "object": "survey_results",
    "survey_id": "svy_01jbnps...",
    "kind": "nps",
    "headline": { "metric": "nps", "value": 41, "responses": 1820, "promoters": 1001, "passives": 571, "detractors": 248, "confidence_95": [37.1, 44.9] },
    "funnel": { "requested": 0, "seen": 15302, "started": 2140, "completed": 1820, "dismissed": 9120, "response_rate": 0.119 },
    "questions": [
      { "id": "svq_01jc...", "type": "nps", "prompt": "How likely are you to recommend Acme?", "answered": 1820, "skipped": 0, "not_shown": 0, "average": 7.9,
        "distribution": [ { "value": 0, "count": 12 }, { "value": 10, "count": 640 } ] },
      { "id": "svq_01jd...", "type": "single_choice", "answered": 1700, "choices": [ { "id": "speed", "label": "Speed", "count": 900 }, { "id": "other", "label": "Other", "count": 40 } ] },
      { "id": "svq_01je...", "type": "text", "answered": 1210, "latest": [ { "response_id": "svr_...", "text": "...", "completed_at": "..." } ] }
    ],
    "trend": [ { "period": "2026-W44", "value": 39, "responses": 410 } ],
    "computed_at": "2026-11-02T10:01:00Z",
    "cache_ttl_seconds": 60
  }
}
```

Default excludes test responses and CSAT responses of conversations now spam or contacts now blocked (`filter[include_spam]=true` to include). Headline per kind: `nps` (score -100..100, promoters 9-10, passives 7-8, detractors 0-6), `csat` (percent of 4-5 on 5-scale or 3 on 3-scale; plus average), `ces` (average on 1..7), `custom` (first scored question average or null). `confidence_95` only when 30+ responses. `trend` bucket `week` default (`?bucket=day|week|month`). `filter[segment]` evaluates segment on current contact attributes (documented: not attributes at response time); responses without contact excluded with `meta.excluded_no_contact`.

### Conversation and contact views

`GET /conversations/{conv}/survey-responses` (conversations:read): CSAT rows of conversation incl. `requested` and `expired` (status timeline). `GET /contacts/{ctc}/survey-responses` (contacts:read + engage:read): all responses of contact. `GET /contacts/{ctc}/engage-receipts`: what contact saw, clicked, dismissed.

## 6. Widget API (`/widget/v1`)

### Config addition (01 owns resource)

```json
"engage": { "active": true, "anonymous": true, "version": 42, "refresh_interval_ms": 600000 }
```

### GET /widget/v1/apps/{app}/engage-items (anonymous, app auth)

Query: `locale` (BCP 47, optional). Headers: `If-None-Match`. `Cache-Control: public, max-age=60, s-maxage=60`, `Vary: Origin` (CORS). Origin check per 01.

```json
{
  "data": [
    {
      "id": "ann_01jbinc...",
      "object": "engage_item",
      "type": "announcement",
      "format": "banner",
      "revision": 3,
      "priority": 100,
      "interruptive": false,
      "bypass_throttle": true,
      "locale": "en",
      "content": {
        "title": null,
        "body_html": "Payments are degraded. <a href=\"https://status.acme.com\" rel=\"nofollow noopener ugc\">Status</a>",
        "ctas": [],
        "banner": { "position": "top", "style": "inline", "dismissible": true, "tone": "critical" }
      },
      "trigger": { "on": "page_view", "url_rules": [], "url_match": "any", "delay_seconds": 0, "device": "any", "event_name": null },
      "frequency": { "mode": "until_engaged", "max_impressions": 20, "min_hours_between": 0 },
      "ends_at": "2026-11-02T18:00:00Z"
    }
  ],
  "meta": { "version": 42, "throttle": { "max_interruptive_per_day": 2, "min_minutes_between": 30, "survey_cooldown_days": 14, "quiet_while_messenger_open": true }, "server_time": "2026-11-02T10:00:00Z" }
}
```

`engage.anonymous = false`: 200 empty list (never 404; config may be stale for 60 s).

### GET /widget/v1/engage-items (session)

Bearer `dmi_ws_`. Query `locale`, `preview` (`ann_`/`svy_`, test sessions only; else 403 `preview_requires_test_session`). `ETag`/`If-None-Match` per README.

Same item shape plus `state`:

```json
{
  "id": "svy_01jbnps...",
  "object": "engage_item",
  "type": "survey",
  "display": "popover",
  "interruptive": true,
  "locale": "en",
  "content": {
    "intro": null,
    "questions": [
      { "id": "svq_01jc...", "type": "nps", "prompt": "How likely are you to recommend Acme to a friend?", "required": true,
        "config": { "min_label": "Not likely", "max_label": "Very likely" }, "branching": [ { "when": { "op": "lte", "value": 6 }, "goto": "svq_01jd..." } ] },
      { "id": "svq_01jd...", "type": "text", "prompt": "What should we fix first?", "required": false, "config": { "multiline": true, "max_length": 1000 } }
    ],
    "thank_you": { "title": "Thanks, Ada!", "body_html": "<p>We read every answer.</p>", "cta": null },
    "settings": { "dismissible": true, "show_progress": true }
  },
  "trigger": { "on": "page_view", "url_rules": [ { "op": "starts_with", "value": "/dashboard" } ], "url_match": "any", "delay_seconds": 20, "device": "any" },
  "frequency": { "mode": "recurring", "recur_days": 90 },
  "state": { "cycle": 2, "impressions": 0, "response_id": null, "partial_answers": [] }
}
```

`state.response_id` + `partial_answers` present when contact has partial response this cycle (resume where left, any device). `meta.throttle` adds `{remaining_today, next_slot_at}`.

### GET /widget/v1/announcements/{ann}, GET /widget/v1/surveys/{svy}

For manual trigger, deep link, CTA `open_survey`, CSAT card. Returns one `engage_item`. Session: audience and status enforced (404 `item_not_available` when not eligible, not live, or out of window; frequency ignored for manual: manual show always allowed unless completed this cycle). App auth: only anonymous-safe items. CSAT: `GET /widget/v1/surveys/{svy}?response_id=svr_...` returns item + `state` for that conversation request (contact must own conversation); `expired` or `completed` reflected in `state.status`.

### POST /widget/v1/engage-events

Auth: bearer session (optional) + `app_id` in body (required when no bearer). 202. Max 50 events, 32 KB.

```json
{
  "app_id": "wgt_01jbd2...",
  "events": [
    { "item_id": "ann_01jbx9...", "kind": "seen", "revision": 1, "at": "2026-11-02T10:00:03Z", "page_url": "https://app.acme.com/dashboard" },
    { "item_id": "ann_01jbx9...", "kind": "clicked", "cta_id": "try", "at": "2026-11-02T10:00:09Z" },
    { "item_id": "svy_01jbnps...", "kind": "dismissed", "at": "2026-11-02T10:01:00Z" }
  ]
}
```

```json
{ "meta": { "accepted": 3, "rejected": [], "throttle": { "remaining_today": 0, "next_slot_at": "2026-11-03T10:00:03Z" } } }
```

Rules: `at` clamped to `[now - 7 days, now + 60 s]` (server time wins beyond). Unknown or not-live item ids: rejected entry `{index, code: "item_not_available"}` (no error status). `kind: seen` with `test` session marks receipt `is_test`. `clicked` implies `seen`. `dismissed` on non-dismissible item: rejected `not_dismissible`.

### POST /widget/v1/surveys/{svy}/responses

Session required (anonymous visitor answering: SDK first creates lazy session via `POST /widget/v1/sessions`, PoW rules per 01). `idempotent`.

```json
{
  "client_id": "8b1f2a8e-6d0c-4b8e-9a51-2b3c4d5e6f70",
  "response_id": null,
  "revision": 4,
  "locale": "en",
  "page_url": "https://app.acme.com/dashboard",
  "answers": [ { "question_id": "svq_01jc...", "value": 6 } ],
  "completed": false
}
```

201:

```json
{
  "data": {
    "id": "svr_01jbz9...",
    "object": "survey_response",
    "status": "partial",
    "next_question_id": "svq_01jd...",
    "completed_at": null
  }
}
```

- CSAT: send `response_id` of `requested` row (from message metadata); 200 with same id; `source` becomes `conversation_widget`.
- `revision` older than current: accepted when questions answered still exist and types same; else 409 `survey_changed` with fresh item (client re-renders from start, keeps compatible answers).
- Already completed this cycle: 409 `response_completed` (SDK shows thanks, no retry).
- `expired` CSAT row: 410 `response_expired`.

### PATCH /widget/v1/surveys/{svy}/responses/{svr}

```json
{ "answers": [ { "question_id": "svq_01jd...", "value": "Faster CSV export" } ], "completed": true }
```

200 `{status: "completed", completed_at, thank_you_variant}`. Answer values per type (README table). Re-answering same question before completion overwrites. After completion: 409 `response_completed`. Text over max: 422 `answer_too_long`. Unknown option: 422 `invalid_option`. Required missing on complete: 422 `answer_required` with question ids.

### GET /widget/v1/engage-assets/{eas}/{filename}

Public. `Cache-Control: public, max-age=31536000, immutable`, `Cross-Origin-Resource-Policy: cross-origin`, `X-Content-Type-Options: nosniff`, content type from row. `scan_status` infected or pending (pending older than 10 min allowed when scan driver `skipped`): 404. Filename mismatch: 301 to canonical.

### POST /widget/v1/conversations (01 owns; 15 addition)

Body adds `announcement_id` (chat format, contact has receipt for it, item live or ended within 7 days). Else 422 `announcement_not_replyable`.

## 7. CSAT email rating page (web)

Minimal server-rendered templ page for CSAT email links only (not a general hosted survey page: later: hosted-survey-page). Zero JS required. Locale: contact locale, `Accept-Language`, survey default.

| Request | Behavior |
|---|---|
| `GET /csat-ratings/{svr}?score=4&expires=..&signature=..` | signature binds `svr` + `expires` (`score` unsigned, preselect only). Renders first question with `score` preselected plus next question when `text` (comment). Never writes. Invalid/expired signature: 403 page "This link is invalid or expired". `completed`: thanks page. `expired` row or survey not live/paused: closed page |
| `POST /csat-ratings/{svr}?expires=..&signature=..` | same signature check. Fields `answers[svq]` (first question required, comment optional). Stores answers (redaction, write-back), `source conversation_email`, completes response (later questions `not_shown`), renders thanks. Already completed: thanks page, no change |

Rate limit `engage.rate.csat_posts_per_ip_hour` (30, `a.Limiter` on Valkey). Response row must be `display = conversation` survey row created by `SendCsatRequest`; other ids 404 page.

## 8. MCP and CLI

Tool inputs mirror REST bodies above (09 renderer derives schemas from OpenAPI). Notable:
- `dimmi_create_announcement`: `{name, format, title, body_markdown, ctas?, segment_id?, url_rules?, ends_at?}` simplified; always `draft`; returns preview URL hint.
- `dimmi_update_announcement`: any PATCH field incl. `status`; `live` goes through confirm gate with preview text "Publish 'Dark mode launch' (modal) to ~18,234 contacts?" using `segment-previews` estimate.
- `dimmi_get_survey_results`: `{survey_id, from?, to?}` -> results; text answers in `untrusted_content` envelope.
- `dimmi_list_survey_responses`: `{survey_id, score_max?, q?, limit<=50}`; contact email included only with `contacts:read`.

CLI front matter example (`dimmi announcements create --file launch.md`):

```markdown
---
name: Dark mode launch
format: modal
title: Dark mode is here
ctas:
  - { id: try, label: Try it, action: url, url: "https://app.acme.com/settings/appearance" }
audience: { visitors: identified, segment_id: seg_01jbx4 }
trigger: { on: page_view, url_rules: [ { op: starts_with, value: /dashboard } ] }
ends_at: 2026-11-30T23:59:59+01:00
---
Toggle it in **Settings > Appearance**.
```

## 9. Error codes (feature)

| Status | Code | When |
|---|---|---|
| 404 | `feature_disabled` | `engage.enabled = false` |
| 404 | `item_not_available` | widget: not live, not eligible, other workspace |
| 409 | `segment_in_use` | delete segment used by live/paused item |
| 409 | `item_running` | delete live/paused item |
| 409 | `csat_survey_exists` | second active CSAT survey |
| 409 | `asset_in_use` | |
| 409 | `stale_resource` | If-Unmodified-Since mismatch |
| 409 | `survey_not_live` | API response to draft |
| 409 | `survey_changed` | widget answer with incompatible revision |
| 409 | `response_completed` | answer after completion |
| 410 | `response_expired` | CSAT request expired |
| 413 | `asset_too_large`, `payload_too_large` | |
| 422 | `invalid_transition` | status machine, with `from`, `to` |
| 422 | `publish_blocked` | publish validation failures (errors map) |
| 422 | `entitlement_limit_reached` | live item, segment caps (`key`, `limit`; entitlement catalog) |
| 422 | `format_locked`, `question_type_locked`, `branch_backwards`, `branch_target_archived`, `order_mismatch`, `no_questions` | |
| 422 | `unknown_field`, `invalid_operator`, `too_many_conditions`, `too_deep`, `invalid_value` | rule validation |
| 422 | `segment_deleted`, `segment_too_slow` | |
| 422 | `answer_required`, `answer_too_long`, `invalid_option`, `answer_not_on_path`, `invalid_answer_value` | |
| 422 | `not_applicable_for_display` | audience/trigger on CSAT survey |
| 422 | `announcement_not_replyable` | chat reply to non-chat or stale announcement |
| 422 | `missing_alt`, `svg_not_supported`, `asset_type_not_allowed`, `image_too_large_pixels` | |
| 422 | `export_too_large`, `question_limit_reached`, `contact_not_found` | |
| 503 | `results_timeout` | results query over 5 s; narrow filters |
| 403 | `preview_requires_test_session` | `preview` param on normal session |
| 403 | `missing_ability` / role errors | standard |
| 429 | `rate_limited` | |

Warnings (`meta.warnings`): `low_contrast`, `long_title`, `no_url_rules_on_modal`, `audience_everyone_interruptive`, `article_unpublished_cta`, `help_tab_disabled_cta` (CTA `open_article` on widget with Help tab off), `broken_link`, `external_http_image`, `translation_incomplete`.

# 13 Changelog API examples

Style: caveman. Shapes follow `specs/00-foundation/api.md` (`data`, `meta`, `object`, prefixed IDs, ISO UTC, null keys present). Route tables in README. `hc_`, `art_`, `ast_` are 08 prefixes; `mem_` 00-accounts; `air_` 07.

## Changelog (singleton under help center)

`GET /api/v1/help-centers/hc_01jcf2.../changelog` 200:
```json
{
  "data": {
    "id": "clg_01jd0a...",
    "object": "changelog",
    "help_center_id": "hc_01jcf2...",
    "enabled": true,
    "title": "Changelog",
    "description": "Everything new in Acme.",
    "show_in_nav": true,
    "show_authors": true,
    "reactions_enabled": true,
    "subscriptions_enabled": true,
    "email": {
      "delay_minutes": 5,
      "content": "full",
      "from_address_id": "ema_01jc...",
      "sending_domain_verified": true,
      "monthly_recipient_cap": 10000,
      "recipients_this_month": 1240
    },
    "og_image": null,
    "urls": {
      "page": "https://help.acme.com/changelog",
      "rss": "https://help.acme.com/changelog/feed.xml",
      "markdown": "https://help.acme.com/changelog.md"
    },
    "counts": { "published": 42, "drafts": 3, "scheduled": 1, "needs_review": 2, "active_subscribers": 1312 },
    "last_published_at": "2026-10-30T09:00:00Z",
    "latest_announced_at": "2026-10-30T09:00:00Z",
    "created_at": "2026-11-01T10:00:00Z",
    "updated_at": "2026-11-01T10:00:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/help-centers/hc_01jcf2.../changelog" }
  }
}
```
`email.monthly_recipient_cap` = entitlement `changelog.email.monthly_recipient_cap` (`Entitlements.Limit`; -1 = unlimited, self-host default). Read-only here; staff override via 12.
Counts cached, refreshed on change.

## Entries

`POST /api/v1/changelog-entries`:
```json
{
  "help_center_id": "hc_01jcf2...",
  "title": "CSV export for conversations",
  "slug": null,
  "summary": "Export any filtered conversation list to CSV in one click.",
  "body_markdown": "You can now export conversations to CSV.\n\n![Export menu](ast_01jd...)\n\n## How it works\n\nOpen any view, press **Export**.\n\n> [!TIP]\n> Filters apply to export.",
  "label_ids": ["cll_01jd0new..."],
  "author_ids": ["mem_01jc..."],
  "article_ids": ["art_01jcf4..."],
  "version_tag": "v1.4.0",
  "cover_asset_id": "ast_01jd...",
  "announce": true,
  "spotlight": true,
  "notify_subscribers": true,
  "feedback_post_ids": ["fbp_01jd..."],
  "notify_voters": true,
  "status": "draft"
}
```
201:
```json
{
  "data": {
    "id": "cle_01jd1b...",
    "object": "changelog_entry",
    "changelog_id": "clg_01jd0a...",
    "help_center_id": "hc_01jcf2...",
    "title": "CSV export for conversations",
    "slug": "csv-export-for-conversations",
    "summary": "Export any filtered conversation list to CSV in one click.",
    "body_markdown": "You can now export ...",
    "status": "draft",
    "announce": true,
    "spotlight": true,
    "notify_subscribers": true,
    "version_tag": "v1.4.0",
    "labels": [ { "id": "cll_01jd0new...", "object": "changelog_label", "name": "New", "slug": "new", "color": "#16a34a" } ],
    "authors": [ { "id": "mem_01jc...", "object": "member", "name": "Ada", "avatar_url": "https://..." } ],
    "articles": [ { "id": "art_01jcf4...", "object": "article", "title": "Export conversations", "url": "https://help.acme.com/articles/export-conversations" } ],
    "feedback_posts": [ { "id": "fbp_01jd...", "object": "feedback_post", "number": 12, "title": "CSV export", "status": "planned" } ],
    "notify_voters": true,
    "cover": { "id": "ast_01jd...", "object": "help_center_asset", "url": "https://help.acme.com/assets/ast_01jd.../export.png", "alt_text": "Export menu", "width": 1600, "height": 900 },
    "seo": { "title": null, "description": null, "noindex": false },
    "reactions": { "thumbs_up": 0, "heart": 0, "tada": 0, "eyes": 0, "thumbs_down": 0 },
    "source": { "kind": "member", "ref": {} },
    "needs_review": false,
    "url": null,
    "markdown_url": null,
    "mailing": null,
    "word_count": 24,
    "lock_version": 1,
    "warnings": [],
    "published_at": null, "first_published_at": null, "scheduled_for": null, "edited_after_publish_at": null, "deleted_at": null,
    "created_at": "2026-11-02T10:00:00Z", "updated_at": "2026-11-02T10:00:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/changelog-entries/cle_01jd1b..." }
  }
}
```
`body_markdown` image reference `ast_...` rewritten to asset URL at render (08 dialect). `feedback_post_ids` (14 posts, max 50, same workspace, else 422 `feedback_post_invalid`): stored by 14 `feedback.LinkChangelogEntry` (column `feedback_posts.changelog_entry_id`); `notify_voters` default true when posts linked (14 emails voters on publish; 13 newsletter skips them). Field absent in body: links unchanged; `[]`: unlink all. `slug: null` -> from title (08 slug generator), reserved list and collision suffix `-2`.

Publish now (`PATCH /api/v1/changelog-entries/cle_01jd1b...`, header `If-Match: "1"`):
```json
{ "status": "published", "notify_subscribers": true }
```
200:
```json
{
  "data": {
    "id": "cle_01jd1b...", "object": "changelog_entry", "status": "published",
    "url": "https://help.acme.com/changelog/csv-export-for-conversations",
    "markdown_url": "https://help.acme.com/changelog/csv-export-for-conversations.md",
    "published_at": "2026-11-02T10:05:00Z",
    "mailing": { "id": "clm_01jd1c...", "object": "changelog_mailing", "status": "scheduled", "send_after": "2026-11-02T10:10:00Z", "recipient_estimate": 1290 },
    "lock_version": 2,
    "...": "..."
  },
  "meta": { "warnings": [ { "code": "missing_alt", "message": "Image on line 5 has no alt text.", "line": 5 } ] }
}
```

Schedule:
```json
{ "status": "scheduled", "scheduled_for": "2026-11-04T08:00:00Z", "notify_subscribers": true }
```
Backdate (history import, no email by default):
```json
{ "status": "published", "published_at": "2025-06-01T09:00:00Z" }
```
Unpublish: `{ "status": "draft", "reason": "Shipped by mistake" }`. Restore from trash: `{ "trashed": false }`.

Illegal transition 422:
```json
{ "message": "Cannot move entry from scheduled to scheduled without scheduled_for.", "code": "invalid_transition", "from": "scheduled", "to": "scheduled" }
```
Stale 409:
```json
{ "message": "Entry changed since you loaded it.", "code": "stale_resource", "current_lock_version": 3, "updated_by": { "type": "member", "name": "Grace" } }
```

Bulk `PATCH /api/v1/changelog-entries`:
```json
{ "ids": ["cle_a...", "cle_b..."], "changes": { "label_ids_add": ["cll_fixed..."] } }
```
200 `{ "data": [ {"id": "cle_a...", "status": "updated"}, {"id": "cle_b...", "status": "failed", "error": {"code": "validation_failed", "errors": {"label_ids": ["max 5"]}}} ], "meta": {"updated": 1, "skipped": 0, "failed": 1} }`.

## Email preview and test

`POST /api/v1/changelog-entries/cle_01jd1b.../email-previews` (no body) 200:
```json
{
  "data": {
    "object": "changelog_email_preview",
    "subject": "CSV export for conversations",
    "preheader": "Export any filtered conversation list to CSV in one click.",
    "from": "Acme <updates@acme.com>",
    "html": "<!doctype html>...",
    "text": "CSV export for conversations\n\nYou can now export ...",
    "size_bytes": 18234,
    "over_budget": false,
    "warnings": []
  }
}
```
`POST .../email-tests` 201 `{ "data": { "object": "changelog_email_test", "to": "ada@acme.com", "status": "sent" } }`. Token caller: 403 `member_required`.

## Mailings

`GET /api/v1/changelog-mailings/clm_01jd1c...` 200:
```json
{
  "data": {
    "id": "clm_01jd1c...", "object": "changelog_mailing", "kind": "entry",
    "entry": { "id": "cle_01jd1b...", "object": "changelog_entry", "title": "CSV export for conversations" },
    "status": "sending", "blocked_reason": null,
    "send_after": "2026-11-02T10:10:00Z", "started_at": "2026-11-02T10:10:02Z", "finished_at": null,
    "subject": "CSV export for conversations",
    "counts": { "recipients": 1290, "sent": 800, "failed": 0, "skipped": 2, "bounced": 1, "complained": 0, "unsubscribed": 3 },
    "created_at": "2026-11-02T10:05:00Z", "updated_at": "2026-11-02T10:10:40Z"
  }
}
```
Cancel: `PATCH {"status": "canceled"}`; `sent`/`canceled`: 409 `mailing_not_cancelable`. Blocked example (cloud):
```json
{ "message": "Verify a sending domain to email more than 50 subscribers.", "code": "sending_domain_required", "manage_url": "https://app.trydimmi.com/settings/email/domains" }
```
(returned on `POST /changelog-mailings` and as mailing `blocked_reason` when blocked at start).

## Subscribers

`POST /api/v1/changelog-subscribers`:
```json
{ "help_center_id": "hc_01jcf2...", "email": "bob@example.org", "name": "Bob", "consent": "attested" }
```
201:
```json
{
  "data": {
    "id": "csub_01jd2a...", "object": "changelog_subscriber", "changelog_id": "clg_01jd0a...",
    "email": "bob@example.org", "name": "Bob", "status": "active",
    "source": "api", "consent": "attested", "contact": null,
    "confirmed_at": "2026-11-02T11:00:00Z", "unsubscribed_at": null, "unsubscribe_reason": null, "last_emailed_at": null,
    "created_at": "2026-11-02T11:00:00Z", "updated_at": "2026-11-02T11:00:00Z"
  }
}
```
Missing `consent`: 422 `consent_required`. Existing unsubscribed: 409 `subscriber_unsubscribed`. `consent_evidence` never returned by API except `GET /changelog-subscribers/{csub}?include=consent_evidence` (admin).

`POST /api/v1/changelog-subscriber-imports`:
```json
{ "help_center_id": "hc_01jcf2...", "consent": "attested",
  "subscribers": [ { "email": "a@x.io", "name": "A" }, { "email": "bad@@x" } ] }
```
200:
```json
{ "data": { "object": "changelog_subscriber_import", "created": 1, "updated": 0, "skipped_unsubscribed": 0, "skipped_invalid": 1, "skipped_duplicate_in_file": 0, "skipped_limit": 0,
  "errors": [ { "index": 1, "code": "email_invalid" } ] } }
```
CSV export `GET /api/v1/changelog-subscribers?filter[help_center]=hc_...&format=csv` columns `id,email,name,status,source,consent,confirmed_at,unsubscribed_at,created_at`.

## Sources

`POST /api/v1/changelog-sources`:
```json
{ "help_center_id": "hc_01jcf2...", "type": "github", "github_repository_id": "ghr_01jd2c...",
  "on_release": "draft_ai", "include_prereleases": false, "default_label_ids": [] }
```
or `"full_name": "acme/app", "connection_id": "ghc_01jd1f..."` instead of `github_repository_id` (13 calls 08 `github.AddRepository`; existing repo row reused). Token and webhook secret never pass through 13 (08 `POST /github-connections`, `POST /github-repositories`). Repo without connection: 422 `github_connection_required`. 201:
```json
{
  "data": {
    "id": "cls_01jd3a...", "object": "changelog_source", "type": "github", "name": "acme/app",
    "github_repository": { "id": "ghr_01jd2c...", "full_name": "acme/app", "webhook_mode": "auto", "webhook_status": "pending", "permissions": { "metadata": "read", "contents": "read", "pull_requests": "read", "webhooks": "write" } },
    "on_release": "draft_ai", "include_prereleases": false,
    "status": "active", "last_error_code": null, "last_event_at": null,
    "created_at": "2026-11-02T11:00:00Z", "updated_at": "2026-11-02T11:00:00Z"
  }
}
```
Webhook: 08 repo adds `release` to hook events (auto mode, job `SyncGithubRepositoryWebhook`); manual mode: `meta.webhook` comes from 08 repo create response only.

`POST /api/v1/changelog-sources/cls_.../tests` 200:
```json
{ "data": { "object": "changelog_source_test", "ok": false, "checks": [
  { "name": "connection", "ok": true }, { "name": "repo_access", "ok": true }, { "name": "webhook", "ok": true, "detail": "active" },
  { "name": "pull_requests_read", "ok": false, "code": "github_permission_missing", "message": "Token lacks Pull requests: Read. Needed for drafts from merged PRs." },
  { "name": "latest_release", "ok": true, "detail": "v1.3.2 (2026-10-12)" } ] } }
```

## Generations

`POST /api/v1/changelog-generations`:
```json
{ "help_center_id": "hc_01jcf2...", "use_ai": true,
  "input": { "kind": "github_range", "source_id": "cls_01jd3a...", "from_ref": "v1.3.2", "to_ref": "v1.4.0", "instructions": "Mention SSO only briefly." } }
```
Text kind (CLI): `"input": {"kind": "text", "text": "feat: csv export\n\nfix: unread badge ...", "version_tag": "v1.4.0"}`.
202:
```json
{ "data": { "id": "cgn_01jd4a...", "object": "changelog_generation", "status": "queued", "use_ai": true,
  "input": { "kind": "github_range", "source_id": "cls_01jd3a...", "from_ref": "v1.3.2", "to_ref": "v1.4.0" },
  "entry": null, "ai_run_id": null, "stats": {}, "error_code": null,
  "created_at": "2026-11-02T11:05:00Z", "links": { "self": "https://app.trydimmi.com/api/v1/changelog-generations/cgn_01jd4a..." } } }
```
Done:
```json
{ "data": { "id": "cgn_01jd4a...", "object": "changelog_generation", "status": "succeeded",
  "entry": { "id": "cle_01jd4b...", "object": "changelog_entry", "title": "SSO, CSV export and faster inbox", "status": "draft", "needs_review": true },
  "ai_run_id": "air_01jd4c...",
  "stats": { "commits": 87, "prs": 23, "prs_used": 23, "truncated": false, "input_tokens": 9120, "output_tokens": 640, "cost_micros": 31400 } } }
```
No ready AI key: 409 `ai_unavailable` at create (`use_ai: false` still works). Cap: 423 `spend_cap_reached`.

## Stats

`GET /api/v1/help-centers/hc_.../changelog/stats?from=2026-10-01&to=2026-10-31`:
```json
{ "data": { "object": "changelog_stats", "from": "2026-10-01", "to": "2026-10-31",
  "totals": { "index_views": 4210, "entry_views": 9800, "widget_views": 3100, "feed_fetches": 22000, "reactions": 410, "subscribes": 96, "unsubscribes": 7 },
  "subscribers": { "active": 1312, "pending": 14 },
  "entries": [ { "id": "cle_...", "title": "CSV export for conversations", "published_at": "2026-10-30T09:00:00Z", "views": 2100, "widget_views": 900,
      "reactions": { "thumbs_up": 40, "heart": 22, "tada": 61, "eyes": 3, "thumbs_down": 1 },
      "mailing": { "sent": 1287, "bounced": 1, "complained": 0, "unsubscribed": 3 } } ],
  "daily": [ { "day": "2026-10-30", "entry_views": 2400, "widget_views": 950, "subscribes": 12 } ] } }
```

## Public routes

Subscribe JSON (hc.js) `POST https://help.acme.com/api/changelog/subscriptions`:
```json
{ "email": "bob@example.org", "ts": "1730541600.b64hmac", "website": "" }
```
Always 202 `{ "status": "check_email" }` (also for existing active, bot-suspected, suppressed). Disabled: 404 `subscriptions_disabled`. Rate: 429 `Retry-After`.

Reaction `POST https://help.acme.com/api/changelog/entries/cle_01jd1b.../reactions` `{ "emoji": "tada", "visitor_key": "q3k9...22 chars base64url" }` 200:
```json
{ "reactions": { "thumbs_up": 40, "heart": 22, "tada": 62, "eyes": 3, "thumbs_down": 1 }, "mine": ["tada"] }
```
Repeat same emoji: 200 unchanged (idempotent). `DELETE .../reactions/tada` with header `Dimmi-Visitor-Key` 200 same shape. No public `data` envelope (public page API like 08 `/api/search`).

RSS item:
```xml
<item>
  <title>CSV export for conversations</title>
  <link>https://help.acme.com/changelog/csv-export-for-conversations</link>
  <guid isPermaLink="false">urn:uuid:0193f1a2-...</guid>
  <pubDate>Thu, 30 Oct 2026 09:00:00 +0000</pubDate>
  <category>New</category>
  <description>Export any filtered conversation list to CSV in one click.</description>
  <content:encoded><![CDATA[<p>You can now export conversations to CSV.</p>...]]></content:encoded>
  <enclosure url="https://help.acme.com/assets/ast_01jd.../export.png" length="183402" type="image/png"/>
</item>
```

Entry `.md` twin `GET /changelog/csv-export-for-conversations.md`:
```markdown
# CSV export for conversations

Published: 2026-10-30 · Labels: New · Version: v1.4.0

> Export any filtered conversation list to CSV in one click.

You can now export conversations to CSV.
...

Learn more: [Export conversations](https://help.acme.com/articles/export-conversations.md)

Source: https://help.acme.com/changelog/csv-export-for-conversations
```

## Widget

`GET /widget/v1/changelog?app_id=wgt_...` (session bearer optional) 200:
```json
{
  "data": {
    "changelog": { "title": "What's new", "url": "https://help.acme.com/changelog", "subscriptions_enabled": true, "reactions_enabled": true, "subscribed": false },
    "entries": [
      { "id": "cle_01jd1b...", "object": "widget_changelog_entry", "title": "CSV export for conversations",
        "summary": "Export any filtered conversation list to CSV in one click.",
        "cover_url": "https://help.acme.com/assets/ast_01jd.../export.png",
        "labels": [ { "name": "New", "color": "#16a34a" } ], "version_tag": "v1.4.0",
        "published_at": "2026-10-30T09:00:00Z", "unread": true, "spotlight": true }
    ],
    "unread_count": 2,
    "last_seen_at": "2026-10-20T08:12:00Z"
  },
  "meta": { "next_cursor": "eyJ...", "per_page": 20 }
}
```
Anonymous (no session): `unread`, `unread_count`, `last_seen_at` null (client computes from localStorage), `subscribed` false.

`PATCH /widget/v1/changelog/read-state` `{ "last_seen_at": "2026-11-02T10:00:00Z" }` 200 `{ "data": { "last_seen_at": "2026-11-02T10:00:00Z", "unread_count": 0 } }`. Value older than stored: stored kept (monotonic). Future value: clamped to server now.

`GET /widget/v1/changelog/entries/cle_...`: deep open target for SDK `show('changelog', {entry: 'cle_...'})` and 15-announcements-surveys CTA `open_changelog_entry`. 200 `widget_changelog_entry` with `body_html`; draft, scheduled, trashed, other help center: 404 `{ "code": "changelog_entry_not_found" }` (SDK opens list, `console.warn`). Tab off: 404 `changelog_not_linked`.

SDK surface (01 `sdk-contract.md`, camelCase in JS, wire snake_case): property `changelogUnread` = `unread_count`; event `changelog_unread_changed {count}`. No "Send feedback" entry tagging at MVP (later: changelog-replies-to-inbox).

`POST /widget/v1/changelog/subscriptions` (identified contact, verified email, no body) 201 `{ "data": { "status": "active", "email": "bob@acme.io" } }`; unverified: 202 `{ "data": { "status": "check_email" } }`.

## MCP tools (09 runtime)

`dimmi_draft_changelog_entry` input:
```json
{ "help_center": "hc_01jcf2... (optional, default help center)", "title": "CSV export for conversations",
  "body_markdown": "...", "summary": "...", "labels": ["New"], "version_tag": "v1.4.0",
  "article_slugs": ["export-conversations"], "source": { "repo": "acme/app", "from_ref": "v1.3.2", "to_ref": "v1.4.0" } }
```
Output text: `Draft cle_01jd... saved. Review: https://app.trydimmi.com/changelog/cle_01jd... Publish only if user asks: dimmi_publish_changelog_entry entry=cle_01jd...`. Labels by name (unknown names returned as warning, not created).

`dimmi_publish_changelog_entry` input `{ entry, notify_subscribers (default true), scheduled_for?, confirm? }`. Preview (`confirmation_required`): title, URL to be, label, subscriber count to email, send time. Elicitation per 09 gate.

## Webhook payloads (10 delivers)

`changelog_entry.published`:
```json
{
  "id": "evt_...", "type": "changelog_entry.published", "created_at": "2026-10-30T09:00:01Z", "workspace_id": "ws_...",
  "data": {
    "object": "changelog_entry", "id": "cle_01jd1b...", "title": "CSV export for conversations",
    "summary": "Export any filtered conversation list to CSV in one click.",
    "url": "https://help.acme.com/changelog/csv-export-for-conversations",
    "labels": [ { "name": "New", "slug": "new" } ], "version_tag": "v1.4.0",
    "published_at": "2026-10-30T09:00:00Z", "notify_subscribers": true,
    "feedback_post_ids": ["fbp_01jd..."], "notify_voters": true,
    "body_markdown": "You can now export ...",
    "links": { "self": "https://app.trydimmi.com/api/v1/changelog-entries/cle_01jd1b..." }
  }
}
```
`body_markdown` omitted when payload would exceed 64 KB (`links.self` to fetch).

`changelog_subscriber.unsubscribed`: `{ object: "changelog_subscriber", id, changelog_id, email, reason }`.

## GitHub release envelope (13 shape, parsed by 08 dispatcher)

Deliveries arrive on 08 shared route `POST /webhooks/github/repositories/{ghr}` (`08-help-center/docs-sync.md` 4.3: signature, `repository.id` check, dedupe, 202/204/401 responses owned by 08). 08 stores compact envelope in `HandleGithubDelivery` args (never raw payload); 13 handler `changelog` receives it:
```json
{ "action": "published", "release_id": 182733, "tag_name": "v1.4.0", "name": "v1.4.0", "body": "## What's new ...",
  "draft": false, "prerelease": false, "html_url": "https://github.com/acme/app/releases/tag/v1.4.0",
  "published_at": "2026-11-02T10:58:00Z", "author_login": "ada" }
```
`body` capped 64 KB (truncated flag `body_truncated: true`). Actions handled: `published`, `released`, `edited`; others dropped by handler (no job).

## Error shape examples

```json
{ "message": "Slug feed is reserved.", "code": "slug_reserved", "errors": { "slug": ["Slug feed is reserved."] } }
{ "message": "This entry was already emailed.", "code": "already_mailed", "mailing_id": "clm_..." }
{ "message": "This person unsubscribed. Only they can subscribe again.", "code": "subscriber_unsubscribed" }
{ "message": "Publishing from an AI agent needs your confirmation.", "code": "human_review_required" }
```

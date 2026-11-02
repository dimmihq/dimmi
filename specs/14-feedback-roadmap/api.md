# 14 API examples

Style: caveman. Owner: 14-feedback-roadmap. Shapes follow `00-foundation/api.md` (`data`, `object`, prefixed ids, ISO UTC, `links`). Route table in README.md, portal/widget tables in portal.md.

## Query parameters (`GET /api/v1/feedback-posts`)

| Param | Values |
|---|---|
| `filter[board]` | `fbd_...` list |
| `filter[status]` | `fbs_...` list or slugs |
| `filter[status_category]` | `open,active,done,closed` (default excludes `closed` unless status filter given) |
| `filter[category]` | `fbct_...` |
| `filter[tag]` | `tag_...` or names |
| `filter[owner]` | `mem_...`, `me`, `none` |
| `filter[author]` | `ctc_...` |
| `filter[company]` | `cmp_...` (posts with votes or requests from company) |
| `filter[contact]` | `ctc_...` (voted or requested) |
| `filter[moderation]` | `approved` (default), `pending`, `rejected`, `spam`, `all` |
| `filter[source]` | post sources |
| `filter[trust]` | `identified,email_verified,member_added,imported` (counts only votes with these trusts for `-votes` sort; default all) |
| `filter[has_requests]` | `true` |
| `filter[number]` | `12` |
| `filter[changelog_entry]` | 13 entry id |
| `filter[merged]` | `false` (default), `true` |
| `filter[trashed]` | `with`, `only` |
| `q` | FTS query (title, body) |
| `sort` | `-trending` (default), `-votes`, `-created_at`, `created_at`, `-revenue`, `-requests`, `-last_activity_at`, `-comments`, `position` (manual roadmap) |
| `include` | `board`, `status`, `category`, `tags`, `owner`, `author`, `top_requests` (3 latest quotes), `merged_posts`, `changelog` |
| `cursor`, `limit` | default 25, max 100 |

Unknown filter 400 `invalid_filter`. Cursor encodes sort key + id.

## Post (member resource)

`GET /api/v1/feedback-posts/fbp_01jcx...?include=board,status,tags,owner,top_requests`

```json
{
  "data": {
    "id": "fbp_01jcx4m7r9w0",
    "object": "feedback_post",
    "number": 12,
    "title": "Export conversations to CSV",
    "slug": "export-conversations-to-csv",
    "body": "We need monthly exports for our finance team.",
    "body_html": "<p>We need monthly exports for our finance team.</p>",
    "board": { "id": "fbd_01jcx...", "object": "feedback_board", "name": "Feature requests", "slug": "feature-requests", "visibility": "public" },
    "category": null,
    "status": { "id": "fbs_01jcx...", "object": "feedback_status", "name": "Planned", "slug": "planned", "color": "#6366f1", "category": "active" },
    "status_changed_at": "2026-11-02T09:12:00Z",
    "moderation_state": "approved",
    "author": { "type": "contact", "id": "ctc_01jc...", "name": "Ada Lovelace", "avatar_url": null },
    "created_by": { "type": "member", "id": "mem_01jc...", "name": "Sam" },
    "owner": { "id": "mem_01jc...", "object": "member", "name": "Sam" },
    "source": "dashboard",
    "tags": [{ "id": "tag_01jc...", "name": "billing" }],
    "pinned": false,
    "comments_locked": false,
    "counts": { "votes": 48, "imported_votes": 0, "comments": 6, "internal_comments": 2, "requests": 11, "subscribers": 52 },
    "revenue": { "amount_minor": 1249900, "currency": "USD", "companies": 9 },
    "trending_rank": 3,
    "last_activity_at": "2026-11-02T09:12:00Z",
    "merged_into": null,
    "shipped_at": null,
    "changelog": null,
    "external_links": [{ "provider": "github", "url": "https://github.com/acme/app/issues/88", "title": "CSV export" }],
    "top_requests": [
      { "id": "fbr_01jc...", "quote": "can we get a CSV of all chats every month?", "contact": { "id": "ctc_...", "name": "Bob" }, "company": { "id": "cmp_...", "name": "Globex" }, "conversation": { "id": "conv_...", "number": 1042 }, "priority": "high", "created_at": "2026-10-30T10:00:00Z" }
    ],
    "public_url": "https://help.acme.dev/feedback/posts/12-export-conversations-to-csv",
    "created_at": "2026-10-20T08:00:00Z",
    "updated_at": "2026-11-02T09:12:00Z",
    "edited_at": null,
    "links": { "self": "https://app.trydimmi.com/api/v1/feedback-posts/fbp_01jcx4m7r9w0" }
  }
}
```

`revenue` null when revenue weighting off. `public_url` null when board private or portal disabled.

## Public post resource (portal, widget)

```json
{
  "data": {
    "id": "fbp_01jcx4m7r9w0",
    "object": "feedback_post",
    "number": 12,
    "title": "Export conversations to CSV",
    "body_html": "<p>We need monthly exports for our finance team.</p>",
    "board": { "id": "fbd_...", "name": "Feature requests", "slug": "feature-requests" },
    "category": null,
    "status": { "name": "Planned", "slug": "planned", "color": "#6366f1", "category": "active" },
    "author": { "label": "Ada", "is_team": false, "avatar_url": null },
    "vote_count": 48,
    "comment_count": 6,
    "voters_preview": null,
    "merged_posts": [{ "number": 8, "title": "CSV export of chats" }],
    "changelog": null,
    "viewer": { "voted": true, "vote_id": "fbv_01jc...", "subscribed": true, "can_vote": true, "can_comment": true, "can_edit": false },
    "url": "https://help.acme.dev/feedback/posts/12-export-conversations-to-csv",
    "created_at": "2026-10-20T08:00:00Z"
  }
}
```

Never: tags, owner, requests, revenue, internal counts, emails, companies, `external_links`. `vote_count` null when `show_vote_counts` false. `voters_preview` (`[{label, avatar_url}]`, max 5) only when `show_voter_names`.

## Create post on behalf of customer, with request (inbox flow)

`POST /api/v1/feedback-posts` (`Idempotency-Key` recommended)

```json
{
  "board_id": "fbd_01jcx...",
  "title": "Slack notifications per inbox",
  "body": "Route #billing inbox alerts to a separate Slack channel.",
  "on_behalf_of": { "contact_id": "ctc_01jc..." },
  "request": { "conversation_id": "conv_01jc...", "message_id": "msg_01jc...", "quote": "I want billing alerts in their own channel", "priority": "medium" },
  "vote": true,
  "subscribe": true,
  "tags": ["notifications"],
  "attachment_ids": []
}
```

201: post resource with `meta: {"vote": {"id": "fbv_..."}, "request": {"id": "fbr_..."}, "similar_posts": [{"id": "fbp_...", "number": 7, "title": "Slack channel per team", "score": 0.81}]}`. `similar_posts` lets UI offer "Merge into #7?" immediately.

Validation: `on_behalf_of` xor author self (member). `request.message_id` must belong to `conversation_id` (422 `message_not_in_conversation`). Request contact defaults to conversation contact; must equal `on_behalf_of` contact when both set (422 otherwise).

## Link conversation to existing post

`POST /api/v1/feedback-posts/fbp_01jcx4m7r9w0/requests`

```json
{ "conversation_id": "conv_01jc...", "message_id": "msg_01jc...", "quote": "can we get a CSV of all chats every month?", "priority": "high", "add_vote": true, "subscribe": true }
```

201:
```json
{
  "data": {
    "id": "fbr_01jcy...",
    "object": "feedback_request",
    "post": { "id": "fbp_01jcx4m7r9w0", "number": 12, "title": "Export conversations to CSV" },
    "contact": { "id": "ctc_01jc...", "object": "contact", "name": "Bob" },
    "company": { "id": "cmp_01jc...", "object": "company", "name": "Globex" },
    "conversation": { "id": "conv_01jc...", "object": "conversation", "number": 1042 },
    "message_id": "msg_01jc...",
    "quote": "can we get a CSV of all chats every month?",
    "priority": "high",
    "source": "inbox",
    "vote": { "id": "fbv_01jcy...", "created": true },
    "created_by": { "id": "mem_01jc...", "name": "Sam" },
    "created_at": "2026-11-02T10:00:00Z"
  }
}
```

Existing request for same post + conversation: 409 `request_exists` `{"existing_id": "fbr_..."}`. Contact already voted: `vote.created: false` (existing vote id).

Inbox side panel: `GET /api/v1/feedback-requests?filter[conversation]=conv_01jc...&include=post` lists linked posts with status.

## Status change with note (and undo)

`PATCH /api/v1/feedback-posts/fbp_01jcx4m7r9w0`

```json
{ "status_id": "fbs_shipped...", "status_note": "Live for everyone. Settings > Export.", "notify": true }
```

200: post resource + 
```json
"meta": {
  "post_update": { "id": "fbu_01jcz...", "object": "feedback_post_update", "kind": "shipped", "state": "scheduled", "scheduled_for": "2026-11-02T10:10:00Z", "recipients_estimate": 52 }
}
```

Undo: `PATCH /api/v1/feedback-post-updates/fbu_01jcz... {"state": "cancelled"}` -> 200 update `state: cancelled`. Status itself stays changed (undo of status = PATCH back; reverting within window auto-cancels update).

Preview without write: `PATCH /api/v1/feedback-posts/{fbp}?preview=true` same body -> 200 `{data: <post as it would be>, meta: {notification_estimate: {recipients: 52, email_recipients: 47, over_daily_limit: false}}}`, no event, no row.

Same status: 200, no update. Moderation pending post status change: allowed, no notification until approved (update `skipped post_hidden` if released while pending).

## Merge

`POST /api/v1/feedback-posts/fbp_target.../merges`

```json
{ "from": ["fbp_dup1...", "fbp_dup2..."], "notify": true }
```

201:
```json
{
  "data": [
    { "id": "fbm_01jd...", "object": "feedback_post_merge", "source": { "id": "fbp_dup1...", "number": 8 }, "target": { "id": "fbp_target...", "number": 12 }, "moved": { "votes": 14, "deduped_votes": 3, "requests": 2, "comments": 5, "subscriptions": 15 }, "created_at": "2026-11-02T10:20:00Z" }
  ],
  "meta": { "target": { "id": "fbp_target...", "vote_count": 59 } }
}
```

Unmerge: `DELETE /api/v1/feedback-posts/fbp_target.../merges/fbm_01jd...` -> 200 merge with `unmerged_at`.

## Vote on behalf

`POST /api/v1/feedback-posts/{fbp}/votes`

```json
{ "contact": { "email": "carol@initech.com", "name": "Carol" }, "subscribe": true }
```

`contact` upserts with 03 match keys (`external_id` then `email`; email match never claims verified identity). 201 vote; existing vote 200 same vote (`meta.created: false`).

Vote list item (member):
```json
{ "id": "fbv_...", "object": "feedback_vote", "contact": { "id": "ctc_...", "name": "Carol", "email": "carol@initech.com" }, "company": { "id": "cmp_...", "name": "Initech", "revenue_minor": 49900 }, "trust": "member_added", "source": "dashboard", "created_by": { "id": "mem_...", "name": "Sam" }, "created_at": "2026-11-02T10:30:00Z" }
```
`email` only with `contacts:read`.

## Suggestions

`GET /api/v1/feedback-suggestions?filter[status]=pending`

```json
{
  "data": [
    {
      "id": "fbsg_01jd...",
      "object": "feedback_suggestion",
      "kind": "link_request",
      "status": "pending",
      "confidence": 0.86,
      "post": { "id": "fbp_...", "number": 12, "title": "Export conversations to CSV" },
      "conversation": { "id": "conv_...", "number": 1050 },
      "message_id": "msg_...",
      "contact": { "id": "ctc_...", "name": "Dee" },
      "quote": "is there any way to dump all our chats into a spreadsheet?",
      "reasoning": "Customer asks for bulk export of conversations, same need as #12.",
      "ai_run": { "id": "air_...", "cost_micros": 410 },
      "created_at": "2026-11-02T11:00:00Z",
      "expires_at": "2026-12-02T11:00:00Z"
    }
  ],
  "meta": { "next_cursor": null, "per_page": 25, "pending_count": 4 }
}
```

Accept: `PATCH /api/v1/feedback-suggestions/fbsg_01jd... {"status": "accepted", "overrides": {"priority": "medium"}}` -> 200 suggestion with `result: {request_id, post_id, vote_id}`.

## Settings

`PATCH /api/v1/feedback/settings`

```json
{
  "portal_enabled": true,
  "access": "public",
  "anonymous_voting": false,
  "show_voter_names": false,
  "author_display": "first_name",
  "sso_login_url": "https://app.acme.dev/api/dimmi/sso",
  "revenue_source": "company",
  "revenue_attribute": "mrr",
  "revenue_currency": "USD",
  "revenue_unit": "minor",
  "notify": { "undo_seconds": 600, "email_enabled": true, "from_name": "Acme Product Updates" },
  "ai": { "duplicate_detection": true, "capture": "suggest", "capture_min_confidence": 0.7 }
}
```

200 settings resource + `meta.portal_url` (`https://help.acme.dev/feedback`), `meta.sign_in_methods` (effective), `meta.warnings` (`sso_unavailable` when no active identity secret, `no_email_sender`, `help_center_disabled`, `ai_unavailable`).

## Roadmap (dashboard)

`GET /api/v1/feedback/roadmap?filter[board]=fbd_...&include_private=true`

```json
{
  "data": {
    "object": "feedback_roadmap",
    "sort": "votes",
    "columns": [
      { "status": { "id": "fbs_...", "name": "Planned", "color": "#6366f1", "category": "active" }, "post_count": 14, "posts": [ { "id": "fbp_...", "number": 12, "title": "Export conversations to CSV", "vote_count": 48, "revenue_minor": 1249900, "board": { "name": "Feature requests", "visibility": "public" } } ], "next_cursor": "eyJ..." }
    ]
  }
}
```

Each column first 50 posts; more via `GET /feedback-posts?filter[status]=...&cursor`.

## Portal: SSO session

`POST https://help.acme.dev/api/feedback/sessions`

```json
{ "grant": "sso", "token": "eyJhbGciOiJIUzI1NiIsImtpZCI6ImlzazpfMDEuLi4ifQ...", "return_to": "/feedback/posts/12-export-conversations-to-csv" }
```

201 (JSON request) with `Set-Cookie: __Host-dimmi_portal=...; Path=/; Secure; HttpOnly; SameSite=Lax`:
```json
{ "data": { "id": "pts_01jd...", "object": "portal_session", "trust": "identified", "grant": "sso", "contact": { "name": "Ada Lovelace", "email_masked": "a***@acme.com", "avatar_url": null }, "expires_at": "2026-12-02T11:00:00Z" }, "meta": { "redirect_to": "/feedback/posts/12-export-conversations-to-csv" } }
```

Form request: 303 `Location: /feedback/posts/12-...`. Invalid JWT: 401 `{"message": "Token expired 4 minutes ago.", "code": "token_expired", "hint": "Sign tokens right before redirect; exp should be a few minutes ahead."}`.

## Portal: email sign-in

`POST /api/feedback/sign-ins {"email": "ada@acme.com", "return_to": "/feedback"}` -> 202 `{"data": {"id": "psi_01jd...", "object": "portal_sign_in", "expires_at": "2026-11-02T11:15:00Z"}}` (always).

`POST /api/feedback/sessions {"grant": "email_code", "sign_in_id": "psi_01jd...", "code": "482913"}` -> 201 session; wrong code 422 `{"code": "sign_in_invalid", "attempts_left": 3}`.

## Widget home

`GET /widget/v1/feedback` (identified widget session)

```json
{
  "data": {
    "object": "feedback_home",
    "title": "Feedback",
    "boards": [{ "id": "fbd_...", "name": "Feature requests", "slug": "feature-requests", "submissions": "open" }],
    "roadmap_enabled": true,
    "portal_url": "https://help.acme.dev/feedback",
    "viewer": { "trust": "identified", "can_vote": true, "can_post": true, "can_comment": true, "sign_in_methods": [] },
    "unread_count": 2,
    "show_vote_counts": true
  }
}
```

Anonymous lead with `anonymous_voting` off: `viewer: {trust: "anonymous", can_vote: false, can_post: false, sign_in_methods: [{"type": "email"}]}`.

## Import

`POST /api/v1/feedback-imports` (multipart): `posts` (CSV), `votes` (CSV, optional), `board_id`, `dry_run=true`, `mapping` JSON:

```json
{
  "posts": { "key": "Post URL", "title": "Title", "body": "Details", "status": "Status", "created_at": "Created", "author_email": "Author Email", "author_name": "Author", "vote_count": "Score", "category": "Category" },
  "votes": { "post_key": "Post URL", "email": "Email", "name": "Name", "created_at": "Voted At" },
  "status_map": { "under review": "fbs_review...", "planned": "fbs_planned...", "complete": "fbs_shipped...", "closed": "fbs_declined..." },
  "create_categories": true
}
```

202 `feedback_import` `status: validating`; then `validated` with `totals` and `errors`. Run: `PATCH /api/v1/feedback-imports/fbi_... {"status": "running"}`. Unmapped status values: 422 at validation listing distinct values. Presets: `mapping_preset: "canny" | "featurebase"` fills column names from known export headers (still overridable).

## Webhook payload (10 envelope, `feedback_post.status_changed`)

```json
{
  "type": "feedback_post.status_changed",
  "timestamp": "2026-11-02T10:00:00Z",
  "data": {
    "object": "feedback_post",
    "id": "fbp_01jcx4m7r9w0",
    "number": 12,
    "title": "Export conversations to CSV",
    "status": { "id": "fbs_...", "name": "Shipped", "category": "done" },
    "previous_status": { "id": "fbs_...", "name": "Planned", "category": "active" },
    "vote_count": 48,
    "request_count": 11,
    "public_url": "https://help.acme.dev/feedback/posts/12-export-conversations-to-csv",
    "board": { "id": "fbd_...", "visibility": "public" }
  }
}
```

Snapshot uses member resource minus `body_html`, `top_requests` (developer fetches if needed).

## Error shape examples

```json
{ "message": "Sign in to vote.", "code": "sign_in_required", "sign_in_methods": [{ "type": "sso", "url": "https://app.acme.dev/login?return_to=..." }, { "type": "email" }] }
```
```json
{ "message": "This post was merged into #12.", "code": "post_merged", "merged_into": { "id": "fbp_01jcx4m7r9w0", "number": 12 } }
```
```json
{ "message": "Voting is closed for declined requests.", "code": "voting_closed" }
```
```json
{ "message": "Status is used by 14 posts. Choose a replacement.", "code": "status_in_use", "post_count": 14 }
```
```json
{ "message": "Cannot change moderation from rejected to pending.", "code": "invalid_transition", "from": "rejected", "to": "pending" }
```

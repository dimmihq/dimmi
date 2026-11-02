# 14 Closing the loop: subscriptions, post updates, notifications, changelog

Style: caveman. Owner: 14-feedback-roadmap. Read with README.md.

## 1. Subscriptions

Subscriber = contact or member following one post. Contacts get customer notifications (this file); members get 06 notifications (`feedback_comment_created`, `feedback_mentioned`).

| Event | Subscription created | Reason |
|---|---|---|
| Contact creates post | author | `author` |
| Contact votes (any surface) | voter | `voted` |
| Member votes on behalf, `subscribe` true (default) | that contact | `voted` |
| Request linked with `subscribe` true (default) | request contact | `request` |
| Contact comments | commenter | `commented` |
| Contact follows | contact | `manual` |
| Member creates, comments, is owner, is mentioned | member | `author`, `commented`, `manual`, `mentioned` |
| Import with `subscribe_voters` | voters | `import` |
| Merge | source subscribers added to target unless existing row | `merge` |

Rules:
- Row with `unsubscribed_at` set is never re-activated by later vote, comment or request (respect explicit choice). Only explicit follow (`PATCH .../subscription {subscribed: true}` or member `POST .../subscriptions`) clears it.
- Retracting vote does not unsubscribe (contact may still care); UI offers unfollow.
- Contact-level opt-out (`feedback_contact_states.email_opt_out_at`) stops all feedback email; feed (widget/portal) continues.
- Anonymous contacts subscribe for feed only (no email).

## 2. Post updates (announcements with undo)

`feedback_post_updates` row = one thing worth telling subscribers. Created in same tx as cause.

| Cause | kind | scheduled_for | Recipients |
|---|---|---|---|
| Status change to status with `notify = true`, category not `done`, `notify` param true | `status_changed` | now + `notify.undo_seconds` (default 600) | contact subscribers |
| Status change into category `done` (shipped), `notify` true | `shipped` | now + undo | contact subscribers |
| Changelog entry published linking post (13) | `shipped` (sets `changelog_entry_id`) | now (no undo; 13 publish is deliberate) | contact subscribers |
| Post merged into target, `notify` true | `merged` (on source, `target_post_id`) | now + undo | source contact subscribers not already subscribed to target before merge |
| Member public comment, `notify` true (default) | `team_comment` | now + `feedback.notify.comment_delay` (2 min) | contact subscribers |
| Contact or member reply to contact comment (child comment) | `reply` | now + comment delay | parent comment author (single), if not actor |

Status note: `status_note` on PATCH creates `feedback_comments` row kind `status_update` (public, member author) and update references it. Email shows note.

### Undo, coalescing, supersede

- `PATCH /feedback-post-updates/{fbu} {state: "cancelled"}` while `scheduled`: cancelled, nothing sent. API/dashboard shows "Notifying 342 people in 9:41. Undo".
- Status reverted before release: new `ChangePostStatus` back to update's `from_status_id` cancels scheduled update automatically (`skip_reason reverted`), no new update created. Status changed again to third status while first still scheduled: first `superseded`, new one created with `from_status_id` = first update's from (customers see "Under review -> In progress", not intermediate), timer restarts.
- Post trashed, rejected, moved to private board, or merged while update scheduled: update `skipped` (`post_hidden`) at release.
- `scheduled_for` sooner: `PATCH {scheduled_for: <now or later>}`; earlier than now: 422.
- Release: periodic job `release_scheduled_post_updates` (every 15 s, leader) claims due rows `FOR UPDATE SKIP LOCKED`, sets `sending`, inserts `fan_out_feedback_post_update`.

### Fan-out

1. Recheck: post approved, not merged, not trashed (kinds except `merged`); status still `to_status_id` for status kinds (else `skipped reverted`). Post on private board: only `shipped` kind proceeds (customers linked by request or vote on behalf deserve "it shipped"); email has title + note, no post link; other kinds `skipped post_hidden`.
2. Chunks of 500 contact subscribers (`unsubscribed_at is null`) by id cursor. `INSERT INTO feedback_notifications ... SELECT ... ON CONFLICT (workspace_id, update_id, contact_id) DO NOTHING` (idempotent on retry). Data snapshot built once per update.
3. Per notification decide email (in same chunk tx), `email_status` pending or skipped with reason:
   - `notify.email_enabled` false or kind not in `notify.kinds`: `email_disabled`.
   - contact anonymous or no email: `no_email` / `anonymous`.
   - contact email unverified and contact kind lead without verified email identity: `unverified_email` (never email addresses typed into widget by unknown visitors; prevents using our sender as spam relay).
   - opt-out: `opted_out`; banned or blocked: `banned`.
   - per-post cooldown: contact emailed about this post within `post_cooldown` (30 min): `coalesced` (next email for this post includes all unsent updates since last email: data merged at send time).
   - daily cap (`contact_daily_max` 10, counter in `feedback_contact_states` with day reset in contact timezone fallback UTC): `daily_cap`.
4. Email job per 100 pending notifications: `RenderFeedbackEmail` (templ subject, HTML, text twin; not an `app/mail` Mailable), call 04 `SendContactNotification{Kind: feedback_update, Unsubscribe: signed URL, IdempotencyKey: fbn_ id}` (wrapper over `SendContactMail{Kind: feedback_update, Purpose: update|shipped|merged|comment|digest}`, customer stream; `outbound_emails.kind = feedback_update`, sub-kind in `metadata.purpose`). 04 result `skipped{suppressed|limited|paused|no_sender|disabled}` mapped to `email_skip_reason` (`suppressed` covers scope `all` and `feedback`; scope `feedback` = earlier complaint on voter mail, stops only voter mail; `paused`/`disabled` -> `limited`). 04 daily limit hit: 04 reschedules (pending) per its rules; `email_status` stays pending until result.
5. Update `recipients_count`, `emailed_count`; state `sent` when all chunks done; event `feedback_post_update.sent`.

Throughput: 10,000 subscribers: 20 chunks, under 60 s to queue (budget); actual email rate bound by 04 limits. Before status change, dashboard may call `PATCH /feedback-posts/{fbp}?preview=true` (no write, returns `meta.notification_estimate {recipients, email_recipients, over_daily_limit}`), same validation path. Cloud unverified sender 200/day: estimate warns "Verify domain to email all 342 voters today".

### Feed (widget, portal)

`feedback_notifications` rows are contact feed: `GET /widget/v1/feedback/notifications`, `GET /api/feedback/notifications`. Unread count in widget session payload (01 edit). Read via bulk PATCH. Feed shows rows even when email skipped. Pruned after 180 days.

## 3. Emails

Template `resources/views/mail/feedback/update.templ` (one layout, partial per kind). Plain text + HTML. Sizes under 30 KB.

| Kind | Subject | Body |
|---|---|---|
| status_changed | `{title}` is now {status} | status badge from -> to, note (rendered Markdown, sanitized), "View request" button (post URL), vote count if shown |
| shipped | Shipped: {title} | "You asked for this. It is live.", note, changelog link + title (if linked), "View request" |
| merged | Your request was merged into "{target title}" | target link; "Your vote moved with it." |
| team_comment | {member name} replied on "{title}" | comment excerpt (1,000 chars), link |
| reply | {name} replied to your comment | excerpt, link |
| coalesced digest | Updates on "{title}" | list of updates since last email (status timeline + comments), max 10 items |

Links point to portal post URL when `portal_enabled` and post public; else no post link (text only, changelog link still shown when present). Never to dashboard.

Headers (via 04): From `"{notify.from_name}" <default sender>`, Reply-To default support address (replies become conversation; footer says "Reply to reach our team"), `Auto-Submitted: auto-generated`, `List-Unsubscribe: <https://{app_host}/feedback-unsubscriptions?c=..&s=all&sig=..>`, `List-Unsubscribe-Post: List-Unsubscribe=One-Click`, `List-Id: <feedback.{workspace slug}.trydimmi>` (self-host: `{app host}`).

Footer: "Unfollow this request" (signed link scope `post`), "Stop all product update emails" (scope `all`), workspace name and postal address field from workspace settings if set (CAN-SPAM friendly; not required for transactional updates, but shown when present).

## 4. Unsubscribe (`/feedback-unsubscriptions`, app host, `routes/web.go`)

- Signed URL via `framework/routing` (`a.URL.SignedRoute("feedback-unsubscriptions.show", params)`, route middleware `signed`; HMAC query param, no own signer): query `w` (workspace public id), `c` (contact public id), `s` (`all` | `post`), `p` (post public id when `s=post`), `signature`. No expiry. Only ever unsubscribes (never subscribes), so leaked link harm limited.
- `GET`: confirm page ("Stop emails about '{title}'?" / "Stop all product update emails from {workspace}?") with POST button. Link scanners never act.
- `POST` (one-click, body `List-Unsubscribe=One-Click` or form): `s=post`: subscription `unsubscribed_at = now` (row created if missing). `s=all`: `email_opt_out_at = now`. 200 page "Done" + "Undo" button (POST same URL with `undo=1`, clears what this link set). CSRF exempt (signature is proof), idempotent.
- Contact erased or workspace deleted: 200 generic page, no action.
- Portal `/feedback/account` and widget have same toggles (`PATCH /api/feedback/preferences {email_updates}`).

## 5. Changelog link (13 seam)

- 13 publishes entry with linked posts: emits `changelog_entry.published{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids, notify_voters}` (real 13 event, launch spec); 14 listener calls `LinkChangelogEntry{EntryID, PostIDs, URL, Title, Notify: notify_voters}`. `Notify` false: link + status move only, `shipped` update created with state `skipped` reason `notify_disabled` (feed row, no email; edge 142).
- Per post (skip posts of other workspace, trashed, merged: merged resolves to target): set `changelog_entry_id`; status not in `done` category: move to workspace's first `done` status (by position) via `ChangePostStatus` with actor = 13 publisher, `notify` false (avoid double), then create `shipped` update with `changelog_entry_id`, scheduled now. Already `done` with pending `shipped` update scheduled: attach changelog to that update and release now. Already shipped and notified before: new `shipped` update only if `notify_voters` and last shipped email older than 24 h, else link only (no second email).
- Post resource exposes `changelog: {id, url, title}` (URL from 13 resolver; 13 absent: from event snapshot stored in update `data`).
- Unlinking entry or entry deleted (13 `changelog_entry.deleted`; FK `on delete set null`): `changelog_entry_id` null; no status change, no email.
- Newsletter dedupe: 13 builds mailing snapshot after this listener commits and excludes `feedback.ShippedRecipientContactIDs(ctx, entryID)` (contacts with a `feedback_notifications` row for an update carrying this entry and `email_status in ('pending','sent','coalesced')` or `email_skip_reason = 'daily_cap'`; opted-out, suppressed, no-email contacts not returned so 13 may still mail them). One mail per person (edge 141).
- Without 13 changelog for workspace (disabled): shipping via status still notifies; email shows note instead of changelog link.

## 6. Member notifications (06)

| Trigger | Fired on | Recipients |
|---|---|---|
| feedback_post_submitted | `feedback_post.created` with author contact and state approved; `feedback_post.moderated` pending -> approved | active members with `feedback:read` (06 audience rules); workspace destinations (Slack; Discord later: discord-notifications) opt in |
| feedback_post_pending | post or comment created `pending` | members with `feedback:write`; email delayed 5 min, skipped if moderated meanwhile |
| feedback_comment_created | public comment by contact, or internal comment, on post | member subscribers minus actor |
| feedback_mentioned | internal comment with mentions | mentioned active members |

## 7. Close-the-loop invariants (tests)

- Every contact subscriber receives at most one notification row per update (unique index).
- Status flapping inside undo window sends zero or one email per contact.
- Shipped via status then changelog within 24 h: one shipped email (with changelog link if changelog first or attached before release).
- Unsubscribed (post) contact still sees feed rows, never email for that post.
- Internal comments never produce contact notifications.
- Private board posts produce contact notifications only for `shipped`, without post link; members notified as usual.

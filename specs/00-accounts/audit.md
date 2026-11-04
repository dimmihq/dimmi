# 00 Accounts: audit export and scope (S-13, S-14)

Style: caveman. Table `audit_logs`, API already in README and [api.md](api.md). Track 07. Builds on F71 fix: assume `invitation.resent` with role changes and `token.revoked` counts on deactivation exist.

## Export and date filter (S-13)

API exists in spec; code lacks CSV. `GET /audit-logs` with `Accept: text/csv` (or `?format=csv`, rest.md), same filters + `filter[created_after]`, `filter[created_before]`.

- Max range 366 days: 422 `range_too_large`. Missing range on CSV: default last 30 days, stated in filename.
- Streamed, `Content-Disposition: attachment; filename="dimmi-audit-<slug>-<from>-<to>.csv"`. Reads with cursor in batches 1,000, one short tx per batch; no long tx.
- Columns: `id,created_at,event,actor_type,actor_id,actor_label,via_kind,via_id,subject_type,subject_id,subject_label,changes,ip,user_agent,request_id`. `changes` compact JSON. Times RFC 3339 UTC. Formula guard: `app/support/csvsafe` ([settings-pages.md](settings-pages.md)).
- Export writes `audit_logs.exported` audit entry with filters and row count, after stream ends. Client disconnect: entry with `completed: false`.
- Limit 10/hour per workspace.
- Action `app/actions/audit/export_audit_logs.go`.

Retention stays `config/retention.go`: cloud 365 days, self-host unlimited by default. UI date picker bounded by retention.

UI `features/settings/pages/audit-log.tsx`: BoardUI `date-picker` range (presets 24 h, 7 d, 30 d, 90 d), IP shown in row detail, "Export CSV" `Button` downloads with current filters. Event filter gets grouped options. E2e extend `tests/e2e/audit-log.spec.ts`: "an admin filters by date and downloads what they see".

## Wider scope (S-14)

Today log has workspace, members, tokens, invitations, teams. Add below. Label for each in `resources/js/features/settings/log-labels.ts`, Unknown name shows raw (existing). Labels translate with the rest of the app ([i18n.md](i18n.md)).

### Colleague sign-in view

Sign-in events live in user-scoped `security_events` (no workspace). Admin of workspace must see them for members.

Mechanism: listener `MirrorSecurityEventToAudit` on security event write. Queued (AfterCommit, never slows login). For each active membership of user: one `audit_logs` row in that workspace. Bounded: `login.failed` already at most 1 per user+IP per minute; cap 50 memberships.

| Security event | Audit event | Subject | Actor |
|---|---|---|---|
| `login.failed` | `member.sign_in_failed` | member | `system`, label "Unknown" |
| `login.locked` | `member.sign_in_locked` | member | `system` |
| `login.succeeded` | `member.signed_in` | member | member |
| `two_factor.enabled` / `.disabled` | `member.two_factor_enabled` / `_disabled` | member | member or system (cli) |
| `auth_factor.added` / `.removed` | `member.auth_factor_added` / `_removed` | member | member |
| `auth_factor.recovery_code_used` | `member.recovery_code_used` | member | member |
| `password.changed` / `password.reset` | `member.password_changed` / `member.password_reset` | member | member |
| `email.changed` | `member.email_changed` | member | member |
| `identity.linked` / `.unlinked` | `member.identity_linked` / `_unlinked` | member | member |
| `sessions.revoked_others` | `member.sessions_revoked` | member | member |
| `sso.signed_in`, `sso.bypass_used` (21) | `member.signed_in` with `method: sso`, `member.sso_bypassed` | member | member |

`changes` carries `ip`, `device_label`, method, failure reason. Never codes, never passwords. Reader sees row IP and user agent as usual.

Privacy: admin of each workspace sees failures for its member, including from any place. Justified: same person's account reaches their workspace. `login.failed` for unknown addresses not logged, no enumeration path: reading requires `audit_logs:read`, and subject already a member. Old entries stay after member leaves. Member's own view unchanged (`/user/security-events`).

Filter `filter[subject]=mem_...` shows one person. UI: member row menu "See sign-in activity" links to audit log with filter + sign-in event group.

### Events owned here

`audit_logs.exported`, `members.exported`, `member_import.completed`, `member.two_factor_reset_requested`, `member.two_factor_reset_completed`, `member.sign_in_lock_cleared`, `member.access_changed`, `member.custom_role_changed`, `role.created`, `role.updated`, `role.deleted`, `workspace.access_policy_changed`, plus 19 list below. Customer-facing name change: plain `workspace.updated` diff. `invitation.resent` (F71).

### 19 events

`sso_connection.created/updated/deleted/activated/enforced/unenforced`, `organization_domain.added/verified/failed/removed`, `scim_token.created/revoked`, `scim.user_provisioned/updated/deprovisioned`, `scim.group_synced`, `member.sso_bypassed`.

### Events other specs must emit

Name + owner, written by `WriteAuditLog` from their own events. Delta lists them; owner adds in its track.

| Intercom class | Audit event | Owner |
|---|---|---|
| Conversation exports | `conversations.exported` | 03 |
| Bulk deletion | `conversations.bulk_deleted`, `contacts.bulk_deleted` | 03 |
| Contact erase, export | `contact.erased`, `contact.exported` | 03 |
| Inbox configuration | `inbox.created/updated/deleted`, `tag.created/deleted`, `assignment_settings.updated` | 03 |
| Macros, saved replies | `saved_reply.created/updated/deleted` (shared only) | 03 |
| Email | `email_address.created/updated/deleted`, `email_domain.created/deleted`, `email_address.raw_viewed` | 04 |
| Secrets viewed | `webhook_endpoint.secret_viewed`, `widget.identity_secret_viewed`, `email_domain.dkim_rotated` | 10, 02, 04 |
| OAuth revocations | `oauth_grant.revoked`, `oauth_client.created/deleted` | 09 |
| Webhooks | `webhook_endpoint.created/updated/deleted` | 10 |
| Widget | `widget.created/updated/deleted` | 01 |

Name mismatch with owner spec: owner's name wins; label file follows.

### Filter groups in UI

`log-labels.ts` exports groups: Sign-in, Security settings, Members and teams, Tokens and apps, Inbox, Email, Data export. Server filter stays `filter[event]` comma list.

## Tests

`tests/feature/workspaces/audit_logs_test.go` adds: "an admin downloads a csv of a date range", "a range over a year is refused", "csv cells cannot start a formula", "exporting leaves its own entry", "a member cannot export", "a token with settings read can export", "a failed sign-in shows in each workspace of that person", "an unknown address leaves no entry", "a colleague's second factor removal shows to admins", "a code or password never lands in changes", "mirror writes after login returns", "a former member's entries stay".

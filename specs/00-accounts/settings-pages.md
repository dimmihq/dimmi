# 00 Accounts: settings screens and member management

Style: caveman. Date: 2026-11-04. Backend routes named here live in [api.md](api.md); flows in [flows.md](flows.md) still hold. This file adds screens and new behavior. Track 07 builds it. Pages: `resources/js/features/settings/pages/*.tsx` on `layout.tsx` parts; rule below binds every page here.

**No dead controls.** Control whose server work is not merged in same change is removed from page, never shown static. `account.tsx` today has static rows: each one below is wired or deleted.

## Own profile (A-01)

`pages/account.tsx` `ProfileSettings`. Split: `features/settings/profile/{name-row,avatar-row,email-row,leave-workspace,delete-account}.tsx`, `lib/api/account.ts`.

| Control | Calls | Behavior |
|---|---|---|
| Full name | `PATCH /user {name}` | 1..120, NFC. Save on button. Fires `user.updated`; sender name on outgoing mail uses it |
| Picture | `PUT /user/avatar`, `DELETE /user/avatar` | BoardUI `file-upload`; 2 MB, png/jpeg/webp/gif, re-encoded. Avatar shows image, initials fallback |
| Email | `POST /email-changes`, confirm page `/email/confirm` | dialog with `recent-auth`; shows pending address with resend + cancel (`POST` again replaces). New SPA route `routes/_auth/email.confirm.tsx` calls `PATCH /email-changes/{ott}` from fragment |
| Leave workspace | `DELETE /user/memberships/{mem}` | confirm dialog; 409 `last_owner` shows hand over text; 409 `managed_by_directory` (21) |
| Delete account | `GET /user/deletion-preview`, `DELETE /user` | lists blocking workspaces; confirm typed email |

Managed by SSO org (21): name and email read-only with "Managed by {org}". Server refuses too: 403 `managed_by_organization`.

Backend: controllers `user_controller.go` Show, Update; `user_avatar_controller.go`; `email_change_controller.go`; actions per README code map, M8 and M10 tasks. Today `routes/api.go` has no user update route; track 07 adds `routes/account.go` (small file, one register line).

Events: `user.updated` realtime on each workspace channel. Audit: `member.email_changed` etc in [audit.md](audit.md).

## Password (A-02)

`SecuritySettings` Password section, `features/settings/password-dialog.tsx`.

- Has password: fields current, new, confirm. `PATCH /user/password {current_password, password}`.
- No password (signed in with link or provider): "Set a password". `recent-auth` first, then `{password}`.
- Result: current session rotated, other sessions + first-party app grants ended, `PasswordChangedMail`, security event (flows.md 9). API tokens stay; response `{tokens_remaining: n}`, dialog says "n API tokens still work" with link to tokens. Reset path revokes tokens: builds on F11 fix. Reason for difference: change proves old password; reset may follow compromise.
- Rules `password_rule.go` (length, common, breached). 422 inline.
- Managed user under enforced SSO: section hidden; server 403 `sso_required`.

## Time zone (A-03)

Preferences page. `PATCH /user {timezone}`; IANA validated by `timezone_rule.go`; `null` = follow workspace.

- Choices from `Intl.supportedValuesOf("timeZone")`, BoardUI `select` with search, first option "Workspace default ({zone})". Button-less: saves on pick.
- First visit with null and browser zone differing from workspace: quiet hint row with "Use {zone}". Never auto-writes.
- `users.timezone` already read by `Location` in `status.go`; 03 snooze presets read it (X6). `/me.user.timezone` drives display formatting in `lib/location.ts`. 06 quiet hours read it.

## Sign-in methods page (A-04)

`SecuritySettings`, section Sign-in methods: `features/settings/sign-in-methods.tsx`, `lib/api/identities.ts`. Replaces static Google/GitHub items.

- Rows from `GET /auth/config.providers` joined with `GET /user/identities`. Linked: email/username + "Unlink". Not linked: "Link" goes to browser `GET /auth/{provider}/redirect?intent=link` (S C; `recent-auth` first).
- Unlink: `DELETE /user/identities/{idn}`; 409 `last_login_method` shows "Set a password first".
- Password row links to its section; passkeys listed under two-factor (`two-factor.tsx`), since they are second factors. Provider unconfigured: row absent.
- SSO identities (21): listed read-only, "Managed by {connection}".
- Callback returns to `/d/settings/security?linked=github`; notification toast. Provider link conflict 409 `identity_in_use`.
E2e `tests/e2e/sign-in-methods.spec.ts`, provider via `Social.Fake` not reachable from e2e, so uses `framework/socialite/drivertest` local provider configured by `tests/e2e/server.ts`.

## Admin helps locked-out teammate (A-05)

Permission `members:reset_two_factor` (group `members:write`; admin, owner). Owner target also `members:manage_owners`.

Routes (U W C): `POST /members/{mem}/two-factor-resets` 201, `DELETE /members/{mem}/sign-in-lock` 204. Guest: `PATCH /two-factor-resets/{ott} {token, password?}`.

Flow:
1. Admin opens member menu "Help with two-factor", dialog says what happens, `recent-auth` confirmation (fresh within `auth.password_confirmation.ttl`).
2. Server checks: target active, target has 2FA (else 409 `two_factor_not_enabled`), target not caller (422), 3 per target per 24 h (429). Target holds active membership in workspace of another organization: 409 `two_factor_reset_blocked`, text "Ask them to use recovery codes or contact support". Reason: admin of one company must never weaken account security for another company's workspace.
3. `one_time_tokens` purpose `two_factor_reset` (new CHECK value, migration 20261201070003 add_two_factor_reset_purpose), 30 min. `TwoFactorResetMail` to target's verified email with `/two-factor-reset#id=ott_...&token=...`. Requires verified email: else 409 `target_unverified`.
4. Target redeems: page asks password when account has one. Wrong password: 422 `invalid_credentials`, 5 tries burn. Passwordless: token alone, email is their primary proof.
5. Success: `RemoveAllAuthFactors{force}`, all sessions and first-party grants ended (`two_factor_disabled`), soft lock cleared, `TwoFactorResetCompletedMail`. Target signs in, meets `two_factor_required` if workspace requires, enrolls.

Mailbox plus password still required; mailbox alone is not enough unless passwordless. Audit: `member.two_factor_reset_requested` (actor admin), `member.two_factor_reset_completed` (actor target); security event both. Cloud support and `user:disable-2fa` stay for blocked case.

Sign-in lock: `DELETE /members/{mem}/sign-in-lock` clears per-email soft lock and IP+email counter for that address (`app/auth/lockout.go` adds `Clear`). Audit `member.sign_in_lock_cleared`. Menu item shown when `sign_in_locked: true` on member resource (`members:write` only).

UI: `features/settings/members/two-factor-help-dialog.tsx`; `routes/_auth/two-factor-reset.tsx`. E2e `tests/e2e/two-factor-help.spec.ts`: "an admin starts a reset and the teammate finishes it from the email", against Mailpit.

## Members page (A-06, X1)

`pages/workspace.tsx` members area moves to `features/settings/members/{member-table,member-row,member-filters,member-access-dialog,export-button,import-dialog}.tsx`. BoardUI `table`.

Columns: person (avatar, name, email), role (with custom role name), teams, 2FA (`Chip`), last active (relative, tooltip absolute), status. 2FA and last active visible only with `members:write`.

Member resource adds (null without `members:write`):

| Field | Source |
|---|---|
| `two_factor_enabled` | `users.two_factor_confirmed_at` |
| `last_active_at` | `workspace_members.last_accessed_at` (5 min accuracy) |
| `sign_in_locked` | `Lockout` state |

`GET /members` filters add `filter[two_factor]=true|false` (`members:write`), `sort=-last_active_at`. Before enabling require 2FA, `require-two-factor.tsx` shows count and link to filtered list (`ListMembersWithoutTwoFactor` exists).

CSV: `GET /members` with `Accept: text/csv` (`members:write`), same filters, streams. Columns: `name,email,role,custom_role,status,teams,two_factor,last_active_at,joined_at`. Teams joined `;`. Formula guard below. Audit `members.exported`.

### Conversation access (X1)

Model and enforcement owned by 03 `access.md` (`conversations.VisibleScope`). Here: API docs and control.

`PATCH /members/{mem}` fields:

| Field | Values | Permission | Ability |
|---|---|---|---|
| `conversation_access` | `all`, `assigned`, `teams`, `all_except_teams` | `members:manage_access` | `members:write` |
| `conversation_access_unassigned` | bool | same | same |
| `access_team_ids` | team ids, required for `teams`, `all_except_teams` | same | same |

Member resource returns all three to callers with `members:manage_access`. Owner targets: always `all`, 422 `invalid_transition`. Fires `member.access_changed`, audit with diff. Cache bust like role change; sockets of that member resubscribed (03).

Dialog: `Select` for access level, `checkbox` "Also see unassigned", team multi-select when level needs it, explanatory sentence per level. Needs track 02 access milestone merged; until then control absent (no dead controls).

## Bulk updates by CSV (A-08)

`POST /member-imports` (U W C `members:import`, group `members:write`; admin, owner). Multipart `file` + `dry_run` bool. Synchronous; 1,000 rows, 2 MB, UTF-8, optional BOM.

Header required, order free:

```
email,role,teams,custom_role,status
grace@acme.com,admin,Support;Billing,,
bob@acme.com,member,,Read only,
gone@acme.com,,,,deactivated
```

| Column | Required | Meaning | Empty |
|---|---|---|---|
| email | yes | key | row error |
| role | no | `admin`, `member`. `owner` never: row error `role_not_allowed` | unchanged; new person: `member` |
| teams | no | `;` names, must exist, replaces membership | unchanged |
| custom_role | no | role name (19 built). Column present before custom roles ship: row error | unchanged |
| status | no | `active`, `deactivated` | unchanged |

Rules:
- Unknown email: invitation with role + teams (`CreateInvitations`, cloud daily limit counts; over limit row error `invitation_limit_reached`). `status=deactivated` on unknown: row error.
- Owner row: role/status row error `owner_not_changed`; teams apply.
- Directory-managed (21): `managed_by_directory` row error for status.
- Row failures do not stop others. Each row own tx through existing actions, so events and audit fire as usual.
- Unknown column: file error 422 `unknown_column`. Duplicate email: second row error.
- `dry_run=true`: no writes, same report.

Response 200:
```json
{ "data": { "object": "member_import", "dry_run": false, "created": 2, "updated": 5, "unchanged": 1, "failed": 1,
  "rows": [ { "row": 2, "email": "grace@acme.com", "result": "updated", "changes": ["role", "teams"] },
            { "row": 3, "email": "x", "result": "failed", "error": { "code": "invalid_email", "message": "..." } } ] } }
```
Limit 5 per hour per workspace. Audit `member_import.completed` with counts. UI dialog: template download, upload, dry run table first, then "Apply". E2e `tests/e2e/member-import.spec.ts`.

Formula guard on every CSV out (members, audit): cell starting `=`, `+`, `-`, `@`, tab, CR gets leading `'`. `encoding/csv` stdlib; helper `app/support/csvsafe`.

## Invitation extras (A-07)

`features/settings/invitations.tsx`.

- Copy link always: `accept_url` returned on create and delivery to any caller with `members:write` (was only without mail). Stored only as hash, so existing pending row copy: `POST /invitations/{inv}/deliveries {channel: "link"}` rotates token, no mail, returns `accept_url`; UI warns "the emailed link stops working". Default channel `email`. Accept still enforces invited email.
- Reminder after 3 days: job `RemindInviters` daily; pending invitations with `last_sent_at` older than 3 days and `reminded_at` null: `InvitationPendingMail` to inviter once. Migration `20261201070002_add_reminded_at_to_invitations`. Inviter gone: skipped.
- Teams on invite: team multi-select in dialog; API `team_ids` exists. Custom role select once 21 M7 ships.
- `invitation.resent` audit builds on F71 fix.
- Custom invitation message stays out (spam vector).

## Team delete with reassign (A-09)

`pages/team.tsx` delete dialog, `features/settings/team-delete-dialog.tsx`.

- `GET /teams/{team}?include=conversation_counts` returns `{open, snoozed, closed}`.
  Admin view of totals, numbers only, not through `VisibleScope`: delete acts on all of team's conversations whatever the admin may read. Exception reported to 03.
- Dialog: counts + required radio: "Move to another team" (`Select`), "Assign to a teammate" (`Select`, active members), "Leave unassigned". No default selected when counts above zero.
- API: `DELETE /teams/{team}?reassign_to_team_id=team_...` or `?reassign_to_member_id=mem_...`; both: 422; none: unassign. Deleted team, deactivated member, foreign id: 422 `errors.reassign_to`.
- Moves open, snoozed and closed; closed keeps a holder if reopened. Team target: team swapped, assignee kept. Member target: team cleared, assignee set.
- Action: `DeleteTeam` hides team, dispatches `team.deleted` payload `{reassign_to}`; 03 listener `UnassignDeletedTeam` calls 03 `ReassignTeamConversations` (chunked, 500, job) then `PurgeTeam`. No per-conversation assignment notification for this bulk move (`silent` flag to 03 action). Audit `team.deleted` with counts and target.
- Today `delete_team.go` unassigns inline: replaced by the job.
- Inbox `default_team_id` set to target team, else null.
E2e extend `teams.spec.ts`: "deleting a team moves its conversations to the chosen team".

## Team join notice (A-10)

`AddTeamMember` already fires `team.member_added`. Payload gains `added_by_member_id`. 06 registers trigger `team_joined` on it (X10): 06 owns delivery and settings row. Here: no mail code. Not fired when actor is the person, or actor is SCIM (21). Until track 11 ships: event fires, nothing delivered, no screen promises it. Test here: event carries payload; delivery tested in 06.

## Preferences (T-02)

`PreferencesSettings`, `features/settings/preferences/*`.

| Control | Works by | State |
|---|---|---|
| Language | `PATCH /user {locale}`, [i18n.md](i18n.md) | list from `/auth/config.locales`; hidden while only one locale enabled |
| Time zone | above | |
| Theme | exists (`features/theme`), per device | keep |
| Available | `PATCH /members/me {availability}`, owned 03 `assignment.md` (X3) | shown when track 02 away milestone merged; today `availability` column only |
| Send with Enter | `PATCH /user {preferences: {send_with_enter: bool}}`; `features/inbox/composer.tsx` reads `me.user.preferences` | default false, matching Cmd+Enter today. On: Enter sends, Shift+Enter newline, Cmd+Enter still sends. Composer change: track 03; control appears with it |
| Email signature | link row to `/d/settings/email#signature`, owned 04 (X4, `email_signatures`) | local toggle + field deleted now; link appears when track 05 ships |

`users.preferences jsonb not null default '{}'`: migration `20261201070001_add_preferences_to_users_table`. Keys registered in `app/auth/preferences.go` with type + default; unknown key 422 `prohibited`. Merge patch per key. `/me.user.preferences` returns all with defaults. `PATCH` from first-party app and session only (U).

Notifications page: T-03 owned by 06. Static matrix is a dead control: track 07 hides nav entry and route (`features/shell/navigation.ts`); track 11 brings it back wired.

## Customer-facing name (T-04)

`workspaces.settings.customer_facing_name` (X5). Key registered, scope workspace. API field `customer_facing_name` on `GET|PATCH /workspace`, `settings:write`, 1..80, NFC, null clears. Workspace page General section row "Name customers see", placeholder workspace name.

Fallback chains consumers read through `Workspace.CustomerFacingName()` (returns setting or `name`):
- Widget: widget `company_name` -> customer-facing name -> workspace name (01).
- Email from: address `display_name` -> customer-facing name (04).
Fires `workspace.updated`. Test: changing it changes email display name when address has none.

## Workspace language row

Workspace page Language `Choice` wired: `PATCH /workspace {locale}`, list from `/auth/config.locales`. Hidden with one locale. Workspace time zone list uses same full picker as A-03.

## Tests

Feature tests under `tests/feature/user/` and `tests/feature/workspaces/`, plain names: "a person changes their name and teammates see it", "a person changes their password and other sessions end", "a person without a password sets one after confirming", "an empty time zone follows the workspace", "an invalid time zone is refused", "unlinking the last way in is refused", "an admin starts a two factor reset and the mail goes to the teammate", "a reset needs the teammate's password", "a reset is refused when the teammate works in another company", "an admin cannot reset an owner", "a member cannot start a reset", "clearing a sign-in lock lets them try again", "the member list shows who has no second factor", "members without manage permission do not see last active", "member csv escapes formula cells", "member import dry run writes nothing", "member import invites unknown addresses", "member import never makes an owner", "copy link rotates the token and the emailed link stops working", "the inviter is reminded once after three days", "deleting a team with a target moves open, snoozed and closed conversations", "deleting a team with both targets is refused", "adding someone to a team carries who added them", "send with enter preference is stored and returned", "an unknown preference is refused", "customer facing name falls back to the workspace name", "conversation access is refused without manage access permission" (with 03 access tests). E2e in `tests/e2e`: `profile.spec.ts`, `password.spec.ts`, `preferences.spec.ts`, plus those named above; extend `workspace-members.spec.ts`, `invitations.spec.ts`.

# 00 Accounts: custom roles and fine permissions (S-05)

Style: caveman. Design keeps permission catalog: policies call `authz.Can`, never role names. Custom role is a named subset of a base role's permissions. Built by track 08 enterprise-identity (migration range track 08). Org layer untouched: roles belong to workspace.

## Rules

- `workspace_members.role` stays: base role, owner semantics, last-owner logic.
- Custom role has `base_role` (`admin` or `member`) and `permissions` subset of that base role's permissions. Can only narrow, never widen. Want more than member? Base admin, narrowed.
- Assigning custom role sets member's `role = base_role` and `custom_role_id` same tx. Effective set = role's permissions exactly.
- Setting `role` directly clears `custom_role_id`. Owners never hold custom role: 422 `invalid_transition`.
- Token effective = custom role permissions ∩ `MintedAsRole` ceiling ∩ expanded abilities. Recomputed per request (existing rule; builds on "token keeps within role minted under").
- One custom role per member. No per-member overrides: one role each keeps audit readable.
- Built-ins stay in `roles.go`; no `system` rows. Differs from extensibility.md sketch (`system`, `organization_id` columns dropped): reconciliation in delta.
- `/me.member.permissions` unchanged shape; clients already gate on it, so custom roles need no client gating change.

## Data model

Migration `20261201080009_create_roles_table`:

### roles (tenant; public `rol_`, reserved prefix now in use)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | `rol_` |
| workspace_id | uuid not null references workspaces on delete cascade | RLS |
| name | text not null | 1..50, NFC |
| description | text | 0..200 |
| base_role | text not null CHECK in ('admin','member') | |
| permissions | text[] not null | validated against catalog + base role in action; stale keys ignored at read |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| created_at, updated_at | timestamptz | |

Unique `(workspace_id, lower(name))`. Limit entitlement `roles.custom_per_workspace` (50/50), over: 429 `entitlement_limit_reached`.

Same migration: `workspace_members.custom_role_id uuid null references roles on delete restrict`, index `(workspace_id, custom_role_id)`; `invitations.custom_role_id uuid null references roles on delete set null`. Invite accept applies it; role deleted meanwhile: falls to invitation `role`.

Query `database/queries/roles.sql`. Member load joins `roles.permissions` in guard lookup, cached with member (30 s otter, busted by pubsub).

## Code

- `app/auth/authz/can.go`: `EffectivePermissions(catalog, role, customPermissions, abilities)`. Nil custom = role map. `authz.Membership` carries `CustomPermissions`.
- `app/auth/permissions.go`: `Assignable(permission) bool`, false for: owner-only permissions in `roles.go`, `system:*`. `permissions.Register` unchanged.
- `lang/en/permissions.json`: label + description per permission. `lang:check` fails on missing. Boot test: every assignable permission has label.
- `app/actions/roles/{create_role,update_role,delete_role,list_roles,assign_role}.go`. `app/policies/role_policy.go`.
- `app/http/controllers/api/v1/{role_controller,permission_controller}.go`. `routes/roles.go`.
- UI `resources/js/features/settings/pages/roles.tsx` (route `/d/settings/roles`, nav entry in `features/shell/navigation.ts`, `views.tsx`), `features/settings/roles/{role-list,role-editor,permission-group,presets}.ts(x)`, `lib/api/roles.ts`.

## API

| Method | Path | Name | Auth | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/roles` | roles.index | A W `members:read` | custom roles + built-ins listed with `kind: built_in` (`id` null) for pickers; `member_count` with `roles:manage` |
| POST | `/api/v1/roles` | roles.store | U W C `roles:manage` | create |
| GET | `/api/v1/roles/{rol}` | roles.show | A W `members:read` | |
| PATCH | `/api/v1/roles/{rol}` | roles.update | U W C `roles:manage` | name, description, permissions. `base_role` immutable: 422 `prohibited` |
| DELETE | `/api/v1/roles/{rol}` | roles.destroy | U W C `roles:manage` | 409 `role_in_use` with `member_count` |
| GET | `/api/v1/permissions` | permissions.index | A W `members:read` | catalog: `{key, group, label, description, assignable, base_roles: [..]}`, grouped by ability group |
| PATCH | `/api/v1/members/{mem}` | members.update | A W | field `custom_role_id`: `roles:manage` |
| PATCH | `/api/v1/members` | members.update-many | A W `members:write` | rest.md rule 8 `{ids, changes}`; `changes` = `role`, `custom_role_id`. Used to clear a role before delete |

Request:
```json
{ "name": "Support agent", "base_role": "member", "permissions": ["conversations:read", "conversations:write", "contacts:read"] }
```
Member resource gains `custom_role: {id, name} | null`.

Validation:
- Permission not in catalog or not `Assignable`: 422 `errors.permissions` `permission_not_assignable`.
- Permission not held by `base_role`: 422 `permission_above_base_role`.
- Escalation: caller must hold every permission in role being created, edited or assigned: 422 `permission_not_held`. Owner holds all.
- Assign to self: 422 `invalid_transition`; avoids locking self out or widening.
- `members:read` always included; UI locks it. Removing: 422. Reason: app shell breaks without it.

Delete: refuses while members hold it. Falling back to base role would silently widen access. UI offers bulk move through `PATCH /members`.

Permissions: `roles:manage`, group `members:write`; admin, owner. Member with custom role holding `roles:manage` still bound by escalation rule.

Not on MCP. CLI: none.

## Finer permissions

Catalog grows where Intercom has controls teams ask for. Each new permission sits in existing ability group, so tokens keep meaning. Default: role keeps what it does today.

| Permission | Group | Member default | Admin | Owner spec | Enforced at |
|---|---|---|---|---|---|
| `conversations:assign` | `conversations:write` | yes | yes | 03 | assign, bulk assign |
| `conversations:merge` | `conversations:write` | yes | yes | 03 | merge |
| `conversations:export` | `conversations:read` | no | yes | 03 | transcript, list export |
| `conversations:manage_notes` | `conversations:write` | no | yes | 03 | delete another's note |
| `views:manage_shared` | `conversations:write` | no | yes | 03 | shared views |
| `contacts:export` | `contacts:read` | no | yes | 03 | contact CSV |

Token with `conversations:read` may already page everything, so that group fits export. Registered by track 08 with `roles.go` rows; call sites in 03 change one policy line each, needs tracks 03, 04 merged for those routes; route not built yet simply uses it when built. Inbox agent informed via delta; if 03 already names a differing key, 03's name wins and this table follows.

## Realtime and audit

- `role.created/updated/deleted`: realtime `workspace:{ws_}` (admins refresh pickers), webhook no.
- `member.custom_role_changed`: realtime yes, webhook yes (same class as `member.role_changed`).
- Role edit: bust auth cache for all holders; new permissions next request; `member.permissions` refreshed by clients on event `role.updated`.
- Audit: `role.created`, `role.updated` (diff of permissions), `role.deleted`, `member.custom_role_changed`.

## UI

`/d/settings/roles`, gated on `roles:manage`. Built-ins shown read-only with expand of what they hold. Editor: name, base role `segmented-control` on create only, permissions grouped by catalog group with `switch` per permission, label + description. Presets button creates draft from template in `presets.ts`: "Support agent" (member minus delete-class), "Read only" (reads only), "Billing admin" (admin base: billing, members read). Presets are starting points, not rows. Member row role picker in `features/settings/members/member-row.tsx` lists built-ins + custom.

E2e `tests/e2e/custom-roles.spec.ts`: "an admin makes a read only role and a member with it cannot reply", light and dark.

## Edge cases

Numbered in [edge-cases.md](edge-cases.md) "Custom roles".

## Tests

`tests/feature/roles/roles_test.go`: "a custom role can only hold what its base role holds", "a caller cannot grant a permission they do not hold", "assigning a role narrows what the member can do", "a token of that member narrows too", "a demoted token ceiling still applies", "editing a role applies on the next request", "a role in use cannot be deleted", "owners cannot hold a custom role", "setting a built-in role clears the custom one", "a role of another workspace is not found", "the catalog lists labels for every assignable permission", "changes land in the audit log". `tests/feature/authz/roles_test.go` snapshot extended.

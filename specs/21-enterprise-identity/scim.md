# 19 Enterprise identity: SCIM 2.0

Style: caveman. RFC 7643, 7644. Base `/scim/v2`. IdP pushes people and groups; dimmi applies through same actions as dashboard.

## Reuse check

Verified 2026-11-04.

| Candidate | License | Status | Verdict |
|---|---|---|---|
| `github.com/elimity-com/scim` (the assignment's "elimity-inc" is this repo) | MIT | pushed 2026-07; tags only v0.1 and v0.1-alpha; README: early stage, minor versions may break, no sort or bulk | rejected as server framework |
| `github.com/scim2/filter-parser/v2` | MIT | pushed 2026-07; same authors; small | **used** for `filter` and PATCH `path` |
| Hand-written protocol | n/a | two resources, about 800 lines | **chosen**, in `framework/scim` |

Why by hand: library says itself not mature, so "mature library wins" does not bite. Must tolerate Entra non-compliant PATCH (below) before schema check; library owns request parsing and tolerance is unverified. Scope small: Users, Groups, PATCH, filter `eq`, list paging, discovery. depguard: add `github.com/scim2/filter-parser/v2` to `framework` allowlist (approved here).

## Wrapper `framework/scim`

Generic package, no product words.

```go
package scim

type Store[T any] interface {
    Create(ctx context.Context, r T) (T, error)
    Get(ctx context.Context, id string) (T, error)
    List(ctx context.Context, q ListQuery) (Page[T], error)
    Replace(ctx context.Context, id string, r T) (T, error)
    Patch(ctx context.Context, id string, ops []Op) (T, error)
    Delete(ctx context.Context, id string) error
}
type Op struct { Kind string; Path *Path; Value any } // Kind lowercase, normalized
type Error struct { Status int; ScimType, Detail string }
func Mount(r Router, opts Options) // Users, Groups, ServiceProviderConfig, Schemas, ResourceTypes
```

Normalizes input, writes `application/scim+json`, error body per RFC 7644 section 3.12.

## Auth

`Authorization: Bearer dmi_scim_...`. `auth_find_scim_token` (SECURITY DEFINER), tenant set from token's `workspace_id`, then normal tx. Checksum rejects typos without DB hit. Revoked or expired: 401 SCIM error. Not `auth`/`W` middleware: not a member, no abilities, no `access.policy`, not in `/api/v1`.

Rate limit: 600/min per token via `framework/http/middleware` `Throttle`, fail open; 429 SCIM error + `Retry-After`. Directories retry.

Actor for audit and events: `contracts.Actor{Type: integration, Via: {Kind: "scim", ID: token id}}`. New via kind `scim` (delta file).

## Endpoints

| Method | Path | Behavior |
|---|---|---|
| GET | `/scim/v2/ServiceProviderConfig` | `patch: true`, `bulk: false`, `filter: {supported: true, maxResults: 200}`, `changePassword: false`, `sort: false`, `etag: false`, auth scheme bearer |
| GET | `/scim/v2/Schemas`, `/ResourceTypes` | static |
| GET | `/scim/v2/Users` | `filter`, `startIndex`, `count` (max 200). Filter: `userName eq`, `externalId eq`, `emails.value eq`, `id eq`, `active eq`. Others: 400 `invalidFilter` |
| POST | `/scim/v2/Users` | provision |
| GET, PUT, PATCH, DELETE | `/scim/v2/Users/{id}` | |
| GET | `/scim/v2/Groups` | `filter` `displayName eq`, `externalId eq`, `id eq`; `excludedAttributes=members` honored |
| POST | `/scim/v2/Groups` | |
| GET, PUT, PATCH, DELETE | `/scim/v2/Groups/{id}` | |

`/Me`, `/Bulk`, `/.search`: 501 SCIM error.

## User mapping

SCIM `id` = member public id `mem_...`.

| SCIM | dimmi | Rule |
|---|---|---|
| `userName` | `users.email` | required, valid email. Case-insensitive unique in workspace |
| `externalId` | `workspace_members.directory_external_id` | |
| `name.givenName` + `familyName`, or `displayName` | `users.name` | written only when `managed_by_organization_id` = this org |
| `emails[primary].value` | `users.email` | when differs from `userName`, `userName` wins |
| `active` | `workspace_members.status` | false: `DeactivateMember`; true: `ReactivateMember` |
| `meta.created`, `lastModified` | from rows | |
| `roles`, `groups` read only | from team links | writes ignored |

Enterprise extension accepted and ignored.

### POST /Users

1. Member with that email or `externalId` in workspace: 409 `uniqueness` SCIM error. Okta and Entra filter first and adopt existing person with `PATCH`: first directory write sets `directory_external_id`, `directory_managed_at`.
2. User with email exists anywhere: reuse user. Set `managed_by_organization_id` only when email under org's verified domain.
3. None: create user. Domain verified: verified email, `created_via=scim`, managed, no password; signs in by SSO. Domain not verified: 400 `invalidValue` detail "verify this domain first". Reason: SCIM token must not create account for arbitrary company's address without proof (README D4).
4. Add member `joined_via=scim`, `directory_managed_at=now`, role `default_role` of connection routed from email domain, else `member`. `member.joined`. No welcome mail when enforced SSO: IdP launches. Otherwise `DirectoryWelcomeMail` once with login link.

### Deprovision

`active:false`, `DELETE`: `DeactivateMember` (owner target: 409 SCIM `mutability` detail "owners are not managed by directory"). Tokens stop under 2 s, sockets closed under 5 s, conversations unassigned (03 listener), invitations withdrawn (F91 fix). `DELETE` then `GET`: 404 per RFC, member keeps row; `directory_external_id` cleared so later re-create reactivates by email. Last owner rule never reached since owners refused.

Owners and members without `directory_managed_at`: read fine, writes on owners refused. Directory-managed members in dashboard: name, email, status read-only (409 `managed_by_directory`); teams linked by group read-only; role read-only only when a link sets role.

## Group mapping

`Group.id` = `scg_...`. `displayName` to `scim_group_links.display_name`. On `POST`: team with same name reused, else created. `team_id` points there. Admin may repoint in settings. Team deleted in dimmi: `team_id` null, link kept, membership pushes no-op until repointed.

Members: `members[].value` = `mem_`. Applied by diff through `AddTeamMember`, `RemoveTeamMember`, batch 100. Unknown member id: skipped with WARN log, 200. Reason: Entra may push group before user, and a 400 would halt its whole job; next sync adds them.

Role: `scim_group_links.role` / `custom_role_id` set by admin only, never by IdP. Member's role = highest among linked groups; removed from all role groups: back to connection `default_role`. Owner untouched. Group sync does not fire 06 team-join trigger (A-10): IdP changes arrive in large batches, person did not get a new duty from a teammate.

`DELETE /Groups/{id}`: link removed, team kept with members, role effect dropped. Reason: team holds conversations; admin deletes via A-09 flow.

## PATCH tolerance

Accepted forms, normalized before `Store.Patch`:

| Quirk | Source | Handling |
|---|---|---|
| `op` any case (`Replace`, `Add`, `Remove`) | Entra without `aadOptscim062020` (Microsoft compatibility doc, updated 2026-02, marks compliant PATCH as flag-only) | lowercased |
| `active` as string `"False"`, `"True"` | Entra same | parsed to bool, only for declared boolean attrs |
| `remove` with `path: members`, `value: [{value}]` | Entra same | treated as `members[value eq "..."]` |
| no `path`, `value` object with dotted keys `name.givenName` | Entra flag form | expanded |
| `emails[type eq "work"].value` | Entra, Okta | parsed by filter parser |
| `replace` `members` full list | Okta | diff |

Unsupported path: 400 `invalidPath`. Unknown attribute in `value`: ignored (RFC mutability).

## Setup in UI

Single sign-on page, Directory sync section: base URL `{app.url}/scim/v2`, token shown once (`use-copy`), last sync time, list of groups with team and role selects. Docs: `docs/` guides for Okta, Entra, Google (no SCIM from Google to custom apps without gallery; says so). Guide text ships with feature; human verifies against live tenant (tasks human step).

## Tests

`framework/scim`: filter parse, error shape, PATCH normalizer table above (unit). `tests/feature/identity/scim_test.go`: "an idp creates a member", "a duplicate user answers conflict", "an unverified domain cannot create an account", "an existing person elsewhere joins without losing their account", "deactivating stops their token and closes sockets", "delete keeps the conversation history and hides the user", "reactivating brings back the same member", "an owner cannot be deactivated by the directory", "a group push creates a team and fills it", "removing from a role group returns the default role", "a capitalized replace from entra is understood", "active as the word false is understood", "a revoked token is refused", "another organization's member is not found", "a dashboard change to a managed member is refused", "actions appear in the audit log with the token", "limit answers with a scim error". Contract fixtures: real request shapes from Okta and Entra docs under `tests/feature/identity/testdata/scim`.

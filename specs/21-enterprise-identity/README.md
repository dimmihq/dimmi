# 19 Enterprise identity: sign-in rules, session length, SAML, OIDC, SCIM, IP allowlists

Style: caveman. Date: 2026-11-04. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`, [../00-accounts/README.md](../00-accounts/README.md). Every feature here is in both editions, default build and `-tags cloud`, every plan. Pricing promise: SSO free when shipped; no entitlement `feature` key gates any of it.

## Files

| File | What |
|---|---|
| README.md | this: goal, decisions, data model, code map, API, permissions, events, UI, gap map |
| [access-policy.md](access-policy.md) | workspace rules: sign-in methods, Google domains, session length, dashboard + API IP allowlists |
| [saml.md](saml.md) | SAML protocol: library, wrapper, flows, validation list |
| [scim.md](scim.md) | SCIM 2.0 protocol: endpoints, mapping, IdP quirks |
| [edge-cases.md](edge-cases.md) | numbered decided behaviors |
| [tasks.md](tasks.md) | build checklist, track 08 |

Custom roles (S-05) are specified in [../00-accounts/roles.md](../00-accounts/roles.md); track 08 builds them.

## Goal

Company with Okta, Entra or Google Workspace connects dimmi in one sitting: verify domain, connect IdP, test, enforce, turn on provisioning. Security review answers yes to: SSO, SCIM, session limits, sign-in method rules, IP allowlists. Admin cannot lock whole workspace out by accident.

## Scope

In:
- Workspace access rules: allowed sign-in methods, Google hosted domains, session length, dashboard IP allowlist, API IP allowlist.
- Verified organization domains by DNS TXT.
- SSO connections per organization: SAML 2.0, enterprise OIDC.
- JIT member creation. Enforcement. Owner break-glass.
- SCIM 2.0 Users + Groups. Groups map to teams, optionally role.

Not in (with reason, also in reply to coordinator):
- SAML Single Logout. Not asked by any gap. SCIM deactivation and `session_max_age` end access; SLO interop is poor across IdPs.
- SCIM `/Bulk`, `/Me`, sort. Okta, Entra and OneLogin do not need them; `ServiceProviderConfig` says unsupported.
- Organization API, org switcher, second workspace in org. Stay out (00-accounts). Org stays 1:1 with workspace.

## Decisions

| # | Decision | Why |
|---|---|---|
| D1 | Enterprise OIDC: **in**. | `framework/socialite/drivers/oidc` already does discovery, PKCE, ID token verify. `sso_connections.protocol` has `oidc`. Small cost; Google Workspace and Entra admins often prefer it, and some small IdPs have no SAML. Shares every rule below SAML uses. |
| D2 | SSO connections, domains, SCIM tokens attach to `organizations`. Access rules stay in `workspaces.settings`. | Identity is company-level (extensibility.md section 3). Rules are evaluated per workspace request with no query, same place as `security.require_two_factor`. Keys marked `Enforceable`, org lock additive. |
| D3 | No `organization_id` in API. Routes run through workspace in play, action resolves `workspace.organization_id`. | Org API stays out; org is 1:1. Resources carry no org field. |
| D4 | Connection signs in only addresses under domains its org verified and routed to it. Exception: person who already linked identity from inside a signed-in session (`intent=link`). | Without this, any customer's IdP could assert `ceo@othercompany.com` and take over a cross-tenant account. |
| D5 | SSO session satisfies workspace 2FA rule only in workspaces of connection's org. Local 2FA challenge skipped after SSO. | IdP owns MFA there. Elsewhere, user's own factors still count. |
| D6 | Owner break-glass: password + own factors (totp or passkey), flag `sso_bypass`, mail to all owners, 1 h session. | IdP outage must not lock company out. Same answer as bad password for non-owners, no enumeration. |
| D7 | IdP-initiated SAML never starts a session. ACS answers with SP-initiated restart. | Unsolicited response has no `InResponseTo` and no browser binding: login CSRF. Tile in Okta still works, one extra redirect. |
| D8 | SSO needs HTTPS `app.url`. Activation on plain http: 422 `https_required`. | SAML POST returns cross-site; state cookie must be `SameSite=None; Secure`. |
| D9 | Self-host may verify domain from CLI: `dimmi sso:verify-domain <domain>`. | Internal domains have no public DNS. Operator already holds full trust. |
| D10 | Library SAML: gosaml2 + goxmldsig. SCIM: hand-written `framework/scim`, filter parser from `scim2/filter-parser/v2`. | Reuse checks in [saml.md](saml.md), [scim.md](scim.md). |
| D11 | Tokens exempt from sign-in method, Google domain, session length, SSO rules. Tokens obey API IP allowlist, 2FA rule, SCIM deactivation. | Token is not a sign-in; no method to judge. Deactivation stops it within 2 s. |
| D12 | SCIM removal deactivates member, keeps user + history. `DELETE /Users/{id}` = `active: false`. | Conversations and audit entries keep author. Reactivation via IdP keeps same member. User row may belong to other workspaces. |

## Data model

Migrations range track 08. `-- lint:org-scoped` tables key on `organization_id`; reached by joins in actions + SECURITY DEFINER functions (security.md).

| Number + name | Content |
|---|---|
| `20261201080001_create_sso_connections_table` | below |
| `20261201080002_create_organization_domains_table` | below |
| `20261201080003_add_sso_connection_to_oauth_identities` | `sso_connection_id uuid null references sso_connections on delete cascade`; uniques become `(provider, sso_connection_id, provider_user_id) NULLS NOT DISTINCT`, `(user_id, provider, sso_connection_id) NULLS NOT DISTINCT` |
| `20261201080004_add_sign_in_context_to_sessions` | `sso_connection_id uuid null references sso_connections on delete set null`, `auth_domain text` (Google `hd` or SSO email domain) |
| `20261201080005_add_managed_by_to_users` | `managed_by_organization_id uuid null references organizations on delete set null`, index where not null |
| `20261201080006_create_scim_tokens_table` | below |
| `20261201080007_create_scim_group_links_table` | below |
| `20261201080008_add_directory_columns_to_workspace_members` | `directory_external_id text`, `directory_managed_at timestamptz`; partial unique `(workspace_id, directory_external_id) WHERE directory_external_id IS NOT NULL` |
| `20261201080009_create_roles_table` | [roles.md](../00-accounts/roles.md) |
| `20261201080010_add_sign_in_context_to_oauth_grants` | ALTER framework `oauth_grants`: `sso_connection_id`, `auth_domain`; ALTER `oauth_authorization_requests` same. Copied from session at approval (09 flow, one line) |
| `20261201080011_create_identity_lookup_functions` | `auth_sso_policy_for_domain(domain citext)` returns connection id, enforced; `auth_find_scim_token(hash bytea)` returns token id, org id, workspace id, revoked_at; `auth_sso_connection(id uuid)` returns connection row for guest routes |

Registry enum values added (no migration): `sessions.auth_method` + `users.created_via` + `oauth_identities.provider`: `sso`. `users.created_via`: `scim`. `workspace_members.joined_via`: `sso`, `scim`. `security_events.type`: `sso.signed_in`, `sso.bypass_used`, `sso.link_added`.

### sso_connections (`-- lint:org-scoped`; public `ssoc_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | `ssoc_` |
| organization_id | uuid not null references organizations on delete cascade | |
| protocol | text not null CHECK in ('saml','oidc') | strict: state machine of code paths, not driver registry |
| name | text not null | 1..80, shown on login button and in audit |
| status | text not null default 'draft' CHECK in ('draft','tested','active','disabled') | [state below](#connection-states) |
| config | jsonb not null default '{}' | SAML: `idp_entity_id`, `idp_sso_url`, `idp_certificates` (PEM list), `idp_metadata_url`, `name_id_format`, `sp_certificate` (PEM). OIDC: `issuer`, `client_id`, `scopes`. Both: `attribute_map` `{email, first_name, last_name, display_name, groups}` |
| secret_encrypted | bytea (`encryption.Sealed`) | SAML: SP private key. OIDC: client secret. Scope `encryption.Scope{Kind: "organization", ID: org}`, AAD connection id. Never returned |
| jit_enabled | boolean not null default false | |
| default_role | text not null default 'member' CHECK in ('admin','member') | never owner |
| default_custom_role_id | uuid null references roles on delete set null | |
| default_team_ids | uuid[] not null default '{}' | missing teams skipped |
| group_role_map | jsonb not null default '[]' | `[{group, role, custom_role_id}]`, max 50; read from `groups` attribute at each sign-in |
| enforced | boolean not null default false | true only when `active` |
| tested_at | timestamptz | last successful test sign-in |
| last_login_at | timestamptz | written at most once per 5 min |
| metadata_refreshed_at | timestamptz | |
| metadata_error | text | last refresh failure, shown in UI |
| last_error | jsonb | `{code, at}`, last sign-in failure reason code, shown to owners; never assertion content |
| created_by_user_id | uuid null references users on delete set null | |
| created_at, updated_at | timestamptz | |

Indexes `(organization_id, status)`. Max per org: entitlement `sso.connections_per_organization`, over limit 429 `entitlement_limit_reached`.

#### Connection states

| From | To | Via | Rule |
|---|---|---|---|
| draft | tested | test sign-in succeeds | any owner runs `POST /sso-connections/{ssoc}/tests` then signs in through IdP in new tab |
| tested | active | `PATCH {status: active}` | at least one verified domain routed to it, `app.url` https; else 422 `no_verified_domain` / `https_required` |
| active | disabled | `PATCH {status: disabled}` | sets `enforced=false`; session rows keep working until policy says otherwise |
| disabled | active | `PATCH {status: active}` | must re-test if config changed |
| any except disabled | draft | config change on protocol fields | `enforced=false`, `tested_at=null`. Prevents silent lockout by typo |

`enforced: true` needs `status=active` and caller's own session signed in through this connection, or `tested_at` within 24 h with `confirm_owners_can_sign_in: true`. Writes `security.sso_enforced_connection_ids` mirror on workspace settings same tx, read by access policy without a query.

### organization_domains (`-- lint:org-scoped`; public `odom_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `odom_` |
| organization_id | uuid not null references organizations on delete cascade | |
| domain | citext not null | lowercase, IDNA A-label |
| verification_token | text not null | `dimmi-domain-verification=<32 random base32>` |
| status | text not null default 'pending' CHECK in ('pending','verified','failed') | |
| verified_at, last_checked_at | timestamptz | |
| verified_via | text | `dns` or `cli` |
| sso_connection_id | uuid null references sso_connections on delete set null | routing |
| failed_since | timestamptz | re-check failing since |
| created_at, updated_at | timestamptz | |

Partial unique `(domain) WHERE status = 'verified'`: one org per domain, first to verify. Unique `(organization_id, domain)`. DNS via `framework/dns` TXT at `_dimmi-verify.<domain>`. Public suffixes (`golang.org/x/net/publicsuffix`) and freemail domains (embedded list `resources/data/freemail_domains.txt`, reused from disposable list build) refused 422 `domain_not_claimable`. Max 20 per org.

Job `RecheckOrganizationDomains` daily. Failing 7 days: status `failed`, connection unenforced, `DomainVerificationLostMail` to owners. `verified_via = cli` never re-checked.

### scim_tokens (`-- lint:org-scoped`; public `sct_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `sct_` (new prefix, delta file) |
| organization_id | uuid not null references organizations on delete cascade | |
| workspace_id | uuid not null references workspaces on delete cascade | target workspace; org is 1:1 today, column makes request tenant explicit |
| name | text not null | |
| hint | text not null | `dmi_scim_Ab12` |
| token_hash | bytea unique not null | SHA-256. Format `dmi_scim_` + 32 base62 + CRC32 (reuse `framework/auth/token`) |
| expires_at | timestamptz | null = never |
| last_used_at, last_used_ip | | coalesced, 60 s |
| revoked_at | timestamptz | |
| created_by_user_id | uuid null references users on delete set null | |
| created_at, updated_at | timestamptz | |

Max 5 live per org.

### scim_group_links (tenant; public `scg_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `scg_`; SCIM Group `id` |
| workspace_id | uuid not null references workspaces on delete cascade | RLS |
| external_id | text | IdP group id |
| display_name | text not null | |
| team_id | uuid null references teams on delete set null | team created on first push named after group; admin may repoint |
| role | text null CHECK in ('admin','member') | optional: directory decides role of members in this group |
| custom_role_id | uuid null references roles on delete set null | |
| created_at, updated_at | timestamptz | |

Unique `(workspace_id, lower(display_name))`, partial unique `(workspace_id, external_id)`.

### Core table changes

Listed in delta: `users.managed_by_organization_id`, `sessions.sso_connection_id` + `auth_domain`, `oauth_identities.sso_connection_id`, `workspace_members.directory_external_id` + `directory_managed_at` + `custom_role_id`. `users.managed_by_organization_id` set when JIT or SCIM creates the account, or when the existing account's email sits under the org's verified domain. Managed effects: email change 403 `managed_by_organization`; account delete only with no active membership in that org; password change allowed unless enforced.

## Code map

### Framework (no product words)
- `framework/saml/`: wrapper, [saml.md](saml.md). `framework/saml/samltest`: local signing IdP for tests.
- `framework/scim/`: protocol, [scim.md](scim.md).
- `framework/socialite/drivers/oidc`: reused. `oidc.New(name, client, clk, Options)` per connection, cached 10 min by connection id + `updated_at`.

### App
- `app/auth/login_policy.go`: add `OrganizationLoginPolicy` driver. `ForEmail`: domain lookup via `auth_sso_policy_for_domain`, otter cache 60 s, busted on connection/domain change by `a.PubSub`. Enforced: `LoginPolicy{RedirectURL: "/auth/sso/<ssoc>/redirect"}`. Else empty. `AllowAll` stays for tests. Builds on hook; no sign-in flow edit except `sso_bypass` (below).
- `app/auth/access_policy.go`: rules in [access-policy.md](access-policy.md).
- `app/auth/providers/sso.go`: `ProviderResolver` extension. Resolves `ssoc_` to `IdentityProvider`; `saml_provider.go`, `oidc_provider.go` return `ExternalIdentity{Provider: "sso", ConnectionID, Subject, Email, EmailVerified, Name, Groups, HostedDomain}`.
- `app/actions/sso/`: `create_connection.go`, `update_connection.go`, `delete_connection.go`, `start_connection_test.go`, `complete_sso_login.go`, `resolve_sso_user.go`, `apply_group_roles.go`, `refresh_connection_metadata.go`, `lookup_sso_for_email.go`, `record_sso_bypass.go`.
- `app/actions/domains/`: `add_domain.go`, `verify_domain.go`, `route_domain.go`, `remove_domain.go`.
- `app/actions/directory/`: `create_scim_token.go`, `revoke_scim_token.go`, `provision_user.go`, `update_directory_user.go`, `deprovision_user.go`, `upsert_directory_group.go`, `sync_group_members.go`, `delete_directory_group.go`. Calls `members.DeactivateMember`, `teams.AddTeamMember`, never own SQL for those.
- `app/actions/accesspolicy/`: `update_access_policy.go`, `show_access_policy.go`.
- `app/http/controllers/api/v1/`: `sso_connection_controller.go`, `sso_connection_test_controller.go`, `organization_domain_controller.go`, `organization_domain_check_controller.go`, `scim_token_controller.go`, `scim_group_link_controller.go`, `sso_lookup_controller.go`, `access_policy_controller.go`.
- `app/http/controllers/web/`: `sso_redirect_controller.go`, `sso_callback_controller.go` (OIDC), `saml_acs_controller.go`, `saml_metadata_controller.go`.
- `app/http/controllers/scim/`: `user_controller.go`, `group_controller.go`, `discovery_controller.go`.
- `routes/identity.go` (`/api/v1`), `routes/sso.go` (web), `routes/scim.go`. Small files, no edit to `routes/api.go` beyond one register call.
- `database/queries/`: `sso_connections.sql`, `organization_domains.sql`, `scim_tokens.sql`, `scim_group_links.sql`.
- `app/console/commands/`: `sso_verify_domain_command.go`, `sso_disable_command.go`, `workspace_clear_access_policy_command.go`.
- `app/jobs/`: `recheck_organization_domains.go`, `refresh_sso_metadata.go`, `warn_expiring_sso_certificates.go`.
- `app/mail/`: `SSOBypassUsedMail`, `DomainVerificationLostMail`, `SSOCertificateExpiringMail`, `SSOEnforcedMail`, `AccessPolicyChangedMail`, `DirectoryWelcomeMail`.
- Factories: `sso_connection_factory.go`, `organization_domain_factory.go`, `scim_token_factory.go`.

### Sign-in changes
- `POST /sessions` gains optional `sso_bypass: true` (grants `password`, `two_factor`; passkey via `two_factor`). Resolver sees flag: address in enforced domain and user is active owner of the org's workspace: allowed. Anyone else: identical `invalid_credentials` path with dummy hash.
- `CompletePrimaryAuth` skips local 2FA when `method = sso`. `StartSession` writes `sso_connection_id`, `auth_domain`. Bypass: `auth_methods` includes `sso_bypass`, `expires_at = now + 1 h`.
- `/auth/config`: capabilities `sso`, `scim`; `sso_available: bool` (any active connection on install). Login page shows "Continue with SSO" only when true.

## HTTP API

Letters per 00-accounts `api.md`. `C` = recent confirmation. All U W C writes refuse tokens.

| Method | Path | Name | Auth | Purpose |
|---|---|---|---|---|
| POST | `/api/v1/sso-lookups` | sso-lookups.store | G | `{email}`; 200 `{object: "sso_lookup", redirect_url: string\|null, name, enforced}`. Domain-level only, no user lookup. 30/min per IP |
| GET | `/api/v1/sso-connections` | sso-connections.index | U W `sso:manage` | list, secrets never |
| POST | `/api/v1/sso-connections` | sso-connections.store | U W C `sso:manage` | `{protocol, name, config, client_secret?}`; generates SP key for SAML. Returns `sp` block: `entity_id`, `acs_url`, `metadata_url` or `redirect_uri` |
| GET | `/api/v1/sso-connections/{ssoc}` | sso-connections.show | U W `sso:manage` | |
| PATCH | `/api/v1/sso-connections/{ssoc}` | sso-connections.update | U W C `sso:manage` | config, `status`, `enforced`, `jit_enabled`, `default_role`, `default_custom_role_id`, `default_team_ids`, `group_role_map`, `client_secret` (write only), `confirm_owners_can_sign_in` |
| DELETE | `/api/v1/sso-connections/{ssoc}` | sso-connections.destroy | U W C `sso:manage` | 409 `connection_enforced` while enforced. Identities cascade; users fall back to other methods |
| POST | `/api/v1/sso-connections/{ssoc}/tests` | sso-connections.tests.store | U W `sso:manage` | 201 `{object: "sso_connection_test", url}`; opens IdP. Result on connection `tested_at`, no session started |
| GET | `/api/v1/organization-domains` | organization-domains.index | U W `sso:manage` | |
| POST | `/api/v1/organization-domains` | organization-domains.store | U W C `sso:manage` | `{domain}`; returns TXT name + value |
| PATCH | `/api/v1/organization-domains/{odom}` | organization-domains.update | U W C `sso:manage` | `{sso_connection_id}` |
| DELETE | `/api/v1/organization-domains/{odom}` | organization-domains.destroy | U W C `sso:manage` | 409 `domain_in_use` when routes an enforced connection |
| POST | `/api/v1/organization-domains/{odom}/checks` | organization-domains.checks.store | U W `sso:manage` | run DNS now; 201 `{status, found_records}` |
| GET | `/api/v1/scim-tokens` | scim-tokens.index | U W `scim:manage` | |
| POST | `/api/v1/scim-tokens` | scim-tokens.store | U W C `scim:manage` | `{name, expires_at?}`; token + `base_url` once, `no-store` |
| DELETE | `/api/v1/scim-tokens/{sct}` | scim-tokens.destroy | U W C `scim:manage` | revoke |
| GET | `/api/v1/scim-group-links` | scim-group-links.index | U W `scim:manage` | groups pushed by IdP |
| PATCH | `/api/v1/scim-group-links/{scg}` | scim-group-links.update | U W C `scim:manage` | `{team_id, role, custom_role_id}` |
| GET | `/api/v1/workspace/access-policy` | workspace.access-policy.show | A W `settings:read` | singleton. [access-policy.md](access-policy.md) |
| PATCH | `/api/v1/workspace/access-policy` | workspace.access-policy.update | U W C `security_policy:manage` | |

Web and protocol (delta: rest.md exceptions):

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/auth/sso/{ssoc}/redirect` | G; `intent=link\|test` needs session | start SP flow |
| GET | `/auth/sso/{ssoc}/callback` | state cookie | OIDC callback |
| POST | `/auth/saml/{ssoc}/acs` | SAML response; CSRF middleware off | SAML POST binding |
| GET | `/auth/saml/{ssoc}/metadata` | public | SP metadata XML |
| * | `/scim/v2/*` | `dmi_scim_` bearer | [scim.md](scim.md) |

Stable errors (added to 00-accounts `api.md`): 409 `sso_required` (with `redirect_url`), 403 `sso_no_access`, 403 `sso_email_domain_not_verified`, 403 `managed_by_organization`, 409 `managed_by_directory`, 422 `no_verified_domain`, `https_required`, `domain_not_claimable`, `saml_redirect_binding_required`, `invalid_saml_metadata`, 409 `connection_enforced`, `domain_in_use`. Access policy failures: [access-policy.md](access-policy.md).

## Permissions

Registered in catalog; rows in `roles.go`.

| Permission | Group | Member | Admin | Owner | Why |
|---|---|---|---|---|---|
| `security_policy:manage` | `settings:write` | no | yes | yes | same trust as `require_two_factor` |
| `sso:manage` | `settings:write` | no | no | yes | wrong config locks out everyone; owner-only |
| `scim:manage` | `settings:write` | no | no | yes | token creates and removes members, reaches every non-owner |
| `roles:manage` | `members:write` | no | yes | yes | [roles.md](../00-accounts/roles.md) |

All write routes U C. Never MCP tools (prompt injection must not change access rules).

## Events

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| `workspace.access_policy_changed` | yes `workspace:{ws_}` (no values) | no | listener disconnects dashboard sockets of workspace's members; reconnect re-checks |
| `sso_connection.created/updated/deleted` | no | no | audit |
| `sso_connection.activated`, `.enforced`, `.unenforced` | no | no | audit |
| `organization_domain.added/verified/failed/removed` | no | no | audit |
| `scim_token.created/revoked` | no | no | audit |
| `scim.user_provisioned/updated/deprovisioned`, `scim.group_synced` | no | no | audit; member events fire too (`member.joined`, `member.deactivated`) |

Audit names and labels: [../00-accounts/audit.md](../00-accounts/audit.md).

## UI

BoardUI blocks, `Section`/`Row`/`Item` from `resources/js/features/settings/layout.tsx`. Every control gated on `member.permissions`.

| Screen | Route | Files |
|---|---|---|
| Workspace security | `/d/settings/workspace-security` (exists) | `pages/admin.tsx` keeps `WorkspaceSecuritySettings`; new `features/settings/workspace-security/{sign-in-rules,session-length,ip-allowlists}.tsx`; `lib/api/access-policy.ts` |
| Single sign-on | `/d/settings/single-sign-on` (new, `features/shell/navigation.ts`, `features/settings/views.tsx`) | `pages/single-sign-on.tsx`, `features/settings/sso/{connections,connection-dialog,connection-steps,domains,attribute-map,group-roles,enforce-dialog}.tsx`; `lib/api/sso.ts` |
| Directory sync | section on single sign-on page | `features/settings/sso/{directory-tokens,directory-groups}.tsx` |
| Login SSO | `/login` | `features/auth` add "Continue with SSO" (BoardUI `social-button`), email step calls `POST /sso-lookups`; bypass link "Owner sign-in without SSO" appears after 409 `sso_required` |
| Access required | shown on any `requirement` 403 | `features/auth/access-required.tsx`: message per `requirement`, button to `action_url`. Replaces ad hoc 2FA redirect; unknown requirement shows server `message` |
| Members | `/d/settings/workspace` | badge "Managed by directory", disabled status control when `directory_managed` |

Connection dialog steps: (1) protocol; (2) copy SP values (`Input` read-only + `use-copy`), paste metadata URL or XML, or OIDC issuer + client id + secret; (3) test in new tab, waits on poll of `tested_at`; (4) activate. Enforce dialog: lists count of members without linked identity, count of users on domain outside workspace, requires typing domain.

IP allowlist editor: `Item` per entry `{cidr, label}`, add row validates with server 422, shows "Your address: 203.0.113.4" from policy resource.

E2e: `tests/e2e/workspace-security.spec.ts`, `tests/e2e/single-sign-on.spec.ts` (local `samltest` IdP started by `tests/e2e/server.ts`), `tests/e2e/custom-roles.spec.ts`. Light + dark, per CLAUDE.md.

## Self-host and cloud differences

None in behavior. Self-host extras: `sso:verify-domain`, `sso:disable`, `workspace:clear-access-policy` (also cloud ops, run by staff). Plain-http installs get every rule except SSO (D8).

## Observability

`dimmi_sso_logins_total{protocol,result}`, `dimmi_access_policy_refusals_total{requirement}`, `dimmi_scim_requests_total{resource,method,status}`. Logs: connection id, never assertions, never tokens. SAML response body never logged, not even on failure; failure reason code only, shown to owner in test result.

## Dependencies

| Spec | Needs |
|---|---|
| 00-accounts | `login_policy.go`, `access_policy.go`, `DeactivateMember`, factor rules, audit writer, `roles.md` |
| 03 access.md (track 02) | deactivation listeners; `access_team_ids` unaffected by group sync |
| 09 | copies `auth_domain`, `sso_connection_id` to grant; `prompt=login` reauth on `reauthentication_required` |
| 11 | `dimmi doctor` flags http + SSO |
| Foundation | delta file: prefixes, exceptions, entitlement keys |

Eraser hook: none (X9). Workspace purge: `PurgeWorkspace` deletes `sso_connections`, `organization_domains`, `scim_tokens` of org when org soft-deleted, releasing domain.

## Gap map

| Gap | Where specified | Tasks milestone |
|---|---|---|
| S-06 | [access-policy.md](access-policy.md) "Session length" | [tasks.md](tasks.md) M1 |
| S-07 | [access-policy.md](access-policy.md) "Sign-in methods", "Google domains"; README "Sign-in changes" | M1 |
| S-08 | [saml.md](saml.md); README "sso_connections", D1, D4-D8 | M3, M4, M5 |
| S-09 | [scim.md](scim.md) | M6 |
| S-10 | [access-policy.md](access-policy.md) "Dashboard allowlist" | M2 |
| P-02 | [access-policy.md](access-policy.md) "API allowlist" | M2 |
| S-05 | [../00-accounts/roles.md](../00-accounts/roles.md) (built here) | M7 |

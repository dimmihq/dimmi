# 21 Enterprise identity: tasks

Style: caveman. Order matters. Every task merged with tests. App logic: integration tests against real Postgres + Valkey (testing.md). Fakes only at IdP boundary: `framework/saml/samltest`, `framework/socialite/drivertest`. Migrations range `20261201080001+`. Other tracks: needs track 07 M16 (audit labels) only for label text; otherwise independent. `oauth_grants` columns: built whether or not mobile ships.

## M1. Sign-in rules and session length (track 08 enterprise-identity)

- [x] `app/models/workspace_security.go`: typed `SecuritySettings`, decode + with-methods, keys per access-policy.md. Test: bad JSON reads as no rules.
- [x] Migration 080004 (sessions context). `StartSession` writes `auth_domain` from Google `hd`. `Principal` adds `IP`, `LastActivityAt`, `SSOConnectionID`, `AuthDomain`. `AccessRule.Check` new signature; `AccessRules` holds clock.
- [x] Rules `SignInMethodRule`, `GoogleDomainRule`, `SessionLengthRule` in `app/auth/access_policy.go` (split files if over limit). Accept: refusal body per table; no query added (test counts queries).
- [x] `GET|PATCH /workspace/access-policy`, permission `security_policy:manage`, guards `sign_in_method_excludes_caller`, `no_owner_can_sign_in`. Audit + `AccessPolicyChangedMail`.
- [x] `routes/realtime` authorizers + refresh call policy. Listener closes sockets on change. Builds on F12 fix.
- [x] UI: `features/settings/workspace-security/{sign-in-rules,session-length}.tsx`, `features/auth/access-required.tsx`, 403 handler in `lib/api/client.ts` branches on `requirement`. E2e `workspace-security.spec.ts`: "an admin sets a session limit and an old session is asked to sign in again", "an admin allows only google and a password session is asked to sign in with it", light and dark.
- [x] Tests from access-policy.md "Tests".

## M2. IP allowlists (track 08 enterprise-identity)

- [x] Rules `DashboardAllowlistRule`, `APIAllowlistRule`. `netip.Prefix` parse once per decode. Accept: IPv4, IPv6, mapped forms.
- [x] `PATCH` guard `ip_allowlist_excludes_caller`; `tokens_outside` from `last_used_ip`; `your_ip`.
- [ ] MCP HTTP and CLI path checked. Test: "mcp over http obeys the api list". JSON-RPC body per `09-agent-native/mcp.md` wrapping 403.
- [ ] `dimmi workspace:clear-access-policy <slug> [--only ip_allowlist]`, audit actor system. `dimmi doctor` proxy warning (11 check registry).
- [x] UI `ip-allowlists.tsx`. E2e: "an admin adds their own address and a range", "saving a list without your address explains why it failed".

## M3. Domains and connections (track 08 enterprise-identity)

- [ ] Migrations 080001, 080002, 080003, 080005, 080011. Prefixes `ssoc_`, `odom_` registered (`app/models/public_ids.go`). Lint annotations.
- [ ] Domain actions + API. DNS via `framework/dns` with test resolver fake (boundary). Freemail list embed. `RecheckOrganizationDomains`, `sso:verify-domain`.
- [ ] Connection CRUD, state machine, secret sealing (`Scope{Kind: "organization"}`), settings mirrors in same tx. Audit events.
- [ ] Permissions `sso:manage`, `scim:manage` in catalog + `roles.go`; label keys.
- [ ] UI `pages/single-sign-on.tsx`, `connections`, `connection-steps`, `domains`. Nav entry gated on `sso:manage`. E2e: "an owner verifies a domain with the operator command and sees it verified" using test resolver env flag only in testing env.

## M4. SAML (track 08 enterprise-identity)

- [ ] First: add gosaml2 + goxmldsig v1.6.1 floor, one `framework/saml` call; `tests/arch` and binary size check pass. Fails: stop, report.
- [ ] `framework/saml` wrapper + `samltest`. Tests list in saml.md. 90% coverage. `doc.go`, no product words.
- [ ] Web routes redirect, ACS (CSRF off for this route only), metadata. Attempt store on Valkey, `__Host-dimmi_sso` cookie, replay guard, IdP-initiated restart.
- [ ] `CompleteSSOLogin`, `ResolveSSOUser`, `ApplyGroupRoles`, JIT. `OrganizationLoginPolicy` replaces `AllowAll` in provider; `CheckLoginPolicy` path for password, magic link, email code, reset, register, OAuth.
- [ ] `POST /sso-lookups`. Login page "Continue with SSO". `POST /sessions {sso_bypass}` + `SSOBypassUsedMail`.
- [ ] Metadata refresh + certificate warning jobs. `connection-dialog`, test flow polling `tested_at`, `attribute-map`, `group-roles`, `enforce-dialog`.
- [ ] Tests `tests/feature/identity/saml_test.go`; e2e `single-sign-on.spec.ts`: "an owner connects a saml idp, tests it and signs in through it", "enforcing sends a password sign-in to the idp", "the owner bypass signs in and warns the other owners". Local IdP started from `tests/e2e/server.ts`.

## M5. Enterprise OIDC (track 08 enterprise-identity)

- [ ] `oidc_provider.go` per-connection `oidc.New`, cache by id + `updated_at`. Callback route. Client secret write-only.
- [ ] Shared core tests re-run for OIDC via table. Dialog protocol step offers OIDC. E2e: "an owner connects an openid provider".

## M6. SCIM (track 08 enterprise-identity)

- [ ] Migrations 080006, 080007, 080008. Prefixes `sct_`, `scg_`. Via kind `scim`.
- [ ] `framework/scim` + `scim2/filter-parser/v2`: mount, discovery, normalizer. Unit tests table.
- [ ] `routes/scim.go`, token guard, `Throttle` 600/min, directory actions on `members`, `teams` actions.
- [ ] Managed member refusals in `PATCH /members`, `DELETE /user/memberships`, email change, delete account.
- [ ] Token + group API, `directory-tokens.tsx`, `directory-groups.tsx`, members badge. E2e: "an owner makes a directory token and sees it once", "a member pushed by the directory shows as managed".
- [ ] Tests list in scim.md. Fixtures Okta and Entra shapes.

## M7. Custom roles (track 08 enterprise-identity)

Tasks in [../00-accounts/tasks.md](../00-accounts/tasks.md) M19 are the build list; listed there because roles live in 00-accounts. Track 08 builds them after M1. Group-to-custom-role wiring (M4, M6) uses `roles` table; if M7 ships after M4, map field accepts `custom_role_id` only once migration 080009 is in; order M7 before M4 when one builder does both.

## M8. Hardening (track 08 enterprise-identity)

- [ ] Log redaction: `SAMLResponse`, `dmi_scim_`, client secret. Grep test on captured logs.
- [ ] Delta applied by coordinator verified: `rest.md` exceptions, prefixes, entitlement catalog keys `sso.connections_per_organization`, `scim.tokens_per_organization`, `roles.custom_per_workspace`.
- [ ] `docs/testing/manual-e2e.md`: flow table rows for every new endpoint; release binary run per CLAUDE.md.
- [ ] Admin guides under `docs/`: Okta SAML, Entra SAML, Google Workspace SAML, Okta OIDC, Okta SCIM, Entra SCIM.

## Human steps

- Live tenant check against Okta developer account, Entra free tenant, Google Workspace test domain: owner or delegate runs guides once before announcing. Not automatable here; automated tests use fixtures.

## Done when

`make lint test` and `make e2e` pass, `make cover` 90% on `framework/saml`, `framework/scim`, `make cover-app` 95% on new packages. Demo: owner verifies domain, connects local IdP, tests, enforces; password sign-in redirected; SCIM deactivation ends token under 2 s and socket under 5 s; session limit asks for sign-in; address outside list refused with its IP.

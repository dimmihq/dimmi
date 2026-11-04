# 21 Enterprise identity: edge cases

Style: caveman. Each: case. Decided behavior.

## Domains

1. Two orgs try same domain. First to verify wins. Second sees `failed` on check, message "already verified by another organization"; does not name it.
2. Domain has freemail or public suffix. 422 `domain_not_claimable`.
3. Subdomains. `eng.acme.com` needs own row. Parent verification never implies child: narrow by default.
4. TXT removed after verify. Daily check, 7 days grace, then `failed`, enforcement lifted, owners mailed. Sign-in continues under normal methods. Reason: security check must not become outage.
5. Domain removed while routed to enforced connection. 409 `domain_in_use`.
6. DNS lookup timeout on check. Status unchanged, `found_records: null`, message retry.
7. Self-host internal domain. `sso:verify-domain`; `verified_via=cli`, never re-checked.

## Connection setup

8. Metadata URL points to internal address. `framework/httpclient` SSRF guard refuses on cloud; self-host allowlist config applies. 422 `invalid_saml_metadata`.
9. Config field edited on active connection. Back to `draft`, `enforced=false`, owners mailed. Sign-in through it refused until re-tested. Name, JIT, role map edits do not reset.
10. Test sign-in by owner whose IdP email differs. Test records success on valid assertion, shows mapped email + "not under a verified domain" warning when so.
11. Plain-http install. Activation 422 `https_required`.
12. Certificate expired, metadata pasted. Sign-in fails `sso_failed`, `last_error.code = invalid_signature`; warning mails at 30, 7, 1 days. Enforcement stays; owner bypass available.
13. Connection deleted while people hold sessions. `sessions.sso_connection_id` nulled; session keeps running under other rules unless policy method rule refuses `sso` without connection, then reauth.
14. Limit hit. 429 `entitlement_limit_reached` key `sso.connections_per_organization`.

## Sign-in

15. Assertion for address under another org's verified domain. 403 `sso_email_domain_not_verified`. No account lookup on that address.
16. Existing user with password, domain now enforced. 409 `sso_required` at password; identity linked on first SSO. Password kept, unused.
17. Existing user is member of other unrelated workspaces, domain claimed by Acme. All sign-in for that address goes through Acme IdP. Enforce dialog shows count of such users before confirm. After signing in, they still reach other workspaces, subject to those workspaces' rules. Workspace with `allowed_sign_in_methods` lacking `sso` refuses them with `sign_in_method_required`; owner of that workspace decides.
18. Enforced, person's IdP account disabled, has live session. Session lives until `session_max_age` or SCIM deactivation. UI recommends both.
19. Person with enforced domain tries `POST /magic-links`, `POST /password-resets`. 409 `sso_required`, domain-level. Register same. Invitation accept for address under enforced domain: sign-in via SSO then accept.
20. Bypass by non-owner. Same `invalid_credentials`, same timing (dummy hash). Bypass by owner without password: same. Operator: `dimmi sso:disable <slug>`.
21. Bypass session after 1 h. Ended by `expires_at`. Writes `sso.bypass_used` security event, audit `member.sso_bypassed`, mail to every owner.
22. Replay of assertion. Refused by Valkey NX. Valkey down: 503, no session.
23. Clock skew. 3 min both sides. Beyond: `sso_failed`, `last_error.code = expired`.
24. SP-initiated attempt expired before IdP returned. Treated as unsolicited: restart. Loop protection: second restart within 1 min for same browser cookie lands on `/login?error=sso_failed`.
25. Assertion without email, NameID persistent. `missing_email`; owner sees it in `last_error`.
26. Email attribute changes at IdP. Identity found by subject. New address under verified domain: updated, security event, `EmailChangedMail` to old. Else refused `sso_email_domain_not_verified`.
27. Second connection, same subject string. Different identity row (unique includes connection).
28. Account deleted, signs in again via JIT. New user row; old anonymized.
29. Instance admin on self-host under enforced domain. Same rules; `sso:disable` is operator path.
30. App (mobile) login under SSO. Browser sheet runs same flow; grant copies `sso_connection_id`, `auth_domain`, `auth_methods`.
31. Personal API token of SSO user, enforced. Keeps working (README D11) until revoked, expired, deactivated. Admin revokes via existing `tokens:manage_any`.
32. `require_two_factor` on, SSO session of this org, user has no factor. Passes. Same user later logs in by bypass with no factor: bypass refused because no second factor exists (bypass demands factor when workspace requires one).
33. JIT with deactivated membership. 403 `membership_deactivated`; never reactivates.
34. JIT seat limits. None; unlimited teammates promise.
35. Group attribute missing from assertion. Role untouched.
36. Role map names deleted custom role. Row ignored, WARN in `last_error`; custom role delete nulls via action scanning map.
37. Group map would demote last admin. Allowed: owner exists always.

## Access policy

38. Caller excluded by own change. 422 per key (access-policy.md).
39. Owner's IP changes after allowlist saved. Locked out; `workspace:clear-access-policy`. Cloud: support verifies owner by mail + recent activity, runs same command. Documented on page.
40. IPv6 client, list only IPv4. Refused. Unmapped form compared. UI hint when caller is IPv6.
41. Trusted proxy misconfigured, all requests from proxy IP. Allowlist then sees proxy IP: `dimmi doctor` warns; save guard shows `your_ip` so operator notices.
42. Rule tightened while socket open. Listener disconnects; subscribe refuses.
43. Reauth by different user on `/login?reauth=1`. 422 `reauth_user_mismatch` (exists).
44. Session length rule, person mid-reply. Next request 403; screen keeps route; 03 draft persists; after reauth back on same conversation.
45. Workspace switch into stricter workspace. 403 there only; other tab in lax workspace continues.
46. `/me` under refusal. Not `W`; returns; includes workspace list so UI can offer others.
47. Token with API list + SCIM from same IP. SCIM not checked.
48. Webhook delivery, widget. Never checked.
49. Self-host no GeoIP. No impact; country not used.
50. `PATCH` by member with `settings:read`. 403 `forbidden`. Token: 403 `first_party_only`.

## SCIM

51. Token leaked. Revoke; audit shows last IP. Blast: non-owners of that workspace, new users only under verified domain.
52. Directory deactivates only owner. 409 mutability; owners out of directory reach by design.
53. Two groups give different roles. Highest (admin over member).
54. Custom role on group. Highest base role wins; tie: most recently updated link. Deterministic.
55. Name with control chars. Stripped, NFC, 1..120.
56. `userName` change to address under unverified domain. 400 `invalidValue`.
57. `userName` change to address taken by other account. 409 `uniqueness`.
58. Re-create after delete. Finds by email; reactivates same member (conversation history keeps person).
59. `count=0`. RFC: returns `totalResults` only.
60. Filter with `or`, `co`. 400 `invalidFilter`.
61. Workspace trashed. 403 SCIM error; suspended same. Restored: tokens work again.
62. Member joined by invitation, later provisioned. Adopted; `joined_via` unchanged.
63. Directory-managed member leaves by themselves (`DELETE /user/memberships`). 409 `managed_by_directory`; leave through IdP. Reason: next sync would rejoin.
64. Account delete of managed user with live membership. 403 `managed_by_organization`.
65. Org purged. `users.managed_by_organization_id` nulled; connections, domains, tokens deleted.

## Audit and privacy

66. SAML assertion in logs. Never; redaction list adds `SAMLResponse`, `dmi_scim_`.
67. CIDR labels. Plain text 1..60, escaped in UI; audit shows values.
68. Client secret on `GET`. Field absent; `client_secret_set: true`.

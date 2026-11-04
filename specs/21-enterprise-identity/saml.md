# 19 Enterprise identity: SAML 2.0 and enterprise OIDC

Style: caveman. Service provider only. Web browser SSO profile: HTTP-Redirect for request, HTTP-POST for response.

## Reuse check

Verified 2026-11-04 (GitHub API, repo files, advisories by search).

| Candidate | License | Status | Verdict |
|---|---|---|---|
| `github.com/russellhaering/gosaml2` | Apache-2.0 | latest tag v0.12.0; pushed 2026-09; not archived; SP only; tested with Okta, Auth0, Shibboleth, OneLogin, Entra (README) | **chosen** |
| `github.com/russellhaering/goxmldsig` | Apache-2.0 | v1.6.1. Pulled in by gosaml2. CVE-2026-33487 (signature bypass) fixed in 1.6.0 | required, floor v1.6.1 |
| `github.com/beevik/etree`, `github.com/mattermost/xml-roundtrip-validator` | BSD-2, Apache-2.0 | direct deps of gosaml2 | transitive; round-trip validator guards XML parser differentials |
| `github.com/crewjam/saml` | BSD-2-Clause | latest tag v0.5.1; pushed 2026-01; carries IdP + SP + `samlsp` | rejected: larger surface (IdP code unused), older push, past bypasses CVE-2020-27846, CVE-2022-41912 |

Why gosaml2: smaller (binary size rule), SP only, newer activity, knobs needed for hardening (`MaximumDecompressedBodySize`, `MaximumXMLTokens`, `Clock`, `ClockSkew`). Its go.mod needs Go 1.25.

Sharp edges read from source, wrapper must cover:
- `VerifyAssertionConditions` reports bad time and wrong audience as warnings (`WarningInfo.InvalidTime`, `NotInAudience`), no error. Wrapper turns each into hard error.
- `Destination` and `InResponseTo` not checked. Wrapper checks.
- `build_request.go` imports `html/template`, executed only in POST-binding builders (`BuildAuthBodyPost*`). Wrapper never calls them: linker rule in CLAUDE.md. `tests/arch` must stay green with gosaml2 linked; first task proves it. Fails: stop, report, do not add exemption.

depguard: add `github.com/russellhaering/gosaml2`, `github.com/russellhaering/goxmldsig` to `framework` allowlist in `.golangci.yml` as new-dependency entries (this spec approves them; no lint limit changes). `golang.org/x/net/publicsuffix` if not listed.

Security watch: `govulncheck` in CI already applies; pin floor versions in go.mod.

## Wrapper `framework/saml`

Generic, no product words. Hides library types.

```go
package saml

type IdentityProvider struct {
    EntityID     string
    SSOURL       string
    Certificates []*x509.Certificate
}

type ServiceProvider struct {
    EntityID    string
    ACSURL      string
    Certificate *x509.Certificate
    PrivateKey  crypto.Signer
}

type Config struct {
    SP           ServiceProvider
    IdP          IdentityProvider
    NameIDFormat string
    Clock        clock.Clock
    ClockSkew    time.Duration // default 3 min
}

type Assertion struct {
    AssertionID  string
    ResponseID   string
    NameID       string
    NameIDFormat string
    Attributes   map[string][]string
    AuthnInstant time.Time
    NotOnOrAfter time.Time
}

func ParseMetadata(xml []byte) (IdentityProvider, error)        // ErrInvalidMetadata, ErrNoRedirectBinding
func GenerateKeyPair(commonName string, c clock.Clock) (*x509.Certificate, crypto.Signer, error) // RSA 2048, 10 y
func New(cfg Config) (*Provider, error)
func (p *Provider) AuthURL(relayState string) (url, requestID string, err error)
func (p *Provider) Metadata() ([]byte, error)
func (p *Provider) Consume(encodedResponse, expectedRequestID string) (Assertion, error)
```

Errors: `ErrInvalidSignature`, `ErrExpired`, `ErrWrongAudience`, `ErrWrongDestination`, `ErrWrongIssuer`, `ErrInResponseTo`, `ErrUnsolicited` (no `InResponseTo`, `expectedRequestID` empty), `ErrMalformed`, `ErrStatusNotSuccess`.

`Consume` checks, in order: size limit (256 KB base64 before parse; `MaximumXMLTokens` 100,000), round-trip validator (library), signature (`SkipSignatureValidation` always false; response or assertion signed), issuer, status success, destination equals ACS, subject confirmation (library), `InResponseTo` equals expected, `WarningInfo.InvalidTime` and `NotInAudience` to errors, exactly one assertion, non-empty NameID. Encrypted assertions decrypted with SP key (library); metadata publishes signing + encryption cert. AuthnRequest signed with SHA-256. Inbound algorithm follows goxmldsig defaults; no extra allowlist.

`samltest` subpackage: test IdP with own key, builds signed responses via goxmldsig, options to tamper. Testing only, refuses outside testing env like every `Fake()`.

## Flows

### SP-initiated

1. `GET /auth/sso/{ssoc}/redirect?intent=login|link|test`. Connection must be `active` (`test`, `link`: `tested`/`active`, session + `sso:manage` for `test`). Unknown or inactive: 404.
2. Create attempt in Valkey key `sso:attempt:<random>` TTL 10 min: `{connection_id, request_id, intent, user_id?, return_to}`. Set `__Host-dimmi_sso` = random value, `HttpOnly`, `Secure`, `SameSite=None`, 10 min. `RelayState` = random second value, compared to attempt. Valkey down: 503, never proceed (fail closed).
3. 302 to IdP redirect URL. `return_to` same-origin path only.

### ACS

`POST /auth/saml/{ssoc}/acs`, form `SAMLResponse`, `RelayState`. CSRF middleware off on this one route; state cookie + RelayState + `InResponseTo` replace it.

1. No matching attempt (cookie missing, or `RelayState` empty): treat as IdP-initiated: `Consume` with empty expected. Only `ErrUnsolicited` allowed through after all other checks pass, then 303 to `/auth/sso/{ssoc}/redirect?intent=login`. No session. Any other error: 303 `/login?error=sso_failed`.
2. With attempt: consume attempt atomically (`GETDEL`), `Consume(resp, request_id)`.
3. Replay: `SET sso:assertion:<conn>:<assertion id> NX EX <until NotOnOrAfter + skew>`. Exists: refused, `sso_failed`. Valkey down: fail closed.
4. Map attributes by `attribute_map`. Email from mapped attribute; else NameID when format `emailAddress`. No email: `sso_failed` reason `missing_email`.
5. `test` intent: set `tested_at`, status `tested`, 303 to settings page with result; no session.
6. Else `CompleteSSOLogin`.

Failure pages never show library error. `sso_connections.last_error` (reason code + time) visible to owners; test intent shows it directly.

### CompleteSSOLogin (shared with OIDC)

Order in one tx:
1. Identity row `(provider=sso, connection, subject)` found: user = its user.
2. Not found, `intent=link`: attach to session's user, whatever email domain (person proved both sides); security event `sso.link_added`; done, return to settings. Member of connection's org only: else 403 `sso_no_access`.
3. Not found: email domain must be verified org domain routed to this connection (README D4). Else 403 `sso_email_domain_not_verified`.
4. User with that email exists: link. Unverified local account: pre-hijack defense from 00-accounts (password cleared, sessions killed). Set `managed_by_organization_id`.
5. No user: `jit_enabled` required, else 403 `sso_no_access`. `CreateUser` verified, `created_via=sso`, managed.
6. Membership: active: continue. Deactivated: 403 `membership_deactivated`, JIT never reactivates. None: `jit_enabled` adds member `joined_via=sso`, `default_role`, `default_custom_role_id`, `default_team_ids`; else 403 `sso_no_access`. Fires `member.joined`.
7. Name sync when attribute present and account managed.
8. `group_role_map` applied (`apply_group_roles`): highest match wins, owner untouched, member's directory-set role wins over this when SCIM links set role. No group attribute: no change. Fires `member.role_changed`.
9. `LoginPolicy` already passed for `sso`. `StartSession(method=sso, connection, auth_domain, skip 2FA)`. Writes audit `member.signed_in` via S-14 listener.

Subject = NameID for SAML, `sub` for OIDC. Persistent NameID recommended; email-format NameID accepted; IdP email change then finds user by identity row, updates email only when new address sits under verified domain, else refused.

## Metadata

Paste XML or URL. URL fetched through `framework/httpclient` (SSRF guard), 1 MB cap. `RefreshSSOMetadata` daily for URL connections: new certs replace list; failure writes `metadata_error`, keeps old. Rollover: list holds several certs. `WarnExpiringSSOCertificates` daily: IdP cert under 30, 7, 1 days: `SSOCertificateExpiringMail` to owners. IdP lacking Redirect binding: 422 `saml_redirect_binding_required`.

SP metadata public at `/auth/saml/{ssoc}/metadata`. Entity id = same URL. ACS = `{app.url}/auth/saml/{ssoc}/acs`.

## Enterprise OIDC

- Config: `issuer`, `client_id`, `client_secret`, `scopes` default `openid email profile`, `attribute_map.groups` optional.
- Redirect URI `{app.url}/auth/sso/{ssoc}/callback`. Same attempt store; PKCE S256, `state`, `nonce` via `framework/socialite` `oidc` driver.
- `email_verified` false in ID token: treated as absent unless domain verified (D4 still checks domain).
- Issuer fetched only over https unless testing; SSRF guard by `framework/httpclient`.
- Secret rotate: `PATCH {client_secret}`; not protocol field, keeps status.

## Tests

`framework/saml` unit + `samltest`: "a valid signed response is accepted", "an unsigned response is refused", "a response with an extra unsigned assertion is refused", "a wrapped signature is refused", "a comment inside the name id does not truncate it", "an expired assertion is refused", "a wrong audience is refused even though the library only warns", "a wrong destination is refused", "a response to another request is refused", "an unsolicited response is reported as unsolicited", "a response signed by a rotated certificate is accepted while listed", "oversized input is refused before parsing", "metadata without a redirect binding is refused". Coverage 90%. Fixtures from Okta and Entra sample responses under `framework/saml/testdata`, re-signed by test key.

`tests/feature/identity/saml_test.go`: "sign in through saml starts a session", "a replayed assertion is refused", "an address outside the verified domain is refused", "an idp cannot sign in a person from another company", "jit creates the member with the default role", "without jit an unknown person gets no access", "a deactivated member stays out", "group mapping sets the role and never touches an owner", "idp initiated sign in restarts from dimmi", "test sign in records the result and starts no session", "enforcement sends password sign-in to the idp", "magic link and reset are refused under enforcement", "an unknown address under the domain gets the same answer", "an owner can bypass with password and second factor", "a non owner bypass looks like a wrong password", "bypass mails every owner and lasts one hour", "losing domain verification lifts enforcement", "a linked contractor from another domain signs in", "a session from another connection does not satisfy the rule". `tests/feature/identity/oidc_test.go` uses `framework/socialite/drivertest` local provider: same core list. E2e: `tests/e2e/single-sign-on.spec.ts`.

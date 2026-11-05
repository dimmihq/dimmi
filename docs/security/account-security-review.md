# Account security review

A walkthrough of the authentication (V2) and session management (V3) chapters of the OWASP Application
Security Verification Standard 4.0.3 at level 2, against how dimmi signs people in and keeps them signed in.
Each row says how the requirement is met, where to look, and whether it is met in full.

Status: **Met**, **Partial** (met for the common path, a gap is named), **Not built** (the feature the
requirement covers does not exist yet), **N/A** (dimmi does not do what the requirement guards).

Re-run this review whenever sign-in, sessions, tokens or second factors change.

## V2 Authentication

| ASVS | Requirement | How dimmi meets it | Status |
|---|---|---|---|
| 2.1.1 | Passwords at least 12 characters | Minimum is configurable, default 10 (`auth.passwords.min_length`, never below 8). Raise the default to 12 to meet the letter of the rule | Partial |
| 2.1.2 | Passwords of 64 characters and more allowed | Up to 256 characters accepted (`StoreUserRequest`) | Met |
| 2.1.3 | No truncation | argon2id hashes the whole password | Met |
| 2.1.4 | Any printable Unicode allowed | No character rules; length counts characters, not bytes | Met |
| 2.1.5 | Users can change their password | `PATCH /user/password` with the current password | Met |
| 2.1.6 | Change needs the current and the new password | Same endpoint requires `current_password` | Met |
| 2.1.7 | Breached and common passwords refused | Embedded list of common passwords refused (`rules.UncommonPassword`). No breached-password lookup service yet | Partial |
| 2.1.8 | Strength meter | None | Not built |
| 2.1.9 | No composition rules | Length is the only rule | Met |
| 2.1.10 | No forced rotation | None | Met |
| 2.1.11 | Paste and password managers allowed | Plain password inputs, `autocomplete` set | Met |
| 2.1.12 | Password can be shown | No show-password control yet | Not built |
| 2.2.1 | Anti-automation | Per address and per email limits, escalating lockout for unknown devices, address block after guessing across accounts, signup limits per address and per network, optional Turnstile captcha on sign-up and sign-in links | Met |
| 2.2.2 | Weak authenticators limited | Email links and codes are single use, short lived, limited per email and address | Met |
| 2.2.3 | Notification after sensitive changes | Mails for password change and reset, second factor on and off, email change, new sign-in, sign-ins locked | Met |
| 2.4.1 | Passwords stored with an approved one-way function | argon2id (`framework/hashing`) | Met |
| 2.4.2 | Salt of at least 32 bits | argon2id with a random salt per hash | Met |
| 2.4.5 | Additional secret (pepper) | Not used | Not built |
| 2.5.1 | Recovery secrets not sent in clear | Reset links carry the id and secret after `#`, kept out of server logs; only the digest is stored | Met |
| 2.5.2 | No security questions | None | Met |
| 2.5.3 | Recovery does not reveal the password | Reset sets a new password | Met |
| 2.5.4 | No shared or default accounts | The first owner is created by the operator or setup, with their own password | Met |
| 2.5.5 | Notification when a factor changes | Mails on second factor added, removed, disabled and on reset | Met |
| 2.5.6 | Recovery is as strong as sign-in | Reset ends every session, app grant and token; second factor stays required | Met |
| 2.5.7 | Lost second factor recovered by identity proofing | Recovery codes; an admin-started reset needs the mailbox and the password; operator command for the rest | Met |
| 2.7.x | Out-of-band verifiers | Email codes and links: random, single use, expire within minutes, rate limited | Met |
| 2.8.1 | Time-based codes have a lifetime | TOTP 30 second step, one step of drift | Met |
| 2.8.3 | Approved algorithms for one-time codes | TOTP (RFC 6238) | Met |
| 2.8.4 | One-time codes used once | A used step is refused | Met |
| 2.8.5 | Reused code logged and notified | Logged as a failed attempt; no separate notification | Partial |
| 2.9.x | Cryptographic verifiers | Passkeys (WebAuthn) as a second factor | Met |
| 2.10.1 | No hard-coded service credentials | Secrets come from configuration; personal access tokens per person, hashed at rest | Met |
| 2.10.2 | Service credentials not default | None shipped | Met |

## V3 Session management

| ASVS | Requirement | How dimmi meets it | Status |
|---|---|---|---|
| 3.1.1 | Session token never in the URL | Cookie only; links carry secrets after `#`, never sent to the server in the address | Met |
| 3.2.1 | New token on sign-in | Every sign-in creates a new session; the old cookie is not reused | Met |
| 3.2.2 | At least 64 bits of entropy | 256-bit random token | Met |
| 3.2.3 | Stored safely in the browser | `HttpOnly`, `Secure`, `__Host-` prefix, `SameSite=Lax` | Met |
| 3.2.4 | Approved generator | `crypto/rand` | Met |
| 3.3.1 | Sign-out ends the session on the server | `DELETE /sessions/current` deletes the stored session | Met |
| 3.3.2 | Re-authentication after idle and absolute limits | Idle 14 days and absolute 30 days by default; sensitive actions need a recent confirmation (`password.confirm`) | Met |
| 3.3.3 | Sign out other sessions after a password change | Password change ends other sessions and app grants; the sessions list ends any one | Met |
| 3.3.4 | Users see and end their sessions | `GET /sessions`, `DELETE /sessions/{id}`, `DELETE /sessions` | Met |
| 3.4.1 | Cookie `Secure` | Set unless the operator runs plain HTTP on a local network | Met |
| 3.4.2 | Cookie `HttpOnly` | Set | Met |
| 3.4.3 | Cookie `SameSite` | `Lax`, plus cross-origin protection on unsafe methods | Met |
| 3.4.4 | Cookie `__Host-` prefix | Set unless plain HTTP | Met |
| 3.4.5 | Cookie path | `/` with the `__Host-` prefix | Met |
| 3.5.1 | Users can revoke tokens of connected apps | Personal access tokens revoked from settings; first-party app grants not listed with sessions yet | Partial |
| 3.5.2 | Session tokens rather than static keys for people | Browsers use sessions; static keys only for scripts, scoped by ability and expiring | Met |
| 3.5.3 | Stateless tokens protected | No stateless session tokens; every token is checked against the database | N/A |
| 3.7.1 | Re-authentication before sensitive actions | Password, second factor, email, ownership and member import changes need a recent confirmation | Met |

## Logs

Tests prove that signing in, minting and using a token, and signing out at debug level write no token and no
session cookie to the logs (`TestLogRedaction`). Mail links are logged only on a self-hosted server that sends
no mail, as the operator's way to hand them on.

## Gaps to close

- Raise the default minimum password length to 12.
- Add a breached-password lookup for the cloud.
- Show the first-party app grants beside browser sessions, so people can end them in one place.
